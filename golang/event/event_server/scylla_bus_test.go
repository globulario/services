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

// TestBucketsFrom verifies the replay window calculation.
func TestBucketsFrom(t *testing.T) {
	// Normal: 2 minutes ago → should produce ~3 buckets (2 ago, 1 ago, now).
	buckets := bucketsFrom(time.Now().Add(-2 * time.Minute))
	if len(buckets) < 2 || len(buckets) > 4 {
		t.Errorf("expected 2-4 buckets for 2min gap, got %d", len(buckets))
	}

	// Large gap: 90 minutes ago → should cap at maxReplayBuckets (60).
	buckets = bucketsFrom(time.Now().Add(-90 * time.Minute))
	if len(buckets) != maxReplayBuckets {
		t.Errorf("expected %d buckets for 90min gap, got %d", maxReplayBuckets, len(buckets))
	}

	// Future: should return just current bucket.
	buckets = bucketsFrom(time.Now().Add(5 * time.Minute))
	if len(buckets) != 1 {
		t.Errorf("expected 1 bucket for future time, got %d", len(buckets))
	}

	// Zero/empty: time.Time{} → should cap at maxReplayBuckets.
	buckets = bucketsFrom(time.Time{})
	if len(buckets) > maxReplayBuckets {
		t.Errorf("expected <= %d buckets for zero time, got %d", maxReplayBuckets, len(buckets))
	}
}

// TestPollDoesNotAdvancePastCurrentBucket is a regression test for the cursor
// race condition where pollOnce() would advance the cursor past the current
// minute bucket when it found no events. Events written to the current bucket
// after the poll would then be permanently invisible (cursor already past them).
//
// The invariant: empty past bucket → safe to skip; empty current bucket → do not skip.
func TestPollDoesNotAdvancePastCurrentBucket(t *testing.T) {
	bus := newScyllaBus(logger)
	if err := bus.connect(); err != nil {
		t.Skipf("ScyllaDB unavailable, skipping: %v", err)
	}
	defer bus.close()

	// Set cursor to the start of the current minute.
	now := time.Now().UTC()
	currentMinute := now.Truncate(time.Minute)
	bus.lastSeq = gocql.MinTimeUUID(currentMinute)

	// Poll with no events in the current bucket.
	events := bus.pollOnce()
	t.Logf("first poll: %d events, cursor=%v", len(events), bus.lastSeq.Time())

	// Skip if the cluster has very high event volume: the poll hit the 500-event limit
	// within the current minute, which means finding the test event is unreliable.
	if len(events) >= 500 {
		t.Skipf("skipping: live cluster event volume too high (%d events in current minute), test unreliable", len(events))
	}

	// The cursor must NOT have advanced past the current minute.
	// If the cursor jumped to currentMinute+1m, the bug is present.
	if bus.lastSeq.Time().After(currentMinute.Add(time.Minute)) {
		t.Fatalf("cursor advanced past current bucket: cursor=%v, current_minute=%v",
			bus.lastSeq.Time(), currentMinute)
	}

	// Now write an event into the current bucket.
	if err := bus.publish("regression.cursor_race", []byte(`{"test":"cursor_race"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Poll again — the event must be visible.
	events = bus.pollOnce()
	found := false
	for _, ev := range events {
		if ev.name == "regression.cursor_race" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("event written to current bucket was not returned by pollOnce (cursor race regression)")
	}
	t.Logf("regression test passed: event in current bucket was visible after empty poll")
}

// TestPollAdvancesPastOldEmptyBuckets verifies that the catch-up optimization
// still works for buckets that are fully in the past.
func TestPollAdvancesPastOldEmptyBuckets(t *testing.T) {
	bus := newScyllaBus(logger)
	if err := bus.connect(); err != nil {
		t.Skipf("ScyllaDB unavailable, skipping: %v", err)
	}
	defer bus.close()

	// Set cursor to 2 hours ago. Those minute-buckets are well past the event window.
	// Using 2h (not 5m) so we skip past buckets that may have real cluster events.
	twoHoursAgo := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Minute)
	bus.lastSeq = gocql.MinTimeUUID(twoHoursAgo)

	// Poll — should advance past the old empty buckets.
	bus.pollOnce()

	// Cursor should have advanced at all (catch-up optimization is working).
	// We can't assert a specific distance because a live cluster may have events
	// at any timestamp, causing the cursor to stop after hitting the event limit.
	if !bus.lastSeq.Time().After(twoHoursAgo) {
		t.Errorf("cursor did not advance past old start: cursor=%v, start=%v",
			bus.lastSeq.Time(), twoHoursAgo)
	}
	t.Logf("catch-up works: cursor advanced from %v to %v", twoHoursAgo, bus.lastSeq.Time())
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
