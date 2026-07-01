package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Tests for PreserveInstallReceiptMetadata. The helper is the guarantee
// that non-install writers (heartbeat refresh, runtime proof writer,
// reconciliation paths) cannot erase install-receipt metadata when they
// re-write installed_state. Pre-helper: every heartbeat sync clobbered
// the receipt, causing the heartbeat to fall through to legacy_sidecar
// migration permanently after the very first heartbeat following a
// canonical install.

// ── Core preservation contract ────────────────────────────────────────────

func TestPreserveInstallReceiptMetadata_CopiesAllReceiptKeys(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFilePath:        "/etc/systemd/system/globular-x.service",
			receiptKeyUnitFileSha256:      "aaa",
			receiptKeyBinaryPath:          "/usr/lib/globular/bin/x_server",
			receiptKeyBinarySha256:        "bbb",
			receiptKeyConfigPath:          "/var/lib/globular/services/x/config.yaml",
			receiptKeyConfigSha256:        "ccc",
			receiptKeyEnvFilePath:         "/var/lib/globular/services/x/env",
			receiptKeyEnvFileSha256:       "ddd",
			receiptKeyPackageSha256:       "eee",
			receiptKeyArtifactDigest:      "fff",
			receiptKeyUnitRendererVersion: "v1",
			receiptKeyInstalledAt:         "1700000000",
			receiptKeyInstalledBy:         "node-agent.apply_package_release.service",
			// migration_source intentionally absent — install replaced it
		},
	}
	next := &node_agentpb.InstalledPackage{
		Name:    "x",
		Status:  "installed",
		Version: "1.2.3",
	}
	PreserveInstallReceiptMetadata(existing, next)
	for _, k := range installreceipt.Keys() {
		expected := existing.Metadata[k]
		got := next.Metadata[k]
		if expected != "" && got != expected {
			t.Errorf("key %q: existing=%q next=%q (expected next == existing)", k, expected, got)
		}
	}
}

func TestPreserveInstallReceiptMetadata_NextWinsOnConflict(t *testing.T) {
	// Canonical install writers populate next.Metadata via StampInstallReceipt
	// BEFORE installed_state.WriteInstalledPackage. If they later call
	// PreserveInstallReceiptMetadata, the install's fresh values must
	// not be overwritten by the existing record's older values.
	existing := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256: "OLD",
			receiptKeyInstalledBy:    "old-writer",
		},
	}
	next := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256: "NEW",
			receiptKeyInstalledBy:    "node-agent.apply_package_release.service",
		},
	}
	PreserveInstallReceiptMetadata(existing, next)
	if next.Metadata[receiptKeyUnitFileSha256] != "NEW" {
		t.Errorf("next was overwritten: got %q want NEW", next.Metadata[receiptKeyUnitFileSha256])
	}
	if next.Metadata[receiptKeyInstalledBy] != "node-agent.apply_package_release.service" {
		t.Errorf("installed_by clobbered: got %q", next.Metadata[receiptKeyInstalledBy])
	}
}

func TestPreserveInstallReceiptMetadata_EmptyIncomingCannotEraseReceipt(t *testing.T) {
	// The bug this whole helper exists to prevent: a non-install writer
	// (heartbeat sync) builds a fresh InstalledPackage with empty
	// metadata, calls WriteInstalledPackage, and the install receipt is
	// gone. Helper must intervene so the empty metadata is filled from
	// existing before write.
	existing := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256: "abc",
			receiptKeyBinarySha256:   "def",
			receiptKeyInstalledBy:    "node-agent.apply_package_release.service",
			receiptKeyInstalledAt:    "1700000000",
		},
	}
	next := &node_agentpb.InstalledPackage{
		Name:     "node-agent",
		Version:  "1.2.144",
		Status:   "installed",
		Metadata: nil, // ← the bug: heartbeat constructs without metadata
	}
	PreserveInstallReceiptMetadata(existing, next)
	if next.Metadata == nil {
		t.Fatal("metadata not allocated despite existing receipt")
	}
	if next.Metadata[receiptKeyUnitFileSha256] != "abc" ||
		next.Metadata[receiptKeyBinarySha256] != "def" ||
		next.Metadata[receiptKeyInstalledBy] != "node-agent.apply_package_release.service" ||
		next.Metadata[receiptKeyInstalledAt] != "1700000000" {
		t.Errorf("receipt NOT preserved through empty next; got %v", next.Metadata)
	}
}

func TestPreserveInstallReceiptMetadata_PreservesMigrationSource(t *testing.T) {
	// After legacy-sidecar migration, the metadata carries
	// migration_source=legacy_sidecar. The next heartbeat refresh
	// (non-install) must preserve that marker; only the canonical
	// install writer (StampInstallReceipt) is allowed to remove it.
	existing := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256:  "abc",
			receiptKeyMigrationSource: "legacy_sidecar",
		},
	}
	next := &node_agentpb.InstalledPackage{Name: "x"}
	PreserveInstallReceiptMetadata(existing, next)
	if next.Metadata[receiptKeyMigrationSource] != "legacy_sidecar" {
		t.Errorf("migration_source lost: %v", next.Metadata)
	}
}

