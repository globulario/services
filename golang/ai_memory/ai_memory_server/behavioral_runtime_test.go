package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

const condNospace = "cond.nospace"

// promoteGoodPrinciple seeds catalog + proposes goodPrinciple + evidence +
// promotes it (ALLOWED), returning the principle id.
func promoteGoodPrinciple(t *testing.T, st *store.MemoryStore, h *behavioralHandler) string {
	t.Helper()
	id := fullySetup(t, st, h, nil)
	resp := promote(t, h, id, "")
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_ALLOWED {
		t.Fatalf("setup promote: decision=%v (%s)", resp.GetDecision(), resp.GetRecord().GetReason())
	}
	return id
}

// seedPromotedPrinciple directly persists a PROMOTED principle and indexes it —
// store-level fixture for CheckAction verdict tests (bypasses the gate so a
// specific gap can be isolated).
func seedPromotedPrinciple(t *testing.T, st *store.MemoryStore, p *api.Principle) {
	t.Helper()
	ctx := context.Background()
	p.Project, p.Domain = testProject, testDomain
	p.Status = api.StatusPromotedPrinciple
	if p.AppliesWhen == nil {
		p.AppliesWhen = []api.ConditionRef{condNospace}
	}
	if err := st.CreatePrinciple(ctx, p); err != nil {
		t.Fatalf("seed principle: %v", err)
	}
	if err := st.IndexPromotedPrinciple(ctx, p); err != nil {
		t.Fatalf("index principle: %v", err)
	}
}

func checkAction(t *testing.T, h *behavioralHandler, req *bpb.CheckActionRequest) *bpb.ActionCheck {
	t.Helper()
	if req.Project == "" {
		req.Project = testProject
	}
	if req.Domain == "" {
		req.Domain = testDomain
	}
	resp, err := h.CheckAction(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckAction: %v", err)
	}
	return resp.GetResult()
}

// §2 PromotePrinciple inserts principles_by_condition entries.
func TestPromoteInsertsConditionIndex(t *testing.T) {
	st, h := newGovHandler()
	id := promoteGoodPrinciple(t, st, h)
	ids, err := st.ListPrincipleIDsByCondition(context.Background(), testProject, testDomain, condNospace)
	if err != nil {
		t.Fatalf("ListPrincipleIDsByCondition: %v", err)
	}
	if len(ids) != 1 || ids[0] != id {
		t.Errorf("index = %v, want [%q]", ids, id)
	}
}

// §2 RevokePrinciple removes the promoted condition lookup.
func TestRevokeRemovesConditionIndex(t *testing.T) {
	st, h := newGovHandler()
	id := promoteGoodPrinciple(t, st, h)
	if _, err := h.RevokePrinciple(context.Background(), &bpb.RevokePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, Action: "REVOKED", Reason: "x",
	}); err != nil {
		t.Fatalf("RevokePrinciple: %v", err)
	}
	ids, _ := st.ListPrincipleIDsByCondition(context.Background(), testProject, testDomain, condNospace)
	if len(ids) != 0 {
		t.Errorf("index after revoke = %v, want empty", ids)
	}
}

// §3 ResolveGovernedContext returns applicable promoted principles for a condition.
func TestResolveReturnsApplicablePromoted(t *testing.T) {
	st, h := newGovHandler()
	id := promoteGoodPrinciple(t, st, h)
	resp, err := h.ResolveGovernedContext(context.Background(), &bpb.ResolveGovernedContextRequest{
		Project: testProject, Domain: testDomain, Conditions: []string{condNospace},
	})
	if err != nil {
		t.Fatalf("ResolveGovernedContext: %v", err)
	}
	c := resp.GetContext()
	if len(c.GetApplicablePrinciples()) != 1 || c.GetApplicablePrinciples()[0].GetId() != id {
		t.Fatalf("applicable principles = %d, want 1 (%q)", len(c.GetApplicablePrinciples()), id)
	}
	// returns required evidence, authorities (on principle), forbidden moves.
	if len(c.GetRequiredEvidence()) == 0 {
		t.Error("expected required evidence refs in bundle")
	}
	if len(c.GetForbiddenMoves()) == 0 {
		t.Error("expected forbidden move refs in bundle")
	}
	if len(c.GetApplicablePrinciples()[0].GetAuthorities()) == 0 {
		t.Error("expected authority refs on applicable principle")
	}
}

