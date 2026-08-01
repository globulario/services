package cluster_operator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

const (
	sProject = "globular-services"
	sDomain  = "cluster_operator"
	sCluster = "cluster-a"
	sReq     = "evidence.doctor.finding_observed"
	sEntity  = "node-4"
	sCond    = "cluster.desired_state.absent"
	sRun     = "run-123"
)

var (
	// Fixed clock — no test depends on wall-clock time.
	evalAt      = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	dispatchAt  = evalAt.Add(-5 * time.Minute)
	verifiedAt  = evalAt.Add(-1 * time.Minute) // after dispatch, inside MaxAge
	tooOldAt    = evalAt.Add(-30 * time.Minute)
	preActionAt = dispatchAt.Add(-1 * time.Minute)
)

// doctorEvidence builds evidence matching what observation.FromDoctorFinding
// actually emits, so it qualifies under sat.doctor.finding_observed unless mutated.
func verification(id string, mut ...func(*api.Evidence)) *api.Evidence {
	e := &api.Evidence{
		ID:             id,
		Project:        sProject,
		Domain:         api.DomainRef(sDomain),
		Kind:           "cluster_doctor_evidence",
		SourceKind:     "cluster_doctor_evidence",
		ClusterID:      sCluster,
		EntityRef:      sEntity,
		ConditionRef:   sCond,
		AuthorityLevel: api.ObservationAuthorityDerived,
		Result:         "claim",
		ObservedAt:     verifiedAt.Unix(),
		Satisfies:      []api.RequiredEvidenceRef{api.RequiredEvidenceRef(sReq)},
		// The action binding is a first-class relation, not a metadata key: the
		// satisfaction index projects a fixed column set, so a binding held in
		// Metadata is enforceable in memory and gone after a Scylla round trip.
		ActionRef: sRun,
	}
	for _, f := range mut {
		f(e)
	}
	return e
}

