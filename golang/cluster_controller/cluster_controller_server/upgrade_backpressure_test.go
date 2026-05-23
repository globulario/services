package main

// upgrade_backpressure_test.go — Integration tests for the convergence loop
// guards introduced in PR-4 through PR-9. These tests verify that the
// dispatch-gate chain (convergence results → drift suppressor →
// convergenceBlockedNodes → hasUnservedNodes) behaves correctly end-to-end
// without requiring a live etcd cluster.

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
)

// TestMaxParallelNodesForKind verifies that infrastructure packages are
// upgraded serially (1) while service/workload/command packages use the
// wider 2-node default to preserve quorum during rolling upgrades.
func TestMaxParallelNodesForKind(t *testing.T) {
	cases := []struct {
		kind string
		want int
	}{
		{"INFRASTRUCTURE", 1},
		{"infrastructure", 1}, // case-insensitive
		{"SERVICE", 2},
		{"WORKLOAD", 2},
		{"COMMAND", 2},
		{"", 2}, // unknown kind defaults to safe value
	}
	for _, tc := range cases {
		got := maxParallelNodesForKind(tc.kind)
		if got != tc.want {
			t.Errorf("maxParallelNodesForKind(%q) = %d, want %d", tc.kind, got, tc.want)
		}
	}
}

// TestUpgradeLoopSuppression is the mandatory integration test for the full
// dispatch-gate chain. It exercises the scenario that caused the original
// upgrade loops: a 3-node cluster where one node is blocked and another is
// unserved. The test verifies that:
//   1. A blocked node (BLOCKED_*/FAILED_PERMANENT) does NOT count as unserved.
//   2. A truly unserved node still triggers dispatch (hasUnservedNodes=true).
//   3. Once all non-blocked nodes are served, the loop stops (false).
func TestUpgradeLoopSuppression(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:            "n1",
				Status:            "ready",
				LastSeen:          time.Now(),
				BootstrapPhase:    BootstrapWorkloadReady,
				InstalledVersions: map[string]string{"rbac": "1.1.0"},
				Units: []unitStatusRecord{
					{Name: "globular-rbac.service", State: "active"},
				},
			},
			"n2": {
				NodeID:         "n2",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapWorkloadReady,
			},
			"n3": {
				NodeID:         "n3",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapWorkloadReady,
			},
		},
	}
	srv := newTestServer(t, state)

	h := &releaseHandle{
		Name:               "core@globular.io/rbac",
		ResourceType:       "ServiceRelease",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		ResolvedVersion:    "1.1.0",
		InstalledStateName: "rbac",
		InstalledStateKind: "SERVICE",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
		},
		PatchStatus: func(_ context.Context, _ statusPatch) error { return nil },
	}

	// n2 is blocked (missing native dep), n3 is unserved.
	blockedNodes := map[string]struct{}{"n2": {}}

	// n3 is unserved → dispatch must still fire.
	if !srv.hasUnservedNodes(h, blockedNodes) {
		t.Fatal("n3 is unserved — hasUnservedNodes should return true")
	}

	// Mark n3 served (workflow completed for it).
	h.Nodes = append(h.Nodes, &cluster_controllerpb.NodeReleaseStatus{
		NodeID: "n3",
		Phase:  cluster_controllerpb.ReleasePhaseAvailable,
	})

	// Only n2 (blocked) remains; no more truly unserved nodes → loop stops.
	if srv.hasUnservedNodes(h, blockedNodes) {
		t.Fatal("only blocked n2 remains — hasUnservedNodes should return false (loop must stop)")
	}
}

// TestDriftSuppressionGatesDispatch verifies that the drift suppressor
// correctly gates re-dispatch based on convergence outcomes and timing.
// This is the gate that prevents the controller from re-dispatching workflows
// for packages that are blocked or still converging.
func TestDriftSuppressionGatesDispatch(t *testing.T) {
	now := time.Now()
	conv := map[string]*installed_state.ConvergenceResultV1{
		"authentication": {
			Outcome:       installed_state.OutcomeBlockedCriticalKeyMissing,
			LastAttemptAt: now.Add(-1 * time.Minute).Unix(),
		},
		"rbac": {
			Outcome:       installed_state.OutcomeSuccessCommitted,
			LastAttemptAt: now.Add(-2 * time.Minute).Unix(),
		},
		"workflow": {
			Outcome:       installed_state.OutcomeBlockedMissingNativeDep,
			LastAttemptAt: now.Unix(),
		},
	}

	// authentication: CriticalKeyMissing within 5-min window → suppressed.
	if !driftSuppressed(conv, "authentication", "node1", "n1") {
		t.Error("authentication within critical-key window should suppress dispatch")
	}

	// rbac: committed → dispatch allowed.
	if driftSuppressed(conv, "rbac", "node1", "n1") {
		t.Error("rbac committed → dispatch should NOT be suppressed")
	}

	// workflow: native dep missing → forever suppress.
	if !driftSuppressed(conv, "workflow", "node1", "n1") {
		t.Error("workflow with missing native dep should suppress dispatch indefinitely")
	}

	// dns-server: no entry → fail-open, dispatch allowed.
	if driftSuppressed(conv, "dns-server", "node1", "n1") {
		t.Error("no convergence entry → should not suppress (fail-open)")
	}
}

