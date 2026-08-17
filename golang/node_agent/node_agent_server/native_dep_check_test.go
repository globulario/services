package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDepMissingNoRegistry(t *testing.T) {
	// Packages with no entry in packageNativeDeps always pass.
	if got := nativeDepMissing("cluster-controller"); got != "" {
		t.Errorf("cluster-controller has no native deps, got %q", got)
	}
	if got := nativeDepMissing("rbac"); got != "" {
		t.Errorf("rbac has no native deps, got %q", got)
	}
}

func TestNativeDepMissingLibPresent(t *testing.T) {
	dir := t.TempDir()
	orig := nativeLibScanDirs
	t.Cleanup(func() { nativeLibScanDirs = orig })
	nativeLibScanDirs = []string{dir}

	// Create the library file.
	if err := os.WriteFile(filepath.Join(dir, "libtest.so.1"), []byte("mock"), 0644); err != nil {
		t.Fatal(err)
	}

	// Register a package that needs this lib.
	origDeps := packageNativeDeps
	t.Cleanup(func() { packageNativeDeps = origDeps })
	packageNativeDeps = map[string][]string{
		"test-service": {"libtest.so.1"},
	}

	if got := nativeDepMissing("test-service"); got != "" {
		t.Errorf("lib is present, expected no missing dep, got %q", got)
	}
}

func TestNativeDepMissingLibAbsent(t *testing.T) {
	dir := t.TempDir()
	orig := nativeLibScanDirs
	t.Cleanup(func() { nativeLibScanDirs = orig })
	nativeLibScanDirs = []string{dir}

	origDeps := packageNativeDeps
	t.Cleanup(func() { packageNativeDeps = origDeps })
	packageNativeDeps = map[string][]string{
		"test-service": {"libmissing.so.99"},
	}

	if got := nativeDepMissing("test-service"); got != "libmissing.so.99" {
		t.Errorf("expected missing lib %q, got %q", "libmissing.so.99", got)
	}
}

func TestNativeDepMissingFirstMissingReturned(t *testing.T) {
	dir := t.TempDir()
	orig := nativeLibScanDirs
	t.Cleanup(func() { nativeLibScanDirs = orig })
	nativeLibScanDirs = []string{dir}

	// Only the first lib is present; second is absent.
	if err := os.WriteFile(filepath.Join(dir, "libfirst.so.1"), []byte("mock"), 0644); err != nil {
		t.Fatal(err)
	}

	origDeps := packageNativeDeps
	t.Cleanup(func() { packageNativeDeps = origDeps })
	packageNativeDeps = map[string][]string{
		"multi-dep-svc": {"libfirst.so.1", "libsecond.so.2"},
	}

	if got := nativeDepMissing("multi-dep-svc"); got != "libsecond.so.2" {
		t.Errorf("expected first missing lib %q, got %q", "libsecond.so.2", got)
	}
}

func TestNativeLibPresentVersionedVariant(t *testing.T) {
	dir := t.TempDir()
	orig := nativeLibScanDirs
	t.Cleanup(func() { nativeLibScanDirs = orig })
	nativeLibScanDirs = []string{dir}

	// Create a versioned variant (libfoo.so.2.0.0) while checking for soname (libfoo.so.2).
	if err := os.WriteFile(filepath.Join(dir, "libfoo.so.2.0.0"), []byte("mock"), 0644); err != nil {
		t.Fatal(err)
	}

	if !nativeLibPresent("libfoo.so.2") {
		t.Error("versioned variant libfoo.so.2.0.0 should satisfy soname libfoo.so.2 check")
	}
}

// TestNativeDepMetadataODBC pins the provider to the package that actually
// ships the library file.
//
// This assertion previously read "debian:unixodbc", which is wrong: on
// Ubuntu 24.04, `dpkg -c unixodbc_2.3.12-1ubuntu0.24.04.1_amd64.deb` contains
// only the odbcinst/isql CLI tools, while libodbc.so.2 (→ libodbc.so.2.0.0)
// ships in libodbc2. The manual `apt install unixodbc` hint happened to work
// because apt pulls libodbc2 in transitively, which masked the error.
//
// It stopped being cosmetic when specgen's native_debs() started deriving
// bundle_debs from this same fact: bundling unixodbc alone would ship the
// tools without the library, `dpkg -i` would fail on the unmet dependency,
// and the ldd preflight in install_package_payload would still abort the sql
// install — the exact failure this bundling exists to prevent.
func TestNativeDepMetadataODBC(t *testing.T) {
	if got := nativeDepProvider("libodbc.so.2"); got != "debian:libodbc2" {
		t.Fatalf("provider=%q, want debian:libodbc2 (libodbc2 ships libodbc.so.2; unixodbc ships only the CLI tools)", got)
	}
	if got := nativeDepManualAction("libodbc.so.2"); got != "sudo apt-get install -y libodbc2" {
		t.Fatalf("manual_action=%q, want sudo apt-get install -y libodbc2", got)
	}
}

// TestSpecgenBundlesNativeDebsForSQL pins the build-side half of the contract:
// the generated sql spec must bundle the runtime library deb AND install it
// before the payload step. install_package_payload runs an ldd preflight on
// the extracted binary and returns NATIVE_LIBRARY_DEPENDENCY_MISSING when a
// SONAME is unresolvable, which failed install_workloads_compute and therefore
// the whole node.join workflow — observed on every compute-profile join before
// this fix, leaving globular-sql.service on disk with no installed_state
// receipt (cluster-doctor unit_receipt_drift, CRITICAL).
func TestSpecgenBundlesNativeDebsForSQL(t *testing.T) {
	src, err := os.ReadFile("../../../generated/specs/sql_service.yaml")
	if err != nil {
		t.Skipf("generated sql spec not present: %v", err)
	}
	spec := string(src)

	provider := strings.TrimPrefix(nativeDepProvider("libodbc.so.2"), "debian:")
	if !strings.Contains(spec, "bundle_debs:") || !strings.Contains(spec, "- "+provider) {
		t.Fatalf("sql spec must bundle %q via bundle_debs so install stays offline", provider)
	}

	debs := strings.Index(spec, "type: install_local_debs")
	payload := strings.Index(spec, "type: install_package_payload")
	if debs < 0 {
		t.Fatal("sql spec must install its bundled native debs (install_local_debs step missing)")
	}
	if payload >= 0 && debs > payload {
		t.Fatal("install_local_debs must precede install_package_payload: the payload step's ldd preflight aborts the install when libodbc.so.2 is still missing")
	}
}