func satQuery(mut ...func(*SatisfactionQuery)) SatisfactionQuery {
	q := SatisfactionQuery{
		Project:            sProject,
		Domain:             api.DomainRef(sDomain),
		ClusterID:          sCluster,
		RequirementID:      sReq,
		EntityRef:          sEntity,
		ConditionRef:       sCond,
		WorkflowRunID:      sRun,
		ActionDispatchedAt: dispatchAt,
		EvaluatedAt:        evalAt,
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

func seedStore(t *testing.T, evs ...*api.Evidence) store.Store {
	t.Helper()
	m := store.NewMemoryStore()
	for _, e := range evs {
		if err := m.PutEvidence(context.Background(), e); err != nil {
			t.Fatalf("PutEvidence(%s): %v", e.ID, err)
		}
	}
	return m
}

func qualify(t *testing.T, s store.Store, q SatisfactionQuery) SatisfactionResult {
	t.Helper()
	res, err := QualifyEvidence(context.Background(), s, q)
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	return res
}

// ── the complete positive trace ─────────────────────────────────────────────

func TestQualify_FullPositiveTrace(t *testing.T) {
	s := seedStore(t, verification("ev-good"))
	res := qualify(t, s, satQuery())

	if !res.Satisfied() {
		t.Fatalf("expected satisfaction, rejected=%+v", res.Rejected)
	}
	if len(res.Qualified) != 1 || res.Qualified[0].ID != "ev-good" {
		t.Fatalf("want [ev-good], got %+v", res.Qualified)
	}
	if len(res.RuleIDs) == 0 || res.RuleIDs[0] != "sat.doctor.finding_observed" {
		t.Errorf("result must name the rule that governed it, got %v", res.RuleIDs)
	}
}

// ── the four outcomes must stay distinct ────────────────────────────────────

func TestQualify_MalformedQueryErrors(t *testing.T) {
	s := seedStore(t)
	for _, q := range []SatisfactionQuery{
		satQuery(func(q *SatisfactionQuery) { q.Project = "" }),
		satQuery(func(q *SatisfactionQuery) { q.RequirementID = "" }),
		satQuery(func(q *SatisfactionQuery) { q.EvaluatedAt = time.Time{} }),
	} {
		if _, err := QualifyEvidence(context.Background(), s, q); err == nil {
			t.Errorf("malformed query must error: %+v", q)
		}
	}
}

// An undeclared requirement is NOT an unsatisfied one. Collapsing them would
// send an operator to gather evidence that could never have qualified.
func TestQualify_UnconfiguredRequirementIsDistinct(t *testing.T) {
	s := seedStore(t, verification("ev"))
	_, err := QualifyEvidence(context.Background(), s, satQuery(func(q *SatisfactionQuery) {
		q.RequirementID = "evidence.not.declared.anywhere"
	}))
	if !errors.Is(err, ErrRequirementNotDeclared) {
		t.Fatalf("want ErrRequirementNotDeclared, got %v", err)
	}
}

func TestQualify_NoCandidatesIsEmptyNotError(t *testing.T) {
	res := qualify(t, seedStore(t), satQuery())
	if res.Satisfied() {
		t.Error("no stored evidence must not satisfy")
	}
	if len(res.Rejected) != 0 {
		t.Errorf("no candidates → no rejections, got %+v", res.Rejected)
	}
}

func TestQualify_CandidatesButNoneQualifyCarryDiagnostics(t *testing.T) {
	s := seedStore(t, verification("ev-stale", func(e *api.Evidence) {
		e.ObservedAt = tooOldAt.Unix()
	}))
	res := qualify(t, s, satQuery())
	if res.Satisfied() {
		t.Fatal("stale evidence must not satisfy")
	}
	if len(res.Rejected) != 1 || res.Rejected[0].Reason != RejectEvidenceStale {
		t.Fatalf("want one evidence_stale rejection, got %+v", res.Rejected)
	}
	if res.Rejected[0].Detail == "" {
		t.Error("rejection must explain the comparison that failed")
	}
}

// ── authority ───────────────────────────────────────────────────────────────

func TestQualify_AuthorityFloor(t *testing.T) {
	cases := []struct {
		name  string
		level api.ObservationAuthorityLevel
		want  bool
	}{
		{"below floor", api.ObservationAuthorityDiagnostic, false},
		{"at floor", api.ObservationAuthorityDerived, true},
		{"above floor", api.ObservationAuthorityTruthPlane, true},
		{"unspecified", api.ObservationAuthorityUnspecified, false},
		{"unknown value", api.ObservationAuthorityLevel("SOMETHING_NEW"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := seedStore(t, verification("ev", func(e *api.Evidence) { e.AuthorityLevel = tc.level }))
			res := qualify(t, s, satQuery())
			if res.Satisfied() != tc.want {
				t.Fatalf("authority %q: satisfied=%v want %v (rejected=%+v)",
					tc.level, res.Satisfied(), tc.want, res.Rejected)
			}
		})
	}
}

// A self-asserted claim must never stand in for owner-verified proof.
func TestQualify_SelfAssertionCannotSatisfyVerification(t *testing.T) {
	s := seedStore(t, verification("ev-self", func(e *api.Evidence) {
		e.AuthorityLevel = api.ObservationAuthorityInterpretation
	}))
	res := qualify(t, s, satQuery())
	if res.Satisfied() {
		t.Fatal("INTERPRETATION must not satisfy a DERIVED_EVIDENCE floor")
	}
	if res.Rejected[0].Reason != RejectAuthorityInsufficient {
		t.Errorf("want authority_insufficient, got %q", res.Rejected[0].Reason)
	}
}

// ── results: existence is not satisfaction ──────────────────────────────────

func TestQualify_ResultMustBeAccepted(t *testing.T) {
	for _, result := range []string{"verification_started", "dispatch_accepted", "finding_still_present", "", "unknown"} {
		s := seedStore(t, verification("ev", func(e *api.Evidence) { e.Result = result }))
		res := qualify(t, s, satQuery())
		if res.Satisfied() {
			t.Errorf("result %q must not satisfy — existence is not satisfaction", result)
		}
	}
	s := seedStore(t, verification("ev"))
	if !qualify(t, s, satQuery()).Satisfied() {
		t.Error("the producer's own result must satisfy")
	}
}

// ── freshness and action binding ────────────────────────────────────────────

func TestQualify_Freshness(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*api.Evidence)
		reason RejectionReason
	}{
		{"stale", func(e *api.Evidence) { e.ObservedAt = tooOldAt.Unix() }, RejectEvidenceStale},
		{"missing timestamp", func(e *api.Evidence) { e.ObservedAt = 0 }, RejectTimestampMissing},
		{"future timestamp", func(e *api.Evidence) { e.ObservedAt = evalAt.Add(time.Hour).Unix() }, RejectTimestampInFuture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := seedStore(t, verification("ev", tc.mutate))
			res := qualify(t, s, satQuery())
			if res.Satisfied() {
				t.Fatalf("%s must not satisfy", tc.name)
			}
			if len(res.Rejected) != 1 || res.Rejected[0].Reason != tc.reason {
				t.Fatalf("want %q, got %+v", tc.reason, res.Rejected)
			}
		})
	}
}

