package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// goodPrinciple returns a proposal that passes ALL promotion-gate checks (once
// its authority/condition catalog rows and evidence are seeded). Tests mutate a
// single field to exercise each block path.
func goodPrinciple() *bpb.Principle {
	return &bpb.Principle{
		Project: testProject, Domain: testDomain, Title: "etcd nospace handling",
		AppliesWhen:      []string{"cond.nospace"},
		Authorities:      []string{"auth.etcd"},
		RequiredEvidence: []string{"req.alarm"},
		ForbiddenMoves:   []string{"forbid.restart_before_quorum"},
		RecommendedAction: "inspect alarms, defrag only if safe",
		RiskLevel:         "low",
		RevocationRule:    "narrow if a newer etcd version changes NOSPACE semantics",
		PromotionReason:   "repeated NOSPACE incidents handled this way",
		ProposedBy:        "agent-1",
		ContradictionChecked: true,
	}
}

func seedCatalog(t *testing.T, st *store.MemoryStore) {
	t.Helper()
	ctx := context.Background()
	if err := st.PutAuthority(ctx, &api.Authority{ID: "auth.etcd", Project: testProject, Domain: testDomain}); err != nil {
		t.Fatalf("seed authority: %v", err)
	}
	if err := st.PutCondition(ctx, &api.Condition{ID: "cond.nospace", Project: testProject, Domain: testDomain}); err != nil {
		t.Fatalf("seed condition: %v", err)
	}
}

func propose(t *testing.T, h *behavioralHandler, p *bpb.Principle) string {
	t.Helper()
	resp, err := h.ProposePrinciple(context.Background(), &bpb.ProposePrincipleRequest{Principle: p})
	if err != nil {
		t.Fatalf("ProposePrinciple: %v", err)
	}
	return resp.GetPrincipleId()
}

func seedEvidenceFor(t *testing.T, h *behavioralHandler, principleID string) {
	t.Helper()
	_, err := h.RecordEvidence(context.Background(), &bpb.RecordEvidenceRequest{
		Evidence: &bpb.Evidence{
			Project: testProject, Domain: testDomain, TargetKind: "principle", TargetId: principleID,
			EvidenceKind: "probe", Lane: bpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED, Result: "pass",
		},
	})
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
}

// fullySetup seeds catalog + proposes goodPrinciple + records evidence, returning id.
func fullySetup(t *testing.T, st *store.MemoryStore, h *behavioralHandler, mutate func(*bpb.Principle)) string {
	t.Helper()
	seedCatalog(t, st)
	p := goodPrinciple()
	if mutate != nil {
		mutate(p)
	}
	id := propose(t, h, p)
	seedEvidenceFor(t, h, id)
	return id
}

func promote(t *testing.T, h *behavioralHandler, id, approvedBy string) *bpb.PromotePrincipleResponse {
	t.Helper()
	resp, err := h.PromotePrinciple(context.Background(), &bpb.PromotePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, ApprovedBy: approvedBy, Actor: "tester",
	})
	if err != nil {
		t.Fatalf("PromotePrinciple (unexpected transport error): %v", err)
	}
	return resp
}

func newGovHandler() (*store.MemoryStore, *behavioralHandler) {
	st := store.NewMemoryStore()
	return st, newBehavioralHandler(st)
}

func TestProposePrincipleCreatesProposed(t *testing.T) {
	st, h := newGovHandler()
	id := propose(t, h, goodPrinciple())
	got, err := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if err != nil {
		t.Fatalf("GetPrinciple: %v", err)
	}
	if got.Status != api.StatusProposedPrinciple {
		t.Errorf("status = %q, want PROPOSED_PRINCIPLE", got.Status)
	}
}

func TestProposeRejectsDirectPromoted(t *testing.T) {
	_, h := newGovHandler()
	p := goodPrinciple()
	p.Status = bpb.GovernanceStatus_PROMOTED_PRINCIPLE
	_, err := h.ProposePrinciple(context.Background(), &bpb.ProposePrincipleRequest{Principle: p})
	if status.Code(err) == codes.OK {
		t.Fatal("ProposePrinciple must reject direct PROMOTED_PRINCIPLE input")
	}
}

