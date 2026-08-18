package observation

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

func testBundle(id string) Bundle {
	return Bundle{
		Signal: api.Signal{
			ID:      id,
			Project: "globular-services",
			Domain:  "cluster_operator",
		},
	}
}

// A bundle missing project/domain is rejected before it can occupy queue space,
// and the rejection is counted rather than silently swallowed.
func TestRecorder_RejectsInvalidBundle(t *testing.T) {
	r := NewRecorder(RecorderOptions{QueueSize: 4, Workers: 0})
	defer func() { _ = r.Close(context.Background()) }()

	if r.Enqueue(Bundle{Signal: api.Signal{ID: "no-project"}}) {
		t.Fatal("bundle without project/domain must be rejected")
	}
	st := r.Stats()
	if st.Dropped != 1 {
		t.Errorf("invalid bundle → dropped=1, got %d", st.Dropped)
	}
	if st.Enqueued != 0 {
		t.Errorf("invalid bundle must not count as enqueued, got %d", st.Enqueued)
	}
	if st.LastError == "" {
		t.Error("rejection must record LastError — a silent drop is invisible learning loss")
	}
}

// The queue is bounded and Enqueue never blocks. With no workers draining, the
// queue fills and further offers are dropped rather than stalling the caller —
// cluster diagnosis must not wait on behavioral persistence.
func TestRecorder_QueueIsBoundedAndNeverBlocks(t *testing.T) {
	const size = 3
	// Constructed without workers on purpose. NewRecorder's applyDefaults turns
	// Workers:0 into 2, and running workers drain the queue concurrently, so the
	// fill/overflow counts would be racy. This test is about Enqueue's own
	// contract — bounded and non-blocking — so it isolates that from delivery.
	r := &Recorder{
		opts:    RecorderOptions{QueueSize: size, MaxAttempts: 1},
		queue:   make(chan Bundle, size),
		stopped: make(chan struct{}),
	}
	defer func() { _ = r.Close(context.Background()) }()

	done := make(chan struct{})
	go func() {
		for i := 0; i < size+5; i++ {
			r.Enqueue(testBundle("sig"))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue blocked — the report path must never stall on a full queue")
	}

	st := r.Stats()
	if st.Enqueued != size {
		t.Errorf("accepted up to QueueSize=%d, got %d", size, st.Enqueued)
	}
	if st.Dropped != 5 {
		t.Errorf("overflow → dropped=5, got %d", st.Dropped)
	}
	if st.QueueDepth != size {
		t.Errorf("QueueDepth=%d, want %d", st.QueueDepth, size)
	}
}

// Enqueue after Close is refused and counted, not panicking on a closed channel.
func TestRecorder_EnqueueAfterCloseIsRefused(t *testing.T) {
	r := NewRecorder(RecorderOptions{QueueSize: 2, Workers: 0})
	if err := r.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if r.Enqueue(testBundle("late")) {
		t.Error("Enqueue after Close must be refused")
	}
	if r.Stats().Dropped == 0 {
		t.Error("post-close refusal must be counted")
	}
}

// Close is idempotent — a second call must not panic on an already-closed
// stop channel.
func TestRecorder_CloseIsIdempotent(t *testing.T) {
	r := NewRecorder(RecorderOptions{QueueSize: 1, Workers: 1})
	if err := r.Close(context.Background()); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatalf("second Close must be safe, got %v", err)
	}
}

// Defaults must be applied so a zero-value options struct yields a usable
// recorder rather than an unbuffered queue with no workers.
func TestRecorder_AppliesDefaults(t *testing.T) {
	var o RecorderOptions
	o.applyDefaults()
	if o.QueueSize <= 0 || o.Workers <= 0 || o.MaxAttempts <= 0 {
		t.Fatalf("defaults not applied: %+v", o)
	}
	if o.BaseBackoff <= 0 || o.CallTimeout <= 0 {
		t.Fatalf("timing defaults not applied: %+v", o)
	}
}

// Delivery failure must be reportable. With an unresolvable endpoint every
// attempt fails, and the recorder must surface that as failed+LastError rather
// than appearing healthy.
// deterministicRecorder drives delivery through the in-package seam so these
// tests do not depend on whether the host is running an AI-memory service.
//
// The previous version passed Addr:"" with a comment claiming it was
// "unresolvable in test env". Empty means the opposite — resolve through
// config.ResolveServiceAddr — so on a Globular host it could reach a real
// service or sit in discovery, and the test proved nothing either way.
func deterministicRecorder(t *testing.T, write func(Bundle) error) *Recorder {
	t.Helper()
	r := NewRecorder(RecorderOptions{
		QueueSize:   4,
		Workers:     1,
		MaxAttempts: 1,
		CallTimeout: 200 * time.Millisecond,
	})
	r.writeBundle = write
	t.Cleanup(func() { _ = r.Close(context.Background()) })
	return r
}

