// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.installer_api.skip_restamp_tests
// @awareness file_role=regression_tests_for_canonical_receipt_restamp_on_install_skip
// @awareness risk=high
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

// writeRestampFile creates a file with the given content and returns its absolute
// path and sha256.
func writeRestampFile(t *testing.T, dir, name, content string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(sum[:])
}

func mustReadRestampFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestStampSkipPathReceipt_StampsCanonicalReceiptOverLegacy is the
// regression for the live envoy unit_file_drift observation
// (2026-06-03): the install skip path proved on-disk content matches
// desired version but never re-stamped the receipt, leaving
// migration_source=legacy_sidecar in place forever. The pure helper
// must produce a canonical receipt that supersedes the legacy marker.
func TestStampSkipPathReceipt_StampsCanonicalReceiptOverLegacy(t *testing.T) {
	dir := t.TempDir()
	unitPath, wantUnitSha := writeRestampFile(t, dir, "globular-envoy.service", "[Unit]\nDescription=Envoy\n[Service]\nExecStart=/usr/lib/globular/bin/envoy\n[Install]\nWantedBy=multi-user.target\n")
	binPath, wantBinSha := writeRestampFile(t, dir, "envoy", "ELF-binary-bytes-for-test")

	pkg := &node_agentpb.InstalledPackage{
		NodeId:  "node-1",
		Name:    "envoy",
		Kind:    "INFRASTRUCTURE",
		Version: "1.35.3",
		Metadata: map[string]string{
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
			installreceipt.KeyUnitFilePath:    unitPath,
			installreceipt.KeyUnitFileSha256:  "STALE_PRE_INSTALL_SHA",
			installreceipt.KeyInstalledAt:     "1700000000",
		},
	}

	if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, wantBinSha) {
		t.Fatal("expected stamp to succeed")
	}

	if got := pkg.Metadata[installreceipt.KeyInstalledBy]; got != "node-agent.grpc_workflow.install_skip_restamp" {
		t.Errorf("installed_by = %q; want node-agent.grpc_workflow.install_skip_restamp", got)
	}
	if _, present := pkg.Metadata[installreceipt.KeyMigrationSource]; present {
		t.Errorf("migration_source must be cleared by canonical stamp; got %q",
			pkg.Metadata[installreceipt.KeyMigrationSource])
	}
	if got := pkg.Metadata[installreceipt.KeyUnitFileSha256]; got != wantUnitSha {
		t.Errorf("unit_file_sha256 = %q; want %q (computed from disk)", got, wantUnitSha)
	}
	if got := pkg.Metadata[installreceipt.KeyBinarySha256]; got != wantBinSha {
		t.Errorf("binary_sha256 = %q; want %q", got, wantBinSha)
	}
	if got := pkg.Metadata["entrypoint_checksum"]; got != wantBinSha {
		t.Errorf("entrypoint_checksum = %q; want %q (binary hash passed by caller)", got, wantBinSha)
	}
}

// TestStampSkipPathReceipt_NilPkgIsSafe proves the helper does not
// panic and refuses to stamp when pkg is nil. Critical because the
// caller fetches existing from etcd best-effort.
func TestStampSkipPathReceipt_NilPkgIsSafe(t *testing.T) {
	if stampSkipPathReceipt(nil, "/tmp/unit", nil, "/tmp/bin", "abc") {
		t.Error("must refuse to stamp nil pkg")
	}
}

// TestStampSkipPathReceipt_EmptyHashRefused proves the helper refuses
// when the caller can't provide a binary hash. Better fail-closed than
// stamp a half-receipt.
func TestStampSkipPathReceipt_EmptyHashRefused(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{Name: "envoy", Kind: "INFRASTRUCTURE"}
	if stampSkipPathReceipt(pkg, "/tmp/unit", nil, "/tmp/bin", "") {
		t.Error("empty hash must be refused")
	}
	if stampSkipPathReceipt(pkg, "/tmp/unit", nil, "/tmp/bin", "   ") {
		t.Error("whitespace-only hash must be refused")
	}
}

