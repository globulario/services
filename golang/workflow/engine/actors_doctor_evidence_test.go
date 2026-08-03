package engine

// Commit 4C: the production path from a real remediation to governed evidence.
//
//	rules.Finding (via ResolveFinding)
//	→ doctor.resolve_finding      (real handler)
//	→ doctor.execute_remediation  (real handler, stamps dispatched_at)
//	→ doctor.verify_convergence   (real handler)
//	→ buildRemediationOutcome     (real outcome, not a fixture)
//	→ observation.FromRemediationOutcome
//	→ BindSatisfies
//	→ MemoryStore.PutEvidence
//	→ ListEvidenceSatisfying
//	→ QualifyEvidence
//	→ SATISFIED
//
// WHY THIS FILE LIVES IN package engine
//
// buildRemediationOutcome is unexported, and it is the only thing that assembles
// an Outcome from real run state. A test elsewhere would have to hand-build the
// Outcome, which is precisely the substitution that produced commit 4A's
// fictional catalog: the artifact and the rule agreed with each other while
// neither agreed with production. The import runs engine(test) → ai_memory, a
// direction ai_memory never takes back, so no cycle exists.
//
// The mutation suite below starts from THIS chain's output and changes exactly
// one thing per case. That is what separates a rule that discriminates from one
// that accepts whatever the producer happens to emit.

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/remediation"
)

const (
	evProject = "globular-services"
	evDomain  = "cluster_operator"
	evReq     = "evidence.remediation.fresh_convergence_verification"
)

// Evaluation sits after verification and well inside the rule's 15m window.
var evEvalAt = lnVerifyAt.Add(1 * time.Minute)

// realEvidence runs the REAL actor chain and maps its outcome through the REAL
// adapter. Nothing here hand-builds an Outcome or an Evidence.
func realEvidence(t *testing.T) (remediation.Outcome, api.Evidence, bool) {
	t.Helper()
	o := chain(t)
	ev, qualifies := mapOutcome(t, o)
	return o, ev, qualifies
}

// mapOutcome applies the production adapter + catalog mapper to an outcome.
func mapOutcome(t *testing.T, o remediation.Outcome) (api.Evidence, bool) {
	t.Helper()
	b, qualifies := observation.FromRemediationOutcome(evProject, api.DomainRef(evDomain), o)
	observation.BindRemediationEvidence(&b)
	if len(b.Evidence) != 1 {
		t.Fatalf("adapter emitted %d evidence rows, want 1", len(b.Evidence))
	}
	return b.Evidence[0], qualifies
}

