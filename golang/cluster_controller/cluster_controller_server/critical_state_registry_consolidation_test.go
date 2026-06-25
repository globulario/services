package main

import (
	"testing"

	"github.com/globulario/services/golang/config"
)

// TestCriticalStateRegistryAgreesWithConfigOwnership is the owner-guard
// consolidation ratchet.
//
// There are two tables describing critical-key ownership: config.CriticalKeyPolicies
// (the single ownership authority — what the runtime write primitive guards every
// critical write against, and what ValidateCriticalKeyWrite now delegates to) and
// this package's criticalStateRegistry (the rich restore / doctor / LKG metadata
// table). config cannot import this package, so the two are necessarily separate
// objects. This test locks their ownership in step: every key must appear in both,
// with the same owner. If they ever drift, there would again be two conflicting
// sources of ownership truth — the exact condition this consolidation removed.
func TestCriticalStateRegistryAgreesWithConfigOwnership(t *testing.T) {
	// 1. Every controller-registry key resolves to the same owner in config.
	for _, rec := range criticalStateRegistry {
		owner, err := config.OwnerForKey(rec.Key)
		if err != nil {
			t.Errorf("registry key %q has no config.CriticalKeyPolicies entry — ownership tables diverged", rec.Key)
			continue
		}
		if owner != rec.Owner {
			t.Errorf("owner mismatch for %q: criticalStateRegistry=%q config.CriticalKeyPolicies=%q", rec.Key, rec.Owner, owner)
		}
		// LookupCriticalKey must round-trip the same record (rich-metadata lookup API).
		if got := LookupCriticalKey(rec.Key); got == nil || got.Owner != rec.Owner {
			t.Errorf("LookupCriticalKey(%q) inconsistent with the registry entry", rec.Key)
		}
	}

	// 2. Every config policy key has a matching registry entry (same key set).
	for _, p := range config.CriticalKeyPolicies {
		if LookupCriticalKey(p.Key) == nil {
			t.Errorf("config policy key %q has no criticalStateRegistry entry — ownership tables diverged", p.Key)
		}
	}
}
