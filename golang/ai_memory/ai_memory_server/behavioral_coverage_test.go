package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

// PR-13: CheckAction must distinguish a GOVERNED allow (an applicable promoted
// principle was evaluated and satisfied) from an UNGOVERNED default-allow (no
// principle applied), and expose a coverage metric — otherwise "allowed" hides
// whether the gate had any reach over the action.

// An allow with no applicable principle is marked ungoverned + explained as such.
func TestCheckActionUngovernedAllowMarked(t *testing.T) {
	_, h := newGovHandler()
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "do-thing", CurrentConditions: []string{"cond.unseeded"}})
	if ac.GetStatus() != "allowed" {
		t.Fatalf("status=%q, want allowed", ac.GetStatus())
	}
	if ac.GetGoverned() {
		t.Error("ungoverned default-allow must have governed=false")
	}
	if !strings.Contains(ac.GetExplanation(), "no applicable promoted principle") {
		t.Errorf("explanation should mark ungoverned, got: %q", ac.GetExplanation())
	}
}

// An allow where a promoted principle applied and was satisfied is governed.
func TestCheckActionGovernedAllowMarked(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-clean", RiskLevel: "low"})
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "do-thing", CurrentConditions: []string{condNospace}})
	if ac.GetStatus() != "allowed" || !ac.GetGoverned() {
		t.Fatalf("status=%q governed=%v, want allowed+governed", ac.GetStatus(), ac.GetGoverned())
	}
}

// A non-allow verdict (blocked) is inherently governed — a principle applied.
func TestCheckActionBlockedIsGoverned(t *testing.T) {
	st, h := newGovHandler()
	_ = st.PutCondition(context.Background(), &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-forbid", ForbiddenMoves: []api.ForbiddenMoveRef{"forbid.x"}, RiskLevel: "low"})
	ac := checkAction(t, h, &bpb.CheckActionRequest{ActionType: "forbid.x", CurrentConditions: []string{condNospace}})
	if ac.GetStatus() != "blocked" || !ac.GetGoverned() {
		t.Fatalf("blocked verdict must be governed: status=%q governed=%v", ac.GetStatus(), ac.GetGoverned())
	}
}

// GetGovernanceCoverage tallies governed vs ungoverned CheckActions and the ratio.
func TestGovernanceCoverageMetric(t *testing.T) {
	st, h := newGovHandler()
	ctx := context.Background()
	_ = st.PutCondition(ctx, &api.Condition{ID: condNospace, Project: testProject, Domain: testDomain})
	seedPromotedPrinciple(t, st, &api.Principle{ID: "p-cov", RiskLevel: "low"})

	// 2 governed (match the promoted principle's condition).
	checkAction(t, h, &bpb.CheckActionRequest{ActionType: "a", CurrentConditions: []string{condNospace}})
	checkAction(t, h, &bpb.CheckActionRequest{ActionType: "b", CurrentConditions: []string{condNospace}})
	// 3 ungoverned (no applicable principle).
	for i := 0; i < 3; i++ {
		checkAction(t, h, &bpb.CheckActionRequest{ActionType: "x", CurrentConditions: []string{"cond.none"}})
	}

	resp, err := h.GetGovernanceCoverage(ctx, &bpb.GetGovernanceCoverageRequest{Project: testProject, Domain: testDomain})
	if err != nil {
		t.Fatalf("GetGovernanceCoverage: %v", err)
	}
	if resp.GetTotal() != 5 || resp.GetGoverned() != 2 || resp.GetUngoverned() != 3 {
		t.Fatalf("coverage = total %d governed %d ungoverned %d, want 5/2/3",
			resp.GetTotal(), resp.GetGoverned(), resp.GetUngoverned())
	}
	if got := resp.GetCoverageRatio(); got < 0.39 || got > 0.41 {
		t.Errorf("coverage_ratio = %v, want ~0.40", got)
	}
}
