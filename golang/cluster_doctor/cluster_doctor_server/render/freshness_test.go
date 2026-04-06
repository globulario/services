package render

import (
	"testing"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// TestBuildHeaderFreshness locks in the freshness contract added for
// ClusterDoctor read surfaces (see docs/endpoint_resolution_policy.md
// and the FreshnessMode doc-comment in cluster_doctor.proto).
//
// The goal: callers must be able to reason about staleness from the
// ReportHeader alone. Every report MUST carry: source, observed_at,
// snapshot_age_seconds, cache_hit, cache_ttl_seconds, freshness_mode.
// This is a structural / contract test — if any of these drop off
// a report, this test fails.
func TestBuildHeaderFreshness(t *testing.T) {
	snapshotObservedAt := time.Now().Add(-3 * time.Second)
	snap := &collector.Snapshot{
		SnapshotID:  "test-snapshot",
		GeneratedAt: snapshotObservedAt,
	}
	fresh := Freshness{
		CacheHit: true,
		CacheTTL: 30 * time.Second,
		Mode:     cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED,
	}

	h := buildHeader(snap, "v0.0.0-test", fresh)

	if h.GetSource() != ReportSourceName {
		t.Errorf("Source = %q, want %q", h.GetSource(), ReportSourceName)
	}
	if h.GetObservedAt() == nil {
		t.Fatal("ObservedAt must be populated")
	}
	if !h.GetObservedAt().AsTime().Equal(snapshotObservedAt) {
		t.Errorf("ObservedAt = %v, want %v", h.GetObservedAt().AsTime(), snapshotObservedAt)
	}
	// Age is computed from time.Since; allow a small slack window.
	if age := h.GetSnapshotAgeSeconds(); age < 2 || age > 10 {
		t.Errorf("SnapshotAgeSeconds = %d, want roughly 3", age)
	}
	if !h.GetCacheHit() {
		t.Errorf("CacheHit should be true when the snapshot came from cache")
	}
	if h.GetCacheTtlSeconds() != 30 {
		t.Errorf("CacheTtlSeconds = %d, want 30", h.GetCacheTtlSeconds())
	}
	if h.GetFreshnessMode() != cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED {
		t.Errorf("FreshnessMode = %v, want FRESHNESS_CACHED", h.GetFreshnessMode())
	}
}

// TestBuildHeaderFreshnessForceFresh confirms the fresh-path contract:
// when the caller asked for FRESH, cache_hit must be false and the
// mode echoed back must be FRESH (not silently downgraded to CACHED).
func TestBuildHeaderFreshnessForceFresh(t *testing.T) {
	snap := &collector.Snapshot{
		SnapshotID:  "test",
		GeneratedAt: time.Now(),
	}
	fresh := Freshness{
		CacheHit: false,
		CacheTTL: 30 * time.Second,
		Mode:     cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH,
	}

	h := buildHeader(snap, "v0.0.0-test", fresh)

	if h.GetCacheHit() {
		t.Errorf("CacheHit should be false after a forced-fresh fetch")
	}
	if h.GetFreshnessMode() != cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH {
		t.Errorf("FreshnessMode = %v, want FRESHNESS_FRESH (must not be downgraded)", h.GetFreshnessMode())
	}
}

// TestBuildHeaderHandlesZeroTime guards against an internal collector
// regression: if a snapshot is returned with a zero GeneratedAt, age
// must report 0 rather than a giant wall-clock delta.
func TestBuildHeaderHandlesZeroTime(t *testing.T) {
	snap := &collector.Snapshot{SnapshotID: "x"} // GeneratedAt left zero
	h := buildHeader(snap, "v", Freshness{})
	if h.GetSnapshotAgeSeconds() != 0 {
		t.Errorf("SnapshotAgeSeconds for zero GeneratedAt = %d, want 0", h.GetSnapshotAgeSeconds())
	}
}
