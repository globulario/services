package cluster_operator

// The real satisfaction rules, reached the way CheckAction reaches them.
//
// The kernel seam is proven in behavioral/core; this proves the cluster half:
// that a real doctor-evidence row qualifies through QualifyRequirement, that the
// scope discriminates, and that every way of not knowing fails closed.
//
// Evidence is hand-built here ONLY in the fields the doctor producer emits —
// the producer-tethered proof lives in observation/lineage_test.go, which
// invokes observation.FromDoctorFinding directly. This file is about the
// ADAPTER, so it varies fields the producer holds constant.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

const (
	quProject = "globular-services"
	quCluster = "cluster-a"
	quEntity  = "svc/repository"
	quInvar   = "cluster.desired_state.absent"
	quFinding = "finding-1"
	quReq     = "evidence.doctor.finding_observed"
)

var (
	quEvalAt    = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	quObserveAt = quEvalAt.Add(-2 * time.Minute) // inside the rule's 15m window
)

// quEvidence mirrors what observation.FromDoctorFinding emits for a finding.
func quEvidence(mut ...func(*api.Evidence)) api.Evidence {
	e := api.Evidence{
		ID: "ev-doctor-1", Project: quProject, Domain: api.DomainRef(DomainName),
		Kind:           "cluster_doctor_evidence",
		SourceKind:     "cluster_doctor_evidence",
		Result:         "claim",
		SourceRef:      quFinding,
		EntityRef:      quEntity,
		ClusterID:      quCluster,
		ConditionRef:   quInvar,
		AuthorityLevel: api.ObservationAuthorityDerived,
		ObservedAt:     quObserveAt.Unix(),
	}
	BindSatisfies(&e)
	for _, f := range mut {
		f(&e)
	}
	return e
}

func quQuery(mut ...func(*domain.RequirementQuery)) domain.RequirementQuery {
	q := domain.RequirementQuery{
		Project: quProject, Domain: DomainName, RequirementID: quReq,
		ClusterID: quCluster, EntityRef: quEntity, ConditionRef: quInvar,
		SourceRef:   quFinding,
		ActionRef:   "run-abc",
		EvaluatedAt: quEvalAt.Unix(),
	}
	for _, f := range mut {
		f(&q)
	}
	return q
}

func quStore(t *testing.T, evs ...api.Evidence) store.Store {
	t.Helper()
	m := store.NewMemoryStore()
	for i := range evs {
		if err := m.PutEvidence(context.Background(), &evs[i]); err != nil {
			t.Fatalf("PutEvidence: %v", err)
		}
	}
	return m
}

func quQualify(t *testing.T, s store.Store, q domain.RequirementQuery) domain.RequirementVerdict {
	t.Helper()
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	v, err := p.QualifyRequirement(context.Background(), s, q)
	if err != nil {
		t.Fatalf("QualifyRequirement: %v", err)
	}
	return v
}

// ── the happy path ──────────────────────────────────────────────────────────

func TestQualifyRequirement_DoctorEvidenceSatisfiesPreActionRequirement(t *testing.T) {
	ev := quEvidence()
	v := quQualify(t, quStore(t, ev), quQuery())

	if !v.Satisfied {
		t.Fatalf("real doctor evidence must satisfy the pre-action requirement; reason=%s detail=%s",
			v.Reason, v.Detail)
	}
	// The verdict must name the row, so the ActionCheck can record what it
	// rested on rather than asserting a bare yes.
	if len(v.EvidenceIDs) != 1 || v.EvidenceIDs[0] != ev.ID {
		t.Errorf("EvidenceIDs = %v, want [%s]", v.EvidenceIDs, ev.ID)
	}
}

