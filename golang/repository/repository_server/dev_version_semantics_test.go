package main

// dev_version_semantics_test.go — required tests for DEV/local version semantics:
// a non-STABLE build is build-number-only and never advances the platform release
// stream. The package version stays at the platform semver; build_number iterates
// within it.

import "testing"

func TestDevLaneVersion_KeepsResolvedPlatformVersion(t *testing.T) {
	got := devLaneVersion("1.2.43", "1.2.44")
	if got != "1.2.44" {
		t.Fatalf("DEV/local must keep the repository-resolved platform version; got %q", got)
	}
}

func TestDevLaneVersion_CleanSemverEqualToReleaseIsKept(t *testing.T) {
	if got := devLaneVersion("1.2.43", "1.2.43"); got != "1.2.43" {
		t.Fatalf("clean platform semver must be kept; got %q", got)
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
	if got := devLaneVersion("", "0.0.1"); got != "0.0.1" {
		t.Fatalf("with no release, DEV/local must keep the resolved version; got %q", got)
	}
}
