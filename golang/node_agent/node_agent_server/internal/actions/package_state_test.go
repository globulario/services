// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.package_state_tests
// @awareness file_role=regression_tests_for_install_receipt_preserve_at_report_state_chokepoint
// @awareness risk=high
package actions

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// strField is a structpb.Value builder for the test arg maps below.
func strField(s string) *structpb.Value {
	return structpb.NewStringValue(s)
}

// TestMergeReportStateMetadata_PreservesNonReceiptFields proves the
// merge step copies entrypoint_checksum (and any other sibling-writer
// fields) verbatim from `existing` into the returned map. Without this,
// every report_state overwrite would erase the proof writer's record.
func TestMergeReportStateMetadata_PreservesNonReceiptFields(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			"entrypoint_checksum":   "abc123",
			"proof_on_disk_sha256":  "def456",
			"proof_binary_path":     "/usr/lib/globular/bin/foo_server",
			"some_other_sibling":    "value",
		},
	}
	got := mergeReportStateMetadata(existing, nil)
	for k, want := range existing.Metadata {
		if got[k] != want {
			t.Errorf("metadata[%q] = %q, want %q", k, got[k], want)
		}
	}
}

// TestMergeReportStateMetadata_SkipsReceiptKeysFromExisting proves the
// merge step does NOT copy install-receipt keys (the canonical chokepoint
// installreceipt.Preserve owns those). Copying receipt keys here would
// pre-populate next.Metadata in a way that defeats Preserve's
// canonical-install detection: the migration_source carry-over rule
// only suppresses when next has installed_by AND no migration_source.
func TestMergeReportStateMetadata_SkipsReceiptKeysFromExisting(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			installreceipt.KeyInstalledBy:      "node-agent",
			installreceipt.KeyInstalledAt:      "1780521285",
			installreceipt.KeyMigrationSource:  installreceipt.MigrationSourceLegacySidecar,
			installreceipt.KeyUnitFileSha256:   "deadbeef",
			"entrypoint_checksum":              "abc123",
		},
	}
	got := mergeReportStateMetadata(existing, nil)
	for _, k := range installreceipt.Keys() {
		if _, present := got[k]; present {
			t.Errorf("receipt key %q must not be copied by merge (got %q); installreceipt.Preserve owns it",
				k, got[k])
		}
	}
	if got["entrypoint_checksum"] != "abc123" {
		t.Errorf("non-receipt key entrypoint_checksum lost: %q", got["entrypoint_checksum"])
	}
}

// TestMergeReportStateMetadata_WorkflowArgsWinOverExisting proves that
// when a workflow step passes a metadata field that also exists on the
// prior record, the workflow value wins. This was the pre-refactor
// semantics and is preserved.
func TestMergeReportStateMetadata_WorkflowArgsWinOverExisting(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{"entrypoint_checksum": "old"},
	}
	fields := map[string]*structpb.Value{
		"entrypoint_checksum": strField("new"),
	}
	got := mergeReportStateMetadata(existing, fields)
	if got["entrypoint_checksum"] != "new" {
		t.Errorf("workflow arg should win; got %q", got["entrypoint_checksum"])
	}
}

// TestMergeReportStateMetadata_TypedFieldsNotCarried proves that the
// typed action contract fields (node_id, name, version, …) are NEVER
// reflected into the open metadata map. They are typed columns on
// InstalledPackage; mirroring them into metadata would double-encode
// and let consumers diverge.
func TestMergeReportStateMetadata_TypedFieldsNotCarried(t *testing.T) {
	fields := map[string]*structpb.Value{
		"node_id":      strField("n1"),
		"name":         strField("foo"),
		"version":      strField("1.0.0"),
		"kind":         strField("SERVICE"),
		"publisher_id": strField("p"),
		"platform":     strField("linux/amd64"),
		"checksum":     strField("c"),
		"operation_id": strField("op"),
		"status":       strField("installed"),
		"build_number": strField("42"),
	}
	got := mergeReportStateMetadata(nil, fields)
	if got != nil {
		t.Fatalf("expected nil metadata when only typed fields present, got %v", got)
	}
}

