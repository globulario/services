package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// Registration and persistence must be driven from one list.
//
// They were not. behavioralRegistry listed cluster_operator and programming
// while loadBehavioralSeed persisted only cluster_operator, so the programming
// pack resolved in-process while its store-backed catalogs stayed empty. The
// discovery and promotion surfaces read the store, so that domain answered every
// catalog query empty while appearing registered — 0% governance coverage with
// no error anywhere to explain it.
func TestEveryShippedPackIsBothRegisteredAndSeeded(t *testing.T) {
	packs := shippedPacks()
	if len(packs) < 2 {
		t.Fatalf("expected the shipped set to include more than one pack, got %d", len(packs))
	}

	reg := behavioralRegistry()
	st := store.NewMemoryStore()
	loadBehavioralSeed(st)

	ctx := context.Background()
	for _, p := range packs {
		pack, err := p.load()
		if err != nil {
			t.Fatalf("shipped pack %q does not load: %v", p.name, err)
		}
		ref := pack.Name()

		// Registered: the kernel can resolve it in-process.
		if _, ok := reg.Lookup(ref); !ok {
			t.Errorf("pack %q is shipped but not registered — the kernel cannot resolve it", p.name)
		}

		// Seeded: the surfaces that actually answer catalog queries read the
		// store, so being registered is not enough.
		authorities, err := st.ListAuthorities(ctx, behavioralSeedProject, ref, 0)
		if err != nil {
			t.Fatalf("list authorities for %q: %v", p.name, err)
		}
		if len(authorities) == 0 {
			t.Errorf("pack %q is registered but its authorities were never persisted — "+
				"discovery and promotion read the store, so this domain is ungoverned at runtime", p.name)
		}
	}
}

// A pack that fails to load must not silently reduce the shipped set.
func TestShippedPacksAllLoad(t *testing.T) {
	for _, p := range shippedPacks() {
		if _, err := p.load(); err != nil {
			t.Errorf("shipped pack %q is invalid: %v", p.name, err)
		}
	}
}

var _ = domain.Registry{}
