package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/rdf"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
)

// End-to-end demonstration of the behavioral-memory operator loop using the
// cluster_operator scenario "No recovery claim without authoritative evidence".
//
//	load/seed principle → satisfy gate → promote → ResolveGovernedContext sees it
//	→ CheckAction refuses / needs_evidence → provide evidence → allowed
//	→ RecordOutcome → RDF export shows the semantic chain
//
// Deterministic and in-memory (no Scylla needed): the RDF Bundle is assembled
// from the store for exactly the entities the loop touched. Run with:
//
//	go test ./ai_memory/ai_memory_server -run Demo -v
const (
	demoPrincipleID = "principle.cluster.no_recovery_claim_without_authoritative_evidence"
	demoCondition   = "condition.cluster.service.desired_observed_mismatch"
	demoForbidden   = "forbidden.cluster.claim_recovery_without_authoritative_evidence"
	demoAuthority   = "authority.cluster.owner_service.runtime_state"
	demoEvDesired   = "evidence.cluster.owner_service.desired_state"
	demoEvObserved  = "evidence.cluster.owner_service.observed_state"
)

func TestDemoOperatorLoop(t *testing.T) {
	ctx := context.Background()
	st, h := loadClusterPack(t) // project=globular-services, domain=cluster_operator

	// ── seed → satisfy gate → promote ─────────────────────────────────────────
	// The seed principle is PROPOSED; promoteClusterPrinciple marks the
	// contradiction check, records gate evidence, and promotes with human
	// approval (it is high-risk).
	promoteClusterPrinciple(t, st, h, demoPrincipleID)
	if p, _ := st.GetPrinciple(ctx, testProject, clusterDomain, demoPrincipleID); p.Status != api.StatusPromotedPrinciple {
		t.Fatalf("principle not promoted: %q", p.Status)
	}
	t.Log("✓ promoted:", demoPrincipleID)

	// ── ResolveGovernedContext sees the promoted principle ───────────────────
	rc, err := h.ResolveGovernedContext(ctx, &bpb.ResolveGovernedContextRequest{
		Project: testProject, Domain: clusterDomain, Goal: "recover service", Conditions: []string{demoCondition},
	})
	if err != nil {
		t.Fatalf("ResolveGovernedContext: %v", err)
	}
	if !principleInContext(rc.GetContext(), demoPrincipleID) {
		t.Fatalf("resolved context does not include %q", demoPrincipleID)
	}
	t.Log("✓ ResolveGovernedContext returns the principle; recommended:", rc.GetContext().GetRecommendedBehavior())

	// ── CheckAction REFUSES the forbidden recovery claim ─────────────────────
	blocked := checkAction(t, h, &bpb.CheckActionRequest{
		Domain: clusterDomain, ActionType: demoForbidden, CurrentConditions: []string{demoCondition},
	})
	if blocked.GetStatus() != "blocked" {
		t.Fatalf("forbidden recovery claim: status=%q, want blocked", blocked.GetStatus())
	}
	t.Log("✓ CheckAction(forbidden claim) → blocked")

	// ── CheckAction needs_evidence when claiming recovery without evidence ───
	needs := checkAction(t, h, &bpb.CheckActionRequest{
		Domain: clusterDomain, ActionType: "claim_recovery", CurrentConditions: []string{demoCondition},
	})
	if needs.GetStatus() != "needs_evidence" {
		t.Fatalf("recovery claim without evidence: status=%q, want needs_evidence", needs.GetStatus())
	}
	if !containsStr(needs.GetMissingEvidence(), demoEvDesired) || !containsStr(needs.GetMissingEvidence(), demoEvObserved) {
		t.Fatalf("missing_evidence = %v, want desired+observed", needs.GetMissingEvidence())
	}
	t.Log("✓ CheckAction(claim_recovery, no evidence) → needs_evidence:", needs.GetMissingEvidence())

	// ── Providing the required evidence (+ human approval) → allowed ──────────
	allowed := checkAction(t, h, &bpb.CheckActionRequest{
		Domain: clusterDomain, ActionType: "claim_recovery", CurrentConditions: []string{demoCondition},
		ProvidedEvidenceRefs: []string{demoEvDesired, demoEvObserved}, HumanApproval: "operator-dave",
	})
	if allowed.GetStatus() != "allowed" {
		t.Fatalf("recovery claim with evidence+approval: status=%q, want allowed", allowed.GetStatus())
	}
	t.Log("✓ CheckAction(claim_recovery, evidence+approval) → allowed; action_check_id:", allowed.GetId())

	// ── RecordOutcome ────────────────────────────────────────────────────────
	oresp, err := h.RecordOutcome(ctx, &bpb.RecordOutcomeRequest{Outcome: &bpb.Outcome{
		Project: testProject, Domain: clusterDomain, ActionCheckId: allowed.GetId(), Status: "success",
		Theme: "recovery_claim", SupportsPrinciples: []string{demoPrincipleID},
	}})
	if err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}
	t.Log("✓ RecordOutcome → outcome_id:", oresp.GetOutcomeId())

	// ── RDF export shows the semantic chain ──────────────────────────────────
	bundle := demoBundle(ctx, st, []string{needs.GetId(), allowed.GetId()}, oresp.GetOutcomeId())
	doc := string(rdf.Project(bundle))

	assertChain := func(label, pred string, ids ...string) {
		if !strings.Contains(doc, "behavioral#"+pred+">") {
			t.Errorf("RDF missing predicate %s (%s)", pred, label)
		}
		for _, id := range ids {
			if !strings.Contains(doc, id) {
				t.Errorf("RDF missing %q for %s", id, label)
			}
		}
	}
	assertChain("Principle→appliesWhen→condition", "appliesWhen", "condition/"+demoCondition)
	assertChain("Principle→requiresEvidence→evidence", "requiresEvidence", "required_evidence/"+demoEvDesired, "required_evidence/"+demoEvObserved)
	assertChain("Principle→forbidsMove→forbidden", "forbidsMove", "forbidden_move/"+demoForbidden)
	assertChain("ActionCheck→missingEvidence→evidence", "missingEvidence", "required_evidence/"+demoEvDesired)
	assertChain("Outcome→resultedFrom→ActionCheck", "resultedFrom", "action_check/"+allowed.GetId())
	t.Logf("✓ RDF projection (%d triples) contains the full semantic chain", strings.Count(doc, " .\n"))
}

