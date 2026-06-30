package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Test fixture helpers ─────────────────────────────────────────────────────

func writeTmpFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writeTmpFile %s: %v", path, err)
	}
	return path
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ── StampInstallReceipt ───────────────────────────────────────────────────

// TestStampInstallReceipt_FullReceipt — happy path: every path supplied,
// all four hashes computed, helper keys set.
func TestStampInstallReceipt_FullReceipt(t *testing.T) {
	dir := t.TempDir()
	unitData := []byte("[Unit]\nDescription=Test\n[Service]\nExecStart=/bin/true\n")
	binData := []byte("\x7fELF...stub binary")
	cfgData := []byte("key = value\n")
	envData := []byte("FOO=bar\n")
	unitPath := writeTmpFile(t, dir, "globular-foo.service", unitData)
	binPath := writeTmpFile(t, dir, "foo_server", binData)
	cfgPath := writeTmpFile(t, dir, "foo.yaml", cfgData)
	envPath := writeTmpFile(t, dir, "foo.env", envData)

	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	err := StampInstallReceipt(pkg, ReceiptOpts{
		UnitFilePath:        unitPath,
		BinaryPath:          binPath,
		ConfigPath:          cfgPath,
		EnvFilePath:         envPath,
		PackageSha256:       "sha256:CAFEBABE",
		ArtifactDigest:      "sha256:DEADBEEF",
		UnitRendererVersion: "v1",
		InstalledBy:         "test-suite",
	})
	if err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}

	checks := map[string]string{
		receiptKeyUnitFilePath:        unitPath,
		receiptKeyUnitFileSha256:      sha256Hex(unitData),
		receiptKeyBinaryPath:          binPath,
		receiptKeyBinarySha256:        sha256Hex(binData),
		receiptKeyConfigPath:          cfgPath,
		receiptKeyConfigSha256:        sha256Hex(cfgData),
		receiptKeyEnvFilePath:         envPath,
		receiptKeyEnvFileSha256:       sha256Hex(envData),
		receiptKeyPackageSha256:       "cafebabe", // sha256: prefix stripped, lowercased
		receiptKeyArtifactDigest:      "deadbeef",
		receiptKeyUnitRendererVersion: "v1",
		receiptKeyInstalledBy:         "test-suite",
	}
	for k, want := range checks {
		if got := pkg.Metadata[k]; got != want {
			t.Errorf("metadata[%q] = %q, want %q", k, got, want)
		}
	}
	if pkg.Metadata[receiptKeyInstalledAt] == "" {
		t.Error("installed_at missing")
	}
}

// TestStampInstallReceipt_OmittedFieldsNotWritten — empty paths in opts
// must NOT produce empty-string metadata entries. The key must be absent.
func TestStampInstallReceipt_OmittedFieldsNotWritten(t *testing.T) {
	dir := t.TempDir()
	unitData := []byte("[Unit]\n")
	unitPath := writeTmpFile(t, dir, "unit", unitData)

	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	if err := StampInstallReceipt(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}
	// unit fields set; binary/config/env/package/artifact must NOT be present.
	if _, ok := pkg.Metadata[receiptKeyBinaryPath]; ok {
		t.Error("binary_path should be absent when BinaryPath empty")
	}
	if _, ok := pkg.Metadata[receiptKeyConfigSha256]; ok {
		t.Error("config_sha256 should be absent when ConfigPath empty")
	}
	if _, ok := pkg.Metadata[receiptKeyEnvFileSha256]; ok {
		t.Error("env_file_sha256 should be absent when EnvFilePath empty")
	}
	if _, ok := pkg.Metadata[receiptKeyPackageSha256]; ok {
		t.Error("package_sha256 should be absent when PackageSha256 empty")
	}
}

// TestStampInstallReceipt_MissingFileIsAtomicFailure — a non-empty path
// pointing at a missing file MUST fail the entire receipt. Partial
// receipts would mislead the heartbeat.
func TestStampInstallReceipt_MissingFileIsAtomicFailure(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))

	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	err := StampInstallReceipt(pkg, ReceiptOpts{
		UnitFilePath: unitPath,
		BinaryPath:   filepath.Join(dir, "does-not-exist"),
	})
	if err == nil {
		t.Fatal("expected error when BinaryPath is missing")
	}
	// Critically: no fields should have been stamped despite the unit
	// file existing. Partial receipts are forbidden.
	if pkg.Metadata != nil && pkg.Metadata[receiptKeyUnitFileSha256] != "" {
		t.Error("unit_file_sha256 leaked despite atomicity contract")
	}
}

// TestStampInstallReceipt_PreservesUnrelatedMetadata — receipt-stamping
// must NOT clobber metadata keys outside the receipt namespace
// (entrypoint_checksum, proof_on_disk_sha256, etc).
func TestStampInstallReceipt_PreservesUnrelatedMetadata(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))

	pkg := &node_agentpb.InstalledPackage{
		Name: "foo", Kind: "SERVICE",
		Metadata: map[string]string{
			"entrypoint_checksum":  "abc",
			"proof_on_disk_sha256": "def",
			"random_key":           "ghi",
		},
	}
	if err := StampInstallReceipt(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}
	for _, k := range []string{"entrypoint_checksum", "proof_on_disk_sha256", "random_key"} {
		if pkg.Metadata[k] == "" {
			t.Errorf("unrelated key %q was cleared", k)
		}
	}
}

