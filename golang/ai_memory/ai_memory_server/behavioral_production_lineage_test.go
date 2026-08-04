package main

// Commit 4D: the COMPLETE production lineage, end to end, against a real store.
//
//	rules.Finding (via the doctor's ResolveFinding callback)
//	→ doctor.resolve_finding      REAL handler, via the exported Router
//	→ doctor.execute_remediation  REAL handler, stamps dispatched_at
//	→ doctor.verify_convergence   REAL handler
//	→ remediation.Outcome         canonical object, captured through the
//	                              PRODUCTION ObserveOutcome hook
//	→ outcomeAsMap receipt        asserted from the real run outputs
//	→ FromRemediationOutcome      real adapter
//	→ BindSatisfies               real catalog mapper
//	→ ScyllaStore.PutEvidence     real store, real cluster
//	→ ListEvidenceSatisfying
//	→ QualifyEvidence
//	→ SATISFIED
//
// HOW THIS BECAME POSSIBLE
//
// 4C's proof was split: the real chain could only be driven from package engine
// (buildRemediationOutcome is unexported) while the schema lives here in package
// main. 4D closes that gap without a refactor, because the production wiring
// point IS the seam: cfg.ObserveOutcome hands out the canonical Outcome, and
// Router.Resolve exposes the real handlers. This test therefore uses the same
// entry points cluster_doctor uses in production — not a reconstruction.
//
// The one hop not exercised here is the gRPC transport. The deployed
// ai_memory is 1.2.288, which predates the action_ref proto field, so pointing
// a Recorder at it would fail for deployment reasons rather than code reasons.
// Both halves of that conversion are proven directly instead:
// observation.TestEvidenceToPB_PreservesActionBinding and
// TestPBToEvidence_PreservesActionBinding below.
//
// Skipped unless BEHAVIORAL_SCYLLA_HOSTS is set. A skip is NOT proof.
//
//	BEHAVIORAL_SCYLLA_HOSTS=10.0.0.63 go test ./ai_memory/ai_memory_server \
//	  -run TestProductionLineage -count=1

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/gocql/gocql"
)

const (
	plFinding = "finding-1"
	plInvar   = "cluster.desired_state.absent"
	plEntity  = "svc/repository" // deliberately NOT the node id
	plNode    = "node-4"
	plRun     = "run-prod-1"
)

var (
	plDispatchAt = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	plVerifyAt   = plDispatchAt.Add(90 * time.Second)
	plEvalAt     = plVerifyAt.Add(1 * time.Minute)
)

