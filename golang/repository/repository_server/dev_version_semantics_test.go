package main

// dev_version_semantics_test.go — required tests for DEV version semantics (P5):
// a DEV build is build-number-only / lane-safe and never advances the release
// stream. The version is pinned off the release stream with a `-dev` pre-release
// suffix; build_number iterates within it.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
)

func TestDevLaneVersion_BumpedReleaseSemverIsPinnedToLatestRelease(t *testing.T) {
	// resolveVersionIntent bumped 1.2.43 → 1.2.44, but the build landed on DEV:
	// it must be pinned to the actual latest release, never the bumped version.
	got := devLaneVersion("1.2.43", "1.2.44")
	if got != "1.2.43-dev.1" {
		t.Fatalf("bumped DEV must pin to latest release 1.2.43; got %q", got)
	}
}

func TestDevLaneVersion_CleanSemverEqualToReleaseIsSuffixed(t *testing.T) {
	if got := devLaneVersion("1.2.43", "1.2.43"); got != "1.2.43-dev.1" {
		t.Fatalf("clean DEV semver must be suffixed; got %q", got)
	}
}

func TestDevLaneVersion_AlreadyLaneSafeVersionsAreKept(t *testing.T) {
	for _, v := range []string{"1.2.43-dev.fix-retry", "1.2.43+local.ryzen.2", "1.2.43-hotfix.1"} {
		if got := devLaneVersion("1.2.43", v); got != v {
			t.Fatalf("already lane-safe version %q must be kept; got %q", v, got)
		}
	}
}

func TestDevLaneVersion_NoReleaseYetSuffixesResolvedVersion(t *testing.T) {
	// First-ever build of a package (no published release): suffix the resolved
	// version so it is still a pre-release, claiming no release.
	if got := devLaneVersion("", "0.0.1"); got != "0.0.1-dev.1" {
		t.Fatalf("with no release, DEV must suffix the resolved version; got %q", got)
	}
}

// The coerced DEV version must be a legal identity lane for a non-official
// publisher, and must semver-order strictly BELOW the release it is pinned to —
// proving it cannot squat or advance the release stream.
func TestDevLaneVersion_CoercedResultIsLaneLegalAndBelowRelease(t *testing.T) {
	got := devLaneVersion("1.2.43", "1.2.44")

	if err := validateLocalIdentityRules("local@ryzen", repopb.ArtifactChannel_DEV, got); err != nil {
		t.Fatalf("coerced DEV version %q must satisfy identity-lane rules; got %v", got, err)
	}
	if !hasLocalVersionSuffix(got) {
		t.Fatalf("coerced DEV version %q must carry a local/dev suffix", got)
	}
	cmp, err := versionutil.Compare(got, "1.2.43")
	if err != nil {
		t.Fatalf("compare %q vs release: %v", got, err)
	}
	if cmp >= 0 {
		t.Fatalf("coerced DEV version %q must order strictly below the release 1.2.43 (cmp=%d)", got, cmp)
	}
}
