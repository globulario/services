package core_test

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/core"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
)

// ratchetStore returns one promoted principle, indexed under condition.always,
// and resolves everything else so CheckAction reaches its forbidden-move match.
type ratchetStore struct {
	store.Store
	principle *api.Principle
}

func (f *ratchetStore) ListPrincipleIDsByCondition(_ context.Context, _, _, condition string) ([]string, error) {
	if condition == string(core.AlwaysConditionRef) {
		return []string{f.principle.ID}, nil
	}
	return nil, nil
}
func (f *ratchetStore) GetPrinciple(_ context.Context, _, _, id string) (*api.Principle, error) {
	if id == f.principle.ID {
		return f.principle, nil
	}
	return nil, store.ErrNotFound
}
func (f *ratchetStore) ListEvidenceForTarget(context.Context, string, string, string) ([]api.Evidence, error) {
	return nil, nil
}
func (f *ratchetStore) ResolveAuthorityRefs(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return nil, nil
}
func (f *ratchetStore) RecordActionCheck(context.Context, *api.ActionCheck) error { return nil }
func (f *ratchetStore) IncrementCoverage(context.Context, string, string, bool) error {
	return nil
}

// Governed Operation Gateway behavioral ratchet (Slice 5): once
// principle.cluster.no_raw_owner_owned_state_write is PROMOTED, CheckAction must
// REFUSE a raw owner-owned-state write presented under any of its natural action
// names (the action_aliases on forbidden.cluster.raw_owner_owned_state_write) — and
// must NOT over-block unrelated actions. The principle and its aliases are read from
// the real cluster_operator seed pack, so a future edit that drops an alias or
// unbinds the forbidden move breaks this test.
func TestRawOwnerStateWritePromotedBlocksRawWrite(t *testing.T) {
	const (
		principleID = "principle.cluster.no_raw_owner_owned_state_write"
		forbiddenID = "forbidden.cluster.raw_owner_owned_state_write"
	)
	pack := cluster_operator.MustNew()

	// Pull the real seed principle and assert it binds the raw-write forbidden move.
	var seed *domain.PrincipleSeed
	for i := range pack.PrincipleSeeds() {
		if pack.PrincipleSeeds()[i].ID == principleID {
			seed = &pack.PrincipleSeeds()[i]
		}
	}
	if seed == nil {
		t.Fatalf("seed principle %q not found in cluster_operator pack", principleID)
	}
	var bindsForbidden bool
	fm := make([]api.ForbiddenMoveRef, 0, len(seed.ForbiddenMoves))
	for _, m := range seed.ForbiddenMoves {
		fm = append(fm, api.ForbiddenMoveRef(m))
		if m == forbiddenID {
			bindsForbidden = true
		}
	}
	if !bindsForbidden {
		t.Fatalf("seed principle %q must bind forbidden move %q", principleID, forbiddenID)
	}

	// Promote it (the state the live store reaches via the governance gate).
	promoted := &api.Principle{
		ID:             seed.ID,
		Project:        "globular-services",
		Domain:         cluster_operator.DomainName,
		Status:         api.StatusPromotedPrinciple,
		RiskLevel:      seed.RiskLevel,
		ForbiddenMoves: fm,
	}
	reg := domain.NewRegistry()
	reg.Register(pack)
	svc := core.New(&ratchetStore{principle: promoted}, reg)
	ctx := context.Background()

	// Every natural raw-write action name must be refused via the alias index.
	rawWriteActions := []string{
		"etcdctl_put", "etcd_delete", "mcp_raw_write", "write_desired_state_directly",
		"patch_resolved_version", "patch_cache_digest", "set_infra_version_raw",
		"services_desired_set_force_cross_kind", "nodeagent_installed_set_raw",
	}
	for _, action := range rawWriteActions {
		resp, err := svc.CheckAction(ctx, &api.CheckActionRequest{
			Project: "globular-services", Domain: cluster_operator.DomainName, ActionType: action,
		})
		if err != nil {
			t.Fatalf("CheckAction(%q): %v", action, err)
		}
		if r := resp.Result; r.Status != "blocked" || r.Allowed {
			t.Errorf("raw-write action %q: status=%q allowed=%v, want blocked", action, r.Status, r.Allowed)
		}
		if !resp.Result.Governed {
			t.Errorf("raw-write action %q: governed=false, want true (engaged via forbidden match)", action)
		}
	}

	// The sentinel rule must NOT over-block an unrelated owner-routed action.
	resp, err := svc.CheckAction(ctx, &api.CheckActionRequest{
		Project: "globular-services", Domain: cluster_operator.DomainName, ActionType: "upsert_desired_via_owner_rpc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r := resp.Result; r.Status != "allowed" || !r.Allowed {
		t.Errorf("unrelated owner-routed action: status=%q allowed=%v, want allowed (no over-block)", r.Status, r.Allowed)
	}
}