// Table of single-field omissions that must each BLOCK promotion.
func TestPromoteBlocksOnMissingGateRequirement(t *testing.T) {
	cases := []struct {
		name        string
		withEvidence bool
		mutate      func(*bpb.Principle)
	}{
		{"without evidence", false, nil},
		{"without provenance", true, func(p *bpb.Principle) { p.ProposedBy = "" }},
		{"without authority", true, func(p *bpb.Principle) { p.Authorities = nil }},
		{"without condition", true, func(p *bpb.Principle) { p.AppliesWhen = nil }},
		{"without contradiction check", true, func(p *bpb.Principle) { p.ContradictionChecked = false }},
		{"without promotion reason", true, func(p *bpb.Principle) { p.PromotionReason = "" }},
		{"without revocation rule", true, func(p *bpb.Principle) { p.RevocationRule = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := store.NewMemoryStore()
			h := newBehavioralHandler(st)
			seedCatalog(t, st)
			p := goodPrinciple()
			if tc.mutate != nil {
				tc.mutate(p)
			}
			id := propose(t, h, p)
			if tc.withEvidence {
				seedEvidenceFor(t, h, id)
			}
			resp := promote(t, h, id, "")
			if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_BLOCKED {
				t.Errorf("decision = %v, want PROMOTION_BLOCKED", resp.GetDecision())
			}
			// principle must NOT have transitioned to PROMOTED.
			got, _ := st.GetPrinciple(context.Background(), testProject, testDomain, id)
			if got.Status == api.StatusPromotedPrinciple {
				t.Errorf("principle was promoted despite a blocked gate")
			}
		})
	}
}

func TestPromoteBlocksWithOpenContradiction(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil)
	// Record an OPEN contradiction referencing the principle.
	_, err := h.RecordContradiction(context.Background(), &bpb.RecordContradictionRequest{
		Contradiction: &bpb.Contradiction{
			Project: testProject, Domain: testDomain, Kind: "rule_conflict",
			LeftRef: id, RightRef: "some-other-ref", // resolution defaults to "open"
		},
	})
	if err != nil {
		t.Fatalf("RecordContradiction: %v", err)
	}
	resp := promote(t, h, id, "")
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_BLOCKED {
		t.Errorf("decision = %v, want PROMOTION_BLOCKED (open contradiction)", resp.GetDecision())
	}
	if len(resp.GetRecord().GetBlockingContradictions()) == 0 {
		t.Error("expected blocking_contradictions to be recorded")
	}
}

func TestPromoteAllowsLowRiskWhenAllPass(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil)
	resp := promote(t, h, id, "")
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_ALLOWED {
		t.Fatalf("decision = %v (%s), want PROMOTION_ALLOWED", resp.GetDecision(), resp.GetRecord().GetReason())
	}
	if resp.GetStatus() != bpb.GovernanceStatus_PROMOTED_PRINCIPLE {
		t.Errorf("status = %v, want PROMOTED_PRINCIPLE", resp.GetStatus())
	}
	got, _ := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if got.Status != api.StatusPromotedPrinciple || got.PromotionDecisionID == "" {
		t.Errorf("principle not persisted as promoted: status=%q decisionID=%q", got.Status, got.PromotionDecisionID)
	}
}

func TestPromoteHighRiskReviewRequiredWithoutApproval(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, func(p *bpb.Principle) { p.RiskLevel = "high" })
	resp := promote(t, h, id, "")
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_REVIEW_REQUIRED {
		t.Fatalf("decision = %v, want PROMOTION_REVIEW_REQUIRED", resp.GetDecision())
	}
	got, _ := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if got.Status == api.StatusPromotedPrinciple {
		t.Error("high-risk principle promoted without approval")
	}
}

func TestPromoteHighRiskAllowedWithApproval(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, func(p *bpb.Principle) { p.RiskLevel = "irreversible" })
	resp := promote(t, h, id, "operator-dave")
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_ALLOWED {
		t.Fatalf("decision = %v (%s), want ALLOWED with approval", resp.GetDecision(), resp.GetRecord().GetReason())
	}
	got, _ := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if got.ApprovedBy != "operator-dave" || got.ApprovedAt == 0 {
		t.Errorf("approval not recorded first-class: approvedBy=%q approvedAt=%d", got.ApprovedBy, got.ApprovedAt)
	}
}

func TestEveryPromoteAttemptRecordsDecision(t *testing.T) {
	st, h := newGovHandler()
	// A blocked attempt (no evidence) must still persist a decision row.
	seedCatalog(t, st)
	id := propose(t, h, goodPrinciple())
	resp := promote(t, h, id, "")
	recID := resp.GetRecord().GetId()
	if recID == "" {
		t.Fatal("blocked promotion returned no decision record id")
	}
	rec, err := st.GetPromotionDecision(context.Background(), testProject, testDomain, recID)
	if err != nil {
		t.Fatalf("GetPromotionDecision: %v", err)
	}
	if rec.Decision != api.PromotionBlocked || rec.PrincipleID != id {
		t.Errorf("decision row = %+v, want BLOCKED for principle %q", rec, id)
	}
}

