package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

func seedPin(t *testing.T, dir, filename string) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed pin %s: %v", p, err)
	}
	return p
}

func seedInstalledMarker(t *testing.T, base, name, version string) {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir marker %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func pinExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// TestPruneStalePinnedArtifacts is the resilience-pin invariant regression:
// after a successful upgrade A->B, the pinned dir must contain only B for that
// package. A stale A-pin is a latent downgrade path — findLocalPackage searches
// pinnedArtifactDir first, so it would re-stage / resilience-reinstall the older
// artifact (the recurring xds cache_digest_mismatch).
func TestPruneStalePinnedArtifacts(t *testing.T) {
	pinTmp := t.TempDir()
	origPin := pinnedArtifactDir
	pinnedArtifactDir = pinTmp
	t.Cleanup(func() { pinnedArtifactDir = origPin })

	markerTmp := t.TempDir()
	origBase := versionutil.BaseDir()
	versionutil.SetBaseDir(markerTmp)
	t.Cleanup(func() { versionutil.SetBaseDir(origBase) })

	// xds installed version is B (1.2.237); pins hold stale A (1.2.235) + current B.
	seedInstalledMarker(t, markerTmp, "xds", "1.2.237")
	staleA := seedPin(t, pinTmp, "xds_1.2.235_linux_amd64.tgz")
	currentB := seedPin(t, pinTmp, "xds_1.2.237_linux_amd64.tgz")
	// A package with no installed marker: its pin is out of scope and must survive.
	orphan := seedPin(t, pinTmp, "torrent_1.2.233_linux_amd64.tgz")

	pruneStalePinnedArtifacts()

	if pinExists(staleA) {
		t.Errorf("stale pin (xds 1.2.235) must be removed after upgrade to 1.2.237")
	}
	if !pinExists(currentB) {
		t.Errorf("current pin (xds 1.2.237) must be kept — it is the resilience safety net")
	}
	if !pinExists(orphan) {
		t.Errorf("pin for a package with no installed marker must be left untouched (out of scope)")
	}
}

// TestParsePinnedArtifactName covers the filename parse the sweep relies on,
// including kebab names and non-semver (RELEASE.*) versions.
func TestParsePinnedArtifactName(t *testing.T) {
	cases := []struct {
		file        string
		wantName    string
		wantVersion string
	}{
		{"xds_1.2.235_linux_amd64.tgz", "xds", "1.2.235"},
		{"scylla-manager_3.11.1_linux_amd64.tgz", "scylla-manager", "3.11.1"},
		{"minio_RELEASE.2025-09-07T16-13-09Z_linux_amd64.tgz", "minio", "RELEASE.2025-09-07T16-13-09Z"},
		{"not-a-pin.tgz", "", ""},
		{"too_few.tgz", "", ""},
	}
	for _, c := range cases {
		n, v := parsePinnedArtifactName(c.file)
		if n != c.wantName || v != c.wantVersion {
			t.Errorf("parsePinnedArtifactName(%q) = (%q,%q), want (%q,%q)", c.file, n, v, c.wantName, c.wantVersion)
		}
	}
}
