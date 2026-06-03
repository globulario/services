// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.sync_installed_package_tests
// @awareness file_role=regression_tests_for_install_workflow_sync_step_clobbers_receipt
// @awareness risk=critical
package main

import (
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestMergeSyncInstalledPackage_PreservesCanonicalReceipt is the
// regression for the live install-workflow wipe observed 2026-06-03
// across node-agent installs 1.2.147 → 1.2.150. The
// sync_installed_state workflow step previously built a fresh
// InstalledPackage{} with no Metadata, which CommitInstalledPackage
// marshalled and wrote — clobbering the canonical install receipt
// stamped by installer-api seconds earlier. The next heartbeat's
// checkUnitHashDrift then fell to legacy_sidecar migration and
// stamped the 4-key wipe shape every cycle.
//
// The pure helper must keep installed_by, unit_file_sha256,
// binary_sha256, entrypoint_checksum, proof_*, and every other
// non-identity metadata key intact.
func TestMergeSyncInstalledPackage_PreservesCanonicalReceipt(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		NodeId:        "node-1",
		Name:          "node-agent",
		Kind:          "SERVICE",
		Version:       "1.2.150",
		Checksum:      "OLD_CHECKSUM",
		BuildId:       "OLD_BUILD_ID",
		InstalledUnix: 1700000000,
		Metadata: map[string]string{
			"installed_by":         "node-agent.installer-api",
			"installed_at":         "1700000001",
			"unit_file_sha256":     "63670501a2f88825aabb",
			"unit_file_path":       "/etc/systemd/system/globular-node-agent.service",
			"binary_sha256":        "ccccdddd0123456789aa",
			"binary_path":          "/usr/lib/globular/bin/node_agent_server",
			"entrypoint_checksum":  "ccccdddd0123456789aa",
			"proof_on_disk_sha256": "ccccdddd0123456789aa",
			"proof_source":         "self_hosted_runtime_proof",
		},
	}
	pkg := mergeSyncInstalledPackage(existing, "node-1", "node-agent", "1.2.151", "NEW_CHECKSUM", "SERVICE", "NEW_BUILD_ID")

	// Cross-validated identity fields MUST be overwritten.
	if pkg.GetVersion() != "1.2.151" {
		t.Errorf("Version not updated: got %q want 1.2.151", pkg.GetVersion())
	}
	if pkg.GetChecksum() != "NEW_CHECKSUM" {
		t.Errorf("Checksum not updated: got %q want NEW_CHECKSUM", pkg.GetChecksum())
	}
	if pkg.GetBuildId() != "NEW_BUILD_ID" {
		t.Errorf("BuildId not updated: got %q want NEW_BUILD_ID", pkg.GetBuildId())
	}
	// Original install timestamp preserved (not zeroed).
	if pkg.GetInstalledUnix() != 1700000000 {
		t.Errorf("InstalledUnix should be preserved from existing; got %d", pkg.GetInstalledUnix())
	}

	// Every metadata key must be present and equal to the existing value.
	for k, want := range existing.Metadata {
		if got := pkg.Metadata[k]; got != want {
			t.Errorf("metadata[%q] = %q, want %q", k, got, want)
		}
	}
	// Specifically: installed_by must be intact (this was the live regression).
	if pkg.Metadata["installed_by"] == "" {
		t.Fatal("installed_by must be preserved; the entire purpose of this fix")
	}
}

// TestMergeSyncInstalledPackage_NilExisting_FreshConstruction proves
// that when no prior row exists (truly first commit for this node /
// kind / name), the helper constructs a fresh package with the
// caller-provided identity. Metadata is nil — that's correct, the
// install path's later writes will populate it.
func TestMergeSyncInstalledPackage_NilExisting_FreshConstruction(t *testing.T) {
	pkg := mergeSyncInstalledPackage(nil, "node-1", "new-service", "1.0.0", "CHK", "SERVICE", "BID")
	if pkg.GetNodeId() != "node-1" {
		t.Errorf("NodeId = %q, want node-1", pkg.GetNodeId())
	}
	if pkg.GetName() != "new-service" {
		t.Errorf("Name = %q, want new-service", pkg.GetName())
	}
	if pkg.GetVersion() != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", pkg.GetVersion())
	}
	if pkg.GetChecksum() != "CHK" {
		t.Errorf("Checksum = %q, want CHK", pkg.GetChecksum())
	}
	if pkg.GetKind() != "SERVICE" {
		t.Errorf("Kind = %q, want SERVICE", pkg.GetKind())
	}
	if pkg.GetBuildId() != "BID" {
		t.Errorf("BuildId = %q, want BID", pkg.GetBuildId())
	}
}