// The pre-action requirement must be satisfiable BEFORE anything is dispatched.
// A gate that needed post-action evidence to authorize the action could never
// open — the evidence only exists after the thing it was meant to authorize.
func TestQualifyRequirement_PreActionGateNeedsNoDispatchTime(t *testing.T) {
	v := quQualify(t, quStore(t, quEvidence()), quQuery(func(q *domain.RequirementQuery) {
		q.ActionDispatchedAt = 0
		q.ActionRef = "" // nothing has been dispatched, so nothing is bound yet
	}))
	if !v.Satisfied {
		t.Fatalf("the pre-action requirement must qualify with no dispatch: reason=%s detail=%s", v.Reason, v.Detail)
	}
}

// ── the scope discriminates ─────────────────────────────────────────────────

func TestQualifyRequirement_ScopeDiscriminates(t *testing.T) {
	cases := []struct {
		name   string
		query  func(*domain.RequirementQuery)
		reason RejectionReason
		// absent marks the cases where the row is not even a candidate.
		absent bool
	}{
		{name: "another cluster", query: func(q *domain.RequirementQuery) { q.ClusterID = "cluster-b" }, absent: true},
		{name: "another entity", query: func(q *domain.RequirementQuery) { q.EntityRef = "svc/other" },
			reason: RejectSubjectMismatch},
		{name: "another invariant", query: func(q *domain.RequirementQuery) { q.ConditionRef = "other.invariant" },
			reason: RejectSubjectMismatch},
		{name: "no entity supplied", query: func(q *domain.RequirementQuery) { q.EntityRef = "" },
			reason: RejectSubjectMismatch},
		{name: "no condition supplied", query: func(q *domain.RequirementQuery) { q.ConditionRef = "" },
			reason: RejectSubjectMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := quQualify(t, quStore(t, quEvidence()), quQuery(tc.query))
			if v.Satisfied {
				t.Fatalf("%s must not satisfy", tc.name)
			}
			if tc.absent {
				if v.Reason != "" {
					t.Errorf("evidence in another cluster is not a candidate at all, got reason %q", v.Reason)
				}
				return
			}
			if v.Reason != string(tc.reason) {
				t.Errorf("reason = %q (%s), want %q", v.Reason, v.Detail, tc.reason)
			}
		})
	}
}

// An empty scope field is a failure to qualify, never a wildcard. Widening on
// absence is how an unrelated cluster's evidence starts authorizing actions.
func TestQualifyRequirement_EmptyClusterIsNotAWildcard(t *testing.T) {
	v := quQualify(t, quStore(t, quEvidence()), quQuery(func(q *domain.RequirementQuery) { q.ClusterID = "" }))
	if v.Satisfied {
		t.Fatal("an empty cluster must select the cluster-less partition, never match every cluster")
	}
}

// Lowering the authority below the rule's floor must unsatisfy the requirement.
func TestQualifyRequirement_InsufficientAuthority(t *testing.T) {
	ev := quEvidence(func(e *api.Evidence) { e.AuthorityLevel = api.ObservationAuthorityInterpretation })
	v := quQualify(t, quStore(t, ev), quQuery())

	if v.Satisfied {
		t.Fatal("evidence below the authority floor must not satisfy")
	}
	if v.Reason != string(RejectAuthorityInsufficient) {
		t.Errorf("reason = %q (%s), want %q", v.Reason, v.Detail, RejectAuthorityInsufficient)
	}
}

func TestQualifyRequirement_StaleEvidence(t *testing.T) {
	ev := quEvidence(func(e *api.Evidence) { e.ObservedAt = quEvalAt.Add(-2 * time.Hour).Unix() })
	v := quQualify(t, quStore(t, ev), quQuery())
	if v.Satisfied {
		t.Fatal("evidence outside the freshness window must not satisfy")
	}
	if v.Reason != string(RejectEvidenceStale) {
		t.Errorf("reason = %q, want %q", v.Reason, RejectEvidenceStale)
	}
}

