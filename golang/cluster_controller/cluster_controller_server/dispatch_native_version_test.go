package main

import (
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

// Regression for failure_mode
// release.platform_upgrade_dispatch_rejects_non_semver_versions.
//
// platform_upgrade.dispatch upserts a desired-state record for every package it
// decides to upgrade, including upstream-native (non-SemVer) packages (ffmpeg
// n8.x, minio/mc RELEASE.x). upsertOne previously normalized the version with
// versionutil.Canonical, which rejects native tags as "invalid semver" — so a
// single native package (ffmpeg) failed the whole dispatch step and the platform
// upgrade reported FAILED even though every SemVer package converged. The fix:
// upsertOne normalizes via normalizeDesiredVersion (NormalizeExact), which
// preserves native tags. These tests fail if anyone reverts to Canonical.
func TestNormalizeDesiredVersion_AcceptsNativeTags(t *testing.T) {
	nativeTags := []string{
		"n8.1.2-20260627",              // ffmpeg (the exact version that broke v1.2.249/250)
		"RELEASE.2025-09-07T16-13-09Z", // minio
		"RELEASE.2025-08-13T08-35-41Z", // mc
	}
	for _, v := range nativeTags {
		got, err := normalizeDesiredVersion(v)
		if err != nil {
			t.Fatalf("normalizeDesiredVersion(%q) rejected a native package tag: %v", v, err)
		}
		if got != v {
			t.Errorf("normalizeDesiredVersion(%q) = %q, want the native tag preserved", v, got)
		}
		// Premise guard: the previous implementation (versionutil.Canonical) DID
		// reject these. If Canonical ever starts accepting them this test is moot.
		if _, err := versionutil.Canonical(v); err == nil {
			t.Errorf("premise stale: Canonical(%q) now succeeds — the native-tag regression no longer reproduces", v)
		}
	}
}

func TestNormalizeDesiredVersion_CanonicalizesSemverAndRejectsGarbage(t *testing.T) {
	if got, err := normalizeDesiredVersion(" v1.2.250 "); err != nil || got != "1.2.250" {
		t.Fatalf("normalizeDesiredVersion(\" v1.2.250 \") = %q, err %v; want \"1.2.250\"", got, err)
	}
	if got, err := normalizeDesiredVersion("not a valid tag!"); err == nil {
		t.Errorf("normalizeDesiredVersion accepted an unsafe tag, got %q", got)
	}
	if _, err := normalizeDesiredVersion(""); err == nil {
		t.Error("normalizeDesiredVersion accepted an empty version")
	}
}

// The no-regression floor must tolerate native versions: they cannot be ordered
// by SemVer, so regressesBelowFloor returns false rather than falsely flagging a
// downgrade. This is what makes the dispatch fix safe — a native upgrade is never
// rejected as a regression, and the repository artifact resolver stays the final
// gate on installability.
func TestRegressesBelowFloor_NativeVersionsAreNotRegressions(t *testing.T) {
	if regressesBelowFloor("n8.1.2-20260627", "n8.1.2-20260621") {
		t.Error("native ffmpeg version pair must not be treated as a regression (cannot be ordered)")
	}
	if regressesBelowFloor("RELEASE.2025-09-07T16-13-09Z", "RELEASE.2025-08-13T08-35-41Z") {
		t.Error("native minio version pair must not be treated as a regression")
	}
}
