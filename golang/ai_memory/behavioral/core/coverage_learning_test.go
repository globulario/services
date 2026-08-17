package core

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

type coverageTestPack struct{ cats domain.Catalogs }

func (p *coverageTestPack) Name() string              { return "coverage_test" }
func (p *coverageTestPack) Catalogs() domain.Catalogs { return p.cats }

func newCoverageLearningService(t *testing.T) (*Service, *store.MemoryStore) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	pack := &coverageTestPack{cats: domain.Catalogs{
		Conditions: []domain.CatalogEntry{{
			ID: "condition.always", Title: "always",
		}},
		Authorities: []domain.CatalogEntry{{
			ID: "authority.test.owner", Title: "test owner",
		}},
		ForbiddenMoves: []domain.CatalogEntry{{
			ID: "forbidden.test.dangerous", Title: "dangerous operation",
			Fields: map[string]string{"action_aliases": "dangerous-op"},
		}},
		Principles: []domain.PrincipleSeed{{
			ID:                "principle.test.dangerous",
			Title:             "Do not perform the dangerous operation",
			AppliesWhen:       []string{"condition.always"},
			Authorities:       []string{"authority.test.owner"},
			ForbiddenMoves:    []string{"forbidden.test.dangerous"},
			RecommendedAction: "choose the safe path",
			RiskLevel:         "high",
			RevocationRule:    "revoke when the operation is no longer dangerous",
			PromotionReason:   "repeated ungoverned attempts",
		}},
	}}
	if _, err := domain.LoadCatalogs(ctx, st, "project-test", pack); err != nil {
		t.Fatalf("load test pack: %v", err)
	}
	reg := domain.NewRegistry()
	reg.Register(pack)
	return New(st, reg), st
}

func checkDangerous(t *testing.T, svc *Service) api.ActionCheck {
	t.Helper()
	resp, err := svc.CheckAction(context.Background(), &api.CheckActionRequest{
		Project:    "project-test",
		Domain:     api.DomainRef("coverage_test"),
		ActionType: "dangerous-op",
		Target:     "node-1",
	})
	if err != nil {
		t.Fatalf("CheckAction: %v", err)
	}
	return resp.Result
}

