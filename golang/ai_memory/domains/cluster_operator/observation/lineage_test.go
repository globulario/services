package observation_test

// lineage_test.go proves the production path end to end using the REAL producer.
//
// It deliberately builds no substitute evidence fixture. Commit 4A's catalog was
// authored from an imagined producer shape and validated by fixtures built from
// the same imagination — catalog and fixtures agreed with each other while
// neither agreed with runtime. Every assertion here starts from
// observation.FromDoctorFinding's actual output, so reality is the input rather
// than something reconstructed from memory.
//
// This file lives in an external test package because observation imports
// cluster_operator for the catalog mapper; an in-package test importing
// cluster_operator back would cycle.

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	lProject = "globular-services"
	lDomain  = "cluster_operator"
	lCluster = "cluster-a"
	lReq     = "evidence.doctor.finding_observed"
	lEntity  = "node-4"
	lInvar   = "cluster.desired_state.absent"
)

// Fixed clock. The catalog rule allows 15m, so observation sits inside it.
var (
	lEvalAt    = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	lObserveAt = lEvalAt.Add(-2 * time.Minute)
)

// realBundle invokes the ACTUAL producer. No hand-built api.Evidence anywhere.
func realBundle(t *testing.T) observation.Bundle {
	t.Helper()
	finding := &cluster_doctorpb.Finding{
		FindingId:   "finding-1",
		InvariantId: lInvar,
		EntityRef:   lEntity,
		Category:    "drift",
		Summary:     "no desired state for any registered node",
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "cluster_controller",
			SourceRpc:     "GetClusterHealthV1",
			Timestamp:     timestamppb.New(lObserveAt),
		}},
	}
	header := &cluster_doctorpb.ReportHeader{
		Source:     "cluster-doctor",
		SnapshotId: "snap-1",
		ObservedAt: timestamppb.New(lObserveAt),
	}
	b := observation.FromDoctorFinding(lProject, api.DomainRef(lDomain), lCluster, header, finding)
	if len(b.Evidence) != 1 {
		t.Fatalf("producer emitted %d evidence rows, want 1 — the test's finding shape is wrong, "+
			"not the producer", len(b.Evidence))
	}
	return b
}

func lQuery(mut ...func(*cluster_operator.SatisfactionQuery)) cluster_operator.SatisfactionQuery {
	q := cluster_operator.SatisfactionQuery{
		Project:       lProject,
		Domain:        api.DomainRef(lDomain),
		ClusterID:     lCluster,
		RequirementID: lReq,
		EntityRef:     lEntity,
		ConditionRef:  lInvar,
		EvaluatedAt:   lEvalAt,
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

func seed(t *testing.T, evs ...api.Evidence) store.Store {
	t.Helper()
	m := store.NewMemoryStore()
	for i := range evs {
		if err := m.PutEvidence(context.Background(), &evs[i]); err != nil {
			t.Fatalf("PutEvidence: %v", err)
		}
	}
	return m
}

// ── 1. the real producer carries the expected binding ───────────────────────

func TestLineage_RealProducerReceivesExpectedSatisfies(t *testing.T) {
	ev := realBundle(t).Evidence[0]

	var got []string
	for _, r := range ev.Satisfies {
		got = append(got, string(r))
	}
	if len(got) != 1 || got[0] != lReq {
		t.Fatalf("real producer output must carry exactly [%s], got %v", lReq, got)
	}
}

// ── 2. idempotent ───────────────────────────────────────────────────────────

func TestLineage_MapperIsIdempotent(t *testing.T) {
	ev := realBundle(t).Evidence[0]
	before := len(ev.Satisfies)

	cluster_operator.BindSatisfies(&ev)
	cluster_operator.BindSatisfies(&ev)

	if len(ev.Satisfies) != before {
		t.Fatalf("re-applying the mapper duplicated bindings: %d → %d", before, len(ev.Satisfies))
	}
}

// ── 3. unknown shapes receive nothing ───────────────────────────────────────

func TestLineage_UnknownShapeGetsNoBinding(t *testing.T) {
	ev := realBundle(t).Evidence[0]
	ev.Satisfies = nil
	ev.Kind = "something_no_catalog_rule_describes"

	cluster_operator.BindSatisfies(&ev)

	if len(ev.Satisfies) != 0 {
		t.Fatalf("unknown evidence kind must receive no binding, got %v", ev.Satisfies)
	}
}

// ── 4. real output qualifies through MemoryStore ────────────────────────────

func TestLineage_RealProducerQualifiesViaMemoryStore(t *testing.T) {
	ev := realBundle(t).Evidence[0]
	s := seed(t, ev)

	res, err := cluster_operator.QualifyEvidence(context.Background(), s, lQuery())
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("real producer output must qualify; rejected=%+v", res.Rejected)
	}
	if res.Qualified[0].ID != ev.ID {
		t.Errorf("qualified the wrong row: %q vs %q", res.Qualified[0].ID, ev.ID)
	}
}

// ── 5. mutating each real field breaks qualification ────────────────────────
//
// This is what distinguishes a rule that DISCRIMINATES from one that accepts
// whatever the producer happens to emit. Each case mutates exactly one field of
// genuine producer output.

func TestLineage_MutationsBreakQualification(t *testing.T) {
	cases := []struct {
		field  string
		mutate func(*api.Evidence)
		query  func(*cluster_operator.SatisfactionQuery)
	}{
		{"kind", func(e *api.Evidence) { e.Kind = "other_kind" }, nil},
		{"source", func(e *api.Evidence) { e.SourceKind = "other_source" }, nil},
		{"result", func(e *api.Evidence) { e.Result = "verification_started" }, nil},
		{"authority", func(e *api.Evidence) { e.AuthorityLevel = api.ObservationAuthorityInterpretation }, nil},
		{"cluster", func(e *api.Evidence) { e.ClusterID = "cluster-b" }, nil},
		{"entity", func(e *api.Evidence) { e.EntityRef = "node-9" }, nil},
		{"condition", func(e *api.Evidence) { e.ConditionRef = "some.other.invariant" }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			ev := realBundle(t).Evidence[0]
			tc.mutate(&ev)
			s := seed(t, ev)

			q := lQuery()
			if tc.query != nil {
				tc.query(&q)
			}
			res, err := cluster_operator.QualifyEvidence(context.Background(), s, q)
			if err != nil {
				t.Fatalf("QualifyEvidence: %v", err)
			}
			if res.Satisfied() {
				t.Fatalf("mutating %s must break qualification — the rule does not discriminate on it", tc.field)
			}
		})
	}
}

// Control: the unmutated artifact still qualifies, so the mutation suite is
// proving discrimination rather than a uniformly broken fixture.
func TestLineage_MutationControl(t *testing.T) {
	ev := realBundle(t).Evidence[0]
	s := seed(t, ev)
	res, _ := cluster_operator.QualifyEvidence(context.Background(), s, lQuery())
	if !res.Satisfied() {
		t.Fatal("control must qualify, otherwise the mutation tests prove nothing")
	}
}
