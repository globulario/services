package main

import (
	"os"
	"path/filepath"
)

// packageNativeDeps maps installed-state-name to the native shared-library
// SONAMEs the package binary requires at runtime (e.g. "libodbc.so.2").
// Core Globular packages are pure Go binaries with no C deps; add entries
// here when a service links against a native library.
var packageNativeDeps = map[string][]string{
	// Example for a future ODBC-dependent service:
	// "my-odbc-service": {"libodbc.so.2"},
}

// nativeLibScanDirs lists the standard library directories on Debian/Ubuntu
// x86_64 and ARM64 hosts. Overridable by tests.
var nativeLibScanDirs = []string{
	"/lib/x86_64-linux-gnu",
	"/usr/lib/x86_64-linux-gnu",
	"/lib/aarch64-linux-gnu",
	"/usr/lib/aarch64-linux-gnu",
	"/lib",
	"/usr/lib",
	"/usr/local/lib",
}

// nativeDepMissing returns the first native library SONAME required by
// pkgName that cannot be found in nativeLibScanDirs. Returns "" if all
// required libraries are present or if the package has no native deps.
func nativeDepMissing(pkgName string) string {
	for _, lib := range packageNativeDeps[pkgName] {
		if !nativeLibPresent(lib) {
			return lib
		}
	}
	return ""
}

// nativeLibPresent checks whether a shared library SONAME (e.g. "libodbc.so.2")
// exists in any of the nativeLibScanDirs using a prefix-glob so that versioned
// variants like "libodbc.so.2.0.0" also match.
func nativeLibPresent(soname string) bool {
	for _, dir := range nativeLibScanDirs {
		matches, err := filepath.Glob(filepath.Join(dir, soname+"*"))
		if err == nil && len(matches) > 0 {
			return true
		}
		// Also check the exact name (symlinks like libfoo.so.2 → libfoo.so.2.0.0).
		if _, err := os.Stat(filepath.Join(dir, soname)); err == nil {
			return true
		}
	}
	return false
}