// plRunChain drives the REAL doctor remediation handlers through the exported
// Router and returns the canonical Outcome captured by the production hook,
// plus the run outputs an out-of-process consumer would read.
func plRunChain(t *testing.T, cluster, run string, mut ...func(*engine.DoctorRemediationConfig)) (remediation.Outcome, map[string]any) {
	t.Helper()

	var captured []remediation.Outcome
	cfg := engine.DoctorRemediationConfig{
		Now: func() time.Time { return plDispatchAt },
		ResolveFinding: func(_ context.Context, _ string, id string, idx uint32) (*engine.ResolvedFinding, error) {
			return &engine.ResolvedFinding{
				FindingID: id, StepIndex: idx, NodeID: plNode,
				ActionType: "SYSTEMCTL_RESTART", Risk: "LOW", Idempotent: true,
				Description: "restart", HasAction: true,
				ClusterID: cluster, InvariantID: plInvar, EntityRef: plEntity,
			}, nil
		},
		ExecuteRemediation: func(context.Context, string, uint32, string, bool) (*engine.ExecutionResult, error) {
			return &engine.ExecutionResult{AuditID: "audit-1", Status: "EXECUTED", Executed: true}, nil
		},
		VerifyConvergence: func(context.Context, string, string, time.Time) (*engine.Verification, error) {
			return &engine.Verification{Converged: true}, nil
		},
		// The PRODUCTION hook. cluster_doctor supplies this same field to reach
		// its bounded recorder; here it hands the outcome to the test.
		ObserveOutcome: func(_ context.Context, o remediation.Outcome) { captured = append(captured, o) },
	}
	for _, f := range mut {
		f(&cfg)
	}

	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, cfg)
	outputs := map[string]any{}
	ctx := context.Background()

	dispatch := func(action string, with map[string]any) {
		t.Helper()
		h, ok := router.Resolve(v1alpha1.ActorClusterDoctor, action)
		if !ok {
			t.Fatalf("%s is not registered — the production router would not dispatch it either", action)
		}
		if _, err := h(ctx, engine.ActionRequest{RunID: run, With: with, Outputs: outputs}); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}

	dispatch("doctor.resolve_finding", map[string]any{"finding_id": plFinding, "step_index": 0})
	dispatch("doctor.execute_remediation", map[string]any{"finding_id": plFinding, "step_index": 0})

	resolved, _ := outputs["resolved_finding"].(map[string]any)
	dispatch("doctor.verify_convergence", map[string]any{
		"finding_id":   plFinding,
		"node_id":      resolved["node_id"],
		"cluster_id":   resolved["cluster_id"],
		"invariant_id": resolved["invariant_id"],
		"entity_ref":   resolved["entity_ref"],
	})

	if len(captured) != 1 {
		t.Fatalf("the production hook must receive exactly one outcome, got %d", len(captured))
	}
	// The engine's clock stamps VerifiedAt from time.Now; pin it so freshness
	// assertions are about the rule, not about how long the test took.
	o := captured[0]
	o.VerifiedAt = plVerifyAt
	return o, outputs
}

func plScope() (project, domain, cluster string) {
	u := strings.ReplaceAll(gocql.TimeUUID().String(), "-", "")[:12]
	return "prodlineage-" + u, "cluster_operator", "cluster-" + u
}

