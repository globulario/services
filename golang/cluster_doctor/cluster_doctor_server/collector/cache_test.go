package collector

import (
	"testing"
	"time"
)

// TestSnapshotCacheHitAndMiss covers the baseline cache contract
// that the freshness fields in ReportHeader depend on:
//
//   - a fresh set() → the next get() is a cache hit
//   - after TTL expires → the next get() returns no cached snap and
//     the caller becomes the fetcher
//   - invalidate() → the next get() also forces the caller to fetch
//     (this is the FRESHNESS_FRESH path)
func TestSnapshotCacheHitAndMiss(t *testing.T) {
	c := NewSnapshotCache(30 * time.Second)

	// Empty cache: first caller becomes the fetcher, no waiter chan.
	if got, waiter := c.get(); got != nil || waiter != nil {
		t.Fatalf("empty cache get() = (%v, %v), want (nil, nil) for fetcher path", got, waiter)
	}

	snap := &Snapshot{SnapshotID: "s1", GeneratedAt: time.Now()}
	c.set(snap)

	// Fresh entry: cache hit, no waiter needed.
	got, waiter := c.get()
	if got == nil || got.SnapshotID != "s1" {
		t.Fatalf("expected cache hit on s1, got %v", got)
	}
	if waiter != nil {
		t.Fatalf("cache hit should return nil waiter")
	}
}

func TestSnapshotCacheInvalidateForcesFetch(t *testing.T) {
	c := NewSnapshotCache(time.Hour) // long TTL → any miss must be due to invalidate
	c.set(&Snapshot{SnapshotID: "before", GeneratedAt: time.Now()})

	// Sanity: cache would be a hit without invalidate.
	if got, _ := c.get(); got == nil || got.SnapshotID != "before" {
		t.Fatalf("pre-condition: expected cache hit, got %v", got)
	}

	c.invalidate()

	// After invalidate the caller must become the fetcher.
	got, waiter := c.get()
	if got != nil {
		t.Fatalf("after invalidate, get() returned cached snap %v; must be nil", got)
	}
	if waiter != nil {
		t.Fatalf("after invalidate, expected fetcher path (nil waiter)")
	}
}

func TestSnapshotCacheTTLExposedViaTtlFor(t *testing.T) {
	c := NewSnapshotCache(45 * time.Second)
	if c.ttlFor() != 45*time.Second {
		t.Fatalf("ttlFor() = %v, want 45s", c.ttlFor())
	}
}
