package main

import (
	"os"
	"path/filepath"
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
