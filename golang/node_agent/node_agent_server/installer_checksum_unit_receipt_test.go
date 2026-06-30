// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.installer_api.unit_receipt
// @awareness file_role=regression_tests_for_writeInstalledStateChecksum_unit_receipt_paths
// @awareness enforces=globular.platform:invariant.desired_installed_runtime_identity_must_match
// @awareness protects=globular.platform:failure_mode.node_agent.install_package_aliases_convergence_hash_into_expected_sha256
// @awareness risk=high
//
// Regression tests for the two bugs fixed in writeInstalledStateChecksum:
//
//  Bug 1 (early return on unhashable binary): infrastructure packages whose
//  binary lives at a system path (minio, etcd) caused an early return with
//  NO receipt written. Fix: log and continue, stamp unit receipt regardless.
//
//  Bug 2 (unit_file_sha256 missing when render fails): when
//  renderCanonicalUnitFromLocalArtifact failed, UnitFilePath was not set in
//  ReceiptOpts, so Stamp wrote installed_by but NOT unit_file_sha256.
//  shouldMigrateFromSidecar then returned "installed_state_missing_or_unproven"
//  on every heartbeat. Fix: set UnitFilePath so Stamp hashes the on-disk file.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestWriteInstalledStateChecksum_BinaryMissing_UnitReceiptStillWritten pins
// Bug 1: when the binary is not hashable (infrastructure package at a system
// path), the unit receipt MUST still be stamped.
//
// The test exercises StampInstallReceipt directly with the post-fix code path:
// BinaryPath empty (binary unhashable), UnitFilePath set to an on-disk file.
// Stamp must write unit_file_sha256 + installed_by even without a binary path.
func TestWriteInstalledStateChecksum_BinaryMissing_UnitReceiptStillWritten(t *testing.T) {
	dir := t.TempDir()

	// Write a fake unit file to stamp.
	unitContent := "[Unit]\nDescription=Test service\n"
	unitPath := filepath.Join(dir, "globular-minio.service")
	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		t.Fatal(err)
	}
	expectedUnitSha := func() string {
		sum := sha256.Sum256([]byte(unitContent))
		return hex.EncodeToString(sum[:])
	}()

	pkg := &node_agentpb.InstalledPackage{
		Name:    "minio",
		Kind:    "INFRASTRUCTURE",
		Version: "1.2.0",
		Metadata: map[string]string{},
	}

	// Post-fix path: BinaryPath is empty (unhashable), UnitFilePath is set.
	opts := installreceipt.ReceiptOpts{
		// BinaryPath intentionally empty — mirrors the post-fix behaviour when
		// cachedSha256 returns an error (binPath set to "").
		InstalledBy:  "node-agent.installer-api",
		UnitFilePath: unitPath,
		// UnitFileContent empty — Stamp reads+hashes from UnitFilePath.
	}
	if err := installreceipt.Stamp(pkg, opts); err != nil {
		t.Fatalf("Stamp returned error with missing binary path: %v", err)
	}

	// Bug 1 contract: installed_by is written even though binary was unhashable.
	if got := pkg.Metadata[installreceipt.KeyInstalledBy]; got == "" {
		t.Error("installed_by not written — early return on binary hash failure not removed")
	}

	// Unit receipt contract: unit_file_sha256 is written from on-disk file.
	if got := pkg.Metadata[installreceipt.KeyUnitFileSha256]; got == "" {
		t.Error("unit_file_sha256 not written — unit receipt missing when binary hash failed")
	} else if got != expectedUnitSha {
		t.Errorf("unit_file_sha256 = %q, want %q", got, expectedUnitSha)
	}

	if got := pkg.Metadata[installreceipt.KeyUnitFilePath]; got != unitPath {
		t.Errorf("unit_file_path = %q, want %q", got, unitPath)
	}

	// binary_sha256 must NOT be written — no binary was available.
	if got := pkg.Metadata[installreceipt.KeyBinarySha256]; got != "" {
		t.Errorf("binary_sha256 should be empty when binary was unhashable, got %q", got)
	}
}