func TestCoverageLearning_FirstGapPersistsButDoesNotQueue(t *testing.T) {
	svc, st := newCoverageLearningService(t)
	ac := checkDangerous(t, svc)

	if ac.Governed || !ac.Allowed {
		t.Fatalf("first check should remain ungoverned default-allow, got governed=%v allowed=%v", ac.Governed, ac.Allowed)
	}
	if got := ac.Metadata["learning_status"]; got != "recorded" {
		t.Fatalf("learning_status=%q want recorded", got)
	}
	if got := ac.Metadata["learning_candidates_queued"]; got != "0" {
		t.Fatalf("learning_candidates_queued=%q want 0", got)
	}

	queued, err := st.ListPromotionCandidates(context.Background(), "project-test", "coverage_test", "", api.PromotionCandidateStatusQueued, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 0 {
		t.Fatalf("first observation queued %d candidate(s); want 0", len(queued))
	}
	all, err := st.ListPromotionCandidates(context.Background(), "project-test", "coverage_test", "", api.PromotionCandidateStatusUnspecified, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].RepeatCount != 1 || all[0].Status != api.PromotionCandidateStatusUnspecified {
		t.Fatalf("pending candidate = %+v, want one repeat-1 private accumulator", all)
	}
	if all[0].MaterializedPrincipleID != "principle.test.dangerous" {
		t.Fatalf("materialized principle=%q", all[0].MaterializedPrincipleID)
	}

	// Learning evidence proves only that this exact action check exposed a gap.
	// It does not target the principle and therefore cannot satisfy promotion's
	// evidence-presence gate by accident.
	evs, err := st.ListEvidenceForTarget(context.Background(), "project-test", "coverage_test", ac.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("coverage evidence rows=%d want 1", len(evs))
	}
	if evs[0].TargetKind != "action_check" || evs[0].SourceRef != ac.ID || len(evs[0].Satisfies) != 0 {
		t.Fatalf("coverage evidence has unsafe shape: %+v", evs[0])
	}
}

func TestCoverageLearning_SecondGapQueuesWithoutPromoting(t *testing.T) {
	svc, st := newCoverageLearningService(t)
	first := checkDangerous(t, svc)
	second := checkDangerous(t, svc)
	if first.ID == second.ID {
		t.Fatal("distinct checks must have distinct audit ids")
	}

	queued, err := st.ListPromotionCandidates(context.Background(), "project-test", "coverage_test", "", api.PromotionCandidateStatusQueued, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 {
		t.Fatalf("queued candidates=%d want 1", len(queued))
	}
	c := queued[0]
	if c.RepeatCount != 2 || len(c.SupportingEvidenceIDs) != 2 {
		t.Fatalf("candidate support repeat=%d evidence=%v", c.RepeatCount, c.SupportingEvidenceIDs)
	}
	if c.DraftPrinciple.ID != "principle.test.dangerous" {
		t.Fatalf("candidate invented a different principle: %q", c.DraftPrinciple.ID)
	}

	p, err := st.GetPrinciple(context.Background(), "project-test", "coverage_test", "principle.test.dangerous")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != api.StatusProposedPrinciple {
		t.Fatalf("learning changed authority: principle status=%q want PROPOSED_PRINCIPLE", p.Status)
	}
	if got := second.Metadata["learning_candidates_queued"]; got != "1" {
		t.Fatalf("second check learning_candidates_queued=%q want 1", got)
	}
}

func TestCoverageLearning_IrreversibleGapQueuesImmediatelyWithoutPromoting(t *testing.T) {
	svc, st := newCoverageLearningService(t)
	ctx := context.Background()

	p, err := st.GetPrinciple(ctx, "project-test", "coverage_test", "principle.test.dangerous")
	if err != nil {
		t.Fatal(err)
	}
	p.RiskLevel = "irreversible"
	if err := st.CreatePrinciple(ctx, p); err != nil {
		t.Fatalf("set irreversible risk: %v", err)
	}

	ac := checkDangerous(t, svc)
	if ac.Governed || !ac.Allowed {
		t.Fatalf("learning must not alter the pre-existing verdict, got governed=%v allowed=%v", ac.Governed, ac.Allowed)
	}
	if got := ac.Metadata["learning_candidates_queued"]; got != "1" {
		t.Fatalf("learning_candidates_queued=%q want 1 for irreversible first occurrence", got)
	}

	queued, err := st.ListPromotionCandidates(ctx, "project-test", "coverage_test", "", api.PromotionCandidateStatusQueued, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 || queued[0].RepeatCount != 1 {
		t.Fatalf("irreversible queued candidates=%+v, want exactly one repeat-1 candidate", queued)
	}

	p, err = st.GetPrinciple(ctx, "project-test", "coverage_test", "principle.test.dangerous")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != api.StatusProposedPrinciple {
		t.Fatalf("irreversible learning changed authority: status=%q want PROPOSED_PRINCIPLE", p.Status)
	}
}

func TestCoverageLearning_NoTemplateIsExplicitlyUncovered(t *testing.T) {
	svc, st := newCoverageLearningService(t)
	resp, err := svc.CheckAction(context.Background(), &api.CheckActionRequest{
		Project: "project-test", Domain: api.DomainRef("coverage_test"), ActionType: "totally-unmodeled-op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Result.Metadata["learning_status"]; got != "uncovered_no_template" {
		t.Fatalf("learning_status=%q want uncovered_no_template", got)
	}
	all, err := st.ListPromotionCandidates(context.Background(), "project-test", "coverage_test", "", api.PromotionCandidateStatusUnspecified, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("unmodeled action invented %d candidate(s)", len(all))
	}
}

func TestCoverageLearning_MissingPersistedSeedDegradesInsteadOfInventing(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	pack := &coverageTestPack{cats: domain.Catalogs{
		Conditions: []domain.CatalogEntry{{ID: "condition.always"}},
		ForbiddenMoves: []domain.CatalogEntry{{
			ID: "forbidden.test.missing", Fields: map[string]string{"action_aliases": "missing-op"},
		}},
		Principles: []domain.PrincipleSeed{{
			ID: "principle.test.missing", Title: "missing", AppliesWhen: []string{"condition.always"},
			Authorities: []string{"authority.test.owner"}, ForbiddenMoves: []string{"forbidden.test.missing"},
			RiskLevel: "high", RevocationRule: "x", PromotionReason: "x",
		}},
	}}
	reg := domain.NewRegistry()
	reg.Register(pack)
	svc := New(st, reg) // deliberately DO NOT persist the pack

	resp, err := svc.CheckAction(ctx, &api.CheckActionRequest{
		Project: "project-test", Domain: api.DomainRef("coverage_test"), ActionType: "missing-op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Result.Metadata["learning_status"]; got != "degraded" {
		t.Fatalf("learning_status=%q want degraded", got)
	}
	all, err := st.ListPromotionCandidates(ctx, "project-test", "coverage_test", "", api.PromotionCandidateStatusUnspecified, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("missing persisted seed caused %d invented candidate(s)", len(all))
	}
}
