package main

// dev_lane_suffix_test.go — tests for the clarified DEV/local version strategy.
// Local builds publish the platform semver unchanged and rely on repository-owned
// build_number for iteration. Legacy suffixed versions remain accepted for
// compatibility, but suffixes are no longer required or generated.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestDevLaneSuffix_CleanSemverDevAccepted(t *testing.T) {
	if err := validateLocalIdentityRules("local@ryzen", repopb.ArtifactChannel_DEV, "1.2.43"); err != nil {
		t.Fatalf("clean platform semver is valid for DEV/local publish; got %v", err)
	}
}

func TestDevLaneSuffix_SuffixedDevAccepted(t *testing.T) {
	for _, v := range []string{"1.2.43-dev.1", "1.2.43-dev.fix-retry", "1.2.43+local.ryzen.1"} {
		if err := validateLocalIdentityRules("local@ryzen", repopb.ArtifactChannel_DEV, v); err != nil {
			t.Fatalf("suffixed DEV version %q must be accepted; got %v", v, err)
		}
	}
}

// Non-DEV channels are unaffected by Rule 4: a clean-semver STABLE (non-official)
// stays legal — the backstop gates only the DEV lane.
func TestDevLaneSuffix_StableCleanSemverUnaffected(t *testing.T) {
	if err := validateLocalIdentityRules("acme", repopb.ArtifactChannel_STABLE, "1.2.43"); err != nil {
		t.Fatalf("clean-semver non-official STABLE must remain legal; got %v", err)
	}
}

// The official publisher may publish a non-STABLE local/dev artifact under the
// platform version; STABLE promotion remains the release-authority gate.
func TestDevLaneSuffix_OfficialDevCleanSemverAccepted(t *testing.T) {
	if err := validateLocalIdentityRules(officialPublisher, repopb.ArtifactChannel_DEV, "1.2.43"); err != nil {
		t.Fatalf("official + DEV platform semver should be accepted; got %v", err)
	}
}
