package main

// channel_eligibility_test.go locks the controller's convergence-eligibility set.
// isConvergeableChannel is the single authority on "may this become desired
// state": release-tier artifacts may converge; DEV artifacts may not.

import (
	"testing"

	"github.com/globulario/services/golang/repository/repositorypb"
)

func TestConvergeableChannel_ReleaseTiersAllowedDevExcluded(t *testing.T) {
	convergeable := map[repositorypb.ArtifactChannel]bool{
		repositorypb.ArtifactChannel_STABLE:        true,
		repositorypb.ArtifactChannel_CHANNEL_UNSET: true, // legacy ⇒ STABLE
		repositorypb.ArtifactChannel_BOOTSTRAP:     true,
		repositorypb.ArtifactChannel_DEV:           false,
		repositorypb.ArtifactChannel_CANDIDATE:     true,
		repositorypb.ArtifactChannel_CANARY:        true,
	}
	for ch, want := range convergeable {
		if got := isConvergeableChannel(ch); got != want {
			t.Fatalf("isConvergeableChannel(%v) = %v, want %v", ch, got, want)
		}
	}
}