func plQuery(project, domain, cluster string, mut ...func(*cluster_operator.SatisfactionQuery)) cluster_operator.SatisfactionQuery {
	q := cluster_operator.SatisfactionQuery{
		Project: project, Domain: api.DomainRef(domain), ClusterID: cluster,
		RequirementID: remReq,
		EntityRef:     plEntity, ConditionRef: plInvar, SourceRef: plFinding,
		WorkflowRunID: plRun, ActionDispatchedAt: plDispatchAt,
		EvaluatedAt: plEvalAt,
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

// plEvidence maps a real outcome through the real adapter + catalog mapper.
func plEvidence(t *testing.T, project, domain string, o remediation.Outcome) api.Evidence {
	t.Helper()
	b, qualifies := observation.FromRemediationOutcome(project, api.DomainRef(domain), o)
	observation.BindRemediationEvidence(&b)
	if !qualifies {
		t.Fatalf("the real chain must produce a qualifying outcome; success=%t defects=%v",
			o.IsSuccess(), o.LineageDefects())
	}
	if len(b.Evidence) != 1 {
		t.Fatalf("adapter emitted %d rows, want 1", len(b.Evidence))
	}
	return b.Evidence[0]
}

// ── the complete lineage ────────────────────────────────────────────────────

func TestProductionLineage_RealChainQualifiesThroughScylla(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	project, domain, cluster := plScope()
	ctx := context.Background()

	o, outputs := plRunChain(t, cluster, plRun)

	// The receipt an out-of-process consumer reads must describe the same run.
	receipt, ok := outputs["remediation_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("verify step must write remediation_outcome, got %T", outputs["remediation_outcome"])
	}
	for k, want := range map[string]any{
		"cluster_id": cluster, "invariant_id": plInvar, "entity_ref": plEntity,
		"node_id": plNode, "finding_id": plFinding, "workflow_run_id": plRun,
		"finding_resolved": true, "lineage_complete": true,
	} {
		if receipt[k] != want {
			t.Errorf("receipt %s = %v, want %v", k, receipt[k], want)
		}
	}
	if receipt["dispatched_at"] != plDispatchAt.UTC().Format(time.RFC3339Nano) {
		t.Errorf("receipt dispatched_at = %v, want %s", receipt["dispatched_at"],
			plDispatchAt.UTC().Format(time.RFC3339Nano))
	}

	ev := plEvidence(t, project, domain, o)
	defer cleanup(project, domain, cluster, ev.ID)

	if err := st.PutEvidence(ctx, &ev); err != nil {
		t.Fatalf("PutEvidence: %v", err)
	}

	res, err := cluster_operator.QualifyEvidence(ctx, st, plQuery(project, domain, cluster))
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("PRODUCTION LINEAGE FAILED — the real chain's evidence did not qualify after "+
			"a real round trip; rejected=%+v", res.Rejected)
	}

	// Everything the rule depends on must have survived persistence, read back
	// from the index projection rather than the in-memory copy.
	got := res.Qualified[0]
	for _, c := range []struct{ name, got, want string }{
		{"cluster", got.ClusterID, cluster},
		{"invariant", got.ConditionRef, plInvar},
		{"entity", got.EntityRef, plEntity},
		{"finding lineage", got.SourceRef, plFinding},
		{"action binding", got.ActionRef, plRun},
		{"result", got.Result, observation.ResultFindingResolved},
		{"authority", string(got.AuthorityLevel), string(api.ObservationAuthorityDerived)},
		{"kind", got.Kind, observation.KindRemediationVerification},
		{"source", got.SourceKind, observation.SourceKindDoctorRemediationWorkflow},
	} {
		if c.got != c.want {
			t.Errorf("%s did not survive persistence: got %q want %q", c.name, c.got, c.want)
		}
	}
	if got.ObservedAt != plVerifyAt.Unix() {
		t.Errorf("verification time did not survive: %d want %d", got.ObservedAt, plVerifyAt.Unix())
	}
	if got.ObservedAt <= plDispatchAt.Unix() {
		t.Error("dispatch ordering violated: the observation must postdate the action")
	}
	if len(got.Satisfies) == 0 {
		t.Error("Satisfies did not survive the round trip")
	}
}

// ── negative proof on PERSISTED fields ──────────────────────────────────────
//
// Each mutation must produce one of three distinguishable failures, and the test
// records WHICH — otherwise "it didn't qualify" could mean the rule worked or
// that the row never reached the index.

type plFailure string

const (
	plNoBinding plFailure = "no_binding" // ineligible at construction
	plNoLookup  plFailure = "no_lookup"  // stored, but outside the queried partition
	plRejected  plFailure = "rejected"   // a candidate the rule refused
)

func TestProductionLineage_PersistedFieldMutations(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	ctx := context.Background()

	cases := []struct {
		field   string
		outcome func(*remediation.Outcome)
		post    func(*api.Evidence)
		want    plFailure
		reason  cluster_operator.RejectionReason
	}{
		{field: "cluster", post: func(e *api.Evidence) { e.ClusterID = "cluster-elsewhere" },
			want: plNoLookup},
		{field: "invariant", post: func(e *api.Evidence) { e.ConditionRef = "some.other.invariant" },
			want: plRejected, reason: cluster_operator.RejectSubjectMismatch},
		{field: "entity", post: func(e *api.Evidence) { e.EntityRef = "svc/other" },
			want: plRejected, reason: cluster_operator.RejectSubjectMismatch},
		{field: "finding_reference", post: func(e *api.Evidence) { e.SourceRef = "finding-earlier" },
			want: plRejected, reason: cluster_operator.RejectSubjectMismatch},
		{field: "action_reference", post: func(e *api.Evidence) { e.ActionRef = "run-elsewhere" },
			want: plRejected, reason: cluster_operator.RejectNotBoundToAction},
		{field: "result", post: func(e *api.Evidence) { e.Result = "succeeded" },
			want: plRejected, reason: cluster_operator.RejectResultNotAccepted},
		{field: "authority", post: func(e *api.Evidence) { e.AuthorityLevel = api.ObservationAuthorityDiagnostic },
			want: plRejected, reason: cluster_operator.RejectAuthorityInsufficient},
		{field: "kind", post: func(e *api.Evidence) { e.Kind = "some_other_kind" },
			want: plRejected, reason: cluster_operator.RejectSubjectMismatch},
		{field: "source", post: func(e *api.Evidence) { e.SourceKind = "some_other_source" },
			want: plRejected, reason: cluster_operator.RejectSubjectMismatch},
		// Verification time discriminates two ways, and the order matters: the
		// age bound is evaluated before the ordering check, so an observation
		// long before dispatch is rejected as STALE. Testing only that would
		// leave the ordering check unproven, hence both cases — one fresh but
		// mis-ordered, one simply old.
		{field: "verification_time_before_dispatch",
			post: func(e *api.Evidence) { e.ObservedAt = plDispatchAt.Add(-time.Minute).Unix() },
			want: plRejected, reason: cluster_operator.RejectNotBoundToAction},
		{field: "verification_time_stale",
			post: func(e *api.Evidence) { e.ObservedAt = plDispatchAt.Add(-time.Hour).Unix() },
			want: plRejected, reason: cluster_operator.RejectEvidenceStale},
		// Dispatch after verification is an impossible ordering; the adapter
		// refuses to emit the qualifying kind at all.
		{field: "dispatch_time",
			outcome: func(o *remediation.Outcome) { o.DispatchedAt = o.VerifiedAt.Add(time.Minute) },
			want:    plNoBinding},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			project, domain, cluster := plScope()
			o, _ := plRunChain(t, cluster, plRun)
			if tc.outcome != nil {
				tc.outcome(&o)
			}

			b, qualifies := observation.FromRemediationOutcome(project, api.DomainRef(domain), o)
			observation.BindRemediationEvidence(&b)
			ev := b.Evidence[0]
			if tc.post != nil {
				tc.post(&ev)
			}
			defer cleanup(project, domain, cluster, ev.ID)

			if tc.want == plNoBinding {
				if qualifies || len(ev.Satisfies) != 0 {
					t.Fatalf("mutating %s must fail eligibility at construction; qualifies=%t satisfies=%v",
						tc.field, qualifies, ev.Satisfies)
				}
				return
			}
			if len(ev.Satisfies) == 0 {
				t.Fatalf("mutating %s left the row unbound — it proves nothing about lookup or the rule", tc.field)
			}

			if err := st.PutEvidence(ctx, &ev); err != nil {
				t.Fatalf("PutEvidence: %v", err)
			}
			res, err := cluster_operator.QualifyEvidence(ctx, st, plQuery(project, domain, cluster))
			if err != nil {
				t.Fatalf("QualifyEvidence: %v", err)
			}
			if res.Satisfied() {
				t.Fatalf("mutating %s must break qualification — nothing enforces it", tc.field)
			}

			switch tc.want {
			case plNoLookup:
				if len(res.Rejected) != 0 {
					t.Errorf("mutating %s should place the row outside the queried partition, "+
						"but it was a candidate and got rejected: %+v", tc.field, res.Rejected)
				}
			case plRejected:
				if len(res.Rejected) == 0 {
					t.Fatalf("mutating %s must produce a REJECTION (want %q), not an empty candidate set",
						tc.field, tc.reason)
				}
				if res.Rejected[0].Reason != tc.reason {
					t.Errorf("rejected for %q (%s), want %q",
						res.Rejected[0].Reason, res.Rejected[0].Detail, tc.reason)
				}
			}
		})
	}
}

