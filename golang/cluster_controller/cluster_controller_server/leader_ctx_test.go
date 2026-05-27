package main

import (
	"testing"
	"time"
)

// TestLeaderCtxCancelledOnResign verifies that the leader-scoped context
// is cancelled when leadership is lost. Leader-only goroutines that derive
// from getLeaderCtx() should observe ctx.Done() and stop promptly.
func TestLeaderCtxCancelledOnResign(t *testing.T) {
	srv := &server{}

	// Before leadership: getLeaderCtx returns a cancelled context.
	ctx := srv.getLeaderCtx()
	select {
	case <-ctx.Done():
		// expected — not leader
	default:
		t.Fatal("expected cancelled context before leadership")
	}

	// Gain leadership.
	srv.setLeader(true, "test-leader", "127.0.0.1:12000")
	ctx = srv.getLeaderCtx()
	select {
	case <-ctx.Done():
		t.Fatal("leader context should NOT be cancelled while leader")
	default:
		// expected — still leader
	}

	// Lose leadership.
	srv.setLeader(false, "", "")
	select {
	case <-ctx.Done():
		// expected — context cancelled on resign
	case <-time.After(100 * time.Millisecond):
		t.Fatal("leader context should be cancelled after resign")
	}
}

// TestLeaderCtxNewOnReelection verifies that gaining leadership again
// creates a fresh context (not the old cancelled one).
func TestLeaderCtxNewOnReelection(t *testing.T) {
	srv := &server{}

	srv.setLeader(true, "leader-1", "addr1")
	ctx1 := srv.getLeaderCtx()

	srv.setLeader(false, "", "")
	select {
	case <-ctx1.Done():
	default:
		t.Fatal("old leader context should be cancelled")
	}

	srv.setLeader(true, "leader-2", "addr2")
	ctx2 := srv.getLeaderCtx()
	select {
	case <-ctx2.Done():
		t.Fatal("new leader context should NOT be cancelled")
	default:
		// expected — fresh context for new term
	}
}

// TestDeployDispatchRespectsLeaderCtx verifies that a goroutine using
// getLeaderCtx() will be cancelled when leadership is lost, preventing
// post-resign mutations.
func TestDeployDispatchRespectsLeaderCtx(t *testing.T) {
	srv := &server{}
	srv.setLeader(true, "leader", "addr")

	ctx := srv.getLeaderCtx()

	// Simulate a long-running operation.
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			return // correctly cancelled
		case <-time.After(10 * time.Second):
			// should not reach here
		}
	}()

	// Lose leadership — goroutine should stop.
	srv.setLeader(false, "", "")

	select {
	case <-done:
		// goroutine stopped — correct
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine should have stopped after leadership loss")
	}
}
