package main

// dev_lane_suffix_test.go — required tests for the DEV-requires-lane-suffix
// backstop (#6c, validateLocalIdentityRules Rule 4). Completes the dev/release
// boundary on the direct publish path: a DEV artifact must carry a local/dev
// version suffix and may never occupy a clean release version. The CLI emits the
// suffix for --channel dev/local by construction; this is the server backstop.

import (
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestDevLaneSuffix_CleanSemverDevRejected(t *testing.T) {
	err := validateLocalIdentityRules("local@ryzen", repopb.ArtifactChannel_DEV, "1.2.43")
	if err == nil {
		t.Fatal("clean-semver DEV must be rejected (Rule 4)")
	}
	if !strings.Contains(err.Error(), "DEV requires a local/dev version suffix") {
		t.Fatalf("error should explain the DEV-suffix requirement; got %v", err)
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

// Official + DEV is still caught by Rule 2 (before Rule 4) with its own message —
// Rule 4 must not change that precedence.
func TestDevLaneSuffix_OfficialDevStillRule2(t *testing.T) {
	err := validateLocalIdentityRules(officialPublisher, repopb.ArtifactChannel_DEV, "1.2.43-dev.1")
	if err == nil {
		t.Fatal("official + DEV must be rejected (Rule 2)")
	}
	if !strings.Contains(err.Error(), "may not publish to DEV channel") {
		t.Fatalf("official+DEV should be rejected by Rule 2, not Rule 4; got %v", err)
	}
}
