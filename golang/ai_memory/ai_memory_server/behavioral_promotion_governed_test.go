package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

// These fixtures pin the promotion-API completeness contract: the gate must be
// satisfiable THROUGH THE KERNEL'S OWN PUBLIC SURFACE alone —
// RegisterCondition + RecordEvidence + MapAuthority + RunContradictionCheck —
// with NO direct store writes (store.PutCondition/PutAuthority) and NO
// ContradictionChecked fixture bit. (Contrast goodPrinciple()/seedCatalog() in
// behavioral_governance_test.go, which seed via the store and set the flag
// directly — that proved the gate logic, not that promotion is reachable from
// outside.) Each block path is exercised by omitting exactly one governed step.

// governedPrinciple is LOW-risk so a fully-resolved gate yields ALLOWED without a
// human approver. ContradictionChecked is intentionally unset — it must be
// established via the RunContradictionCheck RPC, never the fixture field.
func governedPrinciple() *bpb.Principle {
	return &bpb.Principle{
		Project: testProject, Domain: testDomain, Title: "governed promotion path",
		AppliesWhen:       []string{"cond.governed.binary_change"},
		Authorities:       []string{"auth.governed.repository"},
		RequiredEvidence:  []string{"req.governed.boundary_proven"},
		ForbiddenMoves:    []string{"forbid.direct_copy"},
		RecommendedAction: "use the package pipeline",
		RiskLevel:         "low",
		RevocationRule:    "revoke if a sanctioned direct-write path appears",
		PromotionReason:   "exercises the governed promotion surface",
		ProposedBy:        "test",
	}
}

type governedSteps struct{ condition, evidence, authority, check bool }

// driveGoverned proposes the principle and performs the requested gate-satisfying
// steps ONLY through public RPCs. No store access, no fixture fields.
func driveGoverned(t *testing.T, h *behavioralHandler, steps governedSteps) string {
	t.Helper()
	ctx := context.Background()
	pid := propose(t, h, governedPrinciple())

	if steps.condition {
		if _, err := h.RegisterCondition(ctx, &bpb.RegisterConditionRequest{
			Condition: &bpb.Condition{Id: "cond.governed.binary_change", Project: testProject, Domain: testDomain, Title: "service/infra binary change"},
		}); err != nil {
			t.Fatalf("RegisterCondition: %v", err)
		}
	}
	if steps.evidence {
		if _, err := h.RecordEvidence(ctx, &bpb.RecordEvidenceRequest{
			Evidence: &bpb.Evidence{
				Project: testProject, Domain: testDomain, TargetKind: "principle", TargetId: pid,
				EvidenceKind: "probe", Lane: bpb.EvidenceLaneMode_EVIDENCE_LANE_RUNTIME_REQUIRED, Result: "pass",
			},
		}); err != nil {
			t.Fatalf("RecordEvidence: %v", err)
		}
	}
	if steps.authority {
		if _, err := h.MapAuthority(ctx, &bpb.MapAuthorityRequest{
			TargetKind: "principle", TargetId: pid, Project: testProject, Domain: testDomain,
			AuthorityIds: []string{"auth.governed.repository"},
		}); err != nil {
			t.Fatalf("MapAuthority: %v", err)
		}
	}
	if steps.check {
		resp, err := h.RunContradictionCheck(ctx, &bpb.RunContradictionCheckRequest{
			PrincipleId: pid, Project: testProject, Domain: testDomain, Actor: "test",
		})
		if err != nil {
			t.Fatalf("RunContradictionCheck: %v", err)
		}
		if !resp.GetContradictionChecked() {
			t.Fatal("RunContradictionCheck returned contradiction_checked=false")
		}
		if len(resp.GetOpenContradictionIds()) != 0 {
			t.Fatalf("unexpected open contradictions: %v", resp.GetOpenContradictionIds())
		}
	}
	return pid
}

func TestPromotionGate_GovernedSurfaceOnly(t *testing.T) {
	full := governedSteps{condition: true, evidence: true, authority: true, check: true}
	cases := []struct {
		name  string
		steps governedSteps
		want  bpb.PromotionDecision
	}{
		{"missing evidence", governedSteps{condition: true, authority: true, check: true}, bpb.PromotionDecision_PROMOTION_BLOCKED},
		{"unresolved authority", governedSteps{condition: true, evidence: true, check: true}, bpb.PromotionDecision_PROMOTION_BLOCKED},
		{"unresolved condition", governedSteps{evidence: true, authority: true, check: true}, bpb.PromotionDecision_PROMOTION_BLOCKED},
		{"contradiction unchecked", governedSteps{condition: true, evidence: true, authority: true}, bpb.PromotionDecision_PROMOTION_BLOCKED},
		{"all resolved via governed surface", full, bpb.PromotionDecision_PROMOTION_ALLOWED},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newBehavioralHandler(store.NewMemoryStore())
			pid := driveGoverned(t, h, tc.steps)
			resp, err := h.PromotePrinciple(context.Background(), &bpb.PromotePrincipleRequest{
				PrincipleId: pid, Project: testProject, Domain: testDomain, Actor: "test",
			})
			if err != nil {
				t.Fatalf("PromotePrinciple: %v", err)
			}
			if resp.GetDecision() != tc.want {
				t.Fatalf("decision = %v (%s), want %v", resp.GetDecision(), resp.GetRecord().GetReason(), tc.want)
			}
		})
	}
}