// Control: unmutated, the real chain's artifact still qualifies through the same
// store — so the matrix above proves discrimination, not a broken fixture.
func TestProductionLineage_MutationControl(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	project, domain, cluster := plScope()
	ctx := context.Background()

	o, _ := plRunChain(t, cluster, plRun)
	ev := plEvidence(t, project, domain, o)
	defer cleanup(project, domain, cluster, ev.ID)

	if err := st.PutEvidence(ctx, &ev); err != nil {
		t.Fatalf("PutEvidence: %v", err)
	}
	res, err := cluster_operator.QualifyEvidence(ctx, st, plQuery(project, domain, cluster))
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("control must qualify; rejected=%+v", res.Rejected)
	}
}

// ── replay ──────────────────────────────────────────────────────────────────

// A resumed or retried workflow re-records the same fact. Both the evidence row
// and its satisfaction-index row must upsert, not accumulate.
func TestProductionLineage_ReplayProducesNoDuplicate(t *testing.T) {
	st, session, cleanup := remFixture(t)
	defer session.Close()
	project, domain, cluster := plScope()
	ctx := context.Background()

	o, _ := plRunChain(t, cluster, plRun)
	first := plEvidence(t, project, domain, o)
	defer cleanup(project, domain, cluster, first.ID)

	// Run the whole chain a second time, as a workflow resume would.
	o2, _ := plRunChain(t, cluster, plRun)
	second := plEvidence(t, project, domain, o2)

	if first.ID != second.ID {
		t.Fatalf("replay produced a different evidence id (%q vs %q)", first.ID, second.ID)
	}
	if first.ObservedAt != second.ObservedAt {
		t.Fatalf("replay changed observed_at (%d vs %d) — observed_at is in the index clustering key, "+
			"so the same fact would occupy two rows", first.ObservedAt, second.ObservedAt)
	}

	for _, e := range []api.Evidence{first, second} {
		c := e
		if err := st.PutEvidence(ctx, &c); err != nil {
			t.Fatalf("PutEvidence: %v", err)
		}
	}

	rows, err := st.ListEvidenceSatisfying(ctx, store.EvidenceSatisfactionQuery{
		Project: project, Domain: domain, RequiredEvidenceRef: remReq, ClusterID: cluster,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("replay must leave exactly one index row, got %d: %+v", len(rows), rows)
	}
	res, err := cluster_operator.QualifyEvidence(ctx, st, plQuery(project, domain, cluster))
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if len(res.Qualified) != 1 {
		t.Fatalf("replay must leave exactly one qualifying row, got %d", len(res.Qualified))
	}
}

// ── the wire hop ────────────────────────────────────────────────────────────

// The server half of the gRPC conversion. Its counterpart (evidenceToPB) is
// proven in the observation package. A binding dropped on either side is
// indistinguishable from one that was never set, and the receiving end would
// persist evidence no action-bound rule can qualify.
func TestPBToEvidence_PreservesActionBinding(t *testing.T) {
	got := pbToEvidence(&bpb.Evidence{
		Id: "ev-1", Project: "p", Domain: "cluster_operator",
		EvidenceKind: observation.KindRemediationVerification,
		SourceKind:   observation.SourceKindDoctorRemediationWorkflow,
		Result:       observation.ResultFindingResolved,
		SourceRef:    plFinding,
		ActionRef:    plRun,
		EntityRef:    plEntity,
		ClusterId:    "cluster-x",
		ConditionRef: plInvar,
		ObservedAt:   plVerifyAt.Unix(),
		Satisfies:    []string{remReq},
	})

	if got.ActionRef != plRun {
		t.Errorf("action_ref lost across the wire: got %q want %q", got.ActionRef, plRun)
	}
	if got.SourceRef != plFinding {
		t.Errorf("source_ref lost across the wire: got %q want %q", got.SourceRef, plFinding)
	}
	if len(got.Satisfies) != 1 || string(got.Satisfies[0]) != remReq {
		t.Errorf("satisfies lost across the wire: %v", got.Satisfies)
	}
}