// TestMergeSyncInstalledPackage_CannotDowngradeReceiptToFourKeyShape
// is the explicit regression: an existing row with the canonical
// receipt must NEVER come out the other side with the 4-key
// legacy_sidecar shape after the merge.
func TestMergeSyncInstalledPackage_CannotDowngradeReceiptToFourKeyShape(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		NodeId: "node-1",
		Name:   "node-agent",
		Kind:   "SERVICE",
		Metadata: map[string]string{
			"installed_by":     "node-agent.installer-api",
			"installed_at":     "1700000001",
			"unit_file_sha256": "abcdef",
			"unit_file_path":   "/etc/systemd/system/globular-node-agent.service",
			"binary_sha256":    "112233",
			"binary_path":      "/usr/lib/globular/bin/node_agent_server",
		},
	}
	pkg := mergeSyncInstalledPackage(existing, "node-1", "node-agent", "1.2.151", "NEW_CHECKSUM", "SERVICE", "BID")

	if pkg.Metadata["installed_by"] == "" {
		t.Errorf("installed_by must survive the sync step")
	}
	if pkg.Metadata["binary_sha256"] == "" {
		t.Errorf("binary_sha256 must survive the sync step")
	}
	if pkg.Metadata["binary_path"] == "" {
		t.Errorf("binary_path must survive the sync step")
	}
	// The 4-key wipe shape is exactly {installed_at, unit_file_path,
	// unit_file_sha256, migration_source}. If the result reduces to
	// that pattern, the fix is broken.
	keys := make(map[string]struct{}, len(pkg.Metadata))
	for k := range pkg.Metadata {
		keys[k] = struct{}{}
	}
	if len(keys) <= 4 {
		t.Fatalf("metadata reduced to %d keys; regression risk (4-key shape was %v)",
			len(keys), keys)
	}
}

// TestMergeSyncInstalledPackage_EmptyKindUsesExistingKind proves that
// when the caller passes an empty kind (rare but possible during
// transient workflow dispatch), the helper keeps existing.Kind so we
// do not corrupt the etcd row's identity.
func TestMergeSyncInstalledPackage_EmptyKindUsesExistingKind(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		NodeId: "node-1",
		Name:   "x",
		Kind:   "SERVICE",
	}
	pkg := mergeSyncInstalledPackage(existing, "node-1", "x", "1.0.0", "CHK", "", "BID")
	if pkg.GetKind() != "SERVICE" {
		t.Errorf("Kind = %q; expected SERVICE preserved from existing", pkg.GetKind())
	}
}

// TestMergeSyncInstalledPackage_KindOverrideUpdatesKind proves that
// a non-empty kind on input does override existing.Kind (defence
// against a stale row written under the wrong kind).
func TestMergeSyncInstalledPackage_KindOverrideUpdatesKind(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		NodeId: "node-1",
		Name:   "x",
		Kind:   "INFRASTRUCTURE",
	}
	pkg := mergeSyncInstalledPackage(existing, "node-1", "x", "1.0.0", "CHK", "SERVICE", "BID")
	if pkg.GetKind() != "SERVICE" {
		t.Errorf("Kind = %q; expected caller-provided SERVICE to override", pkg.GetKind())
	}
}

// TestMergeSyncInstalledPackage_DoesNotZeroExistingFields proves that
// non-identity fields like InstalledUnix, UpdatedUnix, PublisherID,
// Platform are preserved through the merge.
func TestMergeSyncInstalledPackage_DoesNotZeroExistingFields(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		NodeId:        "node-1",
		Name:          "x",
		Kind:          "SERVICE",
		InstalledUnix: 1700000000,
		UpdatedUnix:   1700000500,
		PublisherId:   "core@globular.io",
		Platform:      "linux/amd64",
	}
	pkg := mergeSyncInstalledPackage(existing, "node-1", "x", "1.0.0", "CHK", "SERVICE", "BID")
	if pkg.GetInstalledUnix() != 1700000000 {
		t.Errorf("InstalledUnix = %d; expected 1700000000 preserved", pkg.GetInstalledUnix())
	}
	if pkg.GetUpdatedUnix() != 1700000500 {
		t.Errorf("UpdatedUnix = %d; expected 1700000500 preserved (CommitInstalledPackage handles stamping)", pkg.GetUpdatedUnix())
	}
	if pkg.GetPublisherId() != "core@globular.io" {
		t.Errorf("PublisherId = %q; expected preserved", pkg.GetPublisherId())
	}
	if pkg.GetPlatform() != "linux/amd64" {
		t.Errorf("Platform = %q; expected preserved", pkg.GetPlatform())
	}
}
