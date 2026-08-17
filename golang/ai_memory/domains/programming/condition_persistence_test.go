package programming_test

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	programming "github.com/globulario/services/golang/ai_memory/domains/programming"
)

// TestConditionDetectionSurvivesCatalogPersistence guards a cross-layer naming
// mismatch: the programming seed once authored `detect_hint`, while the generic
// loader persists only the canonical `detect_spec` field into api.Condition.
// The pack therefore looked populated in-process but lost the semantics that tell
// a runtime caller when each condition applies.
func TestConditionDetectionSurvivesCatalogPersistence(t *testing.T) {
	ctx := context.Background()
	pack, err := programming.New()
	if err != nil {
		t.Fatalf("programming.New: %v", err)
	}
	st := store.NewMemoryStore()
	if _, err := domain.LoadCatalogs(ctx, st, "globular-services", pack); err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}

	for _, seed := range pack.Catalogs().Conditions {
		if seed.Fields["detect_spec"] == "" {
			t.Fatalf("condition %q has no canonical detect_spec in the domain pack", seed.ID)
		}
		persisted, err := st.GetCondition(ctx, "globular-services", programming.DomainName, seed.ID)
		if err != nil {
			t.Fatalf("GetCondition(%s): %v", seed.ID, err)
		}
		if persisted.DetectSpec == "" {
			t.Errorf("condition %q lost detect_spec during persistence", seed.ID)
		}
		if persisted.DetectSpec != seed.Fields["detect_spec"] {
			t.Errorf("condition %q detect_spec=%q want %q", seed.ID, persisted.DetectSpec, seed.Fields["detect_spec"])
		}
	}
}
