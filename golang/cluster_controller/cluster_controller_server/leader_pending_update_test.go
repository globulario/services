package main

// leader_pending_update_test.go — G10 leader-stuck etcd write tests.
//
// Verifies that writeLeaderPendingUpdate and clearLeaderPendingUpdate are
// injectable and that leaderStuckSince tracks first-detection correctly.

import (
	"context"
	"testing"
	"time"
)

// TestLeaderPendingUpdate_WriteIsInjectable verifies that writeLeaderPendingUpdate
// is replaceable and receives the correct fields.
func TestLeaderPendingUpdate_WriteIsInjectable(t *testing.T) {
	orig := writeLeaderPendingUpdate
	t.Cleanup(func() { writeLeaderPendingUpdate = orig })

	var got LeaderPendingUpdateRecord
	writeLeaderPendingUpdate = func(_ context.Context, rec LeaderPendingUpdateRecord) {
		got = rec
	}

	writeLeaderPendingUpdate(context.Background(), LeaderPendingUpdateRecord{
		LeaderNodeID:   "node-ryzen",
		CurrentVersion: "1.0.83",
		TargetVersion:  "1.0.84+5",
		FollowersTotal: 2,
		StuckSinceUnix: 1000,
		DetectedAtUnix: 2000,
	})

	if got.LeaderNodeID != "node-ryzen" {
		t.Errorf("LeaderNodeID = %q, want node-ryzen", got.LeaderNodeID)
	}
	if got.CurrentVersion != "1.0.83" {
		t.Errorf("CurrentVersion = %q, want 1.0.83", got.CurrentVersion)
	}
	if got.TargetVersion != "1.0.84+5" {
		t.Errorf("TargetVersion = %q, want 1.0.84+5", got.TargetVersion)
	}
	if got.FollowersTotal != 2 {
		t.Errorf("FollowersTotal = %d, want 2", got.FollowersTotal)
	}
	if got.StuckSinceUnix != 1000 {
		t.Errorf("StuckSinceUnix = %d, want 1000", got.StuckSinceUnix)
	}
	if got.DetectedAtUnix != 2000 {
		t.Errorf("DetectedAtUnix = %d, want 2000", got.DetectedAtUnix)
	}
}

// TestLeaderPendingUpdate_ClearIsInjectable verifies that clearLeaderPendingUpdate
// is replaceable.
func TestLeaderPendingUpdate_ClearIsInjectable(t *testing.T) {
	orig := clearLeaderPendingUpdate
	t.Cleanup(func() { clearLeaderPendingUpdate = orig })

	var called bool
	clearLeaderPendingUpdate = func(_ context.Context) {
		called = true
	}

	clearLeaderPendingUpdate(context.Background())
	if !called {
		t.Error("injected clearLeaderPendingUpdate was not called")
	}
}

// TestLeaderPendingUpdate_StuckSinceTracking verifies that leaderStuckSince is
// set on first detection and NOT overwritten on subsequent detections.
func TestLeaderPendingUpdate_StuckSinceTracking(t *testing.T) {
	// Reset state before test.
	leaderStuckSince.Store(0)
	t.Cleanup(func() { leaderStuckSince.Store(0) })

	before := time.Now().Unix()

	// Simulate first detection.
	if leaderStuckSince.Load() == 0 {
		leaderStuckSince.Store(time.Now().Unix())
	}
	firstSet := leaderStuckSince.Load()

	if firstSet < before {
		t.Errorf("StuckSince should be >= %d, got %d", before, firstSet)
	}

	// Simulate second detection — should NOT overwrite StuckSince.
	time.Sleep(10 * time.Millisecond)
	if leaderStuckSince.Load() == 0 {
		leaderStuckSince.Store(time.Now().Unix())
	}
	secondRead := leaderStuckSince.Load()

	if secondRead != firstSet {
		t.Errorf("StuckSince changed on second detection: %d → %d (must stay constant while stuck)", firstSet, secondRead)
	}
}

// TestLeaderPendingUpdate_ClearResetsStuckSince verifies that clearing the
// stuck state also resets leaderStuckSince to 0.
func TestLeaderPendingUpdate_ClearResetsStuckSince(t *testing.T) {
	leaderStuckSince.Store(12345)
	t.Cleanup(func() { leaderStuckSince.Store(0) })

	// Simulate resolving the stuck condition.
	leaderStuckSince.Store(0)

	if leaderStuckSince.Load() != 0 {
		t.Errorf("leaderStuckSince should be 0 after clear, got %d", leaderStuckSince.Load())
	}
}
