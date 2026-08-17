package domain_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

type upgradeDomain struct {
	name string
	cats domain.Catalogs
}

func (d upgradeDomain) Name() string              { return d.name }
func (d upgradeDomain) Catalogs() domain.Catalogs { return d.cats }

func upgradeTestDomain() upgradeDomain {
	return upgradeDomain{
		name: "upgrade_test",
		cats: domain.Catalogs{
			Conditions: []domain.CatalogEntry{
				{ID: "condition.always", Title: "universal reach", Fields: map[string]string{"detect_spec": "always"}},
				{ID: "condition.old", Title: "old condition", Fields: map[string]string{"detect_spec": "old"}},
				{ID: "condition.new_semantic", Title: "new semantic condition", Fields: map[string]string{"detect_spec": "new"}},
			},
			Principles: []domain.PrincipleSeed{
				{
					ID:          "principle.upgrade.seed",
					Title:       "seed-backed promoted principle",
					AppliesWhen: []string{"condition.always", "condition.new_semantic"},
				},
			},
		},
	}
}

func TestLoadCatalogsMigratesPromotedSeedToCanonicalAlwaysReach(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	pack := upgradeTestDomain()

	before := api.Principle{
		ID:          "principle.upgrade.seed",
		Project:     "project-test",
		Domain:      api.DomainRef(pack.Name()),
		Title:       "already promoted old seed",
		AppliesWhen: []api.ConditionRef{"condition.old"},
		Status:      api.StatusPromotedPrinciple,
		ApprovedBy:  "human-owner",
		Metadata: map[string]string{
			"source":      "seed",
			"immutable":   "true",
			"domain_pack": pack.Name(),
		},
	}
	if err := st.CreatePrinciple(ctx, &before); err != nil {
		t.Fatalf("CreatePrinciple: %v", err)
	}
	if err := st.IndexPromotedPrinciple(ctx, &before); err != nil {
		t.Fatalf("IndexPromotedPrinciple(old): %v", err)
	}

	if ids, err := st.ListPrincipleIDsByCondition(ctx, before.Project, pack.Name(), "condition.always"); err != nil {
		t.Fatalf("ListPrincipleIDsByCondition(always) before: %v", err)
	} else if len(ids) != 0 {
		t.Fatalf("always index unexpectedly populated before migration: %v", ids)
	}

	res, err := domain.LoadCatalogs(ctx, st, before.Project, pack)
	if err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}
	if res.PromotedPrinciplesReindexed != 1 {
		t.Fatalf("PromotedPrinciplesReindexed=%d want 1", res.PromotedPrinciplesReindexed)
	}
	if res.PrinciplesSkipped != 1 || res.PrinciplesSeeded != 0 {
		t.Fatalf("load result=%+v want one skipped governed principle and no re-seed", res)
	}

	ids, err := st.ListPrincipleIDsByCondition(ctx, before.Project, pack.Name(), "condition.always")
	if err != nil {
		t.Fatalf("ListPrincipleIDsByCondition(always) after: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{before.ID}) {
		t.Fatalf("always index=%v want [%s]", ids, before.ID)
	}

	// A newer seed also introduced this semantic condition. Startup migration must
	// not silently broaden an already-authoritative principle with it.
	ids, err = st.ListPrincipleIDsByCondition(ctx, before.Project, pack.Name(), "condition.new_semantic")
	if err != nil {
		t.Fatalf("ListPrincipleIDsByCondition(new semantic): %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("new semantic condition was silently adopted into active index: %v", ids)
	}

	after, err := st.GetPrinciple(ctx, before.Project, pack.Name(), before.ID)
	if err != nil {
		t.Fatalf("GetPrinciple after migration: %v", err)
	}
	if after.Status != api.StatusPromotedPrinciple || after.ApprovedBy != before.ApprovedBy {
		t.Fatalf("migration changed authority fields: status=%q approved_by=%q", after.Status, after.ApprovedBy)
	}
	wantConditions := []api.ConditionRef{"condition.old", "condition.always"}
	if !reflect.DeepEqual(after.AppliesWhen, wantConditions) {
		t.Fatalf("canonical applies_when=%v want only old scope plus technical sentinel %v", after.AppliesWhen, wantConditions)
	}
	if got := after.Metadata[domain.RuntimeAlwaysReachMetadataKey]; got != domain.RuntimeAlwaysReachSeedMigration {
		t.Fatalf("migration marker=%q want %q", got, domain.RuntimeAlwaysReachSeedMigration)
	}

	// RevokePrinciple delegates to this same deindex operation using the stored
	// canonical AppliesWhen. Prove the migrated sentinel cannot become a stale
	// index ghost after revocation/deindex.
	if err := st.DeindexPromotedPrinciple(ctx, after); err != nil {
		t.Fatalf("DeindexPromotedPrinciple(migrated): %v", err)
	}
	ids, err = st.ListPrincipleIDsByCondition(ctx, before.Project, pack.Name(), "condition.always")
	if err != nil {
		t.Fatalf("ListPrincipleIDsByCondition(always) after deindex: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("migrated always reach survived deindex: %v", ids)
	}
}

func TestLoadCatalogsDoesNotReindexNonSeedPromotedPrinciple(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	pack := upgradeTestDomain()

	p := api.Principle{
		ID:          "principle.upgrade.seed",
		Project:     "project-test",
		Domain:      api.DomainRef(pack.Name()),
		AppliesWhen: []api.ConditionRef{"condition.old"},
		Status:      api.StatusPromotedPrinciple,
		ApprovedBy:  "human-owner",
		Metadata:    map[string]string{"source": "human", "domain_pack": pack.Name()},
	}
	if err := st.CreatePrinciple(ctx, &p); err != nil {
		t.Fatalf("CreatePrinciple: %v", err)
	}
	if err := st.IndexPromotedPrinciple(ctx, &p); err != nil {
		t.Fatalf("IndexPromotedPrinciple(old): %v", err)
	}

	res, err := domain.LoadCatalogs(ctx, st, p.Project, pack)
	if err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}
	if res.PromotedPrinciplesReindexed != 0 {
		t.Fatalf("human-authored promoted principle was reindexed from seed: %+v", res)
	}
	ids, err := st.ListPrincipleIDsByCondition(ctx, p.Project, pack.Name(), "condition.always")
	if err != nil {
		t.Fatalf("ListPrincipleIDsByCondition(always): %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("human-authored principle gained seed-only always reach: %v", ids)
	}
}
