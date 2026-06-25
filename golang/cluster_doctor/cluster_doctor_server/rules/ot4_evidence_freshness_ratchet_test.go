package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ot4ProbeInvariant is a synthetic invariant that always emits one finding whose
// evidence is stamped via kvEvidence (i.e. with timestamppb.Now()). The ratchet
// below runs it through the real EvaluateAll pipeline.
type ot4ProbeInvariant struct{}

func (ot4ProbeInvariant) ID() string       { return "ot4.evidence_freshness_probe" }
func (ot4ProbeInvariant) Category() string { return "test" }
func (ot4ProbeInvariant) Scope() string    { return "cluster" }
func (ot4ProbeInvariant) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	return []Finding{{Evidence: []*cluster_doctorpb.Evidence{kvEvidence("ot4_probe", "probe", nil)}}}
}

// TestEvaluateAll_StampsEvidenceWithCollectionTime is the OT-4 ratchet: it locks in
// the OT-2 freshness fix end-to-end. EvaluateAll must correct every finding's
// evidence timestamp from the rule's Now() to the snapshot's real collection time,
// so the freshness trust gate (findingEvidenceTrust) can detect staleness.
//
// If a future change removes stampEvidenceCollectionTime from the pipeline — or
// adds a new pipeline path that skips it — the probe evidence comes back stamped
// Now() instead of the (old) GeneratedAt and this fails, surfacing the regression
// of meta.binding_outlives_evidence_until_invalidated before it can ship.
func TestEvaluateAll_StampsEvidenceWithCollectionTime(t *testing.T) {
	gen := time.Now().Add(-12 * time.Minute) // snapshot collected 12 min ago (e.g. cache-served)
	snap := &collector.Snapshot{GeneratedAt: gen}

	r := &Registry{invariants: []Invariant{ot4ProbeInvariant{}}}
	out := r.EvaluateAll(snap)

	found := false
	for _, f := range out {
		for _, ev := range f.Evidence {
			if ev.GetSourceService() != "ot4_probe" {
				continue
			}
			found = true
			if got := ev.Timestamp.AsTime().UnixNano(); got != gen.UnixNano() {
				t.Errorf("EvaluateAll must stamp evidence with the snapshot collection time, not Now() — "+
					"the OT-2 freshness correction regressed (stampEvidenceCollectionTime removed/bypassed?). got=%v want=%v",
					ev.Timestamp.AsTime(), gen)
			}
		}
	}
	if !found {
		t.Fatal("ratchet harness broken: probe finding not produced by EvaluateAll")
	}
}
