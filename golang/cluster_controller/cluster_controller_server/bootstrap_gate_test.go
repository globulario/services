package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ── IS3: Control-plane-critical deploy gate ───────────────────────────────────
//
// ControlPlaneCritical services (cluster-controller, cluster-doctor, workflow)
// may deploy once infra mesh is ready (envoy_ready+). Ordinary workloads must
// wait until workload_ready.
//
// bootstrapInfraReady  = BootstrapNone | EnvoyReady | StorageJoining | WorkloadReady
// bootstrapPhaseReady  = BootstrapNone | StorageJoining | WorkloadReady
//                        (note: EnvoyReady intentionally excluded)
//
// The delta (EnvoyReady) is the window where control-plane-critical services
// can deploy but ordinary workloads cannot. This prevents a deadlock where
// a stuck MinIO topology contract (or similar) blocks workload_ready forever,
// starving the controller from being able to issue the fix.

// ── bootstrapInfraReady phase matrix ─────────────────────────────────────────

func TestBootstrapInfraReady_PhaseMatrix(t *testing.T) {
	readyPhases := []BootstrapPhase{
		BootstrapNone,
		BootstrapEnvoyReady,
		BootstrapStorageJoining,
		BootstrapWorkloadReady,
	}
	for _, p := range readyPhases {
		if !bootstrapInfraReady(p) {
			t.Errorf("bootstrapInfraReady(%s) = false; want true (control-plane-critical should deploy here)", p)
		}
	}

	blockedPhases := []BootstrapPhase{
		BootstrapAdmitted,
		BootstrapInfraPreparing,
		BootstrapEtcdJoining,
		BootstrapEtcdReady,
		BootstrapXdsReady,
	}
	for _, p := range blockedPhases {
		if bootstrapInfraReady(p) {
			t.Errorf("bootstrapInfraReady(%s) = true; want false (infra mesh not ready)", p)
		}
	}
}

// ── bootstrapPhaseReady phase matrix ─────────────────────────────────────────

func TestBootstrapPhaseReady_PhaseMatrix(t *testing.T) {
	readyPhases := []BootstrapPhase{
		BootstrapNone,
		BootstrapStorageJoining,
		BootstrapWorkloadReady,
	}
	for _, p := range readyPhases {
		if !bootstrapPhaseReady(p) {
			t.Errorf("bootstrapPhaseReady(%s) = false; want true (ordinary workloads should deploy here)", p)
		}
	}

	blockedPhases := []BootstrapPhase{
		BootstrapAdmitted,
		BootstrapInfraPreparing,
		BootstrapEtcdJoining,
		BootstrapEtcdReady,
		BootstrapXdsReady,
		BootstrapEnvoyReady, // KEY: ordinary workloads blocked here, control-plane-critical allowed
	}
	for _, p := range blockedPhases {
		if bootstrapPhaseReady(p) {
			t.Errorf("bootstrapPhaseReady(%s) = true; want false (ordinary workloads must wait for workload_ready)", p)
		}
	}
}

// TestBootstrapGate_EnvoyReady_IsTheDeltaWindow verifies the critical
// invariant: EnvoyReady is in bootstrapInfraReady but NOT in bootstrapPhaseReady.
// This is the window where control-plane-critical services can deploy to unblock
// nodes stuck before workload_ready (e.g. due to MinIO topology contract).
func TestBootstrapGate_EnvoyReady_IsTheDeltaWindow(t *testing.T) {
	if !bootstrapInfraReady(BootstrapEnvoyReady) {
		t.Fatal("bootstrapInfraReady(envoy_ready) must be true — control-plane-critical deploy window")
	}
	if bootstrapPhaseReady(BootstrapEnvoyReady) {
		t.Fatal("bootstrapPhaseReady(envoy_ready) must be false — ordinary workloads must wait for workload_ready")
	}
}

// ── Catalog: ControlPlaneCritical registration ────────────────────────────────

// TestCatalog_ControlPlaneCritical_Services verifies that exactly the expected
// set of services carry ControlPlaneCritical=true.
func TestCatalog_ControlPlaneCritical_Services(t *testing.T) {
	required := []string{"cluster-controller", "cluster-doctor", "workflow"}
	for _, name := range required {
		entry := CatalogByName(name)
		if entry == nil {
			t.Errorf("CatalogByName(%q) = nil; service must be in catalog", name)
			continue
		}
		if !entry.ControlPlaneCritical {
			t.Errorf("catalog[%q].ControlPlaneCritical = false; must be true so it deploys at envoy_ready", name)
		}
	}
}