func principleInContext(c *bpb.GovernedContext, id string) bool {
	for _, p := range c.GetApplicablePrinciples() {
		if p.GetId() == id {
			return true
		}
	}
	return false
}

// demoBundle assembles an rdf.Bundle from the store for exactly the entities the
// loop touched — proving the projection without needing a full-scan reader.
func demoBundle(ctx context.Context, st *store.MemoryStore, checkIDs []string, outcomeID string) *rdf.Bundle {
	b := &rdf.Bundle{}
	if p, err := st.GetPrinciple(ctx, testProject, clusterDomain, demoPrincipleID); err == nil {
		b.Principles = []api.Principle{*p}
	}
	for _, id := range checkIDs {
		if ac, err := st.GetActionCheck(ctx, testProject, clusterDomain, id); err == nil {
			b.ActionChecks = append(b.ActionChecks, *ac)
		}
	}
	if o, err := st.GetOutcome(ctx, testProject, clusterDomain, outcomeID); err == nil {
		b.Outcomes = []api.Outcome{*o}
	}
	if c, err := st.GetCondition(ctx, testProject, clusterDomain, demoCondition); err == nil {
		b.Conditions = []api.Condition{*c}
	}
	if a, err := st.GetAuthority(ctx, testProject, clusterDomain, demoAuthority); err == nil {
		b.Authorities = []api.Authority{*a}
	}
	return b
}