// TestMergeReportStateMetadata_EmptyResultIsNil proves the helper
// returns nil (not an empty map) so downstream readers can rely on
// "no metadata == nil map" without special-casing.
func TestMergeReportStateMetadata_EmptyResultIsNil(t *testing.T) {
	if got := mergeReportStateMetadata(nil, nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	emptyExisting := &node_agentpb.InstalledPackage{Metadata: map[string]string{}}
	if got := mergeReportStateMetadata(emptyExisting, nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestMergeReportStateMetadata_NilExistingIsSafe proves the helper
// tolerates a nil `existing` argument. A first-time write of a brand-new
// installed package has no prior record to merge from.
func TestMergeReportStateMetadata_NilExistingIsSafe(t *testing.T) {
	fields := map[string]*structpb.Value{
		"entrypoint_checksum": strField("abc"),
	}
	got := mergeReportStateMetadata(nil, fields)
	if got["entrypoint_checksum"] != "abc" {
		t.Errorf("expected new entry to survive, got %v", got)
	}
}

// TestMergeReportStateMetadata_SkipsEmptyExistingValues proves that an
// empty string in existing.Metadata is treated as absent. A prior writer
// might have stored "" by accident; the merge must not re-emit it.
func TestMergeReportStateMetadata_SkipsEmptyExistingValues(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			"empty_key":           "",
			"entrypoint_checksum": "abc",
		},
	}
	got := mergeReportStateMetadata(existing, nil)
	if _, present := got["empty_key"]; present {
		t.Errorf("empty existing value must not be copied")
	}
	if got["entrypoint_checksum"] != "abc" {
		t.Errorf("non-empty key lost: %q", got["entrypoint_checksum"])
	}
}

// TestMergeReportStateMetadata_SkipsEmptyWorkflowArgs proves that a
// workflow step passing an empty string for a metadata field does not
// clobber the existing value. Empty == "not set" at this layer.
func TestMergeReportStateMetadata_SkipsEmptyWorkflowArgs(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{"entrypoint_checksum": "abc"},
	}
	fields := map[string]*structpb.Value{
		"entrypoint_checksum": strField(""),
	}
	got := mergeReportStateMetadata(existing, fields)
	if got["entrypoint_checksum"] != "abc" {
		t.Errorf("empty workflow arg should not clobber existing; got %q",
			got["entrypoint_checksum"])
	}
}

// TestPackageReportState_PreserveAfterCanonicalInstall is the end-to-end
// regression for the bug that motivated the cross-package refactor: a
// canonical install stamped installed_by + cleared migration_source on
// existing; the report_state action must NOT re-introduce migration_source
// and MUST carry installed_by forward.
//
// The test exercises the full merge → installreceipt.Preserve pipeline
// at the action-shape level (without WriteInstalledPackage) by reusing
// the helpers the action calls.
func TestPackageReportState_PreserveAfterCanonicalInstall(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			installreceipt.KeyInstalledBy:    "node-agent.apply_package_release.service",
			installreceipt.KeyInstalledAt:    "1780521285",
			installreceipt.KeyUnitFileSha256: "deadbeef",
			installreceipt.KeyUnitFilePath:   "/etc/systemd/system/globular-foo.service",
			installreceipt.KeyBinarySha256:   "abc123",
			"entrypoint_checksum":            "feedbeef",
			// migration_source intentionally absent: StampInstallReceipt
			// deletes it. This represents the post-canonical-install state.
		},
	}
	next := &node_agentpb.InstalledPackage{
		Metadata: mergeReportStateMetadata(existing, nil),
	}
	installreceipt.Preserve(existing, next)

	for _, k := range []string{
		installreceipt.KeyInstalledBy,
		installreceipt.KeyInstalledAt,
		installreceipt.KeyUnitFileSha256,
		installreceipt.KeyUnitFilePath,
		installreceipt.KeyBinarySha256,
	} {
		if next.Metadata[k] != existing.Metadata[k] {
			t.Errorf("canonical receipt key %q lost: got %q, want %q",
				k, next.Metadata[k], existing.Metadata[k])
		}
	}
	if next.Metadata["entrypoint_checksum"] != "feedbeef" {
		t.Errorf("non-receipt sibling field lost: %q", next.Metadata["entrypoint_checksum"])
	}
	if _, present := next.Metadata[installreceipt.KeyMigrationSource]; present {
		t.Errorf("migration_source must not be re-introduced after canonical install; got %q",
			next.Metadata[installreceipt.KeyMigrationSource])
	}
}

// TestPackageReportState_PreserveBeforeCanonicalInstall is the other
// half of the migration_source carry rule: when existing carries only
// the legacy_sidecar marker (no canonical install has happened yet),
// the action MUST carry migration_source forward — the heartbeat would
// otherwise re-stamp it every cycle.
func TestPackageReportState_PreserveBeforeCanonicalInstall(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			installreceipt.KeyUnitFileSha256:  "deadbeef",
			installreceipt.KeyUnitFilePath:    "/etc/systemd/system/globular-foo.service",
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
			installreceipt.KeyInstalledAt:     "1780521285",
			// installed_by intentionally absent: this is the pre-canonical
			// migration-only state.
		},
	}
	next := &node_agentpb.InstalledPackage{
		Metadata: mergeReportStateMetadata(existing, nil),
	}
	installreceipt.Preserve(existing, next)

	if next.Metadata[installreceipt.KeyMigrationSource] != installreceipt.MigrationSourceLegacySidecar {
		t.Errorf("migration_source must be carried forward when no canonical install present; got %q",
			next.Metadata[installreceipt.KeyMigrationSource])
	}
	if next.Metadata[installreceipt.KeyUnitFileSha256] != "deadbeef" {
		t.Errorf("legacy unit_file_sha256 lost: %q", next.Metadata[installreceipt.KeyUnitFileSha256])
	}
}

