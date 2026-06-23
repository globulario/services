package core

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// fakeCheckStore is a minimal Store for exercising CheckAction's engagement logic.
// It embeds the Store interface so only the methods CheckAction actually calls need
// real behavior; any other call would panic (and would signal an untested path).
type fakeCheckStore struct {
	store.Store
	byCondition map[string][]string
	principles  map[string]*api.Principle
}

func (f *fakeCheckStore) ListPrincipleIDsByCondition(_ context.Context, _, _, condition string) ([]string, error) {
	return f.byCondition[condition], nil
}
func (f *fakeCheckStore) GetPrinciple(_ context.Context, _, _, id string) (*api.Principle, error) {
	if p, ok := f.principles[id]; ok {
		return p, nil
	}
	return nil, store.ErrNotFound
}
func (f *fakeCheckStore) ListEvidenceForTarget(context.Context, string, string, string) ([]api.Evidence, error) {
	return nil, nil
}
func (f *fakeCheckStore) ResolveAuthorityRefs(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return nil, nil // all authorities resolve
}
func (f *fakeCheckStore) RecordActionCheck(context.Context, *api.ActionCheck) error { return nil }
func (f *fakeCheckStore) IncrementCoverage(context.Context, string, string, bool) error {
	return nil
}

// Regression: a high-risk principle promoted under the condition.always sentinel is
// in scope for EVERY action, but it must only BLOCK the action its forbidden move
// names. An unrelated action must NOT inherit the principle's high-risk approval
// requirement — otherwise one always-applicable rule (e.g. never-hot-swap) forces
// needs_human_approval on every action in the domain (the over-blocking the
// re-audit caught on the live cluster).
func TestCheckActionSentinelOnlyDoesNotOverBlock(t *testing.T) {
	hotSwap := &api.Principle{
		ID: "principle.hot_swap", Project: "p", Domain: "d", Status: api.StatusPromotedPrinciple,
		RiskLevel: "high", ForbiddenMoves: []api.ForbiddenMoveRef{"forbidden.hot_swap"},
	}
	s := &Service{store: &fakeCheckStore{
		byCondition: map[string][]string{string(AlwaysConditionRef): {"principle.hot_swap"}},
		principles:  map[string]*api.Principle{"principle.hot_swap": hotSwap},
	}}
	ctx := context.Background()

	// (a) Unrelated action, no conditions declared: the sentinel rule is in scope but
	//     NOT engaged (forbidden move did not match) → allowed + ungoverned.
	resp, err := s.CheckAction(ctx, &api.CheckActionRequest{Project: "p", Domain: "d", ActionType: "inspect_logs"})
	if err != nil {
		t.Fatal(err)
	}
	if r := resp.Result; r.Status != "allowed" || !r.Allowed {
		t.Errorf("unrelated action: status=%q allowed=%v, want allowed (sentinel must not force approval)", r.Status, r.Allowed)
	}
	if resp.Result.Governed {
		t.Error("unrelated action: governed=true, want false (no engaged principle has reach over it)")
	}

	// (b) The named forbidden action: the forbidden move matches via the sentinel →
	//     engaged → blocked.
	resp2, err := s.CheckAction(ctx, &api.CheckActionRequest{Project: "p", Domain: "d", ActionType: "forbidden.hot_swap"})
	if err != nil {
		t.Fatal(err)
	}
	if r := resp2.Result; r.Status != "blocked" || r.Allowed {
		t.Errorf("forbidden action: status=%q allowed=%v, want blocked", r.Status, r.Allowed)
	}
	if !resp2.Result.Governed {
		t.Error("forbidden action: governed=true expected (engaged via forbidden match)")
	}
}

// A high-risk principle scoped to a REAL condition still engages — and demands
// approval — when the caller DECLARES that condition, even if the action does not
// match a forbidden move. Engagement via declared condition must be preserved (only
// sentinel-only-without-match applicability is excluded).
func TestCheckActionDeclaredConditionEngagesGates(t *testing.T) {
	quorum := &api.Principle{
		ID: "principle.quorum", Project: "p", Domain: "d", Status: api.StatusPromotedPrinciple,
		RiskLevel: "high", ForbiddenMoves: []api.ForbiddenMoveRef{"forbidden.restart_before_quorum"},
	}
	s := &Service{store: &fakeCheckStore{
		byCondition: map[string][]string{"condition.etcd.nospace": {"principle.quorum"}},
		principles:  map[string]*api.Principle{"principle.quorum": quorum},
	}}
	resp, err := s.CheckAction(context.Background(), &api.CheckActionRequest{
		Project: "p", Domain: "d", ActionType: "restart_service",
		CurrentConditions: []api.ConditionRef{"condition.etcd.nospace"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Result.Status != "needs_human_approval" {
		t.Errorf("declared high-risk condition: status=%q, want needs_human_approval", resp.Result.Status)
	}
	if !resp.Result.Governed {
		t.Error("declared condition: governed=true expected")
	}
}
