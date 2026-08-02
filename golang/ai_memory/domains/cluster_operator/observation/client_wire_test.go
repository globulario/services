package observation

// The client half of the gRPC conversion.
//
// The proto Evidence message had no action_ref until 4D, so the production
// recorder path would have dropped the binding 4C made first-class — the same
// defect class as holding it in Metadata, one layer up, and equally invisible:
// a dropped binding is indistinguishable from one that was never set, and the
// receiving end persists evidence that no action-bound rule can ever qualify.
//
// Its counterpart (pbToEvidence, the server half) is proven in
// ai_memory_server/behavioral_production_lineage_test.go.

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/remediation"
)

func TestEvidenceToPB_PreservesActionBinding(t *testing.T) {
	dispatchedAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	verifiedAt := dispatchedAt.Add(90 * time.Second)

	// Build from the REAL adapter rather than a hand-made Evidence, so the test
	// carries whatever the producer actually emits.
	b, qualifies := FromRemediationOutcome("p", api.DomainRef("cluster_operator"), remediation.Outcome{
		FindingID: "finding-1", WorkflowRunID: "run-abc",
		ClusterID: "cluster-a", InvariantID: "cluster.desired_state.absent",
		EntityRef: "svc/repository", NodeID: "node-4",
		Dispatched: true, Verified: true, FindingResolved: true,
		DispatchedAt: dispatchedAt, VerifiedAt: verifiedAt,
	})
	if !qualifies || len(b.Evidence) != 1 {
		t.Fatalf("fixture must be a qualifying single row: qualifies=%t rows=%d", qualifies, len(b.Evidence))
	}
	BindRemediationEvidence(&b)
	ev := b.Evidence[0]
	pb := evidenceToPB(ev)

	// Every field the satisfaction rule depends on must cross the wire.
	for _, c := range []struct{ name, got, want string }{
		{"action_ref", pb.GetActionRef(), ev.ActionRef},
		{"source_ref", pb.GetSourceRef(), ev.SourceRef},
		{"entity_ref", pb.GetEntityRef(), ev.EntityRef},
		{"cluster_id", pb.GetClusterId(), ev.ClusterID},
		{"condition_ref", pb.GetConditionRef(), ev.ConditionRef},
		{"evidence_kind", pb.GetEvidenceKind(), ev.Kind},
		{"source_kind", pb.GetSourceKind(), ev.SourceKind},
		{"result", pb.GetResult(), ev.Result},
	} {
		if c.got != c.want {
			t.Errorf("%s lost across the wire: got %q want %q", c.name, c.got, c.want)
		}
	}
	if pb.GetActionRef() == "" {
		t.Fatal("action_ref is empty on the wire — the binding would not survive the recorder path")
	}
	if pb.GetObservedAt() != ev.ObservedAt {
		t.Errorf("observed_at lost: got %d want %d", pb.GetObservedAt(), ev.ObservedAt)
	}
	if len(pb.GetSatisfies()) != 1 || pb.GetSatisfies()[0] != string(ev.Satisfies[0]) {
		t.Errorf("satisfies lost: %v", pb.GetSatisfies())
	}
}
