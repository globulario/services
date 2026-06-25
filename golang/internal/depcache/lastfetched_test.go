package depcache

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestCacheLastFetchedAt proves LastFetchedAt reflects the last SUCCESSFUL fetch,
// and crucially does NOT advance when a stale value is served under StaleIfError —
// so a consumer can detect that Get returned stale-served data even though Get's
// error is nil (the OT-3 freshness signal).
func TestCacheLastFetchedAt(t *testing.T) {
	var fetchErr error
	c := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		if fetchErr != nil {
			return "", fetchErr
		}
		return "v", nil
	}, func(s string) string { return s })

	// No entry before the first fetch.
	if _, ok := c.LastFetchedAt("k"); ok {
		t.Fatal("LastFetchedAt should report no entry before the first fetch")
	}

	// A successful Get records the fetch time.
	if _, err := c.Get(context.Background(), "k"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	first, ok := c.LastFetchedAt("k")
	if !ok || first.IsZero() {
		t.Fatal("LastFetchedAt should report the successful fetch time")
	}

	// Backdate past the TTL but inside StaleIfError, then make the source error.
	// Get serves the stale value with a nil error — and LastFetchedAt must stay put.
	c.entries["k"].fetchedAt = time.Now().Add(-(c.policy.TTL + time.Second))
	stale := c.entries["k"].fetchedAt
	fetchErr = errors.New("etcd down")

	if v, err := c.Get(context.Background(), "k"); err != nil || v != "v" {
		t.Fatalf("expected stale-served value with nil error, got v=%q err=%v", v, err)
	}
	after, ok := c.LastFetchedAt("k")
	if !ok || !after.Equal(stale) {
		t.Errorf("LastFetchedAt must reflect the last SUCCESSFUL fetch — a stale-serve must not advance it: want %v got %v", stale, after)
	}
}
