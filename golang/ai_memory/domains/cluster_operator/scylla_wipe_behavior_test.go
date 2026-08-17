package cluster_operator_test

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/core"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
)

const (
	scyllaTestProject   = "globular-services"
	scyllaWipePrinciple = "principle.cluster.no_raw_scylla_data_wipe"
	scyllaWipeAction    = "scylla.data.wipe"
)

func newScyllaWipeBehaviorService(t *testing.T) (*core.Service, *store.MemoryStore) {
	t.Helper()
	ctx := context.Background()
	pack, err := cluster_operator.New()
	if err != nil {
		t.Fatalf("cluster_operator.New: %v", err)
	}
	st := store.NewMemoryStore()
	if _, err := domain.LoadCatalogs(ctx, st, scyllaTestProject, pack); err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}
	reg := domain.NewRegistry()
	reg.Register(pack)
	return core.New(st, reg), st
}

func checkRawScyllaWipe(t *testing.T, svc *core.Service) api.ActionCheck {
	t.Helper()
	resp, err := svc.CheckAction(context.Background(), &api.CheckActionRequest{
		Project:    scyllaTestProject,
		Domain:     api.DomainRef(cluster_operator.DomainName),
		ActionType: scyllaWipeAction,
		Target:     "globule-ryzen",
	})
	if err != nil {
		t.Fatalf("CheckAction(%s): %v", scyllaWipeAction, err)
	}
	return resp.Result
}

// This reproduces the live defect exactly at the behavioral boundary. With the
// Scylla rule still merely PROPOSED, the first raw wipe remains default-allowed;
// learning may observe the gap but must not rewrite today's governance verdict.
// Because the proposal is irreversible, that single occurrence must immediately
// enter the human review queue rather than waiting for a second destructive ask.
func TestRawScyllaWipeFirstGapQueuesReviewButDoesNotSelfPromote(t *testing.T) {
	svc, st := newScyllaWipeBehaviorService(t)
	ctx := context.Background()

	ac := checkRawScyllaWipe(t, svc)
	if !ac.Allowed || ac.Governed {
		t.Fatalf("pre-promotion verdict allowed=%v governed=%v, want allowed=true governed=false", ac.Allowed, ac.Governed)
	}
	if got := ac.Metadata["learning_candidates_queued"]; got != "1" {
		t.Fatalf("learning_candidates_queued=%q want 1 for irreversible raw wipe", got)
	}

	queued, err := st.ListPromotionCandidates(ctx, scyllaTestProject, cluster_operator.DomainName, "", api.PromotionCandidateStatusQueued, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range queued {
		if c.MaterializedPrincipleID != scyllaWipePrinciple {
			continue
		}
		found = true
		if c.RepeatCount != 1 {
			t.Fatalf("Scylla candidate repeat_count=%d want 1", c.RepeatCount)
		}
		if c.DraftPrinciple.RiskLevel != "irreversible" {
			t.Fatalf("Scylla candidate risk=%q want irreversible", c.DraftPrinciple.RiskLevel)
		}
		if len(c.SupportingEvidenceIDs) != 1 {
			t.Fatalf("Scylla candidate supporting evidence=%v want one exact coverage-gap row", c.SupportingEvidenceIDs)
		}
	}
	if !found {
		t.Fatalf("no queued candidate materialized from %s", scyllaWipePrinciple)
	}

	p, err := st.GetPrinciple(ctx, scyllaTestProject, cluster_operator.DomainName, scyllaWipePrinciple)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != api.StatusProposedPrinciple {
		t.Fatalf("learning manufactured authority: principle status=%q want PROPOSED_PRINCIPLE", p.Status)
	}
}

// Promotion remains an explicit governed act. This test satisfies the existing
// promotion contract through its public kernel operations, then proves the same
// action that was default-allowed above becomes a real forbidden-move decision.
func TestPromotedRawScyllaWipeRuleBlocksAndIsGoverned(t *testing.T) {
	svc, _ := newScyllaWipeBehaviorService(t)
	ctx := context.Background()

	// Evidence here supports the HUMAN promotion decision only. It deliberately
	// does not assert that a wipe is safe. The behavioral proposal itself carries
	// the awareness/source provenance and the promotion gate requires at least one
	// reviewed evidence row targeting the principle.
	if _, err := svc.RecordEvidence(ctx, &api.RecordEvidenceRequest{Evidence: api.Evidence{
		Project:    scyllaTestProject,
		Domain:     api.DomainRef(cluster_operator.DomainName),
		TargetKind: "principle",
		TargetID:   scyllaWipePrinciple,
		Kind:       "governance_design_evidence",
		Lane:       api.LaneStaticOnly,
		Result:     "reviewed",
		SourceKind: "awareness_invariant",
		SourceRef:  "scylla.join_wipe_safe_only_if_never_verified",
		Payload:    "raw wipe bypasses the controller-side ScyllaWasEverVerified guard",
		Provenance: api.Provenance{AgentID: "test.human_reviewer"},
	}}); err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}

	check, err := svc.RunContradictionCheck(ctx, &api.RunContradictionCheckRequest{
		Project:     scyllaTestProject,
		Domain:      api.DomainRef(cluster_operator.DomainName),
		PrincipleID: scyllaWipePrinciple,
		Actor:       "test.human_reviewer",
	})
	if err != nil {
		t.Fatalf("RunContradictionCheck: %v", err)
	}
	if !check.ContradictionChecked || len(check.OpenContradictionIDs) != 0 {
		t.Fatalf("contradiction check=%+v want checked with no open contradiction", check)
	}

	promoted, err := svc.PromotePrinciple(ctx, &api.PromotePrincipleRequest{
		Project:        scyllaTestProject,
		Domain:         api.DomainRef(cluster_operator.DomainName),
		PrincipleID:    scyllaWipePrinciple,
		ApprovedBy:     "test.human_reviewer",
		ApprovalReason: "raw Scylla wipe must stay behind the never-verified controller guard",
		Actor:          "test.human_reviewer",
	})
	if err != nil {
		t.Fatalf("PromotePrinciple: %v", err)
	}
	if promoted.Decision != api.PromotionAllowed || promoted.Status != api.StatusPromotedPrinciple {
		t.Fatalf("promotion=%+v want ALLOWED/PROMOTED_PRINCIPLE", promoted)
	}

	ac := checkRawScyllaWipe(t, svc)
	if ac.Allowed || !ac.Governed || ac.Status != "blocked" {
		t.Fatalf("post-promotion verdict allowed=%v governed=%v status=%q, want false/true/blocked", ac.Allowed, ac.Governed, ac.Status)
	}
	if len(ac.ForbiddenMatched) != 1 || ac.ForbiddenMatched[0] != api.ForbiddenMoveRef("forbidden.cluster.raw_scylla_data_wipe") {
		t.Fatalf("forbidden matches=%v want raw Scylla wipe rule", ac.ForbiddenMatched)
	}
	if len(ac.CheckedAgainstPrinciples) == 0 || ac.CheckedAgainstPrinciples[0] != scyllaWipePrinciple {
		t.Fatalf("checked principles=%v want %s", ac.CheckedAgainstPrinciples, scyllaWipePrinciple)
	}
}