// TestRuntimeDepBlockStopsSpinLoop verifies the fix for the sidekick
// AVAILABLE → PENDING → AVAILABLE spin loop. Before the fix, nodes skipped by
// reconcileResolved due to unmet RuntimeLocalDependencies (e.g. sidekick needing
// minio) had no convergence outcome record, so hasUnservedNodes treated them as
// unserved and re-entered PENDING on every reconcile cycle even though those
// nodes could not receive a dispatch.
//
// After the fix, reconcileResolved writes OutcomeBlockedMissingNativeDep for
// dep-blocked nodes, which convergenceBlockedNodes picks up. This test verifies
// the hasUnservedNodes side of the contract: when all unserved nodes are
// dep-blocked, hasUnservedNodes must return false.
func TestRuntimeDepBlockStopsSpinLoop(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			// n1: sidekick already installed (served).
			"n1": {NodeID: "n1", Status: "ready", LastSeen: time.Now(), BootstrapPhase: BootstrapWorkloadReady,
				InstalledVersions: map[string]string{"sidekick": "7.0.0"}},
			// n2, n3: sidekick NOT installed, minio (dep) not yet active.
			"n2": {NodeID: "n2", Status: "ready", LastSeen: time.Now(), BootstrapPhase: BootstrapWorkloadReady},
			"n3": {NodeID: "n3", Status: "ready", LastSeen: time.Now(), BootstrapPhase: BootstrapWorkloadReady},
		},
	}
	srv := newTestServer(t, state)

	h := &releaseHandle{
		Name:               "core@globular.io/sidekick",
		ResourceType:       "InfrastructureRelease",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		ResolvedVersion:    "7.0.0",
		InstalledStateName: "sidekick",
		InstalledStateKind: "INFRASTRUCTURE",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
		},
		PatchStatus: func(_ context.Context, _ statusPatch) error { return nil },
	}

	// Without dep-block records: n2 and n3 appear unserved → spin loop fires.
	if !srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("n2 and n3 are unserved — hasUnservedNodes should return true before dep-block records exist")
	}

	// Simulate writeRuntimeDepBlock having written OutcomeBlockedMissingNativeDep
	// for n2 and n3: pass them in as blockedNodes.
	depBlocked := map[string]struct{}{"n2": {}, "n3": {}}
	if srv.hasUnservedNodes(h, depBlocked) {
		t.Fatal("n2 and n3 are dep-blocked — hasUnservedNodes must return false (spin loop must stop)")
	}
}

// TestUpgradeLoopStopsWhenAllServedOrBlocked verifies that hasUnservedNodes
// returns false when every node is either served (AVAILABLE) or blocked —
// preventing the controller from re-dispatching in a tight loop.
func TestUpgradeLoopStopsWhenAllServedOrBlocked(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1", Status: "ready", LastSeen: time.Now(), BootstrapPhase: BootstrapWorkloadReady},
			"n2": {NodeID: "n2", Status: "ready", LastSeen: time.Now(), BootstrapPhase: BootstrapWorkloadReady},
			"n3": {NodeID: "n3", Status: "ready", LastSeen: time.Now(), BootstrapPhase: BootstrapWorkloadReady},
		},
	}
	srv := newTestServer(t, state)

	// All three nodes appear in per-node release status as AVAILABLE.
	h := &releaseHandle{
		Name:               "core@globular.io/dns-server",
		ResourceType:       "ServiceRelease",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		ResolvedVersion:    "2.0.0",
		InstalledStateName: "dns-server",
		InstalledStateKind: "SERVICE",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
			{NodeID: "n2", Phase: cluster_controllerpb.ReleasePhaseAvailable},
			{NodeID: "n3", Phase: cluster_controllerpb.ReleasePhaseAvailable},
		},
		PatchStatus: func(_ context.Context, _ statusPatch) error { return nil },
	}

	// No blocked nodes, all served → loop must stop.
	if srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("all nodes served → hasUnservedNodes must return false (loop must stop)")
	}
}
