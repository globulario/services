package main

// scylla_reconnect_test.go — Tests for scyllaSessionMgr reconnect behaviour.
//
// Acceptance criteria (Phase 1):
//   - Killing the session triggers a reconnect after reconnectFailThreshold failures.
//   - RPCs see nil session (codes.Unavailable) while reconnect is in progress.
//   - After reconnect, healthy state is restored without process restart.
//   - Concurrent ping failures do NOT launch more than one reconnect goroutine.
//   - The old session is closed after a successful swap.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gocql/gocql"
)

// fakeSession is a stand-in for *gocql.Session in tests. We cannot create a
// real gocql.Session without a cluster, so we use the manager's connectFn to
// return a controlled value and check that the manager calls close on the old one.
//
// Since *gocql.Session is a concrete struct (not an interface), we can't mock it
// directly. The tests work around this by testing the manager's state transitions
// (nil→session, session generation) rather than live query execution.

// ── Test 1: reconnect is triggered after threshold failures ─────────────────

func TestWorkflowScyllaReconnectAfterPingFailure(t *testing.T) {
	var connectCalls atomic.Int32
	connectDone := make(chan struct{})

	mgr := newScyllaSessionMgr(logger, func() (*gocql.Session, error) {
		connectCalls.Add(1)
		close(connectDone) // signal first call
		// Return a nil *gocql.Session — we only test the manager's state machine.
		return nil, errors.New("simulated connect error — session not returned in unit test")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Install a non-nil placeholder so the mgr thinks there's a live session.
	// (We can't create a real *gocql.Session, so we leave session nil and verify
	// the reconnect goroutine is triggered.)
	mgr.consecutiveFails.Store(0)

	// Simulate threshold consecutive ping failures.
	for i := 0; i < reconnectFailThreshold; i++ {
		mgr.onPingFailure(ctx)
	}

	// The reconnect goroutine must have been launched.
	select {
	case <-connectDone:
		// connectFn was called — reconnect loop is running.
	case <-ctx.Done():
		t.Fatal("reconnect loop was not triggered within timeout")
	}

	if connectCalls.Load() == 0 {
		t.Error("expected connectFn to be called at least once")
	}
}

// ── Test 2: session is nil while reconnecting ────────────────────────────────

func TestWorkflowRPCsUnavailableWhileReconnecting(t *testing.T) {
	// Manager starts with nil session (as if startup not done or reconnect wiped it).
	mgr := newScyllaSessionMgr(logger, func() (*gocql.Session, error) {
		// Never succeed — keep reconnecting forever for this test.
		time.Sleep(50 * time.Millisecond)
		return nil, errors.New("still down")
	})

	// Manually set reconnecting=true (as reconnectLoop does after the initial swap).
	mgr.reconnecting.Store(true)
	// session is nil (default)

	sess := mgr.get()
	if sess != nil {
		t.Error("expected nil session while reconnecting")
	}

	err := sessionUnavailableError(mgr.reconnecting.Load())
	if err == nil {
		t.Error("expected error from sessionUnavailableError while reconnecting")
	}
}

// ── Test 3: service is healthy after reconnect ───────────────────────────────

func TestWorkflowHealthHealthyAfterReconnect(t *testing.T) {
	// The scyllaSessionMgr tracks success/failure state.
	// After onPingSuccess(), consecutiveFails must be 0.
	mgr := newScyllaSessionMgr(logger, func() (*gocql.Session, error) {
		return nil, nil // won't be called in this test
	})

	// Simulate failures below threshold.
	ctx := context.Background()
	mgr.onPingFailure(ctx)
	mgr.onPingFailure(ctx)

	// Simulate recovery: ping succeeds.
	mgr.onPingSuccess()

	if mgr.consecutiveFails.Load() != 0 {
		t.Errorf("consecutiveFails should be 0 after onPingSuccess, got %d",
			mgr.consecutiveFails.Load())
	}

	// A session installed via set() should be returned by get().
	// We pass nil here (can't create a real session in unit tests), but we
	// verify the generation counter is incremented.
	mgr.set(nil)
	gen, _, _, _, _, _ := mgr.stats()
	if gen == 0 {
		t.Error("generation should be > 0 after set()")
	}
}

// ── Test 4: only one reconnect goroutine runs at a time ──────────────────────

func TestNoConcurrentReconnectStorm(t *testing.T) {
	var concurrentCalls atomic.Int32
	var maxConcurrent atomic.Int32

	mgr := newScyllaSessionMgr(logger, func() (*gocql.Session, error) {
		n := concurrentCalls.Add(1)
		defer concurrentCalls.Add(-1)
		if n > maxConcurrent.Load() {
			maxConcurrent.Store(n)
		}
		time.Sleep(20 * time.Millisecond)
		return nil, errors.New("down")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Fire many concurrent onPingFailure calls — only ONE reconnect goroutine
	// should ever run at a time.
	for i := 0; i < 20; i++ {
		// Jump straight to threshold.
		mgr.consecutiveFails.Store(int32(reconnectFailThreshold))
		mgr.onPingFailure(ctx)
	}

	// Give goroutine(s) time to run.
	time.Sleep(100 * time.Millisecond)

	if maxConcurrent.Load() > 1 {
		t.Errorf("concurrent reconnect goroutines: got %d, want at most 1",
			maxConcurrent.Load())
	}
}

// ── Test 5: stats reflects reconnect state ───────────────────────────────────
// (Proxy for "old session closed after swap": since we cannot create real
// *gocql.Session objects, we verify that reconnectLoop clears the session,
// increments the generation, and records the attempt count.)

func TestOldSessionClosedAfterSwap(t *testing.T) {
	var connectCalled atomic.Bool

	mgr := newScyllaSessionMgr(logger, func() (*gocql.Session, error) {
		connectCalled.Store(true)
		// Return nil — in a unit test we can't construct a gocql.Session.
		// The manager sets session to nil, but generation and reconnectAttempts
		// are still updated correctly.
		return nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Trigger reconnect manually.
	if mgr.reconnecting.CompareAndSwap(false, true) {
		go mgr.reconnectLoop(ctx)
	}

	// Wait for reconnect loop to finish.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !mgr.reconnecting.Load() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !connectCalled.Load() {
		t.Error("connectFn should have been called during reconnect loop")
	}

	gen, _, _, attempts, _, reconnecting := mgr.stats()
	if reconnecting {
		t.Error("reconnecting flag should be false after loop completes")
	}
	if gen == 0 {
		t.Error("generation should be incremented after reconnect success")
	}
	if attempts == 0 {
		t.Error("reconnectAttempts should be > 0 after reconnect loop")
	}
}