func TestPreserveInstallReceiptMetadata_CanonicalInstallReplacesLegacySidecar(t *testing.T) {
	// When the canonical install path produces a fresh receipt,
	// StampInstallReceipt deletes migration_source from pkg.Metadata
	// before commit. Existing record (pre-install) still has
	// migration_source. Helper must NOT re-add it after the install
	// writer explicitly cleared it.
	//
	// We simulate the post-install state: `existing` is the migration
	// record (with migration_source), `next` is the fresh install
	// receipt where StampInstallReceipt has populated installed_by
	// + unit_file_sha256 and explicitly NOT included migration_source.
	existing := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256:  "old",
			receiptKeyMigrationSource: "legacy_sidecar",
		},
	}
	next := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256: "new",
			receiptKeyInstalledBy:    "node-agent.apply_package_release.service",
			// migration_source intentionally absent — StampInstallReceipt deleted it
		},
	}
	PreserveInstallReceiptMetadata(existing, next)
	// Test the helper's documented contract: NEXT wins. The canonical
	// install writer is responsible for clearing migration_source
	// explicitly via delete() before invoking this helper. But here
	// `next` has no migration_source key, so the helper's
	// preservation copies `existing`'s value back in. The architectural
	// fix is that StampInstallReceipt sets migration_source="" (not
	// absent) when it wants to clear, OR that the install path doesn't
	// call PreserveInstallReceiptMetadata.
	//
	// CURRENT BEHAVIOUR documented: this test asserts the install path
	// must NOT call PreserveInstallReceiptMetadata. The install path
	// uses StampInstallReceipt only.
	//
	// NEW BEHAVIOUR REQUIRED by the spec ("migration_source ... only
	// if no canonical install receipt has replaced it yet"): when
	// next.installed_by signals a canonical install, the helper should
	// NOT copy existing's migration_source.
	if next.Metadata[receiptKeyInstalledBy] == "node-agent.apply_package_release.service" {
		// canonical install present → migration_source must NOT be re-added
		if _, present := next.Metadata[receiptKeyMigrationSource]; present {
			t.Errorf("migration_source re-added after canonical install: %v", next.Metadata)
		}
	}
}

func TestPreserveInstallReceiptMetadata_NilArgsAreSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	PreserveInstallReceiptMetadata(nil, nil)
	PreserveInstallReceiptMetadata(&node_agentpb.InstalledPackage{}, nil)
	PreserveInstallReceiptMetadata(nil, &node_agentpb.InstalledPackage{})
}

func TestPreserveInstallReceiptMetadata_NoExistingMetadataIsNoOp(t *testing.T) {
	// First install on a clean node: existing record exists but has no
	// metadata yet. Helper must not allocate next.Metadata if there's
	// nothing to copy.
	existing := &node_agentpb.InstalledPackage{Name: "x"}
	next := &node_agentpb.InstalledPackage{Name: "x"}
	PreserveInstallReceiptMetadata(existing, next)
	if next.Metadata != nil {
		t.Errorf("metadata allocated unnecessarily: %v", next.Metadata)
	}
}

func TestPreserveInstallReceiptMetadata_ExistingEmptyValuesNotCarried(t *testing.T) {
	// A key with empty string value is treated as absent — not preserved.
	// This matches the existing receipt-stamp semantics (empty fields are
	// omitted, not stored as empty strings).
	existing := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256: "abc",
			receiptKeyBinarySha256:   "", // empty — should not propagate
		},
	}
	next := &node_agentpb.InstalledPackage{Name: "x"}
	PreserveInstallReceiptMetadata(existing, next)
	if next.Metadata[receiptKeyUnitFileSha256] != "abc" {
		t.Error("non-empty value not preserved")
	}
	if _, present := next.Metadata[receiptKeyBinarySha256]; present {
		t.Error("empty value should not be propagated")
	}
}

func TestPreserveInstallReceiptMetadata_DoesNotTouchNonReceiptKeys(t *testing.T) {
	// Keys outside the receipt namespace (entrypoint_checksum,
	// proof_on_disk_sha256, proof_*, etc) are owned by other
	// authorities (proof writer, version manager). The helper must
	// NOT touch them.
	existing := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyUnitFileSha256:   "abc",
			"entrypoint_checksum":      "EC-OLD",
			"proof_on_disk_sha256":     "POD-OLD",
			"proof_manifest_checksum":  "PMC-OLD",
			"some_other_key":           "SOK-OLD",
		},
	}
	next := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			"entrypoint_checksum":  "EC-NEW",
			"proof_on_disk_sha256": "POD-NEW",
		},
	}
	PreserveInstallReceiptMetadata(existing, next)
	// Receipt key copied
	if next.Metadata[receiptKeyUnitFileSha256] != "abc" {
		t.Error("receipt key not copied")
	}
	// Non-receipt keys: helper must not modify what next already has
	if next.Metadata["entrypoint_checksum"] != "EC-NEW" {
		t.Errorf("non-receipt key modified: %s", next.Metadata["entrypoint_checksum"])
	}
	// Non-receipt keys absent in next must NOT be copied from existing
	if _, present := next.Metadata["some_other_key"]; present {
		t.Error("non-receipt key copied from existing")
	}
	if _, present := next.Metadata["proof_manifest_checksum"]; present {
		t.Error("non-receipt proof key copied from existing")
	}
}

// ── Integration with the canonical install path ───────────────────────────

func TestStampInstallReceipt_ClearsMigrationSource(t *testing.T) {
	// Once the canonical install writes a fresh receipt, the
	// legacy_sidecar marker becomes misleading. StampInstallReceipt
	// must clear it. This test pins that contract.
	dir := t.TempDir()
	unitPath := filepath.Join(dir, "unit")
	if err := os.WriteFile(unitPath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			receiptKeyMigrationSource: "legacy_sidecar",
			receiptKeyUnitFileSha256:  "seeded-from-sidecar",
		},
	}
	if err := StampInstallReceipt(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if _, present := pkg.Metadata[receiptKeyMigrationSource]; present {
		t.Errorf("StampInstallReceipt did not clear migration_source: %v", pkg.Metadata)
	}
	if pkg.Metadata[receiptKeyInstalledBy] == "" {
		t.Errorf("installed_by should be stamped: %v", pkg.Metadata)
	}
}
