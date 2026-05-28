package main

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// withLocalInstallPackageDir temporarily overrides the package-level
// localInstallPackageDir for the duration of the test.
func withLocalInstallPackageDir(t *testing.T, dir string) {
	t.Helper()
	original := localInstallPackageDir
	localInstallPackageDir = dir
	t.Cleanup(func() { localInstallPackageDir = original })
}

// TestMaterializeLocalPackageArchive_WritesTgzToLocalInstallDir — sync's
// success-path materialization must drop the .tgz where node-agent's
// local-only install path searches. This is the bridge between the
// "imported" state (in repository service storage) and the "installable"
// state (in /var/lib/globular/packages/). Without it, every Day-2 upgrade
// hits findLocalPackage with no match.
func TestMaterializeLocalPackageArchive_WritesTgzToLocalInstallDir(t *testing.T) {
	dir := t.TempDir()
	withLocalInstallPackageDir(t, dir)

	payload := []byte("pretend-this-is-a-real-tgz-with-real-content")
	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", payload)

	want := filepath.Join(dir, "repository_1.2.116_linux_amd64.tgz")
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected materialized archive at %s; got read error: %v", want, err)
	}
	if string(got) != string(payload) {
		t.Errorf("materialized content mismatch")
	}
}

// TestMaterializeLocalPackageArchive_RespectsUpstreamFilename — when the
// upstream BOM provided a filename (e.g. awareness-bundle-1.2.116-abc123.tar.gz)
// it must be preserved exactly. Substituting <name>_<version>_<platform>.tgz
// would break the install side's filename match.
func TestMaterializeLocalPackageArchive_RespectsUpstreamFilename(t *testing.T) {
	dir := t.TempDir()
	withLocalInstallPackageDir(t, dir)

	payload := []byte("bundle-bytes")
	materializeLocalPackageArchive(
		"globular-awareness-bundle", "1.2.116", "noarch",
		"awareness-bundle-1.2.116-abc123.tar.gz",
		payload,
	)

	want := filepath.Join(dir, "awareness-bundle-1.2.116-abc123.tar.gz")
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected materialized archive at %s; got error: %v", want, err)
	}
	if string(got) != string(payload) {
		t.Errorf("materialized content mismatch")
	}
}

// TestMaterializeLocalPackageArchive_IsIdempotent — same payload twice must
// leave the same single file with the same content. No partial state, no
// duplicate temp files.
func TestMaterializeLocalPackageArchive_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	withLocalInstallPackageDir(t, dir)

	payload := []byte("identical-payload")
	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", payload)

	statBefore, err := os.Stat(filepath.Join(dir, "repository_1.2.116_linux_amd64.tgz"))
	if err != nil {
		t.Fatalf("first materialize failed: %v", err)
	}

	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", payload)

	statAfter, err := os.Stat(filepath.Join(dir, "repository_1.2.116_linux_amd64.tgz"))
	if err != nil {
		t.Fatalf("second materialize: target missing: %v", err)
	}
	if statBefore.Size() != statAfter.Size() {
		t.Errorf("size changed across idempotent materialize: %d -> %d", statBefore.Size(), statAfter.Size())
	}

	// No temp files lingering.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	tgzCount := 0
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".tgz" {
			tgzCount++
		}
		if filepath.Ext(name) == ".tmp" || (len(name) > 4 && name[len(name)-4:] == ".tmp") {
			t.Errorf("temp file lingered after idempotent materialize: %s", name)
		}
	}
	if tgzCount != 1 {
		t.Errorf("expected exactly 1 .tgz, found %d", tgzCount)
	}
}

// TestMaterializeLocalPackageArchive_FailedImportDoesNotWriteFinal — when
// the underlying write fails (simulated here by pointing at an unwritable
// dir), the final target must NOT exist with corrupt content. Best-effort
// failure is logged; no partial archive is left at the final path.
func TestMaterializeLocalPackageArchive_FailedImportDoesNotWriteFinal(t *testing.T) {
	// Point at a path that cannot be created (a regular file masquerading as
	// a parent dir). The function should log a warning and return without
	// writing the target.
	bogus := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(bogus, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	withLocalInstallPackageDir(t, filepath.Join(bogus, "subdir"))

	payload := []byte("would-have-been-written")
	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", payload)

	target := filepath.Join(bogus, "subdir", "repository_1.2.116_linux_amd64.tgz")
	if _, err := os.Stat(target); err == nil {
		t.Errorf("target was created despite mkdir failure: %s", target)
	}
}

// TestMaterializeLocalPackageArchive_VersionMatchesImportedManifest — the
// version segment in the materialized filename must match the n.Version
// the caller passed in. A v1.2.116 import must NEVER end up at
// repository_1.2.110_linux_amd64.tgz (which is what caused the original
// incident — wildcard fallback to the older archive).
func TestMaterializeLocalPackageArchive_VersionMatchesImportedManifest(t *testing.T) {
	dir := t.TempDir()
	withLocalInstallPackageDir(t, dir)

	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", []byte("payload"))

	expected := filepath.Join(dir, "repository_1.2.116_linux_amd64.tgz")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected %s, got error: %v", expected, err)
	}
	// The version from the prior local state must not be created.
	forbidden := filepath.Join(dir, "repository_1.2.110_linux_amd64.tgz")
	if _, err := os.Stat(forbidden); err == nil {
		t.Errorf("v1.2.110 archive was created for a v1.2.116 import; "+
			"materialize must use the caller's version verbatim")
	}
}

// TestMaterializeLocalPackageArchive_EmptyDataIsNoOp — degenerate input
// (no payload) must not create an empty file at the target path.
func TestMaterializeLocalPackageArchive_EmptyDataIsNoOp(t *testing.T) {
	dir := t.TempDir()
	withLocalInstallPackageDir(t, dir)

	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", nil)
	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", []byte{})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty payload created files in dir: %d entries", len(entries))
	}
}

// TestMaterializeLocalPackageArchive_DigestPreservation — content materialized
// must hash to the same sha256 as the original payload bytes.
func TestMaterializeLocalPackageArchive_DigestPreservation(t *testing.T) {
	dir := t.TempDir()
	withLocalInstallPackageDir(t, dir)

	payload := []byte("real-package-bytes-with-meaningful-content-for-digest-check")
	want := sha256.Sum256(payload)

	materializeLocalPackageArchive("repository", "1.2.116", "linux_amd64", "", payload)

	got, err := os.ReadFile(filepath.Join(dir, "repository_1.2.116_linux_amd64.tgz"))
	if err != nil {
		t.Fatalf("read materialized: %v", err)
	}
	if sha256.Sum256(got) != want {
		t.Errorf("digest mismatch between source payload and materialized archive")
	}
}