// TestCatalog_OrdinaryWorkloads_NotControlPlaneCritical verifies that standard
// workloads do NOT have ControlPlaneCritical=true — they must wait for workload_ready.
func TestCatalog_OrdinaryWorkloads_NotControlPlaneCritical(t *testing.T) {
	ordinary := []string{
		"authentication", "rbac", "dns", "repository",
		"resource", "event", "file", "search",
	}
	for _, name := range ordinary {
		entry := CatalogByName(name)
		if entry == nil {
			continue // not in static catalog is fine (dynamic), skip
		}
		if entry.ControlPlaneCritical {
			t.Errorf("catalog[%q].ControlPlaneCritical = true; ordinary workloads must NOT be control-plane-critical", name)
		}
	}
}

// ── hasUnservedNodes: bootstrap gate integration ──────────────────────────────

// TestHasUnservedNodes_EnvoyReady_ControlPlaneCritical_IsUnserved verifies that
// a ControlPlaneCritical service sees an envoy_ready node as unserved (it should
// deploy there), while an ordinary workload does NOT see it as unserved (gated out).
func TestHasUnservedNodes_EnvoyReady_ControlPlaneCritical_IsUnserved(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapEnvoyReady,
			},
		},
	}
	srv := newTestServer(t, state)

	// cluster-controller is ControlPlaneCritical — should see n1 as unserved.
	hCC := &releaseHandle{
		Name:               "core@globular.io/cluster-controller",
		ResourceType:       "ServiceRelease",
		Phase:              cluster_controllerpb.ReleasePhaseResolved,
		ResolvedVersion:    "1.0.0",
		InstalledStateName: "cluster-controller",
		InstalledStateKind: "SERVICE",
		Nodes:              []*cluster_controllerpb.NodeReleaseStatus{},
		PatchStatus:        func(_ context.Context, _ statusPatch) error { return nil },
	}
	if !srv.hasUnservedNodes(hCC) {
		t.Error("hasUnservedNodes(cluster-controller) at envoy_ready must return true — control-plane-critical node is eligible")
	}

	// authentication is an ordinary workload — must NOT see n1 as unserved (gated).
	hAuth := &releaseHandle{
		Name:               "core@globular.io/authentication",
		ResourceType:       "ServiceRelease",
		Phase:              cluster_controllerpb.ReleasePhaseResolved,
		ResolvedVersion:    "1.0.0",
		InstalledStateName: "authentication",
		InstalledStateKind: "SERVICE",
		Nodes:              []*cluster_controllerpb.NodeReleaseStatus{},
		PatchStatus:        func(_ context.Context, _ statusPatch) error { return nil },
	}
	if srv.hasUnservedNodes(hAuth) {
		t.Error("hasUnservedNodes(authentication) at envoy_ready must return false — ordinary workload gated until workload_ready")
	}
}

// TestHasUnservedNodes_WorkloadReady_BothServed verifies that at workload_ready,
// both ControlPlaneCritical and ordinary services see the node as unserved
// (i.e., both are allowed to deploy — the gate is open for everyone).
func TestHasUnservedNodes_WorkloadReady_BothServed(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapWorkloadReady,
			},
		},
	}
	srv := newTestServer(t, state)

	for _, name := range []string{"cluster-controller", "authentication"} {
		h := &releaseHandle{
			Name:               "core@globular.io/" + name,
			ResourceType:       "ServiceRelease",
			Phase:              cluster_controllerpb.ReleasePhaseResolved,
			ResolvedVersion:    "1.0.0",
			InstalledStateName: name,
			InstalledStateKind: "SERVICE",
			Nodes:              []*cluster_controllerpb.NodeReleaseStatus{},
			PatchStatus:        func(_ context.Context, _ statusPatch) error { return nil },
		}
		if !srv.hasUnservedNodes(h) {
			t.Errorf("hasUnservedNodes(%s) at workload_ready must return true — gate is open for all workloads", name)
		}
	}
}

// TestHasUnservedNodes_EtcdReady_NeitherServed verifies that at etcd_ready
// (pre-envoy), neither ControlPlaneCritical nor ordinary services see the
// node as eligible — infra mesh is not yet up.
func TestHasUnservedNodes_EtcdReady_NeitherServed(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapEtcdReady,
			},
		},
	}
	srv := newTestServer(t, state)

	for _, name := range []string{"cluster-controller", "workflow", "authentication"} {
		h := &releaseHandle{
			Name:               "core@globular.io/" + name,
			ResourceType:       "ServiceRelease",
			Phase:              cluster_controllerpb.ReleasePhaseResolved,
			ResolvedVersion:    "1.0.0",
			InstalledStateName: name,
			InstalledStateKind: "SERVICE",
			Nodes:              []*cluster_controllerpb.NodeReleaseStatus{},
			PatchStatus:        func(_ context.Context, _ statusPatch) error { return nil },
		}
		if srv.hasUnservedNodes(h) {
			t.Errorf("hasUnservedNodes(%s) at etcd_ready must return false — infra mesh not up yet, no workload may deploy", name)
		}
	}
}
