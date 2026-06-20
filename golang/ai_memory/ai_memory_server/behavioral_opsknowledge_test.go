package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

// A compiler-generated principle, loaded via the pack alongside hand-authored
// seed (PR-5A merge), is stored PROPOSED with its lineage preserved.
const genPrinciple = "principle.cluster.observe_plan_execute_verify_before_recovery_claim"

func TestGeneratedPrincipleLoadsAsProposed(t *testing.T) {
	st, _ := loadClusterPack(t)
	ctx := context.Background()
	// generated authority + condition resolve
	if _, err := st.GetAuthority(ctx, testProject, clusterDomain, "authority.cluster.cluster_controller.runtime_state"); err != nil {
		t.Errorf("generated authority not loaded: %v", err)
	}
	if _, err := st.GetCondition(ctx, testProject, clusterDomain, "condition.cluster.recovery.in_progress"); err != nil {
		t.Errorf("generated condition not loaded: %v", err)
	}
	p, err := st.GetPrinciple(ctx, testProject, clusterDomain, genPrinciple)
	if err != nil {
		t.Fatalf("generated principle not loaded: %v", err)
	}
	if p.Status != api.StatusProposedPrinciple {
		t.Errorf("generated principle status = %q, want PROPOSED_PRINCIPLE (never auto-promoted)", p.Status)
	}
	if len(p.SourceRefs) == 0 || len(p.GeneratedFrom) == 0 {
		t.Errorf("generated principle lineage dropped on load: %+v", p)
	}
}

// A generated principle still goes through the promotion gate — no bypass.
func TestGeneratedPrincipleGateNotBypassed(t *testing.T) {
	_, h := loadClusterPack(t)
	resp, err := h.PromotePrinciple(context.Background(), &bpb.PromotePrincipleRequest{
		PrincipleId: genPrinciple, Project: testProject, Domain: clusterDomain, ApprovedBy: "op",
	})
	if err != nil {
		t.Fatalf("PromotePrinciple: %v", err)
	}
	if resp.GetDecision() != bpb.PromotionDecision_PROMOTION_BLOCKED {
		t.Fatalf("generated principle promote = %v, want BLOCKED (gate not bypassed)", resp.GetDecision())
	}
	if resp.GetRecord().GetId() == "" {
		t.Error("blocked generated-principle promotion must still record a decision")
	}
}