func awaitStats(t *testing.T, r *Recorder, want func(Stats) bool, what string) Stats {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st := r.Stats(); want(st) {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("recorder never reached %s: %+v", what, r.Stats())
	return Stats{}
}

func TestRecorder_PersistentFailureIsVisible(t *testing.T) {
	r := deterministicRecorder(t, func(Bundle) error {
		return errors.New("behavioral-memory unavailable")
	})
	r.Enqueue(testBundle("will-fail"))

	st := awaitStats(t, r, func(s Stats) bool { return s.Failed > 0 }, "a terminal failure")
	if st.LastError == "" {
		t.Error("failure must record LastError")
	}
	if st.LastFailureAt.IsZero() {
		t.Error("failure must stamp LastFailureAt")
	}
	if got := st.Health(); got != RecorderFailing {
		t.Errorf("health = %q, want %q: an accepted bundle that was lost must be visible", got, RecorderFailing)
	}
}

func TestRecorder_SuccessfulDeliveryIsVisible(t *testing.T) {
	r := deterministicRecorder(t, func(Bundle) error { return nil })
	r.Enqueue(testBundle("will-succeed"))

	st := awaitStats(t, r, func(s Stats) bool { return s.Persisted > 0 }, "a successful delivery")
	if st.LastSuccessAt.IsZero() {
		t.Error("success must stamp LastSuccessAt")
	}
	if got := st.Health(); got != RecorderHealthy {
		t.Errorf("health = %q, want %q", got, RecorderHealthy)
	}
}

// The state must be reachable by polling, without a further enqueue. A bundle
// accepted and then lost is exactly the case an enqueue-rejection signal misses.
func TestRecorder_AcceptedThenFailedIsVisibleWithoutFurtherEnqueue(t *testing.T) {
	r := deterministicRecorder(t, func(Bundle) error { return errors.New("gone") })
	r.Enqueue(testBundle("accepted-then-lost"))

	before := r.Stats().Enqueued
	st := awaitStats(t, r, func(s Stats) bool { return s.Health() == RecorderFailing }, "a visible failure")
	if st.Enqueued != before {
		t.Fatalf("failure only became visible after another enqueue (%d → %d)", before, st.Enqueued)
	}
}

// Recovery must follow observed success, not merely the passage of time.
func TestRecorder_RecoveryRequiresObservedSuccess(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	r := deterministicRecorder(t, func(Bundle) error {
		if fail.Load() {
			return errors.New("still down")
		}
		return nil
	})

	r.Enqueue(testBundle("fails"))
	awaitStats(t, r, func(s Stats) bool { return s.Health() == RecorderFailing }, "a failing state")

	fail.Store(false)
	r.Enqueue(testBundle("succeeds"))
	st := awaitStats(t, r, func(s Stats) bool { return s.Health() == RecorderRecovered }, "recovery")
	if !st.LastSuccessAt.After(st.LastFailureAt) {
		t.Fatal("recovery must be justified by a success later than the failure it clears")
	}
}

// Silence is not health. "No behavioral events occurred" must not read the same
// as "events were accepted and lost".
func TestRecorder_NoAttemptIsNotHealthy(t *testing.T) {
	r := deterministicRecorder(t, func(Bundle) error { return nil })
	if got := r.Stats().Health(); got != RecorderIdle {
		t.Fatalf("health = %q before any delivery, want %q", got, RecorderIdle)
	}
}

// Queue pressure and terminal delivery failure are different operator problems
// and must not collapse into one state.
func TestRecorder_QueueDropAndTerminalFailureStayDistinct(t *testing.T) {
	block := make(chan struct{})
	r := NewRecorder(RecorderOptions{
		QueueSize: 1, Workers: 1, MaxAttempts: 1, CallTimeout: 200 * time.Millisecond,
	})
	r.writeBundle = func(Bundle) error { <-block; return nil }
	t.Cleanup(func() { close(block); _ = r.Close(context.Background()) })

	for i := 0; i < 12; i++ {
		r.Enqueue(testBundle("pressure"))
	}
	st := awaitStats(t, r, func(s Stats) bool { return s.Dropped > 0 }, "queue pressure")
	if st.Failed != 0 {
		t.Fatalf("a dropped bundle was counted as a delivery failure: %+v", st)
	}
	if got := st.Health(); got != RecorderQueuePressure {
		t.Errorf("health = %q, want %q", got, RecorderQueuePressure)
	}
}
