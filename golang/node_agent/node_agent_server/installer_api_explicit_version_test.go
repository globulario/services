package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withLocalPackageDirs temporarily overrides the package-level localPackageDirs
// for the duration of the test. The dirs argument is searched in order, just
// like the production list.
func withLocalPackageDirs(t *testing.T, dirs ...string) {
	t.Helper()
	original := localPackageDirs
	localPackageDirs = dirs
	t.Cleanup(func() { localPackageDirs = original })
}

func writeTgz(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not-a-real-tgz"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestFindLocalPackage_ExplicitVersion_ReturnsExactMatch — happy path.
// Explicit version present in cache → returns it.
func TestFindLocalPackage_ExplicitVersion_ReturnsExactMatch(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "repository_1.2.115_linux_amd64.tgz")
	writeTgz(t, want)
	writeTgz(t, filepath.Join(dir, "repository_1.2.110_linux_amd64.tgz"))

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("repository", "1.2.115", "linux_amd64")
	if got != want {
		t.Fatalf("findLocalPackage = %q, want %q", got, want)
	}
}

// TestFindLocalPackage_ExplicitVersion_NoExact_ReturnsEmpty — the regression
// for the v1.2.115 install incident: when the caller passed an explicit
// version and only an OLDER package was on disk, the previous code returned
// the older package via wildcard fallback. After the fix, this must return
// empty so the installer surfaces "not found" instead of silently installing
// the wrong binary.
func TestFindLocalPackage_ExplicitVersion_NoExact_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeTgz(t, filepath.Join(dir, "repository_1.2.110_linux_amd64.tgz"))
	writeTgz(t, filepath.Join(dir, "repository_1.2.114_linux_amd64.tgz"))

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("repository", "1.2.115", "linux_amd64")
	if got != "" {
		t.Fatalf("findLocalPackage with explicit version 1.2.115 returned %q; "+
			"expected empty when no exact match exists. Wildcard fallback "+
			"must NOT substitute an older version for an explicit request.", got)
	}
}

// TestFindLocalPackage_ExplicitVersion_v1_2_115_MustNotResolve_v1_2_110 —
// exact regression case from the incident: explicit v1.2.115 must not return
// v1.2.110 archive sitting next to it.
func TestFindLocalPackage_ExplicitVersion_v1_2_115_MustNotResolve_v1_2_110(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "repository_1.2.110_linux_amd64.tgz")
	writeTgz(t, older)

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("repository", "1.2.115", "linux_amd64")
	if got == older {
		t.Fatalf("findLocalPackage('repository', '1.2.115', ...) returned %q "+
			"— v1.2.110 archive must NEVER satisfy an explicit v1.2.115 request",
			got)
	}
	if got != "" {
		t.Fatalf("findLocalPackage returned %q, want \"\" (no exact match exists)", got)
	}
}

// TestFindLocalPackage_EmptyVersion_AllowsWildcard — Day-1 bootstrap path.
// Empty version means caller has no authoritative version; wildcard fallback
// is permitted to pick up Day-0-staged packages.
func TestFindLocalPackage_EmptyVersion_AllowsWildcard(t *testing.T) {
	dir := t.TempDir()
	writeTgz(t, filepath.Join(dir, "envoy_1.35.3_linux_amd64.tgz"))

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("envoy", "", "linux_amd64")
	if got == "" {
		t.Fatalf("findLocalPackage with empty version returned empty; "+
			"expected wildcard fallback to find envoy_1.35.3_linux_amd64.tgz")
	}
}

// TestFindLocalPackage_DevVersion_AllowsWildcard — the explicit "0.0.0-dev"
// sentinel also opts into wildcard. Local builds without ldflags inject
// "0.0.0-dev"; before the proper version is known, wildcard is acceptable.
func TestFindLocalPackage_DevVersion_AllowsWildcard(t *testing.T) {
	dir := t.TempDir()
	writeTgz(t, filepath.Join(dir, "envoy_1.35.3_linux_amd64.tgz"))

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("envoy", "0.0.0-dev", "linux_amd64")
	if got == "" {
		t.Fatalf("findLocalPackage with 0.0.0-dev returned empty; "+
			"expected wildcard fallback to find envoy_1.35.3_linux_amd64.tgz")
	}
}

// TestFindLocalPackage_ExplicitVersion_RejectsBareUnversionedArchive locks in
// the AWG re-audit gap: an explicit-version request must not be satisfied by
// the un-versioned "<name>.tgz" candidate, which carries no version in its
// filename and would install an unknown binary under the pinned label — the
// same wrong-binary-declares-success class as the wildcard fallback
// (installer.explicit_version_requires_exact_artifact, INC-2026-0012).
func TestFindLocalPackage_ExplicitVersion_RejectsBareUnversionedArchive(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "repository.tgz")
	writeTgz(t, bare)

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("repository", "1.2.115", "linux_amd64")
	if got == bare {
		t.Fatalf("explicit version 1.2.115 resolved to un-versioned %q — a bare "+
			"<name>.tgz must never satisfy an explicit-version request", got)
	}
	if got != "" {
		t.Fatalf("findLocalPackage = %q, want \"\" (no versioned match exists)", got)
	}
}

// TestFindLocalPackage_EmptyVersion_AllowsBareUnversionedArchive — the
// complement: the non-explicit (Day-1 bootstrap) path may still use the bare
// archive when the caller has no authoritative version.
func TestFindLocalPackage_EmptyVersion_AllowsBareUnversionedArchive(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "envoy.tgz")
	writeTgz(t, bare)

	withLocalPackageDirs(t, dir)
	srv := &NodeAgentServer{}
	got := srv.findLocalPackage("envoy", "", "linux_amd64")
	if got != bare {
		t.Fatalf("empty version should accept bare envoy.tgz, got %q", got)
	}
}

// TestIsExplicitVersion verifies the predicate directly.
func TestIsExplicitVersion(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"1.2.115", true},
		{"1.0.0", true},
		{"v1.2.3", true}, // already has v prefix but still explicit
		{"2.1.80", true},
		{"RELEASE.2025-08-13T08-35-41Z", true},
		{"", false},
		{"   ", false},
		{"0.0.0-dev", false},
	}
	for _, tc := range cases {
		if got := isExplicitVersion(tc.version); got != tc.want {
			t.Errorf("isExplicitVersion(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}