// Nothing stored at all is distinguishable from stored-and-refused: an operator
// told to gather evidence that is already there has been misdirected.
func TestQualifyRequirement_NothingStoredCarriesNoRejection(t *testing.T) {
	v := quQualify(t, quStore(t), quQuery())
	if v.Satisfied {
		t.Fatal("no evidence must not satisfy")
	}
	if v.Reason != "" {
		t.Errorf("an empty store yields no rejection reason, got %q", v.Reason)
	}
}

// ── failing closed ──────────────────────────────────────────────────────────

// A requirement no rule describes is UNSATISFIED, not unconstrained. Treating it
// as waived would let a principle read as satisfied on the strength of nothing.
func TestQualifyRequirement_UndeclaredRequirementIsUnsatisfied(t *testing.T) {
	v := quQualify(t, quStore(t, quEvidence()), quQuery(func(q *domain.RequirementQuery) {
		q.RequirementID = "evidence.cluster.etcd.member_health" // declared in the pack, no satisfaction rule
	}))
	if v.Satisfied {
		t.Fatal("a requirement with no satisfaction rule must never be treated as satisfied")
	}
	if v.Reason != string(RejectRequirementNotDeclared) {
		t.Errorf("reason = %q, want %q — the operator must see a CATALOG gap, "+
			"not be sent to gather evidence no rule could accept", v.Reason, RejectRequirementNotDeclared)
	}
}

// An unreadable store must error, never return a quiet false.
func TestQualifyRequirement_UnreadableStoreErrors(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, err := p.QualifyRequirement(context.Background(), store.Unconfigured{}, quQuery()); err == nil {
		t.Fatal("an unreadable store must produce an error — a governor that cannot read " +
			"its evidence has not learned the evidence is absent")
	}
}

// The clock must be injected. A verdict that silently used wall-clock time would
// be irreproducible for the same stored state.
func TestQualifyRequirement_RequiresInjectedClock(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	_, err = p.QualifyRequirement(context.Background(), quStore(t, quEvidence()),
		quQuery(func(q *domain.RequirementQuery) { q.EvaluatedAt = 0 }))
	if err == nil {
		t.Fatal("a missing clock must error rather than default to now")
	}
	if !strings.Contains(err.Error(), "evaluated_at") {
		t.Errorf("the error must name the missing input, got %v", err)
	}
}

// The pack satisfies the kernel's optional interface, so the seam engages.
func TestQualifyRequirement_PackImplementsTheKernelSeam(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	var d domain.Domain = p
	if _, ok := d.(domain.EvidenceQualifier); !ok {
		t.Fatal("the pack must satisfy domain.EvidenceQualifier, or CheckAction silently " +
			"falls back to the weaker default lane")
	}
}

// The post-action requirement must NOT be satisfiable before dispatch — it is
// outcome proof, not authorization for the action that produces it.
func TestQualifyRequirement_PostActionRequirementCannotAuthorizeItsOwnAction(t *testing.T) {
	v := quQualify(t, quStore(t, quEvidence()), quQuery(func(q *domain.RequirementQuery) {
		q.RequirementID = "evidence.remediation.fresh_convergence_verification"
		q.ActionDispatchedAt = 0
	}))
	if v.Satisfied {
		t.Fatal("post-action verification evidence must never authorize the action it verifies")
	}
	if v.Reason == string(RejectRequirementNotDeclared) {
		t.Fatal("the requirement IS declared — this must fail on evidence, not on a catalog gap")
	}
}

// The kernel keys its self-assertion fallback on this exact string. If the two
// drift, a requirement this domain RULED on would silently become assertable —
// a caller could then bypass every discriminator by claiming to hold evidence.
func TestQualifyRequirement_NotDeclaredMatchesKernelContract(t *testing.T) {
	if string(RejectRequirementNotDeclared) != domain.ReasonRequirementNotDeclared {
		t.Fatalf("reason code drift: pack %q vs kernel contract %q",
			RejectRequirementNotDeclared, domain.ReasonRequirementNotDeclared)
	}
}

var _ = errors.New