// §3 ResolveGovernedContext ignores non-promoted principles.
func TestResolveIgnoresNonPromoted(t *testing.T) {
	st, h := newGovHandler()
	// One promoted (indexed), one merely proposed (never indexed), same condition.
	promoted := promoteGoodPrinciple(t, st, h)
	_ = propose(t, h, goodPrinciple()) // proposed only, not promoted, not indexed

	resp, _ := h.ResolveGovernedContext(context.Background(), &bpb.ResolveGovernedContextRequest{
		Project: testProject, Domain: testDomain, Conditions: []string{condNospace},
	})
	got := resp.GetContext().GetApplicablePrinciples()
	if len(got) != 1 || got[0].GetId() != promoted {
		t.Errorf("applicable = %d, want only the promoted principle %q", len(got), promoted)
	}
}

// Defensive: a stale index entry pointing at a non-promoted principle is excluded.
func TestResolveExcludesStaleIndexedNonPromoted(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()
	p := &api.Principle{ID: "stale-1", Project: testProject, Domain: testDomain, AppliesWhen: []api.ConditionRef{condNospace}, Status: api.StatusRevoked}
	_ = st.CreatePrinciple(ctx, p)
	_ = st.IndexPromotedPrinciple(ctx, p) // force a stale (revoked) index entry
	resp, _ := h.ResolveGovernedContext(ctx, &bpb.ResolveGovernedContextRequest{
		Project: testProject, Domain: testDomain, Conditions: []string{condNospace},
	})
	if len(resp.GetContext().GetApplicablePrinciples()) != 0 {
		t.Error("revoked principle returned despite stale index entry")
	}
}

// §3 bundle includes open contradictions touching applicable principles.
func TestResolveReturnsContradictions(t *testing.T) {
	st, h := newGovHandler()
	id := promoteGoodPrinciple(t, st, h)
	if _, err := h.RecordContradiction(context.Background(), &bpb.RecordContradictionRequest{
		Contradiction: &bpb.Contradiction{Project: testProject, Domain: testDomain, Kind: "rule_conflict", LeftRef: id, RightRef: "other"},
	}); err != nil {
		t.Fatalf("RecordContradiction: %v", err)
	}
	resp, _ := h.ResolveGovernedContext(context.Background(), &bpb.ResolveGovernedContextRequest{
		Project: testProject, Domain: testDomain, Conditions: []string{condNospace},
	})
	if len(resp.GetContext().GetKnownContradictions()) == 0 {
		t.Error("expected known (open) contradictions in bundle")
	}
}

// §4 CheckAction returns blocked when a forbidden move matches the action.
func TestCheckActionBlockedOnForbidden(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-forbid", ForbiddenMoves: []api.ForbiddenMoveRef{"forbid.restart"}, RiskLevel: "low"})
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "forbid.restart", CurrentConditions: []string{condNospace}})
	if ac.GetStatus() != "blocked" || ac.GetAllowed() {
		t.Errorf("status=%q allowed=%v, want blocked", ac.GetStatus(), ac.GetAllowed())
	}
	if len(ac.GetForbiddenMatched()) == 0 {
		t.Error("expected forbidden_matched populated")
	}
}

// §4 CheckAction returns needs_evidence when required evidence is missing.
func TestCheckActionNeedsEvidence(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-ev", RequiredEvidence: []api.RequiredEvidenceRef{"req.alarm"}, RiskLevel: "low"})
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "do-thing", CurrentConditions: []string{condNospace}})
	if ac.GetStatus() != "needs_evidence" {
		t.Errorf("status=%q, want needs_evidence", ac.GetStatus())
	}
	if len(ac.GetMissingEvidence()) == 0 {
		t.Error("expected missing_evidence populated")
	}
}

// §4 CheckAction returns needs_authority when authority is unresolved.
func TestCheckActionNeedsAuthority(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	// authority "auth.missing" is never seeded → unresolvable.
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-auth", Authorities: []api.AuthorityRef{"auth.missing"}, RiskLevel: "low"})
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "do-thing", CurrentConditions: []string{condNospace}})
	if ac.GetStatus() != "needs_authority" {
		t.Errorf("status=%q, want needs_authority", ac.GetStatus())
	}
	if len(ac.GetUnresolvedAuthority()) == 0 {
		t.Error("expected unresolved_authority populated")
	}
}