func TestRevokeRecordsWithoutDeleting(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil)
	promote(t, h, id, "")
	resp, err := h.RevokePrinciple(context.Background(), &bpb.RevokePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, Action: "REVOKED", Reason: "superseded by better runbook",
	})
	if err != nil {
		t.Fatalf("RevokePrinciple: %v", err)
	}
	if resp.GetStatus() != bpb.GovernanceStatus_REVOKED {
		t.Errorf("status = %v, want REVOKED", resp.GetStatus())
	}
	// principle still exists, with a revocation rule link.
	got, err := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if err != nil {
		t.Fatalf("principle was deleted: %v", err)
	}
	if got.Status != api.StatusRevoked || got.RevocationRuleID == "" {
		t.Errorf("principle not revoked-in-place: status=%q ruleID=%q", got.Status, got.RevocationRuleID)
	}
	if _, err := st.GetRevocationRule(context.Background(), testProject, testDomain, got.RevocationRuleID); err != nil {
		t.Errorf("revocation rule not persisted: %v", err)
	}
}

func TestRevokeSupersededRequiresSupersededBy(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil)
	promote(t, h, id, "")
	// Missing superseded_by → error.
	_, err := h.RevokePrinciple(context.Background(), &bpb.RevokePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, Action: "SUPERSEDED",
	})
	if status.Code(err) == codes.OK {
		t.Fatal("SUPERSEDED without superseded_by must fail")
	}
	// With superseded_by → ok.
	resp, err := h.RevokePrinciple(context.Background(), &bpb.RevokePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, Action: "SUPERSEDED", SupersededBy: "principle-v2",
	})
	if err != nil {
		t.Fatalf("RevokePrinciple SUPERSEDED: %v", err)
	}
	if resp.GetStatus() != bpb.GovernanceStatus_SUPERSEDED {
		t.Errorf("status = %v, want SUPERSEDED", resp.GetStatus())
	}
	got, _ := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if got.SupersededBy != "principle-v2" {
		t.Errorf("superseded_by = %q, want principle-v2", got.SupersededBy)
	}
}

func TestRevokeNarrowedRequiresScope(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil)
	promote(t, h, id, "")
	_, err := h.RevokePrinciple(context.Background(), &bpb.RevokePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, Action: "NARROWED",
	})
	if status.Code(err) == codes.OK {
		t.Fatal("NARROWED without narrowed_scope must fail")
	}
	resp, err := h.RevokePrinciple(context.Background(), &bpb.RevokePrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain, Action: "NARROWED", NarrowedScope: "only single-node etcd",
	})
	if err != nil {
		t.Fatalf("RevokePrinciple NARROWED: %v", err)
	}
	if resp.GetStatus() != bpb.GovernanceStatus_NARROWED {
		t.Errorf("status = %v, want NARROWED", resp.GetStatus())
	}
}

func TestExplainPrincipleReturnsGovernanceLinks(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil)
	promote(t, h, id, "")
	resp, err := h.ExplainPrinciple(context.Background(), &bpb.ExplainPrincipleRequest{
		PrincipleId: id, Project: testProject, Domain: testDomain,
	})
	if err != nil {
		t.Fatalf("ExplainPrinciple: %v", err)
	}
	if resp.GetPrinciple().GetId() != id {
		t.Errorf("principle id = %q, want %q", resp.GetPrinciple().GetId(), id)
	}
	if len(resp.GetConditions()) != 1 {
		t.Errorf("conditions = %d, want 1", len(resp.GetConditions()))
	}
	if len(resp.GetAuthorities()) != 1 {
		t.Errorf("authorities = %d, want 1", len(resp.GetAuthorities()))
	}
	if len(resp.GetEvidence()) != 1 {
		t.Errorf("evidence = %d, want 1", len(resp.GetEvidence()))
	}
	if len(resp.GetPromotionHistory()) != 1 {
		t.Errorf("promotion_history = %d, want 1", len(resp.GetPromotionHistory()))
	}
	if resp.GetExplanation() == "" {
		t.Error("explanation is empty")
	}
}

// Ingestion (signal/claim/evidence) must never auto-promote a principle.
func TestIngestionDoesNotAutoPromote(t *testing.T) {
	st, h := newGovHandler()
	id := fullySetup(t, st, h, nil) // proposes + evidence, but never promotes
	// Run more ingestion targeting the principle.
	sig := recordTestSignal(t, h)
	_ = extractTestClaim(t, h, sig)
	seedEvidenceFor(t, h, id)
	got, _ := st.GetPrinciple(context.Background(), testProject, testDomain, id)
	if got.Status == api.StatusPromotedPrinciple {
		t.Error("ingestion auto-promoted a principle — forbidden")
	}
	if got.Status != api.StatusProposedPrinciple {
		t.Errorf("principle status = %q, want PROPOSED_PRINCIPLE (untouched by ingestion)", got.Status)
	}
}
