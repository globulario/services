package main

import (
	"testing"
	"time"
)

// makeStorageNode returns a minimal nodeState that is fully eligible: workload
// ready, storage profile, Scylla verified.
func makeStorageNode(nodeID string) *nodeState {
	return &nodeState{
		NodeID:  nodeID,
		Status:  "active",
		Profiles: []string{"storage", "control-plane"},
		BootstrapPhase:     BootstrapWorkloadReady,
		ScyllaJoinPhase:    ScyllaJoinVerified,
		ScyllaJoinStartedAt: time.Now().Add(-10 * time.Minute),
	}
}

// TestIsNodeVerifiedStorageEligible_HealthyNodeCounts verifies that a fully
// ready storage/control-plane node is counted.
func TestIsNodeVerifiedStorageEligible_HealthyNodeCounts(t *testing.T) {
	n := makeStorageNode("ryzen")
	if !IsNodeVerifiedStorageEligible(n) {
		t.Fatalf("healthy workload_ready+verified node must be eligible, reason=%q",
			nodeStorageEligibilityReason(n))
	}
}

// TestIsNodeVerifiedStorageEligible_RemovedNodeExcluded verifies that a node
// with status=removed is excluded from the count.
func TestIsNodeVerifiedStorageEligible_RemovedNodeExcluded(t *testing.T) {
	n := makeStorageNode("dead-node")
	n.Status = "removed"
	if IsNodeVerifiedStorageEligible(n) {
		t.Fatal("removed node must not be eligible")
	}
	if r := nodeStorageEligibilityReason(n); r != "status=removed" {
		t.Fatalf("expected reason=status=removed, got %q", r)
	}
}

// TestIsNodeVerifiedStorageEligible_BlockedNodeExcluded verifies that a node
// with status=blocked is excluded.
func TestIsNodeVerifiedStorageEligible_BlockedNodeExcluded(t *testing.T) {
	n := makeStorageNode("blocked-node")
	n.Status = "blocked"
	if IsNodeVerifiedStorageEligible(n) {
		t.Fatal("blocked node must not be eligible")
	}
	if r := nodeStorageEligibilityReason(n); r != "status=blocked" {
		t.Fatalf("expected reason=status=blocked, got %q", r)
	}
}

// TestIsNodeVerifiedStorageEligible_BootstrappingNodeExcluded verifies that a
// node still in an intermediate bootstrap phase is excluded.
func TestIsNodeVerifiedStorageEligible_BootstrappingNodeExcluded(t *testing.T) {
	bootstrappingPhases := []BootstrapPhase{
		BootstrapAdmitted,
		BootstrapInfraPreparing,
		BootstrapEtcdJoining,
		BootstrapEtcdReady,
		BootstrapXdsReady,
		BootstrapEnvoyReady,
		BootstrapAwarenessReady,
	}
	for _, phase := range bootstrappingPhases {
		n := makeStorageNode("joining-node")
		n.BootstrapPhase = phase
		if IsNodeVerifiedStorageEligible(n) {
			t.Errorf("node in bootstrap phase %q must not be eligible", phase)
		}
		r := nodeStorageEligibilityReason(n)
		if r == "" {
			t.Errorf("expected non-empty reason for phase %q", phase)
		}
	}
}

// TestIsNodeVerifiedStorageEligible_UnreachableNodeExcluded verifies that a
// node with status=unreachable is excluded.
func TestIsNodeVerifiedStorageEligible_UnreachableNodeExcluded(t *testing.T) {
	n := makeStorageNode("gone-node")
	n.Status = "unreachable"
	if IsNodeVerifiedStorageEligible(n) {
		t.Fatal("unreachable node must not be eligible")
	}
	if r := nodeStorageEligibilityReason(n); r != "status=unreachable" {
		t.Fatalf("expected reason=status=unreachable, got %q", r)
	}
}

// TestIsNodeVerifiedStorageEligible_ScyllaJoinInProgressExcluded verifies that
// a node whose Scylla join is tracked and in progress is excluded.
func TestIsNodeVerifiedStorageEligible_ScyllaJoinInProgressExcluded(t *testing.T) {
	inProgressPhases := []ScyllaJoinPhase{
		ScyllaJoinPrepared,
		ScyllaJoinConfigured,
		ScyllaJoinStarted,
	}
	startedAt := time.Now().Add(-1 * time.Minute)
	for _, phase := range inProgressPhases {
		n := makeStorageNode("joining-scylla-node")
		n.ScyllaJoinPhase = phase
		n.ScyllaJoinStartedAt = startedAt
		if IsNodeVerifiedStorageEligible(n) {
			t.Errorf("node with scylla phase %q (in progress) must not be eligible", phase)
		}
		r := nodeStorageEligibilityReason(n)
		if r == "" {
			t.Errorf("expected non-empty reason for scylla phase %q", phase)
		}
	}
}

// TestIsNodeVerifiedStorageEligible_ScyllaFailedExcluded verifies that a node
// with ScyllaJoinFailed is excluded regardless of whether the start time is set.
// This is the "unknown health fails closed" case.
func TestIsNodeVerifiedStorageEligible_ScyllaFailedExcluded(t *testing.T) {
	// With start time set.
	n := makeStorageNode("failed-scylla-node")
	n.ScyllaJoinPhase = ScyllaJoinFailed
	n.ScyllaJoinStartedAt = time.Now().Add(-5 * time.Minute)
	if IsNodeVerifiedStorageEligible(n) {
		t.Fatal("scylla_join_failed node must not be eligible")
	}
	if r := nodeStorageEligibilityReason(n); r != "scylla_join_failed" {
		t.Fatalf("expected reason=scylla_join_failed, got %q", r)
	}

	// Without start time — still excluded (defense in depth).
	n2 := makeStorageNode("failed-scylla-node-no-start")
	n2.ScyllaJoinPhase = ScyllaJoinFailed
	n2.ScyllaJoinStartedAt = time.Time{}
	if IsNodeVerifiedStorageEligible(n2) {
		t.Fatal("scylla_join_failed node (no start time) must not be eligible")
	}
}

// TestIsNodeVerifiedStorageEligible_LegacyNoneEligible verifies that a node
// with ScyllaJoinNone and no start time (legacy cluster node that predates the
// join state machine) is still counted. This prevents a mass-exclusion of all
// existing healthy nodes on upgrade. Phase D will harden this case.
func TestIsNodeVerifiedStorageEligible_LegacyNoneEligible(t *testing.T) {
	n := makeStorageNode("legacy-node")
	n.ScyllaJoinPhase = ScyllaJoinNone
	n.ScyllaJoinStartedAt = time.Time{} // never tracked
	if !IsNodeVerifiedStorageEligible(n) {
		t.Fatalf("legacy node (phase=none, no start time) must be eligible (backward compat): reason=%q",
			nodeStorageEligibilityReason(n))
	}
}

// TestIsNodeVerifiedStorageEligible_BootstrapFailedExcluded verifies that a
// node in the bootstrap_failed terminal state is excluded.
func TestIsNodeVerifiedStorageEligible_BootstrapFailedExcluded(t *testing.T) {
	n := makeStorageNode("failed-bootstrap-node")
	n.BootstrapPhase = BootstrapFailed
	if IsNodeVerifiedStorageEligible(n) {
		t.Fatal("bootstrap_failed node must not be eligible")
	}
	if r := nodeStorageEligibilityReason(n); r != "bootstrap_failed" {
		t.Fatalf("expected reason=bootstrap_failed, got %q", r)
	}
}
