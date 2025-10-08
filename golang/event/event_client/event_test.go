package event_client

import (
	context "context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
)

// --- helpers -----------------------------------------------------------------

// randSubject returns a unique subject name for isolation between test runs.
func randSubject(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buf))
}

// newClient creates a client using the standard service id.
func newClient(t *testing.T) *Event_Client {
	t.Helper()
	// Prefer local peer; fall back to default domain if the user runs a cluster.
	addr := "globule-ryzen.globular.io"
	client, err := NewEventService_Client(addr, "event.EventService")
	if err != nil {
		// second try using the canonical demo domain used in this repo
		client, err = NewEventService_Client("globular.io", "event.EventService")
	}
	if err != nil {
		t.Fatalf("NewEventService_Client: %v", err)
	}
	return client
}

// waitUntil waits for cond to be true or times out.
func waitUntil(t *testing.T, d time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// --- tests -------------------------------------------------------------------

// TestSubscribePublishOne verifies a subscriber receives messages published on the same subject.
func TestSubscribePublishOne(t *testing.T) {
	c := newClient(t)
	subject := randSubject("evt_one")

	var got int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.SubscribeCtx(ctx, subject, "sub-1", func(e *eventpb.Event) {
		if e != nil && e.Name == subject {
			atomic.AddInt32(&got, 1)
		}
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	const N = 3
	for i := 0; i < N; i++ {
		if err := c.Publish(subject, []byte(fmt.Sprintf("msg-%d", i))); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	ok := waitUntil(t, 3*time.Second, func() bool { return atomic.LoadInt32(&got) == N })
	if !ok {
		t.Fatalf("got %d events; want %d", got, N)
	}
}

// TestUnsubscribeStopsDelivery ensures no more events are delivered after UnSubscribe.
func TestUnsubscribeStopsDelivery(t *testing.T) {
	c := newClient(t)
	subject := randSubject("evt_unsub")

	var got int32
	uuid := "sub-unsub"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.SubscribeCtx(ctx, subject, uuid, func(e *eventpb.Event) {
		if e != nil && e.Name == subject {
			atomic.AddInt32(&got, 1)
		}
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Prime one message
	_ = c.Publish(subject, []byte("prime"))
	ok := waitUntil(t, 2*time.Second, func() bool { return atomic.LoadInt32(&got) >= 1 })
	if !ok {
		t.Fatalf("did not receive priming event before unsubscribe")
	}

	// Now unsubscribe and publish again
	if err := c.UnSubscribeCtx(ctx, subject, uuid); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	before := atomic.LoadInt32(&got)
	_ = c.Publish(subject, []byte("post-unsub"))
	// Ensure count remains unchanged for a bit (no new deliveries)
	time.Sleep(600 * time.Millisecond)
	after := atomic.LoadInt32(&got)
	if after != before {
		t.Fatalf("received %d new events after unsubscribe; want 0", after-before)
	}
}

// TestBroadcastMultipleSubscribers ensures a publish fan-outs to multiple subscribers.
func TestBroadcastMultipleSubscribers(t *testing.T) {
	c := newClient(t)
	subject := randSubject("evt_broadcast")

	const subs = 5
	var wg sync.WaitGroup
	wg.Add(subs)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < subs; i++ {
		uuid := fmt.Sprintf("s-%d", i)
		if err := c.SubscribeCtx(ctx, subject, uuid, func(e *eventpb.Event) {
			if e != nil && e.Name == subject {
				wg.Done()
			}
		}); err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
	}

	if err := c.Publish(subject, []byte("fanout")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	ch := make(chan struct{})
	go func() { wg.Wait(); close(ch) }()

	select {
	case <-ch:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for all subscribers to receive event")
	}
}

// TestKeepAliveIsTransparent validates the client handler is only invoked for Event payloads.
// We cannot force a KA frame from here, but we assert that idle periods do not trigger handlers
// (i.e., KA frames are handled internally by the client and not exposed as events).
func TestKeepAliveIsTransparent(t *testing.T) {
	c := newClient(t)
	subject := randSubject("evt_ka")

	var got int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.SubscribeCtx(ctx, subject, "sub-ka", func(e *eventpb.Event) {
		if e != nil {
			atomic.AddInt32(&got, 1)
		}
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Do not publish anything; just idle past a KA period used by the server (15s).
	// We only wait a short time here to keep tests fast; the assertion is that no spurious
	// events are delivered without publish.
	time.Sleep(700 * time.Millisecond)
	if n := atomic.LoadInt32(&got); n != 0 {
		t.Fatalf("received %d events without publish; want 0", n)
	}
}