// The three action-binding policies, exercised directly at the freshness layer.
// FreshnessBoundToAction is the one sat.remediation.fresh_convergence_verified
// uses; the other two remain covered so their semantics cannot rot.
func TestFreshness_ActionBindingPolicies(t *testing.T) {
	base := SatisfactionRule{Freshness: FreshnessNewerThanAction, MaxAge: 10 * time.Minute}
	q := satQuery()

	if rej := evaluateFreshness(base, *verification("ok"), q); rej != nil {
		t.Fatalf("evidence after dispatch, same run must pass: %+v", rej)
	}
	// Evidence gathered BEFORE the action cannot show the action worked.
	pre := verification("pre", func(e *api.Evidence) { e.ObservedAt = preActionAt.Unix() })
	if rej := evaluateFreshness(base, *pre, q); rej == nil || rej.Reason != RejectNotBoundToAction {
		t.Errorf("pre-action evidence must be rejected, got %+v", rej)
	}
	// A fresh timestamp from an UNRELATED remediation must not qualify.
	other := verification("other", func(e *api.Evidence) { e.ActionRef = "run-somebody-else" })
	if rej := evaluateFreshness(base, *other, q); rej == nil || rej.Reason != RejectNotBoundToAction {
		t.Errorf("evidence from another run must be rejected, got %+v", rej)
	}
	// same_workflow_run binds without requiring an ordering relative to dispatch.
	runOnly := SatisfactionRule{Freshness: FreshnessSameWorkflowRun, MaxAge: 10 * time.Minute}
	if rej := evaluateFreshness(runOnly, *other, q); rej == nil || rej.Reason != RejectNotBoundToAction {
		t.Errorf("same_workflow_run must reject a foreign run, got %+v", rej)
	}

	// bound_to_action: both halves mandatory.
	bound := SatisfactionRule{Freshness: FreshnessBoundToAction, MaxAge: 10 * time.Minute}
	if rej := evaluateFreshness(bound, *verification("ok"), q); rej != nil {
		t.Errorf("matching run observed after dispatch must pass: %+v", rej)
	}
	if rej := evaluateFreshness(bound, *other, q); rej == nil || rej.Reason != RejectNotBoundToAction {
		t.Errorf("bound_to_action must reject a foreign run, got %+v", rej)
	}
	if rej := evaluateFreshness(bound, *pre, q); rej == nil || rej.Reason != RejectNotBoundToAction {
		t.Errorf("bound_to_action must reject pre-action evidence, got %+v", rej)
	}
	// The difference from newer_than_action: a query with no run identity is
	// REJECTED rather than quietly evaluated on time alone. Failing open here
	// would let any fresh verification in the cluster authorize any action.
	noRun := satQuery(func(sq *SatisfactionQuery) { sq.WorkflowRunID = "" })
	if rej := evaluateFreshness(bound, *verification("ok"), noRun); rej == nil || rej.Reason != RejectNotBoundToAction {
		t.Errorf("bound_to_action must reject a query with no action identity, got %+v", rej)
	}
	if rej := evaluateFreshness(base, *verification("ok"), noRun); rej != nil {
		t.Errorf("newer_than_action is the relaxed policy and must still pass here: %+v", rej)
	}
}

// ── isolation ───────────────────────────────────────────────────────────────

