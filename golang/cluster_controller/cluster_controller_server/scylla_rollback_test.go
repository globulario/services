package main

import (
	"context"
	"testing"
	"time"
)

// newRollbackMgr returns a scylla manager whose enqueueNodeRemoval records the
// node IDs it was asked to remove.
func newRollbackMgr(enq *[]string) *scyllaClusterManager {
	mgr := newScyllaClusterManager()
	mgr.enqueueNodeRemoval = func(ctx context.Context, nodeID, hostname, ip, agentEndpoint string) error {
		*enq = append(*enq, nodeID)
		return nil
	}
	return mgr
}

func candidate(phase ScyllaJoinPhase, everVerified bool, startedAt time.Time) *nodeState {
	return &nodeState{
		NodeID:                "cand1",
		Identity:              storedIdentity{Hostname: "cand", Ips: []string{"10.0.0.9"}},
		Profiles:              []string{"scylla"},
		ScyllaJoinPhase:       phase,
		ScyllaWasEverVerified: everVerified,
		ScyllaJoinStartedAt:   startedAt,
	}
}

// TestScyllaRollback_FailedFreshJoinSchedulesRemoval: a never-verified failed
// candidate enqueues removal and transitions to RollbackPending.
func TestScyllaRollback_FailedFreshJoinSchedulesRemoval(t *testing.T) {
	var enq []string
	mgr := newRollbackMgr(&enq)
	node := candidate(ScyllaJoinFailed, false, time.Now())
	nodes := []*nodeState{node}

	dirty := mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if !dirty {
		t.Fatal("expected dirty")
	}
	if node.ScyllaJoinPhase != ScyllaJoinRollbackPending {
		t.Fatalf("phase = %s, want rollback_pending", node.ScyllaJoinPhase)
	}
	if len(enq) != 1 || enq[0] != "cand1" {
		t.Fatalf("expected removal enqueued for cand1, got %v", enq)
	}
}

// TestScyllaRollback_RefusesVerifiedMember: an owning member (ScyllaWasEverVerified)
// is NEVER decommissioned on failure — the hard fence.
func TestScyllaRollback_RefusesVerifiedMember(t *testing.T) {
	var enq []string
	mgr := newRollbackMgr(&enq)
	node := candidate(ScyllaJoinFailed, true, time.Now()) // ever-verified = owning member
	nodes := []*nodeState{node}

	mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if len(enq) != 0 {
		t.Fatalf("owning member must not be decommissioned; enqueued %v", enq)
	}
	if node.ScyllaJoinPhase != ScyllaJoinFailed {
		t.Fatalf("owning member phase = %s, want unchanged (failed)", node.ScyllaJoinPhase)
	}
}

// TestScyllaRollback_SuccessPathNeverDecommissions: a verified node is not touched.
func TestScyllaRollback_SuccessPathNeverDecommissions(t *testing.T) {
	var enq []string
	mgr := newRollbackMgr(&enq)
	node := candidate(ScyllaJoinVerified, true, time.Now())
	node.ScyllaWasEverVerified = true
	node.Units = []unitStatusRecord{{Name: "scylla-server.service", State: "active"}}
	nodes := []*nodeState{node}

	mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if len(enq) != 0 {
		t.Fatalf("verified node must never be decommissioned; enqueued %v", enq)
	}
}

// TestScyllaRollback_PendingReEnqueuesOnlyAfterTimeout: RollbackPending is a
// bounded, idempotent retry — no re-dispatch before the timeout, one after.
func TestScyllaRollback_PendingReEnqueuesOnlyAfterTimeout(t *testing.T) {
	var enq []string
	mgr := newRollbackMgr(&enq)

	// Recent RollbackPending: within timeout → no re-enqueue (throttled).
	fresh := candidate(ScyllaJoinRollbackPending, false, time.Now())
	mgr.reconcileScyllaJoinPhases(context.Background(), []*nodeState{fresh})
	if len(enq) != 0 {
		t.Fatalf("must not re-enqueue within timeout; got %v", enq)
	}

	// Stale RollbackPending: past timeout → exactly one re-enqueue.
	stale := candidate(ScyllaJoinRollbackPending, false, time.Now().Add(-scyllaJoinTimeout-time.Minute))
	mgr.reconcileScyllaJoinPhases(context.Background(), []*nodeState{stale})
	if len(enq) != 1 || enq[0] != "cand1" {
		t.Fatalf("expected one re-enqueue after timeout, got %v", enq)
	}
}

// TestScyllaRollback_NilHookIsSafe: with no removal capability wired, a failed
// candidate stays Failed (no panic, no phantom rollback).
func TestScyllaRollback_NilHookIsSafe(t *testing.T) {
	mgr := newScyllaClusterManager() // enqueueNodeRemoval == nil
	node := candidate(ScyllaJoinFailed, false, time.Now())
	nodes := []*nodeState{node}

	mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if node.ScyllaJoinPhase != ScyllaJoinFailed {
		t.Fatalf("with nil hook, phase = %s, want failed (unchanged)", node.ScyllaJoinPhase)
	}
}
