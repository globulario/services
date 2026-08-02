package observation

import (
	"context"
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
func TestRecorder_PersistentFailureIsVisible(t *testing.T) {
	r := NewRecorder(RecorderOptions{
		QueueSize:   2,
		Workers:     1,
		MaxAttempts: 1,
		Addr:        "", // unresolvable in test env → conn() errors
		CallTimeout: 200 * time.Millisecond,
	})
	defer func() { _ = r.Close(context.Background()) }()

	r.Enqueue(testBundle("will-fail"))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st := r.Stats(); st.Failed > 0 {
			if st.LastError == "" {
				t.Error("failure must record LastError")
			}
			if !st.LastFailureAt.After(time.Time{}) {
				t.Error("failure must stamp LastFailureAt")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("persistent delivery failure never surfaced in Stats — degraded learning would be invisible")
}
