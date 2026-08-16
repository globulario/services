package main

// state_persist_observation_test.go
//
// The Phase-36 content-hash dedup (state_persist_dedup_test.go) skipped the Put
// only when the marshalled state was byte-identical. In production it therefore
// almost never fired: every node heartbeat moves last_seen/reported_at, restamps
// the admission proof's checked_at, and nudges disk_free_bytes, so the full
// ~78 KB blob was rewritten roughly 4x/minute with no state change at all.
//
// On 2026-08-16 that was measured at ~30 MB/hour of MVCC history and was the
// largest single contributor to filling etcd's 2 GiB backend quota: NOSPACE on
// 4 of 5 members, the control plane read-only, and no desired-state write
// possible until compact + defrag + disarm. A two-sample diff of the live key
// showed the ONLY differences between consecutive 78 KB writes were those four
// observation fields.
//
// These tests pin the corrected contract: observation drift alone must not
// trigger a write, a real state change must still write immediately, and the
// floor must eventually let drift through so etcd does not go stale forever.

import (
	"testing"
	"time"
)

// nodeWithObservations returns a state carrying one node with all four of the
// volatile observation fields populated.
func nodeWithObservations(t *testing.T) (*controllerState, *nodeState) {
	t.Helper()
	s := freshState()
	n := &nodeState{
		Status:   "ready",
		LastSeen: time.Unix(1_700_000_100, 0).UTC(),
		Capabilities: &storedCapabilities{
			DiskFreeBytes: 500_000_000_000,
		},
		LastAdmissionProof: &AdmissionProofStatus{
			CheckedAt: time.Unix(1_700_000_100, 0).UTC(),
		},
	}
	n.ReportedAt = time.Unix(1_700_000_100, 0).UTC()
	s.Nodes["node-a"] = n
	return s, n
}

// driftObservations simulates one heartbeat: every volatile field moves, no
// state changes.
func driftObservations(n *nodeState, by time.Duration) {
	n.LastSeen = n.LastSeen.Add(by)
	n.ReportedAt = n.ReportedAt.Add(by)
	n.Capabilities.DiskFreeBytes -= 819_200 // the observed per-cycle jitter
	n.LastAdmissionProof.CheckedAt = n.LastAdmissionProof.CheckedAt.Add(by)
}

func TestSaveToEtcd_ObservationDriftAloneDoesNotWrite(t *testing.T) {
	rec := withStubPut(t)
	s, n := nodeWithObservations(t)
	cli := fakeEtcdClient()

	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("initial save calls=%d want 1", rec.calls)
	}

	// Twenty heartbeats' worth of pure observation drift.
	for i := 0; i < 20; i++ {
		driftObservations(n, 30*time.Second)
		if err := s.saveToEtcd(cli); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	if rec.calls != 1 {
		t.Errorf("calls=%d want 1 — observation drift must not rewrite the state blob; "+
			"this is the regression that filled etcd's backend quota", rec.calls)
	}
}

func TestSaveToEtcd_SemanticChangeWritesImmediately(t *testing.T) {
	rec := withStubPut(t)
	s, n := nodeWithObservations(t)
	cli := fakeEtcdClient()

	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// Drift alone: still one write.
	driftObservations(n, 30*time.Second)
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("drift save: %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("after drift calls=%d want 1", rec.calls)
	}

	// A real state transition must not be delayed by the floor.
	n.Status = "draining"
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("semantic save: %v", err)
	}
	if rec.calls != 2 {
		t.Errorf("after status change calls=%d want 2 — a state change must write immediately, "+
			"never wait for the observation floor", rec.calls)
	}
}

// The admission proof's VERDICT is state even though its checked_at stamp is
// not: a proof flipping to failed must write, while a re-check that confirms
// the same verdict must not.
func TestSaveToEtcd_AdmissionProofVerdictIsStateNotObservation(t *testing.T) {
	rec := withStubPut(t)
	s, n := nodeWithObservations(t)
	cli := fakeEtcdClient()

	n.LastAdmissionProof.Reason = "ok"
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("initial calls=%d want 1", rec.calls)
	}

	// Re-checked, same verdict — only checked_at moved.
	n.LastAdmissionProof.CheckedAt = n.LastAdmissionProof.CheckedAt.Add(time.Minute)
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("recheck save: %v", err)
	}
	if rec.calls != 1 {
		t.Errorf("recheck calls=%d want 1 — restamping checked_at is not a state change", rec.calls)
	}

	// Verdict changed — that IS state.
	n.LastAdmissionProof.Reason = "disk below floor"
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("verdict save: %v", err)
	}
	if rec.calls != 2 {
		t.Errorf("verdict-change calls=%d want 2 — a changed proof verdict must persist", rec.calls)
	}
}

func TestSaveToEtcd_ObservationDriftWritesOnceFloorElapses(t *testing.T) {
	rec := withStubPut(t)
	s, n := nodeWithObservations(t)
	cli := fakeEtcdClient()

	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// Backdate the last successful persist past the floor, then drift.
	s.lastPersistedAt = time.Now().Add(-observationPersistFloor - time.Second)
	driftObservations(n, 30*time.Second)
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("post-floor save: %v", err)
	}
	if rec.calls != 2 {
		t.Errorf("calls=%d want 2 — once the floor elapses, drifted observations must reach etcd "+
			"rather than being suppressed forever", rec.calls)
	}
}

// A parse failure must degrade to always-write, never to a false "unchanged"
// that would silently suppress real state from reaching etcd.
func TestSemanticStateHash_UnparseableFallsBackToRawBytes(t *testing.T) {
	a := semanticStateHash([]byte("not json"))
	b := semanticStateHash([]byte("not json"))
	c := semanticStateHash([]byte("also not json"))
	if a != b {
		t.Error("same bytes must hash equal")
	}
	if a == c {
		t.Error("different bytes must hash differently — a fallback that collapses distinct " +
			"states into one hash would suppress writes")
	}
}