// §4 CheckAction returns needs_human_approval for high/irreversible risk w/o approval.
func TestCheckActionNeedsHumanApproval(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-risk", RiskLevel: "irreversible"})
	// no approval → needs_human_approval
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "do-thing", CurrentConditions: []string{condNospace}})
	if ac.GetStatus() != "needs_human_approval" {
		t.Fatalf("status=%q, want needs_human_approval", ac.GetStatus())
	}
	// with approval → allowed
	ac2 := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "do-thing", CurrentConditions: []string{condNospace}, HumanApproval: "operator-dave"})
	if ac2.GetStatus() != "allowed" {
		t.Errorf("status with approval=%q, want allowed", ac2.GetStatus())
	}
}

// §4 CheckAction returns allowed when all requirements are satisfied.
func TestCheckActionAllowed(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()
	_ = st.PutCondition(ctx, &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	_ = st.PutAuthority(ctx, &api.Authority{ID: "auth.etcd", Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{
		ID: "p-ok", Authorities: []api.AuthorityRef{"auth.etcd"}, RequiredEvidence: []api.RequiredEvidenceRef{"req.alarm"}, RiskLevel: "low",
	})
	ac := checkAction(t, h, &bpb.CheckActionRequest{
		ActionType: "do-thing", CurrentConditions: []string{condNospace}, ProvidedEvidenceRefs: []string{"req.alarm"},
	})
	if ac.GetStatus() != "allowed" || !ac.GetAllowed() {
		t.Errorf("status=%q allowed=%v, want allowed", ac.GetStatus(), ac.GetAllowed())
	}
}

// §4 CheckAction persists an action_checks audit row for every verdict.
func TestCheckActionPersistsAuditRow(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-audit", ForbiddenMoves: []api.ForbiddenMoveRef{"forbid.x"}, RiskLevel: "low"})
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "forbid.x", CurrentConditions: []string{condNospace}})
	got, err := st.GetActionCheck(context.Background(), testProject, testDomain, ac.GetId())
	if err != nil {
		t.Fatalf("action_checks row not persisted: %v", err)
	}
	if got.Status != "blocked" || got.ActionType != "forbid.x" {
		t.Errorf("persisted audit row = %+v, want blocked/forbid.x", got)
	}
}

// §5 RecordOutcome persists outcome + maintains outcomes_by_theme; supports
// success/failure/severe/human_marked.
func TestRecordOutcomePersistsAndIndexesByTheme(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()
	resp, err := h.RecordOutcome(ctx, &bpb.RecordOutcomeRequest{
		Outcome: &bpb.Outcome{
			Project: testProject, Domain: testDomain, ActionCheckId: "ac-1", Status: "failure",
			Severe: true, HumanMarked: true, IncidentId: "INC-1", Theme: "etcd.nospace",
			SupportsPrinciples: []string{"p1"}, WeakensPrinciples: []string{"p2"},
		},
	})
	if err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}
	got, err := st.GetOutcome(ctx, testProject, testDomain, resp.GetOutcomeId())
	if err != nil {
		t.Fatalf("GetOutcome: %v", err)
	}
	if got.Status != "failure" || !got.Severe || !got.HumanMarked || got.IncidentID != "INC-1" {
		t.Errorf("outcome = %+v, want failure/severe/human_marked/INC-1", got)
	}
	// outcomes_by_theme maintained
	list, err := st.ListOutcomesByTheme(ctx, testProject, testDomain, "etcd.nospace")
	if err != nil || len(list) != 1 || list[0].ID != resp.GetOutcomeId() {
		t.Errorf("outcomes_by_theme = %+v err %v, want one entry", list, err)
	}
}

// §10 No CheckAction or ResolveGovernedContext path promotes or revokes.
func TestRuntimeRPCsDoNotMutatePrincipleStatus(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()
	id := promoteGoodPrinciple(t, st, h)
	before, _ := st.GetPrinciple(ctx, testProject, testDomain, id)

	_, _ = h.CheckAction(ctx, &bpb.CheckActionRequest{Project: testProject, Domain: testDomain, ActionType: "x", CurrentConditions: []string{condNospace}})
	_, _ = h.ResolveGovernedContext(ctx, &bpb.ResolveGovernedContextRequest{Project: testProject, Domain: testDomain, Conditions: []string{condNospace}})
	_, _ = h.RecordOutcome(ctx, &bpb.RecordOutcomeRequest{Outcome: &bpb.Outcome{Project: testProject, Domain: testDomain, Status: "success", Theme: "t"}})

	after, _ := st.GetPrinciple(ctx, testProject, testDomain, id)
	if after.Status != before.Status || after.Status != api.StatusPromotedPrinciple {
		t.Errorf("runtime RPC mutated principle status: before=%q after=%q", before.Status, after.Status)
	}
}
