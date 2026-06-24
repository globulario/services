package main

// channel_eligibility_test.go — locks the controller's convergence-eligibility
// set (#6 / B). isConvergeableChannel is the SINGLE authority on "may this become
// desired state": STABLE / CHANNEL_UNSET only. It is deliberately a DIFFERENT set
// from the repository's default-list visibility (isDefaultListChannel), which
// also includes BOOTSTRAP. BOOTSTRAP is discoverable/servable but NEVER an
// auto-convergence target — see docs/design/package-lifecycle.md §3.4.5. This
// test exists so the asymmetry cannot be "cleaned up" into agreement by accident.

import (
	"testing"

	"github.com/globulario/services/golang/repository/repositorypb"
)

func TestConvergeableChannel_BootstrapExcluded(t *testing.T) {
	convergeable := map[repositorypb.ArtifactChannel]bool{
		repositorypb.ArtifactChannel_STABLE:        true,
		repositorypb.ArtifactChannel_CHANNEL_UNSET: true,  // legacy ⇒ STABLE
		repositorypb.ArtifactChannel_BOOTSTRAP:     false, // discoverable, NOT convergeable
		repositorypb.ArtifactChannel_DEV:           false,
		repositorypb.ArtifactChannel_CANDIDATE:     false,
		repositorypb.ArtifactChannel_CANARY:        false,
	}
	for ch, want := range convergeable {
		if got := isConvergeableChannel(ch); got != want {
			t.Fatalf("isConvergeableChannel(%v) = %v, want %v (contract §3.4.5: do not widen)", ch, got, want)
		}
	}
}
