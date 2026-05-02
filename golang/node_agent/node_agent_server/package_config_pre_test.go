package main

// package_config_pre_test.go — Phase F-final tests for the pre-install
// config gate's classifier + the snapshot-aware post-install classifier.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// mkConfigFile writes a config-test file and returns its path + lowercase
// hex sha256. Distinct name avoids colliding with package's `writeFile`.
func mkConfigFile(t *testing.T, dir, name, content string) (string, string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return p, hex.EncodeToString(sum[:])
}

func TestSnapshotAware_PreservedWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	p, sum := mkConfigFile(t, dir, "echo.json", "default-config")

	resolved := &repositorypb.PackageConfigFile{
		Path:       p,
		ConfigKind: repositorypb.ConfigKind_CONFIG_DEFAULT,
		MergeStrategy: repositorypb.MergeStrategy_MERGE_PRESERVE,
	}
	snap := &configSnapshot{
		Resolved:          resolved,
		ChecksumBefore:    sum,
		ExistedPreInstall: true,
	}
	action, before, after := classifyOutcomeWithSnapshot(resolved, snap)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED {
		t.Fatalf("got %s, want PRESERVED", action)
	}
	if before == "" || after == "" || before != after {
		t.Errorf("before/after mismatch: before=%q after=%q", before, after)
	}
}

func TestSnapshotAware_ReplacedWhenContentChanged(t *testing.T) {
	dir := t.TempDir()
	p, beforeSum := mkConfigFile(t, dir, "echo.json", "default-config")
	// Simulate the install replacing the file.
	if err := os.WriteFile(p, []byte("brand-new-config"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved := &repositorypb.PackageConfigFile{
		Path:       p,
		ConfigKind: repositorypb.ConfigKind_CONFIG_DEFAULT,
		MergeStrategy: repositorypb.MergeStrategy_MERGE_REPLACE,
	}
	snap := &configSnapshot{
		Resolved:          resolved,
		ChecksumBefore:    beforeSum,
		ExistedPreInstall: true,
	}
	action, before, after := classifyOutcomeWithSnapshot(resolved, snap)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_REPLACED {
		t.Fatalf("got %s, want REPLACED", action)
	}
	if before == after {
		t.Error("before should differ from after when content changed")
	}
}

func TestSnapshotAware_GeneratedKeepsAfterChecksum(t *testing.T) {
	dir := t.TempDir()
	p, _ := mkConfigFile(t, dir, "g.conf", "rendered")
	resolved := &repositorypb.PackageConfigFile{
		Path:       p,
		ConfigKind: repositorypb.ConfigKind_CONFIG_GENERATED,
	}
	snap := &configSnapshot{
		Resolved: resolved,
		// No before content — caller may not have classified it.
	}
	action, _, after := classifyOutcomeWithSnapshot(resolved, snap)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_GENERATED {
		t.Fatalf("got %s, want GENERATED", action)
	}
	if after == "" {
		t.Error("GENERATED must report the post-render checksum")
	}
}

func TestSnapshotAware_SecretAlwaysSkipped(t *testing.T) {
	resolved := &repositorypb.PackageConfigFile{
		Path:       "/var/lib/globular/secret.key",
		ConfigKind: repositorypb.ConfigKind_CONFIG_SECRET,
	}
	snap := &configSnapshot{Resolved: resolved}
	action, before, after := classifyOutcomeWithSnapshot(resolved, snap)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_SKIPPED_SECRET {
		t.Fatalf("got %s, want SKIPPED_SECRET", action)
	}
	if before != "" || after != "" {
		t.Fatal("SECRET classifier must not surface checksums (no content read)")
	}
}

func TestSnapshotAware_NilSnapshotFallsBackToStatOnly(t *testing.T) {
	dir := t.TempDir()
	p, _ := mkConfigFile(t, dir, "x.conf", "hello")
	resolved := &repositorypb.PackageConfigFile{
		Path:       p,
		ConfigKind: repositorypb.ConfigKind_CONFIG_DEFAULT,
	}
	// Nil snapshot → fallback to classifyConfigOutcome (no checksum_at_install
	// → REPLACED with empty before).
	action, before, _ := classifyOutcomeWithSnapshot(resolved, nil)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_REPLACED {
		t.Fatalf("got %s, want REPLACED via fallback", action)
	}
	if before != "" {
		// classifyConfigOutcome with no manifest checksum returns "" before.
		t.Errorf("nil-snapshot fallback should report empty before, got %q", before)
	}
}

func TestSnapshotAware_FileMissingPostInstallIsFailed(t *testing.T) {
	resolved := &repositorypb.PackageConfigFile{
		Path:       "/path/that/does/not/exist.conf",
		ConfigKind: repositorypb.ConfigKind_CONFIG_DEFAULT,
	}
	snap := &configSnapshot{
		Resolved:       resolved,
		ChecksumBefore: "abc",
	}
	action, _, _ := classifyOutcomeWithSnapshot(resolved, snap)
	if action != repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_FAILED {
		t.Fatalf("missing post-install file must yield FAILED, got %s", action)
	}
}