// TestStampSkipPathReceipt_UnreadableDeclaredFileRefusesStamp proves
// the helper does NOT commit a partial receipt when a declared file
// path cannot be hashed. The original drift bug was exactly the
// "half-stamped receipt" anti-pattern — this guard keeps it from
// re-entering through the skip path.
func TestStampSkipPathReceipt_UnreadableDeclaredFileRefusesStamp(t *testing.T) {
	dir := t.TempDir()
	binPath, binSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")
	missingUnit := filepath.Join(dir, "does-not-exist.service")

	pkg := &node_agentpb.InstalledPackage{
		Name: "envoy",
		Kind: "INFRASTRUCTURE",
		Metadata: map[string]string{
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
		},
	}
	if stampSkipPathReceipt(pkg, missingUnit, nil, binPath, binSha) {
		t.Fatal("must refuse to stamp when declared unit file unreadable")
	}
	// Metadata must NOT have been partially mutated to a half-receipt
	// shape. The migration_source must still be present (cleared only
	// on successful Stamp).
	if pkg.Metadata[installreceipt.KeyMigrationSource] != installreceipt.MigrationSourceLegacySidecar {
		t.Errorf("migration_source unexpectedly cleared on failed stamp")
	}
	if pkg.Metadata[installreceipt.KeyInstalledBy] == "node-agent.grpc_workflow.install_skip_restamp" {
		t.Errorf("installed_by must NOT be set when Stamp failed")
	}
	if pkg.Metadata[installreceipt.KeyBinarySha256] != "" {
		t.Errorf("binary_sha256 must NOT be set when Stamp failed; got %q", pkg.Metadata[installreceipt.KeyBinarySha256])
	}
}

// TestStampSkipPathReceipt_StampsBinaryOnlyWhenNoUnitFile proves the
// helper handles the wrapper-package-without-unit case correctly
// (rare but possible for COMMAND-flavoured infra). When unitPath is
// empty, Stamp is called with only BinaryPath — succeeds and produces
// installed_by + binary_sha256, no unit_file_sha256.
func TestStampSkipPathReceipt_StampsBinaryOnlyWhenNoUnitFile(t *testing.T) {
	dir := t.TempDir()
	binPath, wantBinSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")

	pkg := &node_agentpb.InstalledPackage{
		Name:     "envoy",
		Kind:     "INFRASTRUCTURE",
		Metadata: map[string]string{installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar},
	}
	if !stampSkipPathReceipt(pkg, "", nil, binPath, wantBinSha) {
		t.Fatal("expected stamp to succeed when only binary path provided")
	}
	if got := pkg.Metadata[installreceipt.KeyInstalledBy]; got == "" {
		t.Errorf("installed_by must be set even without unit file")
	}
	if got := pkg.Metadata[installreceipt.KeyBinarySha256]; got != wantBinSha {
		t.Errorf("binary_sha256 = %q; want %q", got, wantBinSha)
	}
	if got := pkg.Metadata[installreceipt.KeyUnitFileSha256]; got != "" {
		t.Errorf("unit_file_sha256 must NOT be set when no unit file declared; got %q", got)
	}
	if _, present := pkg.Metadata[installreceipt.KeyMigrationSource]; present {
		t.Errorf("migration_source must be cleared on canonical stamp success")
	}
}

// TestStampSkipPathReceipt_IdempotentOnAlreadyCanonicalReceipt proves
// that calling the helper twice with the same inputs produces the
// same canonical receipt content (only installed_at moves forward).
// This matters because the skip path may fire on every workflow
// sweep — the receipt must remain stable across calls.
func TestStampSkipPathReceipt_IdempotentOnAlreadyCanonicalReceipt(t *testing.T) {
	dir := t.TempDir()
	unitPath, wantUnitSha := writeRestampFile(t, dir, "globular-envoy.service", "[Unit]\nDescription=Envoy\n")
	binPath, wantBinSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")

	pkg := &node_agentpb.InstalledPackage{Name: "envoy", Kind: "INFRASTRUCTURE"}
	if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, wantBinSha) {
		t.Fatal("first stamp failed")
	}
	firstBinSha := pkg.Metadata[installreceipt.KeyBinarySha256]
	firstUnitSha := pkg.Metadata[installreceipt.KeyUnitFileSha256]
	firstInstalledBy := pkg.Metadata[installreceipt.KeyInstalledBy]

	if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, wantBinSha) {
		t.Fatal("second stamp failed")
	}
	if pkg.Metadata[installreceipt.KeyBinarySha256] != firstBinSha {
		t.Errorf("binary_sha256 changed across idempotent calls")
	}
	if pkg.Metadata[installreceipt.KeyUnitFileSha256] != firstUnitSha {
		t.Errorf("unit_file_sha256 changed across idempotent calls")
	}
	if pkg.Metadata[installreceipt.KeyInstalledBy] != firstInstalledBy {
		t.Errorf("installed_by changed across idempotent calls")
	}
	// Sanity: the receipt content reflects the files we wrote.
	if pkg.Metadata[installreceipt.KeyUnitFileSha256] != wantUnitSha {
		t.Errorf("unit_file_sha256 = %q; want %q", pkg.Metadata[installreceipt.KeyUnitFileSha256], wantUnitSha)
	}
	if pkg.Metadata[installreceipt.KeyBinarySha256] != wantBinSha {
		t.Errorf("binary_sha256 = %q; want %q", pkg.Metadata[installreceipt.KeyBinarySha256], wantBinSha)
	}
}

