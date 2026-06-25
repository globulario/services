package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestStampEvidenceCollectionTime proves the OT-2 fix: evidence the rules stamped
// with kvEvidence(Now()) is corrected to the snapshot's real collection time so the
// freshness trust gate can detect staleness — with prometheus evidence dated by the
// (older) scrape time, and a fail-safe that never moves a timestamp forward.
func TestStampEvidenceCollectionTime(t *testing.T) {
	now := time.Now()
	gen := now.Add(-10 * time.Minute)  // snapshot collected 10 min ago (e.g. cache-served)
	prom := now.Add(-15 * time.Minute) // prometheus scrape even older
	older := gen.Add(-5 * time.Minute) // evidence that already carries an older real time

	mkEv := func(service string, ts time.Time) *cluster_doctorpb.Evidence {
		return &cluster_doctorpb.Evidence{SourceService: service, Timestamp: timestamppb.New(ts)}
	}

	snap := &collector.Snapshot{GeneratedAt: gen, PromTS: prom}
	findings := []Finding{{Evidence: []*cluster_doctorpb.Evidence{
		mkEv("cluster_controller", now), // kvEvidence(Now()) → corrected back to GeneratedAt
		mkEv("prometheus", now),         // prometheus → corrected to PromTS (older than GeneratedAt)
		mkEv("etcd", older),             // already older than GeneratedAt → must NOT move forward
	}}}

	out := stampEvidenceCollectionTime(findings, snap)
	ev := out[0].Evidence

	if got := ev[0].Timestamp.AsTime().UnixNano(); got != gen.UnixNano() {
		t.Errorf("controller evidence: want GeneratedAt, got %v", ev[0].Timestamp.AsTime())
	}
	if got := ev[1].Timestamp.AsTime().UnixNano(); got != prom.UnixNano() {
		t.Errorf("prometheus evidence: want PromTS (scrape time), got %v", ev[1].Timestamp.AsTime())
	}
	if got := ev[2].Timestamp.AsTime().UnixNano(); got != older.UnixNano() {
		t.Errorf("already-older evidence must be left untouched (fail-safe), got %v", ev[2].Timestamp.AsTime())
	}
}

// TestStampEvidenceCollectionTime_NilSnapshotIsNoop guards the degenerate inputs.
func TestStampEvidenceCollectionTime_NilSnapshotIsNoop(t *testing.T) {
	f := []Finding{{Evidence: []*cluster_doctorpb.Evidence{{SourceService: "x", Timestamp: timestamppb.Now()}}}}
	if out := stampEvidenceCollectionTime(f, nil); len(out) != 1 {
		t.Fatal("nil snapshot should be a no-op")
	}
	// Zero GeneratedAt is also a no-op (nothing to correct toward).
	if out := stampEvidenceCollectionTime(f, &collector.Snapshot{}); len(out) != 1 {
		t.Fatal("zero GeneratedAt should be a no-op")
	}
}