// TestPackageReportState_NoExistingPkg proves that a first-time write
// (no existing installed_state row, no workflow-arg metadata) results
// in a clean InstalledPackage with nil Metadata and no panic.
func TestPackageReportState_NoExistingPkg(t *testing.T) {
	next := &node_agentpb.InstalledPackage{
		Metadata: mergeReportStateMetadata(nil, nil),
	}
	installreceipt.Preserve(nil, next)
	if next.Metadata != nil {
		t.Errorf("expected nil metadata on first-time write, got %v", next.Metadata)
	}
}

// TestStampReceiptForReportState_NilSafeAndShortCircuits proves the
// helper tolerates nil pkg and empty Name without panic.
func TestStampReceiptForReportState_NilSafeAndShortCircuits(t *testing.T) {
	stampReceiptForReportState(nil)
	stampReceiptForReportState(&node_agentpb.InstalledPackage{})
}

// TestStampReceiptForReportState_StampsInstalledByEvenWithoutFiles
// proves the helper writes installed_by even when conventional file
// paths do not exist. The receipt is forensic — partial receipts are
// allowed when binaries/units are not at conventional paths; what is
// NOT allowed is a missing installed_by, because that is the canonical
// signal that report_state is the install commit.
func TestStampReceiptForReportState_StampsInstalledByEvenWithoutFiles(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{
		Name:    "test-no-files-on-disk-12345",
		Kind:    "SERVICE",
		Version: "1.0.0",
	}
	stampReceiptForReportState(pkg)
	if pkg.Metadata[installreceipt.KeyInstalledBy] != reportStateInstalledBy {
		t.Errorf("installed_by not stamped; got %q want %q",
			pkg.Metadata[installreceipt.KeyInstalledBy], reportStateInstalledBy)
	}
	if pkg.Metadata[installreceipt.KeyInstalledAt] == "" {
		t.Errorf("installed_at not stamped")
	}
}

// TestStampReceiptForReportState_ClearsMigrationSource proves the
// helper's Stamp call clears any prior legacy_sidecar marker, which
// is the contract for "a first-hand install observation has replaced
// the legacy seed." This is the regression for the missing canonical-
// stamp bug observed on the live cluster after node-agent install
// 1.2.146 → 1.2.148: migration_source persisted because no stamp site
// existed in the workflow install path.
func TestStampReceiptForReportState_ClearsMigrationSource(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{
		Name: "x",
		Kind: "SERVICE",
		Metadata: map[string]string{
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
			installreceipt.KeyUnitFileSha256:  "deadbeef",
		},
	}
	stampReceiptForReportState(pkg)
	if _, present := pkg.Metadata[installreceipt.KeyMigrationSource]; present {
		t.Errorf("migration_source not cleared; got %q",
			pkg.Metadata[installreceipt.KeyMigrationSource])
	}
	if pkg.Metadata[installreceipt.KeyInstalledBy] != reportStateInstalledBy {
		t.Errorf("installed_by not stamped; got %q", pkg.Metadata[installreceipt.KeyInstalledBy])
	}
}

// TestConventionalBinaryPath proves the binary-path convention used
// when the manifest is unavailable: SERVICE kind probes <name>_server
// first then plain <name>; INFRASTRUCTURE returns <name>.
func TestConventionalBinaryPath(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want string
	}{
		{"foo", "SERVICE", "/usr/lib/globular/bin/foo_server"},
		{"scylla-manager", "SERVICE", "/usr/lib/globular/bin/scylla_manager_server"},
		{"etcd", "INFRASTRUCTURE", "/usr/lib/globular/bin/etcd"},
		{"my-app", "APPLICATION", "/usr/lib/globular/bin/my-app"},
		{"", "SERVICE", ""},
	}
	for _, tt := range tests {
		got := conventionalBinaryPath(tt.name, tt.kind)
		// SERVICE kind probes filesystem first — if the file doesn't
		// exist, falls through to the underscore-converted name.
		// We can't easily mock the file existence here, so just
		// assert the path starts with the expected prefix.
		if tt.want == "" {
			if got != "" {
				t.Errorf("conventionalBinaryPath(%q,%q) = %q, want empty",
					tt.name, tt.kind, got)
			}
			continue
		}
		// For SERVICE kind, either the _server or plain variant is
		// acceptable depending on filesystem.
		altWant := strings.TrimSuffix(tt.want, "_server")
		if got != tt.want && got != altWant {
			t.Errorf("conventionalBinaryPath(%q,%q) = %q, want %q or %q",
				tt.name, tt.kind, got, tt.want, altWant)
		}
	}
}
