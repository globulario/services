package main

import (
	"context"
	"strings"
	"testing"

	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

// TestPromotionDurabilityGate_RebuildLosesPromotionButReconciliationFlagsIt
// mechanizes the invariant behavioral.promotion_must_survive_rebuild_or_be_flagged
// (ai-memory incident aab023cb, 2026-07-09; docs/design/behavioral-promotion-durability.md).
//
// A governed promotion is runtime-written state that is NOT part of the immutable
// behavioral seed. The seed carries authorities + the conditions catalog; it does
// NOT carry promotions. So a behavioral-store rebuild/reset restores the catalog
// but DROPS every promotion, silently leaving a once-governed lane at ungoverned
// default-allow (the success-by-assertion / fail-open class that evaporated the
// deploy-lane principle 8a1cdef8 and forced re-promotion as 34db74ee).
//
// The durability contract is an OR: EITHER the promotion survives the rebuild, OR
// boot/check reconciliation flags the loss. This gate proves the DETECTION branch
// (Option B) end-to-end: after a rebuild loses a promotion, the same
// GenerateReconciliationReport that surfaced the live drift MUST fire
// AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE — so an evaporated promotion
// can never be silently treated as still-enforcing.
func TestPromotionDurabilityGate_RebuildLosesPromotionButReconciliationFlagsIt(t *testing.T) {
	ctx := context.Background()

	// --- store A: promote a principle through the governed surface ---
	stA, hA := newGovHandler()
	promoteGoodPrinciple(t, stA, hA) // goodPrinciple: applies_when cond.nospace, forbids forbid.restart_before_quorum

	before, err := hA.CheckAction(ctx, &bpb.CheckActionRequest{
		Project: testProject, Domain: testDomain,
		ActionType: "forbid.restart_before_quorum", CurrentConditions: []string{condNospace},
	})
	if err != nil {
		t.Fatalf("CheckAction(before rebuild): %v", err)
	}
	if before.GetResult().GetAllowed() || !before.GetResult().GetGoverned() {
		t.Fatalf("pre-rebuild expected blocked+governed, got allowed=%v governed=%v", before.GetResult().GetAllowed(), before.GetResult().GetGoverned())
	}

	// --- simulate a behavioral-store rebuild/reset ---
	// A fresh store re-seeded from the catalog models exactly what survives a
	// rebuild: authorities + conditions, but NOT the runtime promotion.
	stB, hB := newGovHandler()
	seedCatalog(t, stB)

	after, err := hB.CheckAction(ctx, &bpb.CheckActionRequest{
		Project: testProject, Domain: testDomain,
		ActionType: "forbid.restart_before_quorum", CurrentConditions: []string{condNospace},
	})
	if err != nil {
		t.Fatalf("CheckAction(after rebuild): %v", err)
	}
	// The promotion did NOT survive the rebuild — the lane is now ungoverned.
	// (If a future change makes promotions seed-durable, this branch flips and the
	// OR contract is satisfied by survival instead; update the gate accordingly.)
	if after.GetResult().GetGoverned() {
		t.Fatalf("post-rebuild expected ungoverned (promotion evaporated), got governed=%v", after.GetResult().GetGoverned())
	}

	// --- durability SAFETY NET: reconciliation MUST flag the evaporation ---
	rep, err := hB.GenerateReconciliationReport(ctx, &bpb.GenerateReconciliationReportRequest{
		Project: testProject, Domain: testDomain, Theme: "cluster_operator.deploy_lane",
		RuntimeRelevant:   true,
		AwgFailureModeIds: []string{"deploy.binary_copy_bypasses_package_pipeline_and_splits_cluster"},
		Actor:             "durability-gate",
	})
	if err != nil {
		t.Fatalf("GenerateReconciliationReport: %v", err)
	}
	if !strings.Contains(strings.Join(rep.GetReport().GetFindings(), ","), "AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE") {
		t.Fatalf("durability gate FAILED: a rebuild lost the promotion but reconciliation did not flag it; "+
			"findings=%v (silent governance evaporation is the exact regression this gate forbids)",
			rep.GetReport().GetFindings())
	}
}
