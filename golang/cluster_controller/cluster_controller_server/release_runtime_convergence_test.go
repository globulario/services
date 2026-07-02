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
		"", // Phase 38: no entrypoint binding for this stale-runtime test
		false,
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
		"", // Phase 38: no entrypoint binding for COMMAND-kind test
		false,
		&node_agentpb.InstalledPackage{Version: "1.2.3"},
		time.Now(),
	)
	if pc.RuntimeState != RuntimeNotNeeded || !pc.RuntimeOK || pc.RepairRequired {
		t.Fatalf("expected command package converged without runtime checks, got state=%s ok=%v repair=%v reason=%s",
			pc.RuntimeState, pc.RuntimeOK, pc.RepairRequired, pc.Reason)
	}
}

func TestClassifyPackageConvergence_ServiceChecksumIsBinaryNotDesiredHash(t *testing.T) {
	binary := "0e91e8f830b6e2e8ad40dbcd2ac7f515e8ef56420aa0af270f280704d43382b3"
	desiredHash := "ded015e4c8e61ad796038a6bb7301fde0510fd85408a7212f840b7981da4460c"
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-node-agent.service", State: "active"},
		},
	}
	pc := classifyPackageConvergence(
		node,
		"node-agent",
		"SERVICE",
		"1.2.265",
		desiredHash,
		"019f1f21-6343-743e-af63-4d755825e07a",
		binary,
		true,
		&node_agentpb.InstalledPackage{
			Version:  "1.2.265",
			Checksum: binary,
			BuildId:  "019f1f21-6343-743e-af63-4d755825e07a",
			Metadata: map[string]string{"entrypoint_checksum": binary},
		},
		time.Now(),
	)
	if pc.RepairRequired {
		t.Fatalf("service binary checksum must not be compared to desired_hash; reason=%s", pc.Reason)
	}
	if !pc.HashOK {
		t.Fatal("service hash dimension should be satisfied by schema, not desired_hash equality")
	}
}

func TestNoDuplicateRuntimeRepairCooldown(t *testing.T) {
	key := runtimeRepairCooldownKey("n1", "minio", "INFRASTRUCTURE", "1.0.0", "", "")
	runtimeRepairCooldownByTarget.Delete(key)
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

	if !srv.hasUnservedNodes(h, map[string]struct{}{}) {
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
		ResolvedBuildID:    "0191-rbac-build", // D3: a build-backed release must carry a resolved build_id
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseApplying},
		},
	}

	if srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("expected unserved=false when version, build_id, and runtime all converge")
	}
}

// D3: a build-backed release with NO resolved build_id is "missing desired build
// identity" — it must report unserved nodes, not silently converge on
// version+runtime alone.
func TestHasUnservedNodes_MissingBuildIDIsUnserved(t *testing.T) {
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
		// ResolvedBuildID deliberately empty — missing build identity.
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseApplying},
		},
	}

	if !srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("expected unserved=true when the build-backed release has no resolved build_id (missing build identity)")
	}
}

func TestHasUnservedNodes_IgnoresProfileEligibleNodeOutsideTargetList(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"n1": {
					NodeID:            "n1",
					Profiles:          []string{"core"},
					BootstrapPhase:    BootstrapWorkloadReady,
					InstalledVersions: map[string]string{"rbac": "1.2.3"},
					Units: []unitStatusRecord{
						{Name: "globular-rbac.service", State: "active"},
					},
					LastSeen: time.Now(),
				},
				"n2": {
					NodeID:            "n2",
					Profiles:          []string{"core"},
					BootstrapPhase:    BootstrapWorkloadReady,
					InstalledVersions: map[string]string{},
					LastSeen:          time.Now(),
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
		ResolvedBuildID:    "0191-rbac-build",
		TargetNodeIDs:      []string{"n1"},
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseApplying},
		},
	}

	if srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("expected non-target node to be ignored even when profile-eligible and not installed")
	}
}