func evQuery(mut ...func(*cluster_operator.SatisfactionQuery)) cluster_operator.SatisfactionQuery {
	q := cluster_operator.SatisfactionQuery{
		Project:            evProject,
		Domain:             api.DomainRef(evDomain),
		ClusterID:          lnCluster,
		RequirementID:      evReq,
		EntityRef:          lnEntity,
		ConditionRef:       lnInvar,
		SourceRef:          lnFinding,
		WorkflowRunID:      lnRun,
		ActionDispatchedAt: lnDispatchAt,
		EvaluatedAt:        evEvalAt,
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

func evSeed(t *testing.T, evs ...api.Evidence) store.Store {
	t.Helper()
	m := store.NewMemoryStore()
	for i := range evs {
		if err := m.PutEvidence(context.Background(), &evs[i]); err != nil {
			t.Fatalf("PutEvidence: %v", err)
		}
	}
	return m
}

func evQualify(t *testing.T, s store.Store, q cluster_operator.SatisfactionQuery) cluster_operator.SatisfactionResult {
	t.Helper()
	res, err := cluster_operator.QualifyEvidence(context.Background(), s, q)
	if err != nil {
		t.Fatalf("QualifyEvidence: %v", err)
	}
	return res
}

// ── 1. the full chain ───────────────────────────────────────────────────────

func TestEvidence_RealChainQualifies(t *testing.T) {
	o, ev, qualifies := realEvidence(t)

	if !qualifies {
		t.Fatalf("a successful, lineage-complete remediation must qualify; defects=%v", o.LineageDefects())
	}
	if ev.Kind != observation.KindRemediationVerification {
		t.Errorf("kind = %q, want %q", ev.Kind, observation.KindRemediationVerification)
	}
	if ev.Result != observation.ResultFindingResolved {
		t.Errorf("result = %q, want %q", ev.Result, observation.ResultFindingResolved)
	}
	if ev.AuthorityLevel != api.ObservationAuthorityDerived {
		t.Errorf("authority = %q, want DERIVED_EVIDENCE", ev.AuthorityLevel)
	}

	// Identity must come from the outcome, not be re-derived or defaulted.
	for _, c := range []struct{ name, got, want string }{
		{"cluster", ev.ClusterID, lnCluster},
		{"condition", ev.ConditionRef, lnInvar},
		{"entity", ev.EntityRef, lnEntity},
		{"source_ref(finding)", ev.SourceRef, lnFinding},
		{"action_ref(run)", ev.ActionRef, lnRun},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if ev.ObservedAt != lnVerifyAt.Unix() {
		t.Errorf("observed_at = %d, want the verification time %d", ev.ObservedAt, lnVerifyAt.Unix())
	}

	// The producer must bind before persistence: the satisfaction index is
	// written from Satisfies, so an unbound row is not rejected — it is never a
	// candidate, and the governor reports "no evidence" for evidence that exists.
	if len(ev.Satisfies) != 1 || string(ev.Satisfies[0]) != evReq {
		t.Fatalf("Satisfies = %v, want exactly [%s]", ev.Satisfies, evReq)
	}

	res := evQualify(t, evSeed(t, ev), evQuery())
	if !res.Satisfied() {
		t.Fatalf("real chain output must qualify; rejected=%+v", res.Rejected)
	}
	if res.Qualified[0].ID != ev.ID {
		t.Errorf("qualified the wrong row: %q vs %q", res.Qualified[0].ID, ev.ID)
	}
}

// EntityRef is the finding's subject; NodeID is where the action ran. The chain
// uses a service-scoped entity so a silent substitution is visible.
func TestEvidence_EntityRefIsNotNodeID(t *testing.T) {
	_, ev, _ := realEvidence(t)
	if ev.EntityRef == lnNode {
		t.Fatal("EntityRef was replaced by NodeID — the verification would be attributed to the wrong subject")
	}
	if ev.Metadata["node_id"] != lnNode {
		t.Errorf("node_id metadata = %q, want %q", ev.Metadata["node_id"], lnNode)
	}
}

// Applying the mapper again must not duplicate bindings.
func TestEvidence_BindingIsIdempotent(t *testing.T) {
	_, ev, _ := realEvidence(t)
	before := len(ev.Satisfies)
	cluster_operator.BindSatisfies(&ev)
	cluster_operator.BindSatisfies(&ev)
	if len(ev.Satisfies) != before {
		t.Fatalf("re-applying the mapper duplicated bindings: %d → %d", before, len(ev.Satisfies))
	}
}

// ── 2. eligibility: both conditions are load-bearing ────────────────────────

// A repair that genuinely worked but cannot be attributed is recordable history,
// never citable verification. The distinction is carried by the KIND, so the row
// cannot acquire the requirement even if a caller re-runs the mapper.
func TestEvidence_SuccessWithIncompleteLineageDoesNotBind(t *testing.T) {
	o := chain(t)
	o.ClusterID = "" // still IsSuccess(), no longer attributable

	ev, qualifies := mapOutcome(t, o)

	if qualifies {
		t.Fatal("a lineage-incomplete outcome must not qualify")
	}
	if ev.Kind != observation.KindRemediationDiagnostic {
		t.Errorf("kind = %q, want the non-qualifying diagnostic kind", ev.Kind)
	}
	if len(ev.Satisfies) != 0 {
		t.Fatalf("non-qualifying evidence must receive no binding, got %v", ev.Satisfies)
	}
	if ev.Result == observation.ResultFindingResolved {
		t.Error("a non-qualifying row must not carry the accepted result")
	}
	if ev.Metadata["not_qualifying_reason"] == "" {
		t.Error("an operator must be able to see WHY the row does not qualify")
	}
}

// Complete provenance around a repair that did not work is not success.
func TestEvidence_CompleteLineageButUnsuccessfulDoesNotBind(t *testing.T) {
	o := chain(t)
	o.FindingResolved = false // verified, still present → DEGRADED

	ev, qualifies := mapOutcome(t, o)

	if qualifies {
		t.Fatal("an unsuccessful remediation must not qualify however complete its lineage")
	}
	if !o.LineageComplete() {
		t.Fatalf("this case must isolate success, not lineage; defects=%v", o.LineageDefects())
	}
	if len(ev.Satisfies) != 0 {
		t.Fatalf("unsuccessful outcome must receive no binding, got %v", ev.Satisfies)
	}
}

// Dispatch accepted, finding still there. The action ran; nothing was fixed.
func TestEvidence_DispatchAcceptedButUnresolvedDoesNotQualify(t *testing.T) {
	o := chain(t)
	o.FindingResolved = false

	ev, _ := mapOutcome(t, o)
	if ev.Result == observation.ResultFindingResolved {
		t.Fatal("dispatch acceptance must never be reported as finding_resolved")
	}
	if res := evQualify(t, evSeed(t, ev), evQuery()); res.Satisfied() {
		t.Fatal("an unresolved finding must not satisfy a convergence requirement")
	}
}

// Verification stamped before the action cannot be evidence the action worked.
func TestEvidence_VerificationBeforeDispatchDoesNotQualify(t *testing.T) {
	o := chain(t)
	o.VerifiedAt = o.DispatchedAt.Add(-1 * time.Minute)

	ev, qualifies := mapOutcome(t, o)
	if qualifies {
		t.Fatal("verification predating dispatch must not qualify")
	}

	// Second line of defence: even if such a row reached the index, the rule
	// rejects it on the newer-than-dispatch check.
	forced := ev
	forced.Kind = observation.KindRemediationVerification
	forced.Result = observation.ResultFindingResolved
	forced.Satisfies = []api.RequiredEvidenceRef{api.RequiredEvidenceRef(evReq)}
	if res := evQualify(t, evSeed(t, forced), evQuery()); res.Satisfied() {
		t.Fatal("the rule must reject evidence observed before the action, not rely on the producer alone")
	}
}

// ── 3. mutation matrix ──────────────────────────────────────────────────────
//
// One field at a time, starting from the real qualifying artifact. Upstream
// fields are mutated on the OUTCOME so propagation is proven; shape fields the
// outcome does not own are mutated on the evidence AFTER binding, which forces
// the rule itself to discriminate rather than leaning on an empty index.

func TestEvidence_MutationMatrix(t *testing.T) {
	cases := []struct {
		field   string
		outcome func(*remediation.Outcome)
		post    func(*api.Evidence)
		query   func(*cluster_operator.SatisfactionQuery)
		// wantUnbound marks the cases where eligibility fails at construction:
		// the row is emitted under the diagnostic kind and never becomes a
		// candidate. Distinguished from wantReason so "the index was empty" can
		// never be mistaken for "the rule rejected it".
		wantUnbound bool
		wantReason  cluster_operator.RejectionReason
	}{
		// Cluster is part of the index partition — the row lands elsewhere and
		// is structurally unreachable rather than rejected.
		{field: "cluster_id", outcome: func(o *remediation.Outcome) { o.ClusterID = "cluster-b" }},

		{field: "invariant_id", outcome: func(o *remediation.Outcome) { o.InvariantID = "some.other.invariant" },
			wantReason: cluster_operator.RejectSubjectMismatch},
		{field: "entity_ref", outcome: func(o *remediation.Outcome) { o.EntityRef = "svc/other" },
			wantReason: cluster_operator.RejectSubjectMismatch},
		{field: "workflow_run", outcome: func(o *remediation.Outcome) { o.WorkflowRunID = "run-other" },
			wantReason: cluster_operator.RejectNotBoundToAction},
		// Persisted finding reference, as opposed to the eligibility case below:
		// a verification of a DIFFERENT finding — same cluster, entity,
		// invariant, run and recency — must not stand in for this one.
		{field: "finding_reference", post: func(e *api.Evidence) { e.SourceRef = "finding-earlier" },
			wantReason: cluster_operator.RejectSubjectMismatch},

		// Eligibility failures: no finding id, or an impossible dispatch/verify
		// ordering, leave the outcome unattributable.
		{field: "finding_lineage", outcome: func(o *remediation.Outcome) { o.FindingID = "" }, wantUnbound: true},
		{field: "dispatch_timestamp", outcome: func(o *remediation.Outcome) {
			// Dispatch after verification: the ordering lineage forbids.
			o.DispatchedAt = o.VerifiedAt.Add(1 * time.Minute)
		}, wantUnbound: true},
		{field: "finding_resolved", outcome: func(o *remediation.Outcome) { o.FindingResolved = false }, wantUnbound: true},

		// Verification time discriminates in both directions.
		{field: "verification_timestamp_stale",
			query:      func(q *cluster_operator.SatisfactionQuery) { q.EvaluatedAt = lnVerifyAt.Add(1 * time.Hour) },
			wantReason: cluster_operator.RejectEvidenceStale},
		{field: "verification_timestamp_in_future",
			outcome:    func(o *remediation.Outcome) { o.VerifiedAt = lnDispatchAt.Add(3 * time.Hour) },
			wantReason: cluster_operator.RejectTimestampInFuture},

		// Shape fields the outcome does not own, mutated after binding so the
		// row stays a candidate and the RULE has to do the rejecting.
		{field: "result", post: func(e *api.Evidence) { e.Result = "succeeded" },
			wantReason: cluster_operator.RejectResultNotAccepted},
		{field: "authority", post: func(e *api.Evidence) { e.AuthorityLevel = api.ObservationAuthorityDiagnostic },
			wantReason: cluster_operator.RejectAuthorityInsufficient},
		{field: "kind", post: func(e *api.Evidence) { e.Kind = "some_other_kind" },
			wantReason: cluster_operator.RejectSubjectMismatch},
		{field: "source", post: func(e *api.Evidence) { e.SourceKind = "some_other_source" },
			wantReason: cluster_operator.RejectSubjectMismatch},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			o := chain(t)
			if tc.outcome != nil {
				tc.outcome(&o)
			}
			ev, _ := mapOutcome(t, o)
			if tc.post != nil {
				tc.post(&ev)
			}

			if tc.wantUnbound {
				if len(ev.Satisfies) != 0 {
					t.Fatalf("mutating %s must fail eligibility at construction, got binding %v", tc.field, ev.Satisfies)
				}
			} else if len(ev.Satisfies) == 0 {
				t.Fatalf("mutating %s left the row unbound — it proves nothing about the rule", tc.field)
			}

			res := evQualify(t, evSeed(t, ev), evQuery(queryMuts(tc.query)...))
			if res.Satisfied() {
				t.Fatalf("mutating %s must break qualification — nothing enforces it", tc.field)
			}
			if tc.wantReason == "" {
				return
			}
			if len(res.Rejected) == 0 {
				t.Fatalf("mutating %s must produce a REJECTION (want %q), not an empty candidate set",
					tc.field, tc.wantReason)
			}
			if res.Rejected[0].Reason != tc.wantReason {
				t.Errorf("rejected for %q (%s), want %q",
					res.Rejected[0].Reason, res.Rejected[0].Detail, tc.wantReason)
			}
		})
	}
}

func queryMuts(f func(*cluster_operator.SatisfactionQuery)) []func(*cluster_operator.SatisfactionQuery) {
	if f == nil {
		return nil
	}
	return []func(*cluster_operator.SatisfactionQuery){f}
}

// Control: the unmutated artifact still qualifies, so the matrix above is
// proving discrimination rather than a uniformly broken fixture.
func TestEvidence_MutationControl(t *testing.T) {
	_, ev, qualifies := realEvidence(t)
	if !qualifies {
		t.Fatal("control must qualify")
	}
	if res := evQualify(t, evSeed(t, ev), evQuery()); !res.Satisfied() {
		t.Fatalf("control must qualify, otherwise the mutation tests prove nothing; rejected=%+v", res.Rejected)
	}
}

// ── 4. the action binding is mandatory, not best-effort ─────────────────────
//
// A caller who omits the action identity must get a rejection, not the more
// permissive answer. Failing open here would mean any fresh verification in the
// cluster could authorize any action.

func TestEvidence_QueryWithoutActionBindingIsRejected(t *testing.T) {
	_, ev, _ := realEvidence(t)
	s := evSeed(t, ev)

	res := evQualify(t, s, evQuery(func(q *cluster_operator.SatisfactionQuery) { q.WorkflowRunID = "" }))
	if res.Satisfied() {
		t.Fatal("a query with no workflow run must not qualify action-bound evidence")
	}
	if len(res.Rejected) == 0 || res.Rejected[0].Reason != cluster_operator.RejectNotBoundToAction {
		t.Errorf("want evidence_not_bound_to_action, got %+v", res.Rejected)
	}

	res = evQualify(t, s, evQuery(func(q *cluster_operator.SatisfactionQuery) { q.ActionDispatchedAt = time.Time{} }))
	if res.Satisfied() {
		t.Fatal("a query with no dispatch time cannot prove the evidence postdates the action")
	}
}

// Another run's verification must not satisfy this run's requirement, even when
// subject, cluster, result and recency all match.
func TestEvidence_AnotherRunsVerificationDoesNotCount(t *testing.T) {
	_, ev, _ := realEvidence(t)
	if res := evQualify(t, evSeed(t, ev),
		evQuery(func(q *cluster_operator.SatisfactionQuery) { q.WorkflowRunID = "run-elsewhere" })); res.Satisfied() {
		t.Fatal("evidence bound to one run must not satisfy a requirement about another")
	}
}