// TestStampSkipPathReceipt_DoesNotAdvanceUpdatedUnix is the regression
// for the live envoy / torrent service.old_pid_after_upgrade observed
// after commit 72ecf067 added this helper. The previous version
// bumped pkg.UpdatedUnix = time.Now().Unix() on every restamp; the
// verifier (cluster_doctor/collector/verification.go) computes
// ApplyTime = max(installedUnix, updatedUnix), so every restamp
// looked like a fresh apply at wall clock — and any process whose
// start time predated the restamp fired the critical finding.
//
// Same INC-2026-0016 class of bug the proof writer was hardened
// against. The restamp is metadata-only — it MUST NOT advance
// either timestamp.
func TestStampSkipPathReceipt_DoesNotAdvanceUpdatedUnix(t *testing.T) {
	dir := t.TempDir()
	unitPath, _ := writeRestampFile(t, dir, "globular-envoy.service", "[Unit]\n")
	binPath, binSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")

	const realInstallTime = 1700000000 // 4 hours-ago equivalent
	const realUpdateTime = 1700000500  // 4 hours ago, set by the real install
	pkg := &node_agentpb.InstalledPackage{
		Name:          "envoy",
		Kind:          "INFRASTRUCTURE",
		Version:       "1.35.3",
		InstalledUnix: realInstallTime,
		UpdatedUnix:   realUpdateTime,
		Metadata: map[string]string{
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
		},
	}
	if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, binSha) {
		t.Fatal("stamp failed")
	}
	if pkg.InstalledUnix != realInstallTime {
		t.Errorf("InstalledUnix was advanced from %d to %d; skip-path restamp must NOT touch InstalledUnix",
			realInstallTime, pkg.InstalledUnix)
	}
	if pkg.UpdatedUnix != realUpdateTime {
		t.Errorf("UpdatedUnix was advanced from %d to %d; skip-path restamp must NOT touch UpdatedUnix "+
			"(verifier reads max(installedUnix, updatedUnix) as ApplyTime; wall-clock bump → "+
			"false service.old_pid_after_upgrade for every running process)",
			realUpdateTime, pkg.UpdatedUnix)
	}
	// Receipt content WAS written — the metadata.installed_at audit
	// field is the right place to record when the restamp ran.
	if pkg.Metadata[installreceipt.KeyInstalledBy] == "" {
		t.Errorf("installed_by must still be stamped — only the timestamp fields are off-limits")
	}
	if pkg.Metadata[installreceipt.KeyInstalledAt] == "" {
		t.Errorf("installed_at must still be stamped (forensic field, not consumed by verifier)")
	}
}

// TestStampSkipPathReceipt_PreservesTimestampsAcrossRepeatedRestamps
// proves the timestamp-preservation rule holds under the realistic
// scenario where the skip path fires every workflow sweep (every
// few seconds on a busy cluster). InstalledUnix and UpdatedUnix
// MUST stay equal to their original install-time values across
// arbitrarily many restamp cycles.
func TestStampSkipPathReceipt_PreservesTimestampsAcrossRepeatedRestamps(t *testing.T) {
	dir := t.TempDir()
	unitPath, _ := writeRestampFile(t, dir, "globular-envoy.service", "[Unit]\n")
	binPath, binSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")

	const realInstallTime = 1700000000
	const realUpdateTime = 1700000500
	pkg := &node_agentpb.InstalledPackage{
		Name:          "envoy",
		Kind:          "INFRASTRUCTURE",
		InstalledUnix: realInstallTime,
		UpdatedUnix:   realUpdateTime,
	}
	for i := 0; i < 10; i++ {
		if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, binSha) {
			t.Fatalf("stamp #%d failed", i)
		}
	}
	if pkg.InstalledUnix != realInstallTime {
		t.Errorf("InstalledUnix drifted after 10 restamps: %d != %d", pkg.InstalledUnix, realInstallTime)
	}
	if pkg.UpdatedUnix != realUpdateTime {
		t.Errorf("UpdatedUnix drifted after 10 restamps: %d != %d", pkg.UpdatedUnix, realUpdateTime)
	}
}

