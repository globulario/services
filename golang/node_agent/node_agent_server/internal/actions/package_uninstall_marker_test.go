package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

func writeMarker(t *testing.T, base, name, version string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir marker %s: %v", dir, err)
	}
	mk := filepath.Join(dir, "version")
	if err := os.WriteFile(mk, []byte(version+"\n"), 0o644); err != nil {
		t.Fatalf("write marker %s: %v", mk, err)
	}
	return mk
}

// TestRemoveSyncReadVersionMarker is the regression for the torrent stub bug:
// uninstall used to leave the version marker that the installed-state sync reads
// (versionutil.MarkerPath = BaseDir/<name>/version), so syncInstalledStateToEtcd
// re-discovered the package and re-minted a degraded installed-state record — the
// orphan that survived uninstall. The uninstall paths must remove that marker so
// the uninstall is idempotent (reconciliation.must_be_idempotent_and_bounded).
func TestRemoveSyncReadVersionMarker(t *testing.T) {
	tmp := t.TempDir()
	orig := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(orig) })

	target := writeMarker(t, tmp, "torrent", "1.2.233")
	// An unrelated package's marker must be untouched.
	other := writeMarker(t, tmp, "dns", "1.2.235")

	removeSyncReadVersionMarker("torrent")

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("torrent version marker must be removed (sync re-mints a stub otherwise); stat err=%v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated package marker (dns) must survive uninstall, got err=%v", err)
	}
}

// TestRemoveSyncReadVersionMarker_LegacyUnderscore covers packages whose marker
// was written under the legacy underscore form (e.g. scylla_manager) while the
// canonical name is hyphenated (scylla-manager). Both must be removed.
func TestRemoveSyncReadVersionMarker_LegacyUnderscore(t *testing.T) {
	tmp := t.TempDir()
	orig := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(orig) })

	legacy := writeMarker(t, tmp, "scylla_manager", "3.11.1")

	removeSyncReadVersionMarker("scylla-manager")

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy underscore marker (scylla_manager) must be removed; stat err=%v", err)
	}
}

// TestRemoveSyncReadVersionMarker_EmptyNameIsNoop guards against a blank name
// wiping the marker base directory.
func TestRemoveSyncReadVersionMarker_EmptyNameIsNoop(t *testing.T) {
	tmp := t.TempDir()
	orig := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(orig) })

	keep := writeMarker(t, tmp, "dns", "1.2.235")

	removeSyncReadVersionMarker("   ")

	if _, err := os.Stat(keep); err != nil {
		t.Errorf("empty name must be a no-op and never touch the marker base, got err=%v", err)
	}
}
