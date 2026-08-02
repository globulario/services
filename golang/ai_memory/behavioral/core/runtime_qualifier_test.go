package core

// Commit 5: CheckAction resolves required evidence through the domain pack's
// satisfaction rules.
//
// Before this, the evidence lane listed rows whose TargetID happened to be the
// principle and accepted any Satisfies match. That answers "does evidence for
// this ref exist somewhere in the project" — not "does evidence that COUNTS
// exist, for this cluster, this subject, this action, now". Every discriminator
// built in 4A–4D was unreachable from the gate.
//
// These tests are about the SEAM: that the pack is consulted, that its verdict
// decides, that the scope reaches it intact, and above all that every way of not
// knowing fails closed. The cluster-semantics themselves are proven in
// domains/cluster_operator.

import (
	"context"
	"errors"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

const (
	qProject = "p"
	qDomain  = "test_domain"
	qCond    = "condition.test.applies"
	qReq     = "evidence.test.required"
	qAction  = "test.restart_service"
)

// fakeQualifier is a domain pack that also owns satisfaction rules. It records
// what the kernel asked so the scope can be asserted end to end.
type fakeQualifier struct {
	catalogs domain.Catalogs
	verdict  domain.RequirementVerdict
	err      error
	seen     []domain.RequirementQuery
}

func (f *fakeQualifier) Name() string              { return qDomain }
func (f *fakeQualifier) Catalogs() domain.Catalogs { return f.catalogs }

func (f *fakeQualifier) QualifyRequirement(_ context.Context, _ store.Store, q domain.RequirementQuery) (domain.RequirementVerdict, error) {
	f.seen = append(f.seen, q)
	return f.verdict, f.err
}

// plainPack is a domain pack WITHOUT satisfaction rules, so the kernel's older
// evidence lane applies.
type plainPack struct{ catalogs domain.Catalogs }

func (p *plainPack) Name() string              { return qDomain }
func (p *plainPack) Catalogs() domain.Catalogs { return p.catalogs }

// qSetup registers a promoted principle requiring qReq under condition qCond.
func qSetup(t *testing.T, d domain.Domain) (*Service, store.Store) {
	t.Helper()
	st := store.NewMemoryStore()
	reg := domain.NewRegistry()
	if d != nil {
		reg.Register(d)
	}
	svc := New(st, reg)

	p := &api.Principle{
		ID: "principle.test.governed", Project: qProject, Domain: api.DomainRef(qDomain),
		Title:            "governed test action",
		AppliesWhen:      []api.ConditionRef{qCond},
		RequiredEvidence: []api.RequiredEvidenceRef{qReq},
		RiskLevel:        "low",
		Status:           api.StatusPromotedPrinciple,
	}
	if err := st.CreatePrinciple(context.Background(), p); err != nil {
		t.Fatalf("CreatePrinciple: %v", err)
	}
	if err := st.IndexPromotedPrinciple(context.Background(), p); err != nil {
		t.Fatalf("IndexPromotedPrinciple: %v", err)
	}
	return svc, st
}

func qCheck(t *testing.T, svc *Service, mut ...func(*api.CheckActionRequest)) *api.CheckActionResponse {
	t.Helper()
	req := &api.CheckActionRequest{
		Project: qProject, Domain: api.DomainRef(qDomain),
		ActionType:        qAction,
		CurrentConditions: []api.ConditionRef{qCond},
		ActionScope: api.ActionScope{
			ClusterID: "cluster-a", EntityRef: "svc/repository",
			ConditionRef: "cluster.desired_state.absent", SourceRef: "finding-1",
			ActionRef: "run-abc",
		},
		EvaluatedAt: 1785555000,
	}
	for _, f := range mut {
		f(req)
	}
	resp, err := svc.CheckAction(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckAction: %v", err)
	}
	return resp
}

// ── the seam is used ────────────────────────────────────────────────────────

func TestCheckAction_QualifierDecidesAllow(t *testing.T) {
	q := &fakeQualifier{verdict: domain.RequirementVerdict{Satisfied: true, EvidenceIDs: []string{"ev-1"}}}
	svc, _ := qSetup(t, q)

	resp := qCheck(t, svc)

	if !resp.Result.Allowed || resp.Result.Status != "allowed" {
		t.Fatalf("a satisfied requirement must allow; status=%q missing=%v",
			resp.Result.Status, resp.Result.MissingEvidence)
	}
	if !resp.Result.Governed {
		t.Error("an applicable promoted principle was evaluated — the check is governed")
	}
	// The verdict must name what it rested on. An allow that cannot say which
	// evidence satisfied it is an assertion, not an audit record.
	if got := resp.Result.Metadata["qualified_evidence"]; got != qReq+"=ev-1" {
		t.Errorf("qualified_evidence = %q, want %q", got, qReq+"=ev-1")
	}
	if got := resp.Result.Metadata["evidence_lane"]; got != "domain_qualifier" {
		t.Errorf("evidence_lane = %q, want domain_qualifier", got)
	}
}

func TestCheckAction_QualifierDecidesBlock(t *testing.T) {
	q := &fakeQualifier{verdict: domain.RequirementVerdict{
		Satisfied: false, Reason: "cluster_mismatch", Detail: `cluster: want "cluster-a" got "cluster-b"`,
	}}
	svc, _ := qSetup(t, q)

	resp := qCheck(t, svc)

	if resp.Result.Allowed {
		t.Fatal("an unsatisfied requirement must not allow")
	}
	if resp.Result.Status != "needs_evidence" {
		t.Errorf("status = %q, want needs_evidence", resp.Result.Status)
	}
	// Present-but-disqualified must not read as absent: an operator told to
	// "gather" evidence that is already stored has been sent to do the wrong
	// thing.
	steps := resp.Result.RecommendedSteps
	if len(steps) == 0 || !contains(steps[0], "does not qualify") || !contains(steps[0], "cluster_mismatch") {
		t.Errorf("the recommendation must say the evidence exists and why it was refused, got %v", steps)
	}
}

// The scope must reach the pack intact. A gate that drops the subject would ask
// about the wrong thing and get a confident, wrong answer.
func TestCheckAction_ScopeReachesTheQualifier(t *testing.T) {
	q := &fakeQualifier{verdict: domain.RequirementVerdict{Satisfied: true}}
	svc, _ := qSetup(t, q)
	qCheck(t, svc)

	if len(q.seen) != 1 {
		t.Fatalf("the pack must be asked exactly once per requirement, got %d", len(q.seen))
	}
	got := q.seen[0]
	for _, c := range []struct{ name, got, want string }{
		{"project", got.Project, qProject},
		{"domain", got.Domain, qDomain},
		{"requirement", got.RequirementID, qReq},
		{"cluster", got.ClusterID, "cluster-a"},
		{"entity", got.EntityRef, "svc/repository"},
		{"condition", got.ConditionRef, "cluster.desired_state.absent"},
		{"finding", got.SourceRef, "finding-1"},
		{"action", got.ActionRef, "run-abc"},
	} {
		if c.got != c.want {
			t.Errorf("%s reached the pack as %q, want %q", c.name, c.got, c.want)
		}
	}
	if got.EvaluatedAt != 1785555000 {
		t.Errorf("evaluated_at = %d, want the injected clock 1785555000", got.EvaluatedAt)
	}
	// A PRE-action gate has not dispatched anything.
	if got.ActionDispatchedAt != 0 {
		t.Errorf("a pre-action check must not claim a dispatch time, got %d", got.ActionDispatchedAt)
	}
}

// ── failing closed ──────────────────────────────────────────────────────────

// A store failure inside the pack must surface as an error, never as a verdict.
// The alternative — treating "I could not read the evidence" as "the evidence is
// absent" — is the milder-looking half of the same bug: both let the gate speak
// with confidence it has not earned, and the block direction at least does not
// authorize anything. But the caller must still learn the gate was degraded, so
// it can apply its own safety rules rather than believing a governed verdict.
func TestCheckAction_QualifierErrorFailsClosedAsError(t *testing.T) {
	q := &fakeQualifier{err: errors.New("scylla unavailable")}
	svc, _ := qSetup(t, q)

	_, err := svc.CheckAction(context.Background(), &api.CheckActionRequest{
		Project: qProject, Domain: api.DomainRef(qDomain), ActionType: qAction,
		CurrentConditions: []api.ConditionRef{qCond},
		EvaluatedAt:       1785555000,
	})
	if err == nil {
		t.Fatal("an unreadable evidence store must produce an ERROR, not a verdict — " +
			"a gate that cannot read its evidence has not learned the evidence is absent")
	}
	if !contains(err.Error(), qReq) {
		t.Errorf("the error must name the requirement it could not answer, got %v", err)
	}
}

// A satisfied verdict returned alongside an error must never be honoured.
func TestCheckAction_ErrorWinsOverSatisfied(t *testing.T) {
	q := &fakeQualifier{
		verdict: domain.RequirementVerdict{Satisfied: true},
		err:     errors.New("partial read"),
	}
	svc, _ := qSetup(t, q)

	if _, err := svc.CheckAction(context.Background(), &api.CheckActionRequest{
		Project: qProject, Domain: api.DomainRef(qDomain), ActionType: qAction,
		CurrentConditions: []api.ConditionRef{qCond}, EvaluatedAt: 1785555000,
	}); err == nil {
		t.Fatal("an error must win over a Satisfied verdict returned with it")
	}
}

// ── self-assertion must not override a judgement ────────────────────────────

// A caller claiming to hold evidence must not be able to overturn a rule that
// already looked and said no. Otherwise every discriminator a satisfaction rule
// exists to apply — cluster, subject, authority, recency, action binding — is
// bypassable by a caller that simply asserts the requirement.
func TestCheckAction_SelfAssertionCannotOverrideARuledRequirement(t *testing.T) {
	q := &fakeQualifier{verdict: domain.RequirementVerdict{
		Satisfied: false, Reason: "authority_insufficient", Detail: "floor DERIVED_EVIDENCE got INTERPRETATION",
	}}
	svc, _ := qSetup(t, q)

	resp := qCheck(t, svc, func(r *api.CheckActionRequest) {
		r.ProvidedEvidenceRefs = []string{qReq}
	})

	if resp.Result.Allowed {
		t.Fatal("a requirement the domain RULED unsatisfied must not become satisfied by assertion")
	}
	if _, flagged := resp.Result.Metadata["self_asserted_evidence"]; flagged {
		t.Error("the assertion must be refused outright, not recorded as an accepted weaker lane")
	}
}

// Where the domain expresses NO opinion, self-assertion stays available — with
// its provenance caveat intact. The distinction is the point: a judged
// requirement is closed, an unjudged one is not.
func TestCheckAction_SelfAssertionStillCoversAnUnjudgedRequirement(t *testing.T) {
	q := &fakeQualifier{verdict: domain.RequirementVerdict{
		Satisfied: false, Reason: domain.ReasonRequirementNotDeclared,
	}}
	svc, _ := qSetup(t, q)

	resp := qCheck(t, svc, func(r *api.CheckActionRequest) {
		r.ProvidedEvidenceRefs = []string{qReq}
	})

	if !resp.Result.Allowed {
		t.Fatalf("an unjudged requirement remains assertable; status=%q", resp.Result.Status)
	}
	if resp.Result.Metadata["self_asserted_evidence"] != qReq {
		t.Errorf("the weaker provenance must survive onto the audit row, got %v", resp.Result.Metadata)
	}
}

// ── the older lane still works for packs without rules ──────────────────────

func TestCheckAction_PackWithoutRulesKeepsTheDefaultLane(t *testing.T) {
	svc, st := qSetup(t, &plainPack{})

	// The default lane accepts a recorded row TARGETING the principle.
	ev := &api.Evidence{
		ID: "ev-legacy", Project: qProject, Domain: api.DomainRef(qDomain),
		TargetKind: "principle", TargetID: "principle.test.governed",
		Satisfies: []api.RequiredEvidenceRef{qReq},
	}
	if err := st.PutEvidence(context.Background(), ev); err != nil {
		t.Fatalf("PutEvidence: %v", err)
	}

	resp := qCheck(t, svc)
	if !resp.Result.Allowed {
		t.Fatalf("a pack declaring no satisfaction rules must keep the original lane; status=%q",
			resp.Result.Status)
	}
	if resp.Result.Metadata["evidence_lane"] == "domain_qualifier" {
		t.Error("the strict lane must not be reported when the pack owns no rules")
	}
}

// With no pack at all the kernel still functions — and still blocks, because
// nothing satisfied the requirement.
func TestCheckAction_NoPackStillBlocksOnMissingEvidence(t *testing.T) {
	svc, _ := qSetup(t, nil)
	resp := qCheck(t, svc)
	if resp.Result.Allowed {
		t.Fatal("no pack and no evidence must not allow")
	}
	if resp.Result.Status != "needs_evidence" {
		t.Errorf("status = %q, want needs_evidence", resp.Result.Status)
	}
}

// ── ungoverned ──────────────────────────────────────────────────────────────

// No applicable principle: the pack is never consulted, the check is recorded as
// ungoverned, and no governed verdict is fabricated.
func TestCheckAction_UngovernedDoesNotConsultThePack(t *testing.T) {
	q := &fakeQualifier{verdict: domain.RequirementVerdict{Satisfied: true}}
	svc, _ := qSetup(t, q)

	resp := qCheck(t, svc, func(r *api.CheckActionRequest) {
		r.CurrentConditions = []api.ConditionRef{"condition.test.unrelated"}
	})

	if resp.Result.Governed {
		t.Fatal("no applicable principle — the check must be recorded as UNGOVERNED")
	}
	if len(q.seen) != 0 {
		t.Errorf("the pack must not be asked when no principle is engaged, got %d calls", len(q.seen))
	}
	if !resp.Result.Allowed {
		t.Error("an ungoverned action keeps the existing default; the caller's own gates decide")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
