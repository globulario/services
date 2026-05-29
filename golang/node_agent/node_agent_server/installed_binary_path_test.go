package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

// Project T (INC-2026-0020) regression tests.
//
// Pre-fix, installedBinaryPath inferred the binary path from the package
// name with strings.ReplaceAll(name, "-", "_") for SERVICE and the raw
// name for INFRASTRUCTURE/COMMAND. Both diverge from the actual binary
// when the package name uses hyphens but the binary uses underscores
// (e.g. scylla-manager → scylla_manager). The verifier looked at the
// wrong path, reported the binary missing, and the controller's drift
// reconciler dispatched a reinstall loop until an operator added a
// hand-written symlink.

func withInstalledBinaryPathSetup(t *testing.T) (binDir string, restoreFn func()) {
	t.Helper()
	bin := t.TempDir()
	state := t.TempDir()
	prevBin := globularBinDir
	globularBinDir = bin
	versionutil.SetBaseDir(state)
	return bin, func() { globularBinDir = prevBin }
}

// 1. The literal bug repro: a hyphen-named INFRASTRUCTURE package with an
// underscore entrypoint must resolve to the underscore path.
func TestInstalledBinaryPath_InfraHyphenName_UnderscoreEntrypoint_UsesSidecar(t *testing.T) {
	bin, restore := withInstalledBinaryPathSetup(t)
	defer restore()

	// Install-time persistence:
	if err := versionutil.WriteEntrypoint("scylla-manager", "bin/scylla_manager"); err != nil {
		t.Fatal(err)
	}

	got := installedBinaryPath("scylla-manager", "INFRASTRUCTURE")
	want := filepath.Join(bin, "scylla_manager")
	if got != want {
		t.Errorf("INFRASTRUCTURE hyphen-name should resolve via entrypoint sidecar\n got=%q\nwant=%q", got, want)
	}
	// Critically — must NOT return the pre-fix inferred hyphen path.
	if got == filepath.Join(bin, "scylla-manager") {
		t.Error("Project T fix did not take effect; pre-fix hyphen path returned")
	}
}

// 2. Same shape for a SERVICE package that mixes hyphen + underscore.
func TestInstalledBinaryPath_ServiceHyphenName_UnderscoreEntrypoint_UsesSidecar(t *testing.T) {
	bin, restore := withInstalledBinaryPathSetup(t)
	defer restore()

	if err := versionutil.WriteEntrypoint("backup-manager", "bin/backup_manager_server"); err != nil {
		t.Fatal(err)
	}

	got := installedBinaryPath("backup-manager", "SERVICE")
	want := filepath.Join(bin, "backup_manager_server")
	if got != want {
		t.Errorf("SERVICE entrypoint sidecar should override _server inference\n got=%q\nwant=%q", got, want)
	}
}

// 3. Legacy fallback: package with NO entrypoint sidecar (pre-fix install)
// must still resolve via the original inferred logic so existing
// installations don't regress.
func TestInstalledBinaryPath_NoSidecar_FallsBackToLegacyInfer(t *testing.T) {
	bin, restore := withInstalledBinaryPathSetup(t)
	defer restore()

	// No sidecar written.
	// SERVICE without sidecar: legacy logic probes <bin>/<name>_server first.
	// Create the file so the stat-then-return branch executes.
	if err := os.WriteFile(filepath.Join(bin, "echo_server"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := installedBinaryPath("echo", "SERVICE")
	want := filepath.Join(bin, "echo_server")
	if got != want {
		t.Errorf("legacy SERVICE fallback should still find _server\n got=%q\nwant=%q", got, want)
	}
}

// 4. Legacy fallback for INFRASTRUCTURE: no sidecar, no hyphen issues —
// existing INFRA packages must still resolve to the raw name path.
func TestInstalledBinaryPath_NoSidecar_LegacyInfraUsesRawName(t *testing.T) {
	bin, restore := withInstalledBinaryPathSetup(t)
	defer restore()

	got := installedBinaryPath("etcdctl", "INFRASTRUCTURE")
	want := filepath.Join(bin, "etcdctl")
	if got != want {
		t.Errorf("legacy INFRASTRUCTURE fallback should use raw name\n got=%q\nwant=%q", got, want)
	}
}

// 5. Sidecar takes precedence even when the inferred path would have
// found a binary (defensive — the manifest must win).
func TestInstalledBinaryPath_SidecarOverridesInferredHit(t *testing.T) {
	bin, restore := withInstalledBinaryPathSetup(t)
	defer restore()

	// Pre-create the legacy inferred path so the fallback would succeed
	// if reached. This proves the sidecar branch wins unconditionally.
	if err := os.WriteFile(filepath.Join(bin, "service_legacy_server"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := versionutil.WriteEntrypoint("service-legacy", "bin/service_actual"); err != nil {
		t.Fatal(err)
	}

	got := installedBinaryPath("service-legacy", "SERVICE")
	want := filepath.Join(bin, "service_actual")
	if got != want {
		t.Errorf("entrypoint sidecar must take precedence over inferred path that exists\n got=%q\nwant=%q", got, want)
	}
}
