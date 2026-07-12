package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestHashUnitFile_MatchesRawFileSha verifies the upgrader hashes the on-disk
// unit exactly as installreceipt.Stamp / checkUnitHashDrift do: raw file bytes,
// lowercase hex sha256. If this diverges, node-agent self-upgrades produce a
// receipt that mismatches the doctor's verification → permanent unit_file_drift.
func TestHashUnitFile_MatchesRawFileSha(t *testing.T) {
	dir := t.TempDir()
	systemdUnitDir = dir
	t.Cleanup(func() { systemdUnitDir = "/etc/systemd/system" })

	unit := "globular-node-agent.service"
	content := []byte("[Unit]\nDescription=node agent\n[Service]\nExecStart=/usr/lib/globular/bin/node_agent_server\n")
	if err := os.WriteFile(filepath.Join(dir, unit), content, 0o644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])

	path, got := hashUnitFile(unit)
	if got != want {
		t.Fatalf("hashUnitFile sha = %q, want %q", got, want)
	}
	if path != filepath.Join(dir, unit) {
		t.Fatalf("hashUnitFile path = %q, want %q", path, filepath.Join(dir, unit))
	}
}

func TestHashUnitFile_MissingOrEmpty(t *testing.T) {
	dir := t.TempDir()
	systemdUnitDir = dir
	t.Cleanup(func() { systemdUnitDir = "/etc/systemd/system" })

	if p, s := hashUnitFile(""); p != "" || s != "" {
		t.Fatalf("empty unit name should yield empty path/sha, got %q/%q", p, s)
	}
	if p, s := hashUnitFile("does-not-exist.service"); p != "" || s != "" {
		t.Fatalf("missing unit file should yield empty path/sha, got %q/%q", p, s)
	}
}

// TestMergeReceiptMetadata_StampsUnitReceiptOnSuccess is the core regression:
// a successful self-upgrade MUST stamp unit_file_sha256/unit_file_path so the
// node-agent unit receipt matches the on-disk unit (no perpetual drift), while
// preserving proof metadata the original canonical install stamped.
func TestMergeReceiptMetadata_StampsUnitReceiptOnSuccess(t *testing.T) {
	dir := t.TempDir()
	systemdUnitDir = dir
	t.Cleanup(func() { systemdUnitDir = "/etc/systemd/system" })

	unit := "globular-node-agent.service"
	content := []byte("[Unit]\nDescription=node agent v2\n")
	if err := os.WriteFile(filepath.Join(dir, unit), content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	wantSha := hex.EncodeToString(sum[:])

	// Existing receipt from the original canonical install carries proof fields
	// AND a stale unit hash + a stale error from a prior failed attempt.
	existing := map[string]interface{}{
		"binary_sha256":       "deadbeef",
		"entrypoint_checksum": "cafef00d",
		unitFileSha256Key:     "STALE-OLD-HASH",
		"error":               "previous restart failed",
	}

	md := mergeReceiptMetadata(existing, unit, "installed", "")

	if md[unitFileSha256Key] != wantSha {
		t.Fatalf("unit_file_sha256 = %v, want fresh on-disk hash %q", md[unitFileSha256Key], wantSha)
	}
	if md[unitFilePathKey] != filepath.Join(dir, unit) {
		t.Fatalf("unit_file_path = %v, want %q", md[unitFilePathKey], filepath.Join(dir, unit))
	}
	// Proof fields preserved (read-merge, not clobber).
	if md["binary_sha256"] != "deadbeef" || md["entrypoint_checksum"] != "cafef00d" {
		t.Fatalf("proof metadata not preserved: %+v", md)
	}
	// Stale error cleared on success.
	if _, ok := md["error"]; ok {
		t.Fatalf("stale error should be cleared on success: %+v", md)
	}
}

// TestMergeReceiptMetadata_FailurePreservesReceipt confirms a failed restart
// records the error WITHOUT invalidating the existing unit receipt.
func TestMergeReceiptMetadata_FailurePreservesReceipt(t *testing.T) {
	existing := map[string]interface{}{
		unitFileSha256Key: "good-hash",
		"binary_sha256":   "deadbeef",
	}
	md := mergeReceiptMetadata(existing, "globular-node-agent.service", "failed", "restart failed: exit 1")

	if md[unitFileSha256Key] != "good-hash" {
		t.Fatalf("failure must not overwrite unit receipt, got %v", md[unitFileSha256Key])
	}
	if md["error"] != "restart failed: exit 1" {
		t.Fatalf("failure error not recorded: %+v", md)
	}
}

// TestMergeReceiptMetadata_NilStart ensures a first-ever install (no prior
// record) still produces a complete unit receipt rather than nil-panicking.
func TestMergeReceiptMetadata_NilStart(t *testing.T) {
	dir := t.TempDir()
	systemdUnitDir = dir
	t.Cleanup(func() { systemdUnitDir = "/etc/systemd/system" })

	unit := "globular-node-agent.service"
	if err := os.WriteFile(filepath.Join(dir, unit), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	md := mergeReceiptMetadata(nil, unit, "installed", "")
	if md[unitFileSha256Key] == nil {
		t.Fatalf("nil-start success must still stamp unit receipt: %+v", md)
	}
}
