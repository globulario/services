package main

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestPackageRuntimeHealthyOnNode_ServiceActive(t *testing.T) {
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-rbac.service", State: "active"},
		},
	}
	ok, _ := packageRuntimeHealthyOnNode(node, "rbac", "SERVICE")
	if !ok {
		t.Fatal("expected runtime healthy for active service unit")
	}
}

func TestPackageRuntimeHealthyOnNode_ServiceMissing(t *testing.T) {
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-dns.service", State: "active"},
		},
	}
	ok, reason := packageRuntimeHealthyOnNode(node, "rbac", "SERVICE")
	if ok {
		t.Fatal("expected runtime unhealthy when service unit is missing")
	}
	if reason == "" {
		t.Fatal("expected non-empty runtime reason")
	}
}

func TestPackageRuntimeHealthyOnNode_InfraOverrideUnit(t *testing.T) {
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "scylla-server.service", State: "active"},
		},
	}
	ok, _ := packageRuntimeHealthyOnNode(node, "scylladb", "INFRASTRUCTURE")
	if !ok {
		t.Fatal("expected runtime healthy for scylladb override unit")
	}
}

func TestRuntimeStaleIsNotConverged(t *testing.T) {
	node := &nodeState{
		BootstrapPhase: BootstrapWorkloadReady,
		LastSeen:       time.Now().Add(-10 * time.Minute),
		Units: []unitStatusRecord{
			{Name: "globular-gateway.service", State: "active"},
		},
	}
	pc := classifyPackageConvergence(
		node,
		"gateway",
		"INFRASTRUCTURE",
		"1.0.0",
		"",
		"",
		&node_agentpb.InstalledPackage{Version: "1.0.0"},
		time.Now(),
	)
	if pc.RuntimeState != RuntimeStale {
		t.Fatalf("expected RuntimeStale, got %s (%s)", pc.RuntimeState, pc.Reason)
	}
	if !pc.RepairRequired {
		t.Fatal("expected repair required for stale runtime")
	}
}

func TestCommandPackageDoesNotRequireRuntime(t *testing.T) {
	node := &nodeState{
		LastSeen: time.Now().Add(-24 * time.Hour), // stale should not matter for command packages
	}
	pc := classifyPackageConvergence(
		node,
		"rclone",
		"COMMAND",
		"1.2.3",
		"",
		"",
		&node_agentpb.InstalledPackage{Version: "1.2.3"},
		time.Now(),
	)
	if pc.RuntimeState != RuntimeNotNeeded || !pc.RuntimeOK || pc.RepairRequired {
		t.Fatalf("expected command package converged without runtime checks, got state=%s ok=%v repair=%v reason=%s",
			pc.RuntimeState, pc.RuntimeOK, pc.RepairRequired, pc.Reason)
	}
}

func TestNoDuplicateRuntimeRepairCooldown(t *testing.T) {
	key := runtimeRepairCooldownKey("n1", "minio", "INFRASTRUCTURE", "1.0.0", "", "")
	now := time.Now()
	ok, _ := shouldDispatchRuntimeRepair(key, now)
	if !ok {
		t.Fatal("first dispatch should be allowed")
	}
	ok, wait := shouldDispatchRuntimeRepair(key, now.Add(5*time.Second))
	if ok {
		t.Fatal("second dispatch should be blocked by cooldown")
	}
	if wait <= 0 {
		t.Fatalf("expected positive wait duration, got %s", wait)
	}
	ok, _ = shouldDispatchRuntimeRepair(key, now.Add(runtimeRepairCooldown+time.Second))
	if !ok {
		t.Fatal("dispatch after cooldown should be allowed")
	}
}

func TestHasUnservedNodes_VersionMatchButRuntimeInactive(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"n1": {
					NodeID:            "n1",
					BootstrapPhase:    BootstrapWorkloadReady,
					InstalledVersions: map[string]string{"rbac": "1.2.3"},
					Units: []unitStatusRecord{
						{Name: "globular-rbac.service", State: "inactive"},
					},
					LastSeen: time.Now(),
				},
			},
		},
	}

	h := &releaseHandle{
		Name:               "rbac",
		ResourceType:       "ServiceRelease",
		InstalledStateKind: "SERVICE",
		InstalledStateName: "rbac",
		ResolvedVersion:    "1.2.3",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseApplying},
		},
	}

	if !srv.hasUnservedNodes(h) {
		t.Fatal("expected unserved=true when version matches but runtime is inactive")
	}
}

func TestHasUnservedNodes_VersionMatchAndRuntimeActive(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"n1": {
					NodeID:            "n1",
					BootstrapPhase:    BootstrapWorkloadReady,
					InstalledVersions: map[string]string{"rbac": "1.2.3"},
					Units: []unitStatusRecord{
						{Name: "globular-rbac.service", State: "active"},
					},
					LastSeen: time.Now(),
				},
			},
		},
	}

	h := &releaseHandle{
		Name:               "rbac",
		ResourceType:       "ServiceRelease",
		InstalledStateKind: "SERVICE",
		InstalledStateName: "rbac",
		ResolvedVersion:    "1.2.3",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseApplying},
		},
	}

	if srv.hasUnservedNodes(h) {
		t.Fatal("expected unserved=false when version and runtime both converge")
	}
}
