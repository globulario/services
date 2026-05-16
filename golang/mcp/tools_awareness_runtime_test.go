package main

import (
	"testing"
	"time"

	"github.com/globulario/awareness/runtime"
)

// TestAwarenessSnapshotToMap_SourceHealthExposed verifies that source_health,
// confidence, coverage, and blind_spots are present in the map output so callers
// can distinguish noop snapshots from genuinely empty healthy ones.
func TestAwarenessSnapshotToMap_SourceHealthExposed(t *testing.T) {
	snap := &runtime.RuntimeSnapshot{
		ID:         "test-snap",
		CapturedAt: time.Now(),
		NodeID:     "node-1",
		SourceHealth: []runtime.SourceHealth{
			{Source: runtime.SourceDoctor, Backend: "cluster_doctor.grpc", Healthy: true, CollectedAt: time.Now().Format(time.RFC3339)},
			{Source: runtime.SourceWorkflows, Backend: "noop", Healthy: false, EmptyDueToNoop: true, CollectedAt: time.Now().Format(time.RFC3339)},
		},
	}

	m := awarenessSnapshotToMap(snap)

	// source_health must be present and have entries.
	sh, ok := m["source_health"]
	if !ok {
		t.Fatal("source_health missing from awarenessSnapshotToMap output")
	}
	shList, ok := sh.([]map[string]interface{})
	if !ok {
		t.Fatalf("source_health is %T, want []map[string]interface{}", sh)
	}
	if len(shList) != 2 {
		t.Errorf("expected 2 source_health entries, got %d", len(shList))
	}

	// confidence must be present.
	conf, ok := m["confidence"]
	if !ok {
		t.Fatal("confidence missing from awarenessSnapshotToMap output")
	}
	if conf == "" {
		t.Error("confidence must not be empty string")
	}

	// coverage must list healthy sources.
	cov, ok := m["coverage"]
	if !ok {
		t.Fatal("coverage missing from awarenessSnapshotToMap output")
	}
	covList, ok := cov.([]string)
	if !ok {
		t.Fatalf("coverage is %T, want []string", cov)
	}
	if len(covList) != 1 || covList[0] != "doctor" {
		t.Errorf("expected coverage=[doctor], got %v", covList)
	}

	// blind_spots must list noop sources.
	bs, ok := m["blind_spots"]
	if !ok {
		t.Fatal("blind_spots missing from awarenessSnapshotToMap output")
	}
	bsList, ok := bs.([]string)
	if !ok {
		t.Fatalf("blind_spots is %T, want []string", bs)
	}
	if len(bsList) != 1 || bsList[0] != "workflows" {
		t.Errorf("expected blind_spots=[workflows], got %v", bsList)
	}
}

// TestAwarenessSnapshotToMap_AllNoopIsNoopConfidence verifies the confidence
// classification when every source is noop.
func TestAwarenessSnapshotToMap_AllNoopIsNoopConfidence(t *testing.T) {
	snap := &runtime.RuntimeSnapshot{
		ID:         "noop-snap",
		CapturedAt: time.Now(),
		SourceHealth: []runtime.SourceHealth{
			{Source: runtime.SourceDoctor, Backend: "noop", EmptyDueToNoop: true, CollectedAt: time.Now().Format(time.RFC3339)},
			{Source: runtime.SourceWorkflows, Backend: "noop", EmptyDueToNoop: true, CollectedAt: time.Now().Format(time.RFC3339)},
		},
	}

	m := awarenessSnapshotToMap(snap)
	conf, _ := m["confidence"].(string)
	if conf != "noop" {
		t.Errorf("expected noop confidence when all sources are noop, got %q", conf)
	}
}

// TestAwarenessSnapshotToMap_EmptySourceHealthIsUnknown verifies the confidence
// classification when no source health records are present.
func TestAwarenessSnapshotToMap_EmptySourceHealthIsUnknown(t *testing.T) {
	snap := &runtime.RuntimeSnapshot{ID: "empty", CapturedAt: time.Now()}
	m := awarenessSnapshotToMap(snap)
	conf, _ := m["confidence"].(string)
	if conf != "unknown" {
		t.Errorf("expected unknown confidence with no source health, got %q", conf)
	}
}
