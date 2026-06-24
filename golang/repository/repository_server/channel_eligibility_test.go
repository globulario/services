package main

// channel_eligibility_test.go — locks the repository's default-list channel set
// (#6 / B). isDefaultListChannel is a QUERY-VISIBILITY filter, deliberately a
// DIFFERENT set from the controller's convergence eligibility
// (isConvergeableChannel). BOOTSTRAP is discoverable here but NOT convergeable
// there — see docs/design/package-lifecycle.md §3.4.5. This test exists so the
// asymmetry cannot be "cleaned up" into agreement by accident.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestDefaultListChannel_DiscoverableSet(t *testing.T) {
	visible := map[repopb.ArtifactChannel]bool{
		repopb.ArtifactChannel_STABLE:        true,
		repopb.ArtifactChannel_CHANNEL_UNSET: true,
		repopb.ArtifactChannel_BOOTSTRAP:     true, // discoverable by design (not convergeable)
		repopb.ArtifactChannel_DEV:           false,
		repopb.ArtifactChannel_CANDIDATE:     false,
		repopb.ArtifactChannel_CANARY:        false,
	}
	for ch, want := range visible {
		if got := isDefaultListChannel(ch); got != want {
			t.Fatalf("isDefaultListChannel(%v) = %v, want %v", ch, got, want)
		}
	}
}

// BOOTSTRAP is included in the repository's default-list set ON PURPOSE. If this
// ever flips, it must be a deliberate contract change (and the controller's
// isConvergeableChannel asymmetry re-evaluated), not an incidental edit.
func TestDefaultListChannel_BootstrapIsDiscoverable(t *testing.T) {
	if !isDefaultListChannel(repopb.ArtifactChannel_BOOTSTRAP) {
		t.Fatal("BOOTSTRAP must remain discoverable in default listings (contract §3.4.5)")
	}
}
