package main

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
)

// TestScyllaBusPublishAndPoll verifies that events published to ScyllaDB
// are visible via pollOnce — the core of cross-instance event delivery.
func TestScyllaBusPublishAndPoll(t *testing.T) {
	bus := newScyllaBus(logger)
	if err := bus.connect(); err != nil {
		t.Skipf("ScyllaDB unavailable, skipping: %v", err)
	}
	defer bus.close()

	// Publish a few events.
	if err := bus.publish("test.hello", []byte(`{"msg":"hi"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := bus.publish("test.world", []byte(`{"msg":"world"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Reset lastSeq to just before now so we see the events we just published.
	// (The bus seeds lastSeq to "now" on connect, so we may have raced.)
	bus.lastSeq = gocql.MinTimeUUID(time.Now().Add(-2 * time.Second))

	events := bus.pollOnce()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	found := map[string]bool{}
	for _, ev := range events {
		found[ev.name] = true
		t.Logf("polled: name=%s data=%s", ev.name, string(ev.data))
	}
	if !found["test.hello"] {
		t.Error("missing test.hello event")
	}
	if !found["test.world"] {
		t.Error("missing test.world event")
	}
}

// TestScyllaBusPollDeduplication verifies that polling twice doesn't return
// the same events (lastSeq advances).
func TestScyllaBusPollDeduplication(t *testing.T) {
	bus := newScyllaBus(logger)
	if err := bus.connect(); err != nil {
		t.Skipf("ScyllaDB unavailable, skipping: %v", err)
	}
	defer bus.close()

	bus.lastSeq = gocql.MinTimeUUID(time.Now().Add(-2 * time.Second))

	if err := bus.publish("dedup.first", []byte("1")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	events1 := bus.pollOnce()
	if len(events1) == 0 {
		t.Fatal("expected events on first poll")
	}
	t.Logf("first poll: %d events", len(events1))

	// Second poll without new publishes should return nothing.
	events2 := bus.pollOnce()
	for _, ev := range events2 {
		if ev.name == "dedup.first" {
			t.Error("dedup.first should not appear in second poll")
		}
	}
	t.Logf("second poll: %d events (should be 0 for dedup.first)", len(events2))
}

// TestScyllaBusQueryEvents verifies the QueryEvents path.
func TestScyllaBusQueryEvents(t *testing.T) {
	bus := newScyllaBus(logger)
	if err := bus.connect(); err != nil {
		t.Skipf("ScyllaDB unavailable, skipping: %v", err)
	}
	defer bus.close()

	// Publish some events.
	for i := 0; i < 5; i++ {
		if err := bus.publish("query.test", []byte("data")); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	afterSeq := gocql.MinTimeUUID(time.Now().Add(-1 * time.Minute))
	events, latestSeq := bus.queryEvents("query.", afterSeq, 100)
	if len(events) < 5 {
		t.Errorf("expected at least 5 events, got %d", len(events))
	}
	if latestSeq.Time().Before(afterSeq.Time()) {
		t.Error("latestSeq should be after afterSeq")
	}
	t.Logf("queryEvents: %d events, latestSeq=%v", len(events), latestSeq.Time())

	// Filter by name prefix.
	events2, _ := bus.queryEvents("nonexistent.", afterSeq, 100)
	if len(events2) != 0 {
		t.Errorf("expected 0 events for nonexistent prefix, got %d", len(events2))
	}
}

// TestMatchesChannel verifies wildcard pattern matching.
func TestMatchesChannel(t *testing.T) {
	cases := []struct {
		pattern, event string
		want           bool
	}{
		{"cluster.health", "cluster.health", true},
		{"cluster.*", "cluster.health", true},
		{"cluster.*", "cluster.drift", true},
		{"cluster.*", "alert.auth", false},
		{"*", "anything", true},
		{"exact", "different", false},
	}
	for _, tc := range cases {
		got := matchesChannel(tc.pattern, tc.event)
		if got != tc.want {
			t.Errorf("matchesChannel(%q, %q) = %v, want %v", tc.pattern, tc.event, got, tc.want)
		}
	}
}