// TestStampSkipPathReceipt_StillStampsUnitFileSha256_AfterTimestampFix
// is the contract guard for the surgical fix: removing the
// UpdatedUnix bump MUST NOT regress the original purpose of the
// restamp helper — clearing legacy_sidecar by writing canonical
// unit_file_sha256 + installed_by. This test asserts both
// timestamp-preservation AND the original stamping behavior on the
// same call.
func TestStampSkipPathReceipt_StillStampsUnitFileSha256_AfterTimestampFix(t *testing.T) {
	dir := t.TempDir()
	unitPath, wantUnitSha := writeRestampFile(t, dir, "globular-envoy.service", "[Unit]\nDescription=Envoy\n")
	binPath, wantBinSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")

	const realInstallTime = 1700000000
	pkg := &node_agentpb.InstalledPackage{
		Name:          "envoy",
		Kind:          "INFRASTRUCTURE",
		InstalledUnix: realInstallTime,
		UpdatedUnix:   realInstallTime,
		Metadata: map[string]string{
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
			installreceipt.KeyUnitFileSha256:  "STALE_PRE_INSTALL_SHA",
		},
	}
	if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, wantBinSha) {
		t.Fatal("stamp failed")
	}
	// Timestamp preservation:
	if pkg.InstalledUnix != realInstallTime || pkg.UpdatedUnix != realInstallTime {
		t.Errorf("timestamps moved: InstalledUnix=%d UpdatedUnix=%d want both %d",
			pkg.InstalledUnix, pkg.UpdatedUnix, realInstallTime)
	}
	// Original stamping behavior:
	if pkg.Metadata[installreceipt.KeyUnitFileSha256] != wantUnitSha {
		t.Errorf("unit_file_sha256 not refreshed from disk: got %q want %q",
			pkg.Metadata[installreceipt.KeyUnitFileSha256], wantUnitSha)
	}
	if _, present := pkg.Metadata[installreceipt.KeyMigrationSource]; present {
		t.Errorf("migration_source must be cleared by canonical stamp; still present: %q",
			pkg.Metadata[installreceipt.KeyMigrationSource])
	}
	if pkg.Metadata[installreceipt.KeyInstalledBy] != "node-agent.grpc_workflow.install_skip_restamp" {
		t.Errorf("installed_by attribution wrong: %q", pkg.Metadata[installreceipt.KeyInstalledBy])
	}
}

// TestStampSkipPathReceipt_PreservesSiblingNonReceiptFields proves
// the helper does NOT clobber sibling fields like proof_on_disk_sha256
// or proof_source that the heartbeat / proof writer manages. Only
// receipt-namespace fields are touched.
func TestStampSkipPathReceipt_PreservesSiblingNonReceiptFields(t *testing.T) {
	dir := t.TempDir()
	unitPath, _ := writeRestampFile(t, dir, "globular-envoy.service", "[Unit]\n")
	binPath, binSha := writeRestampFile(t, dir, "envoy", "ELF-bytes")

	pkg := &node_agentpb.InstalledPackage{
		Name: "envoy",
		Kind: "INFRASTRUCTURE",
		Metadata: map[string]string{
			"proof_on_disk_sha256": "PROOF_SHA_FROM_HEARTBEAT",
			"proof_source":         "self_hosted_runtime_proof",
			"proof_binary_path":    "/usr/lib/globular/bin/envoy",
		},
	}
	if !stampSkipPathReceipt(pkg, unitPath, mustReadRestampFile(t, unitPath), binPath, binSha) {
		t.Fatal("stamp failed")
	}
	if pkg.Metadata["proof_on_disk_sha256"] != "PROOF_SHA_FROM_HEARTBEAT" {
		t.Errorf("proof_on_disk_sha256 was clobbered")
	}
	if pkg.Metadata["proof_source"] != "self_hosted_runtime_proof" {
		t.Errorf("proof_source was clobbered")
	}
	if pkg.Metadata["proof_binary_path"] != "/usr/lib/globular/bin/envoy" {
		t.Errorf("proof_binary_path was clobbered")
	}
}