func TestQualify_ClusterIsolation(t *testing.T) {
	s := seedStore(t, verification("ev-b", func(e *api.Evidence) { e.ClusterID = "cluster-b" }))
	res := qualify(t, s, satQuery())
	if res.Satisfied() {
		t.Fatal("evidence from another cluster must never satisfy")
	}
	// Structural: the index partition excludes it, so it is not even a candidate.
	if len(res.Rejected) != 0 {
		t.Errorf("cross-cluster evidence should not surface as a candidate, got %+v", res.Rejected)
	}
}

func TestQualify_SubjectIsolation(t *testing.T) {
	s := seedStore(t,
		verification("ev-other-entity", func(e *api.Evidence) { e.EntityRef = "node-5" }),
		verification("ev-other-condition", func(e *api.Evidence) { e.ConditionRef = "some.other.invariant" }),
	)
	res := qualify(t, s, satQuery())
	if res.Satisfied() {
		t.Fatal("evidence about another entity or invariant must not satisfy")
	}
	if len(res.Rejected) != 2 {
		t.Fatalf("both must be rejected with diagnostics, got %+v", res.Rejected)
	}
	for _, r := range res.Rejected {
		if r.Reason != RejectSubjectMismatch {
			t.Errorf("want subject_mismatch, got %q", r.Reason)
		}
	}
}

// ── determinism ─────────────────────────────────────────────────────────────

func TestQualify_DeterministicNewestFirst(t *testing.T) {
	s := seedStore(t,
		verification("b", func(e *api.Evidence) { e.ObservedAt = evalAt.Add(-3 * time.Minute).Unix() }),
		verification("a", func(e *api.Evidence) { e.ObservedAt = evalAt.Add(-1 * time.Minute).Unix() }),
		verification("c", func(e *api.Evidence) { e.ObservedAt = evalAt.Add(-2 * time.Minute).Unix() }),
	)
	first := qualify(t, s, satQuery())
	if len(first.Qualified) != 3 {
		t.Fatalf("want 3 qualified, got %d", len(first.Qualified))
	}
	if first.Qualified[0].ID != "a" || first.Qualified[2].ID != "b" {
		t.Errorf("want newest-first a,c,b — got %s,%s,%s",
			first.Qualified[0].ID, first.Qualified[1].ID, first.Qualified[2].ID)
	}
	// Repeated evaluation over identical state and clock is byte-equivalent.
	second := qualify(t, s, satQuery())
	for i := range first.Qualified {
		if first.Qualified[i].ID != second.Qualified[i].ID {
			t.Fatalf("non-deterministic ordering at %d: %q vs %q",
				i, first.Qualified[i].ID, second.Qualified[i].ID)
		}
	}
}

// ── catalog hygiene ─────────────────────────────────────────────────────────

// Every rule must be honestly complete. A permissive entry — no authority floor,
// no accepted results, no freshness — is indistinguishable from no governance
// while looking like governance.
func TestCatalog_RulesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range satisfactionCatalog {
		if r.ID == "" || r.RequirementID == "" {
			t.Errorf("rule missing id or requirement: %+v", r)
		}
		if seen[r.ID] {
			t.Errorf("duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if _, ok := api.AuthorityRank(r.MinimumAuthority); !ok {
			t.Errorf("rule %s: authority floor %q is not rankable — it would never be satisfiable",
				r.ID, r.MinimumAuthority)
		}
		if len(r.AcceptedResults) == 0 {
			t.Errorf("rule %s: no accepted results — existence would become satisfaction", r.ID)
		}
		if r.Freshness != FreshnessNone && r.MaxAge <= 0 && r.Freshness == FreshnessMaxAge {
			t.Errorf("rule %s: max_age policy without a MaxAge", r.ID)
		}
	}
}

func TestRulesForRequirement_StableOrderAndUnknown(t *testing.T) {
	if got := RulesForRequirement("evidence.nope"); len(got) != 0 {
		t.Errorf("unknown requirement → no rules, got %+v", got)
	}
	a := RulesForRequirement(sReq)
	b := RulesForRequirement(sReq)
	if len(a) == 0 {
		t.Fatal("expected rules for the verification requirement")
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatal("rule ordering must not depend on map iteration")
		}
	}
}
