package main

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/preflight"
)

// newTestGraph opens a temporary in-memory graph for testing.
func newLiveSnapshotTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.Open(":memory:")
	if err != nil {
		t.Fatalf("open test graph: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

// TestLiveSnapshot_IncludesCollectorHealth verifies that a live snapshot record
// stores collector health items that are retrievable via LatestLiveSnapshotRecord.
func TestLiveSnapshot_IncludesCollectorHealth(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	items := []graph.CollectorHealthItem{
		{CollectorID: "systemd", SourceTier: "systemd_runtime", Status: "skipped", Priority: "P0"},
		{CollectorID: "pki", SourceTier: "cluster_security", Status: "skipped", Priority: "P1"},
	}

	if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, "/repo", "", "", graph.BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord: %v", err)
	}
	if err := g.SetBuildCollectorHealth(ctx, graph.LiveSnapshotBuildID, items); err != nil {
		t.Fatalf("SetBuildCollectorHealth: %v", err)
	}

	rec, err := g.LatestLiveSnapshotRecord(ctx)
	if err != nil {
		t.Fatalf("LatestLiveSnapshotRecord: %v", err)
	}
	if rec == nil {
		t.Fatal("expected live snapshot record, got nil")
	}
	if len(rec.CollectorHealth) != 2 {
		t.Fatalf("expected 2 collector health items, got %d", len(rec.CollectorHealth))
	}
	if rec.CollectorHealth[0].CollectorID != "systemd" {
		t.Errorf("expected first collector 'systemd', got %q", rec.CollectorHealth[0].CollectorID)
	}
}

// TestLiveSnapshot_NoStaticGraphRebuildRequired verifies that a static build
// record is not affected when a live snapshot record is written.
// LatestBuildRecord must exclude the live-snapshot record.
func TestLiveSnapshot_NoStaticGraphRebuildRequired(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	// Write static build record first.
	if err := g.UpsertBuildRecord(ctx, "build-2026-001", "/repo", "abc123", "", graph.BuildStats{Nodes: 42}); err != nil {
		t.Fatalf("UpsertBuildRecord static: %v", err)
	}

	// Write live snapshot AFTER (newer timestamp).
	if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, "/repo", "", "", graph.BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord live: %v", err)
	}

	// LatestBuildRecord must return the static build, not the live snapshot.
	rec, err := g.LatestBuildRecord(ctx)
	if err != nil {
		t.Fatalf("LatestBuildRecord: %v", err)
	}
	if rec == nil {
		t.Fatal("expected static build record, got nil")
	}
	if rec.ID == graph.LiveSnapshotBuildID {
		t.Error("LatestBuildRecord returned live-snapshot record — must exclude it")
	}
	if rec.Stats.Nodes != 42 {
		t.Errorf("expected static build stats (nodes=42), got nodes=%d", rec.Stats.Nodes)
	}
}

// TestBuild_IncludesLiveOverlay verifies that after a live-snapshot run,
// LatestLiveSnapshotRecord returns the snapshot data independently of the static build.
func TestBuild_IncludesLiveOverlay(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	// Simulate a static build.
	if err := g.UpsertBuildRecord(ctx, "build-2026-002", "/repo", "def456", "", graph.BuildStats{Nodes: 100}); err != nil {
		t.Fatalf("UpsertBuildRecord static: %v", err)
	}

	// Initially no live snapshot.
	rec, err := g.LatestLiveSnapshotRecord(ctx)
	if err != nil {
		t.Fatalf("LatestLiveSnapshotRecord (before): %v", err)
	}
	if rec != nil {
		t.Error("expected nil live snapshot before first run, got non-nil")
	}

	// Simulate a live-snapshot run.
	items := []graph.CollectorHealthItem{
		{CollectorID: "workflow_execution", SourceTier: "live_runtime", Status: "ok", NodesEmitted: 5, Priority: "P1"},
	}
	if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, "/repo", "", "", graph.BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord live: %v", err)
	}
	if err := g.SetBuildCollectorHealth(ctx, graph.LiveSnapshotBuildID, items); err != nil {
		t.Fatalf("SetBuildCollectorHealth: %v", err)
	}

	// Now both records are retrievable independently.
	liveRec, err := g.LatestLiveSnapshotRecord(ctx)
	if err != nil || liveRec == nil {
		t.Fatalf("LatestLiveSnapshotRecord (after): %v, rec=%v", err, liveRec)
	}
	if len(liveRec.CollectorHealth) != 1 || liveRec.CollectorHealth[0].CollectorID != "workflow_execution" {
		t.Errorf("unexpected live collector health: %+v", liveRec.CollectorHealth)
	}

	staticRec, err := g.LatestBuildRecord(ctx)
	if err != nil || staticRec == nil {
		t.Fatalf("LatestBuildRecord: %v", err)
	}
	if staticRec.Stats.Nodes != 100 {
		t.Errorf("static build stats corrupted: nodes=%d", staticRec.Stats.Nodes)
	}
}