// TestWriteInstalledStateChecksum_RenderFails_OnDiskUnitFileHashed pins Bug 2:
// when renderCanonicalUnitFromLocalArtifact fails, the on-disk unit file must
// be hashed as a fallback so unit_file_sha256 is written.
//
// Pre-fix behaviour: UnitFilePath was not set → Stamp wrote installed_by but
// NOT unit_file_sha256 → shouldMigrateFromSidecar returned
// "installed_state_missing_or_unproven" on every heartbeat.
//
// Post-fix behaviour: UnitFilePath is always set when the unit file exists on
// disk, so Stamp hashes it (UnitFileContent empty → file-hash path).
func TestWriteInstalledStateChecksum_RenderFails_OnDiskUnitFileHashed(t *testing.T) {
	dir := t.TempDir()

	unitContent := "[Unit]\nDescription=Cluster controller\n[Service]\nExecStart=/usr/lib/globular/bin/cluster_controller_server\n"
	unitPath := filepath.Join(dir, "globular-cluster-controller.service")
	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		t.Fatal(err)
	}
	expectedUnitSha := func() string {
		sum := sha256.Sum256([]byte(unitContent))
		return hex.EncodeToString(sum[:])
	}()

	binContent := "ELF-binary-stub"
	binPath := filepath.Join(dir, "cluster_controller_server")
	if err := os.WriteFile(binPath, []byte(binContent), 0o755); err != nil {
		t.Fatal(err)
	}

	pkg := &node_agentpb.InstalledPackage{
		Name:    "cluster-controller",
		Kind:    "SERVICE",
		Version: "1.2.200",
		Metadata: map[string]string{},
	}

	// Post-fix path: render failed → UnitFilePath set, UnitFileContent empty.
	// This is the fallback branch added to fix Bug 2.
	opts := installreceipt.ReceiptOpts{
		BinaryPath:   binPath,
		InstalledBy:  "node-agent.installer-api",
		UnitFilePath: unitPath,
		// UnitFileContent intentionally empty — mirrors the fallback when render fails.
	}
	if err := installreceipt.Stamp(pkg, opts); err != nil {
		t.Fatalf("Stamp returned error: %v", err)
	}

	// Bug 2 contract: unit_file_sha256 must be written from on-disk hash.
	if got := pkg.Metadata[installreceipt.KeyUnitFileSha256]; got == "" {
		t.Error("unit_file_sha256 not written — this is the installed_state_missing_or_unproven bug")
	} else if got != expectedUnitSha {
		t.Errorf("unit_file_sha256 = %q, want on-disk hash %q", got, expectedUnitSha)
	}

	if got := pkg.Metadata[installreceipt.KeyInstalledBy]; got == "" {
		t.Error("installed_by not written")
	}
	if got := pkg.Metadata[installreceipt.KeyUnitFilePath]; got != unitPath {
		t.Errorf("unit_file_path = %q, want %q", got, unitPath)
	}
}

// TestWriteInstalledStateChecksum_RenderFails_PreFix_WouldMissUnitSha documents
// the pre-fix failure shape: Stamp with UnitFilePath="" and UnitFileContent=nil
// writes installed_by but NOT unit_file_sha256, which is exactly what causes
// shouldMigrateFromSidecar to return "installed_state_missing_or_unproven".
func TestWriteInstalledStateChecksum_RenderFails_PreFix_WouldMissUnitSha(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "cluster_controller_server")
	if err := os.WriteFile(binPath, []byte("ELF"), 0o755); err != nil {
		t.Fatal(err)
	}

	pkg := &node_agentpb.InstalledPackage{
		Name:    "cluster-controller",
		Kind:    "SERVICE",
		Version: "1.2.200",
		Metadata: map[string]string{},
	}

	// Pre-fix shape: UnitFilePath not set (render failed and old code skipped it).
	opts := installreceipt.ReceiptOpts{
		BinaryPath:  binPath,
		InstalledBy: "node-agent.installer-api",
		// UnitFilePath intentionally empty — this was the pre-fix omission.
	}
	if err := installreceipt.Stamp(pkg, opts); err != nil {
		t.Fatalf("Stamp returned error: %v", err)
	}

	// Pre-fix shape results in installed_by present but unit_file_sha256 absent.
	// This is the shape that triggers "installed_state_missing_or_unproven".
	if got := pkg.Metadata[installreceipt.KeyInstalledBy]; got == "" {
		t.Error("test setup: installed_by should be written by Stamp")
	}
	if got := pkg.Metadata[installreceipt.KeyUnitFileSha256]; got != "" {
		t.Errorf("test documents the pre-fix shape: unit_file_sha256 should be absent when UnitFilePath not set, got %q", got)
	}
	// Document: this shape causes shouldMigrateFromSidecar to return
	// "installed_state_missing_or_unproven" — installed_by set, unit_file_sha256 absent.
}
