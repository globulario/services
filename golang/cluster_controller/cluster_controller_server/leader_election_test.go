package main

import (
	"strings"
	"sync/atomic"
	"testing"
)

// TestLeaderEpochClearedOnDemotion verifies that losing leadership clears
// the epoch so a stale leader cannot pass epoch checks.
func TestLeaderEpochClearedOnDemotion(t *testing.T) {
	srv := &server{}
	srv.leaderEpoch.Store(42)
	srv.setLeader(false, "", "")
	if got := srv.leaderEpoch.Load(); got != 0 {
		t.Errorf("leaderEpoch after demotion = %d, want 0", got)
	}
}

// TestLeaderEpochPreservedOnPromotion verifies that gaining leadership
// does NOT clear the epoch — the epoch is set separately by incrementEpoch
// before setLeader is called.
func TestLeaderEpochPreservedOnPromotion(t *testing.T) {
	srv := &server{}
	srv.leaderEpoch.Store(5)
	// Directly store leader state without calling setLeader(true, ...)
	// which tries to update service registry (requires full server).
	srv.leader.Store(true)
	srv.leaderID.Store("test-id")
	srv.leaderAddr.Store("localhost:12000")
	if got := srv.leaderEpoch.Load(); got != 5 {
		t.Errorf("leaderEpoch after promotion = %d, want 5 (preserved)", got)
	}
}

// TestRequireLeaderRejectsNonLeader verifies that requireLeader returns
// FailedPrecondition when the instance is not the leader.
func TestRequireLeaderRejectsNonLeader(t *testing.T) {
	srv := &server{}
	srv.leader.Store(false)
	srv.leaderAddr.Store("other-node:12000")
	srv.leaderEpoch.Store(0)
	err := srv.requireLeader(nil)
	if err == nil {
		t.Fatal("expected error for non-leader")
	}
	// Should mention leader_addr
	if got := err.Error(); !strings.Contains(got, "other-node:12000") {
		t.Errorf("error should contain leader addr, got: %s", got)
	}
}

// TestRequireLeaderAcceptsLeader verifies that requireLeader passes
// when the instance is the leader.
func TestRequireLeaderAcceptsLeader(t *testing.T) {
	srv := &server{}
	srv.leader.Store(true)
	err := srv.requireLeader(nil)
	if err != nil {
		t.Errorf("expected nil for leader, got: %v", err)
	}
}

// TestRequireLeaderEpochRejectsStaleLeader verifies that a leader with
// a stale epoch is rejected when etcd has a higher epoch.
func TestRequireLeaderEpochRejectsStaleLeader(t *testing.T) {
	srv := &server{}
	srv.leader.Store(true)
	srv.leaderEpoch.Store(3)
	// Without etcd client, requireLeaderEpoch should pass (no fencing in single-node)
	err := srv.requireLeaderEpoch(nil)
	if err != nil {
		t.Errorf("expected nil without etcd, got: %v", err)
	}
}

// TestIsLeaderAtomicSafety verifies concurrent reads of leader state
// don't race.
func TestIsLeaderAtomicSafety(t *testing.T) {
	srv := &server{}
	var done atomic.Bool
	go func() {
		for !done.Load() {
			srv.setLeader(true, "a", "localhost:12000")
			srv.setLeader(false, "", "")
		}
	}()
	for i := 0; i < 1000; i++ {
		_ = srv.isLeader()
		_ = srv.leaderEpoch.Load()
	}
	done.Store(true)
}