// TestPreflight_LiveOverlayFresh verifies that a recent live snapshot is reported
// as fresh in the preflight report.
func TestPreflight_LiveOverlayFresh(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	// Write a live snapshot just now.
	if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, "/repo", "", "", graph.BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord: %v", err)
	}

	now := time.Now()
	lof := preflight.ComputeLiveOverlayFreshness(ctx, g, now)
	if lof == nil {
		t.Fatal("expected non-nil LiveOverlayFreshness")
	}
	if lof.Status != "fresh" {
		t.Errorf("expected status=fresh, got %q (age=%.0fs)", lof.Status, lof.AgeSeconds)
	}
}

// TestPreflight_LiveOverlayStaleLowersConfidence verifies that an old live snapshot
// is reported as stale, which should lower confidence.
func TestPreflight_LiveOverlayStaleLowersConfidence(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	// Write a live snapshot record, but check it with a "now" that is 20 minutes later.
	if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, "/repo", "", "", graph.BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord: %v", err)
	}

	// Simulate time advancing 20 minutes past the snapshot.
	futureNow := time.Now().Add(20 * time.Minute)
	lof := preflight.ComputeLiveOverlayFreshness(ctx, g, futureNow)
	if lof == nil {
		t.Fatal("expected non-nil LiveOverlayFreshness")
	}
	if lof.Status == "fresh" {
		t.Errorf("expected stale/absent status after 20 minutes, got %q", lof.Status)
	}
}

// TestLiveSnapshot_AbsentWhenNeverRun verifies that a graph with no live snapshot
// returns status "absent" from computeLiveOverlayFreshness.
func TestLiveSnapshot_AbsentWhenNeverRun(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	lof := preflight.ComputeLiveOverlayFreshness(ctx, g, time.Now())
	if lof == nil {
		t.Fatal("expected non-nil LiveOverlayFreshness")
	}
	if lof.Status != "absent" {
		t.Errorf("expected status=absent when no snapshot exists, got %q", lof.Status)
	}
}

// TestHealthPulse_RefreshLiveReportsFailures verifies that when live collectors
// fail, the snapshot records partial/failed status and not "fresh".
func TestHealthPulse_RefreshLiveReportsFailures(t *testing.T) {
	ctx := context.Background()
	g := newLiveSnapshotTestGraph(t)

	// Simulate all collectors failing.
	failedItems := []graph.CollectorHealthItem{
		{CollectorID: "systemd", Status: "error", Error: "connection refused", Priority: "P0"},
		{CollectorID: "pki", Status: "error", Error: "no such file or directory", Priority: "P1"},
	}
	if err := g.UpsertBuildRecord(ctx, graph.LiveSnapshotBuildID, "/repo", "", "", graph.BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord: %v", err)
	}
	if err := g.SetBuildCollectorHealth(ctx, graph.LiveSnapshotBuildID, failedItems); err != nil {
		t.Fatalf("SetBuildCollectorHealth: %v", err)
	}

	lof := preflight.ComputeLiveOverlayFreshness(ctx, g, time.Now())
	if lof == nil {
		t.Fatal("expected non-nil LiveOverlayFreshness")
	}
	if lof.Status == "fresh" {
		t.Error("expected non-fresh status when all collectors failed, got 'fresh'")
	}
	if len(lof.Collectors) != 2 {
		t.Errorf("expected 2 collector summaries, got %d", len(lof.Collectors))
	}
}

// TestLiveOverlayStatus_StatusFromCollectors verifies the status computation logic.
func TestLiveOverlayStatus_StatusFromCollectors(t *testing.T) {
	tests := []struct {
		name       string
		collectors []LiveCollectorSummary
		want       string
	}{
		{"empty", nil, "absent"},
		{"all ok", []LiveCollectorSummary{{Status: "ok"}, {Status: "skipped"}}, "fresh"},
		{"all failed", []LiveCollectorSummary{{Status: "error"}, {Status: "failed"}}, "failed"},
		{"partial", []LiveCollectorSummary{{Status: "ok"}, {Status: "error"}}, "partial"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := liveOverlayStatusFromCollectors(tc.collectors)
			if got != tc.want {
				t.Errorf("status=%q, want %q", got, tc.want)
			}
		})
	}
}