// TestStampInstallReceipt_SupersedesMigrationMarker — re-stamping over a
// receipt that was previously seeded from a legacy sidecar must remove
// the migration_source marker; the install action has now produced a
// first-hand receipt and the forensic marker would become misleading.
func TestStampInstallReceipt_SupersedesMigrationMarker(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))

	pkg := &node_agentpb.InstalledPackage{
		Name: "foo", Kind: "SERVICE",
		Metadata: map[string]string{
			receiptKeyMigrationSource: "legacy_sidecar",
			receiptKeyUnitFileSha256:  "previous-seeded-value",
			receiptKeyUnitFilePath:    "/old/path",
		},
	}
	if err := StampInstallReceipt(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}
	if _, ok := pkg.Metadata[receiptKeyMigrationSource]; ok {
		t.Error("migration_source marker should be cleared by a fresh receipt")
	}
	if got := pkg.Metadata[receiptKeyUnitFileSha256]; got == "previous-seeded-value" {
		t.Error("unit_file_sha256 should be replaced by fresh computation")
	}
	if got := pkg.Metadata[receiptKeyUnitFilePath]; got != unitPath {
		t.Errorf("unit_file_path = %q, want %q", got, unitPath)
	}
}

// TestStampInstallReceipt_UnitFileContentWinsOverDisk proves the canonical
// renderer bytes, when supplied, are the receipt authority even if the on-disk
// unit file at UnitFilePath contains different bytes.
func TestStampInstallReceipt_UnitFileContentWinsOverDisk(t *testing.T) {
	dir := t.TempDir()
	unitPath := filepath.Join(dir, "globular-svc.service")
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=disk-bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	canonical := []byte("[Unit]\nDescription=canonical-bytes\n")
	pkg := &node_agentpb.InstalledPackage{Name: "svc"}

	if err := StampInstallReceipt(pkg, ReceiptOpts{
		UnitFilePath:        unitPath,
		UnitFileContent:     canonical,
		UnitRendererVersion: "artifact-canonical-v1",
	}); err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}

	want := sha256Hex(canonical)
	if got := pkg.Metadata[receiptKeyUnitFileSha256]; got != want {
		t.Fatalf("unit_file_sha256 = %q, want canonical-bytes hash %q", got, want)
	}
}

// TestStampInstallReceipt_NilPkgIsError — defensive guard.
func TestStampInstallReceipt_NilPkgIsError(t *testing.T) {
	if err := StampInstallReceipt(nil, ReceiptOpts{}); err == nil {
		t.Fatal("expected error for nil pkg")
	}
}

// TestStampInstallReceipt_DefaultInstalledBy — empty InstalledBy resolves
// to "node-agent" (the canonical caller); explicit value is preserved.
func TestStampInstallReceipt_DefaultInstalledBy(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))

	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	if err := StampInstallReceipt(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}
	if got := pkg.Metadata[receiptKeyInstalledBy]; got != "node-agent" {
		t.Errorf("installed_by default = %q, want %q", got, "node-agent")
	}
}

// TestStampInstallReceipt_Sha256PrefixNormalization — package_sha256 and
// artifact_digest are normalized: "sha256:" prefix stripped, lowercased.
// Eliminates downstream comparison-bug surface.
func TestStampInstallReceipt_Sha256PrefixNormalization(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))

	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	if err := StampInstallReceipt(pkg, ReceiptOpts{
		UnitFilePath:   unitPath,
		PackageSha256:  "SHA256:ABCDEF",
		ArtifactDigest: "  sha256:ABC123  ",
	}); err != nil {
		t.Fatalf("StampInstallReceipt: %v", err)
	}
	if got := pkg.Metadata[receiptKeyPackageSha256]; got != "abcdef" {
		t.Errorf("package_sha256 = %q, want %q", got, "abcdef")
	}
	if got := pkg.Metadata[receiptKeyArtifactDigest]; got != "abc123" {
		t.Errorf("artifact_digest = %q, want %q", got, "abc123")
	}
}

// ── Accessor helpers ──────────────────────────────────────────────────────

func TestReceiptUnitFileSha256_Nil(t *testing.T) {
	if got := receiptUnitFileSha256(nil); got != "" {
		t.Errorf("nil pkg → expected empty, got %q", got)
	}
}

func TestReceiptUnitFileSha256_NoMetadata(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{Name: "x"}
	if got := receiptUnitFileSha256(pkg); got != "" {
		t.Errorf("nil metadata → expected empty, got %q", got)
	}
}

func TestReceiptUnitFileSha256_Present(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{receiptKeyUnitFileSha256: "abc"},
	}
	if got := receiptUnitFileSha256(pkg); got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestReceiptUnitFilePath_Present(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{receiptKeyUnitFilePath: "/etc/systemd/system/x.service"},
	}
	if got := receiptUnitFilePath(pkg); got != "/etc/systemd/system/x.service" {
		t.Errorf("got %q, unexpected", got)
	}
}

// ── stampMigrationFromLegacySidecar ───────────────────────────────────────

func TestStampMigrationFromLegacySidecar_FreshPkg(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{Name: "foo"}
	stampMigrationFromLegacySidecar(pkg, "/etc/systemd/system/foo.service", "ABCDEF")

	if got := pkg.Metadata[receiptKeyUnitFilePath]; got != "/etc/systemd/system/foo.service" {
		t.Errorf("unit_file_path = %q", got)
	}
	if got := pkg.Metadata[receiptKeyUnitFileSha256]; got != "abcdef" {
		t.Errorf("unit_file_sha256 = %q (expected lowercased)", got)
	}
	if got := pkg.Metadata[receiptKeyMigrationSource]; got != "legacy_sidecar" {
		t.Errorf("migration_source = %q, want %q", got, "legacy_sidecar")
	}
	if pkg.Metadata[receiptKeyInstalledAt] == "" {
		t.Error("installed_at missing")
	}
}

func TestStampMigrationFromLegacySidecar_NilPkgIsSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on nil pkg: %v", r)
		}
	}()
	stampMigrationFromLegacySidecar(nil, "/etc/foo", "abc")
}
