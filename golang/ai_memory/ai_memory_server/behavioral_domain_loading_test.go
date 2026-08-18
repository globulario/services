package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	"google.golang.org/grpc"
)

// TestRegisteredBehavioralDomainsArePersisted guards the split-brain startup
// bug where a pack was present in the in-process registry but absent from the
// store-backed discovery/promotion path. Runtime registration and persistence
// must derive from the same registry.
func TestRegisteredBehavioralDomainsArePersisted(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	reg := behavioralRegistry()

	loads := loadRegisteredBehavioralDomains(ctx, st)
	if len(loads) != len(reg.Names()) {
		t.Fatalf("loaded %d domains, registry contains %d", len(loads), len(reg.Names()))
	}
	for _, load := range loads {
		if load.Err != nil {
			t.Fatalf("load domain %q: %v", load.Name, load.Err)
		}
	}

	for _, name := range reg.Names() {
		pack, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("registry lost domain %q during load", name)
		}
		cats := pack.Catalogs()

		auths, err := st.ListAuthorities(ctx, behavioralSeedProject, name, 0)
		if err != nil {
			t.Fatalf("list %s authorities: %v", name, err)
		}
		if len(auths) != len(cats.Authorities) {
			t.Errorf("%s authorities persisted=%d want=%d", name, len(auths), len(cats.Authorities))
		}

		conds, err := st.ListConditions(ctx, behavioralSeedProject, name, 0)
		if err != nil {
			t.Fatalf("list %s conditions: %v", name, err)
		}
		if len(conds) != len(cats.Conditions) {
			t.Errorf("%s conditions persisted=%d want=%d", name, len(conds), len(cats.Conditions))
		}

		for _, seeded := range cats.Principles {
			p, err := st.GetPrinciple(ctx, behavioralSeedProject, name, seeded.ID)
			if err != nil {
				t.Errorf("%s seed principle %q not persisted: %v", name, seeded.ID, err)
				continue
			}
			if p.Status != api.StatusProposedPrinciple {
				t.Errorf("%s seed principle %q status=%q want PROPOSED_PRINCIPLE", name, seeded.ID, p.Status)
			}
		}
	}
}

// This is the exact production regression behind #249/#248: programming can be
// registered while its store-backed authority catalog is empty.
func TestProgrammingCatalogPersistsFromRegisteredDomains(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	loadRegisteredBehavioralDomains(ctx, st)

	auths, err := st.ListAuthorities(ctx, behavioralSeedProject, "programming", 0)
	if err != nil {
		t.Fatalf("list programming authorities: %v", err)
	}
	if len(auths) == 0 {
		t.Fatal("programming is registered but persisted 0 authorities")
	}
}

// TestRegisterBehavioralServiceSeedsProgramming proves the real composition
// root invokes the shared loader. A helper-only test would stay green if someone
// later removed the production hook and recreated the original split-brain bug.
func TestRegisterBehavioralServiceSeedsProgramming(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	gs := grpc.NewServer()
	defer gs.Stop()

	registerBehavioralService(gs, st)

	auths, err := st.ListAuthorities(ctx, behavioralSeedProject, "programming", 0)
	if err != nil {
		t.Fatalf("list programming authorities after service registration: %v", err)
	}
	if len(auths) == 0 {
		t.Fatal("registerBehavioralService left programming catalog empty")
	}
}
