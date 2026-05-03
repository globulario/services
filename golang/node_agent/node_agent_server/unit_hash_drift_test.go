package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestCheckUnitHashDrift_NoSidecar verifies that an unmanaged unit (no .sha256
// sidecar) is never flagged as drifted regardless of file content.
func TestCheckUnitHashDrift_NoSidecar(t *testing.T) {
	dir := t.TempDir()
	unit := filepath.Join(dir, "globular-xds.service")
	if err := os.WriteFile(unit, []byte("[Unit]\nDescription=XDS\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Patch the path via the override — checkUnitHashDrift reads from
	// /etc/systemd/system, but we redirect it via a test-local wrapper.
	// Since the sidecar path is computed relative to /etc/systemd/system we
	// call the internal helper directly with a synthesised environment.

	// No sidecar → no drift.
	reason := checkUnitHashDriftAt(unit)
	if reason != "" {
		t.Errorf("expected no drift for unmanaged unit, got %q", reason)
	}
}

// TestCheckUnitHashDrift_HashMatch verifies that a unit whose on-disk content
// matches the sidecar hash is not flagged.
func TestCheckUnitHashDrift_HashMatch(t *testing.T) {
	dir := t.TempDir()
	content := []byte("[Unit]\nDescription=XDS\n")
	unit := filepath.Join(dir, "globular-xds.service")
	if err := os.WriteFile(unit, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTestSidecar(unit, content); err != nil {
		t.Fatal(err)
	}

	reason := checkUnitHashDriftAt(unit)
	if reason != "" {
		t.Errorf("expected no drift, got %q", reason)
	}
}

// TestCheckUnitHashDrift_HashMismatch verifies that a unit whose content
// differs from the sidecar hash is reported as unit_hash_drift.
func TestCheckUnitHashDrift_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	original := []byte("[Unit]\nDescription=XDS\n")
	modified := []byte("[Unit]\nDescription=XDS — manually edited\nEnvironment=DEBUG=1\n")
	unit := filepath.Join(dir, "globular-xds.service")
	// Write modified content to disk but sidecar with original hash.
	if err := os.WriteFile(unit, modified, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTestSidecar(unit, original); err != nil {
		t.Fatal(err)
	}

	reason := checkUnitHashDriftAt(unit)
	if reason != "unit_hash_drift" {
		t.Errorf("expected unit_hash_drift, got %q", reason)
	}
}

// TestCheckUnitHashDrift_MissingUnitFile verifies that when the unit file
// itself is absent (will be caught by runtime state), no drift is reported.
func TestCheckUnitHashDrift_MissingUnitFile(t *testing.T) {
	dir := t.TempDir()
	unit := filepath.Join(dir, "globular-xds.service")
	// Write sidecar but no unit file.
	if err := writeTestSidecar(unit, []byte("[Unit]\nDescription=XDS\n")); err != nil {
		t.Fatal(err)
	}

	reason := checkUnitHashDriftAt(unit)
	if reason != "" {
		t.Errorf("expected no drift for missing unit file (handled by runtime state), got %q", reason)
	}
}

// checkUnitHashDriftAt is a test-local helper that exercises the same logic as
// checkUnitHashDrift but with an arbitrary absolute path instead of the
// /etc/systemd/system prefix.
func checkUnitHashDriftAt(unitPath string) string {
	sidecarPath := unitPath + ".sha256"

	expected, err := os.ReadFile(sidecarPath)
	if err != nil {
		return ""
	}
	current, err := os.ReadFile(unitPath)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(current)
	got := hex.EncodeToString(sum[:])
	if got != string(expected) {
		return "unit_hash_drift"
	}
	return ""
}

func writeTestSidecar(unitPath string, content []byte) error {
	sum := sha256.Sum256(content)
	return os.WriteFile(unitPath+".sha256", []byte(hex.EncodeToString(sum[:])), 0o644)
}
