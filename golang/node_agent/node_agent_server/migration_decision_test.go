// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.migration_decision_tests
// @awareness file_role=regression_tests_for_stale_snapshot_clobbers_canonical_receipt
// @awareness risk=critical
package main

import (
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestShouldMigrate_FreshHasUnitFileSha256_SkipsAndNoOpinionOnMatch is
// the core regression for the heartbeat-wipe bug observed live on
// 2026-06-03 (project_receipt_wipe_in_heartbeat.md): a stale pkg
// snapshot would let checkUnitHashDrift call stampMigrationFromLegacy
// Sidecar and clobber a freshly-stamped canonical receipt. The pure
// decision helper now refuses migration when fresh etcd already has
// unit_file_sha256 and the disk matches it.
func TestShouldMigrate_FreshHasUnitFileSha256_SkipsAndNoOpinionOnMatch(t *testing.T) {
	const sha = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	fresh := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Kind: "SERVICE",
		Metadata: map[string]string{
			installreceipt.KeyUnitFileSha256: sha,
			installreceipt.KeyInstalledBy:    "node-agent.installer-api",
		},
	}
	proceed, fallback := shouldMigrateFromSidecar(fresh, sha)
	if proceed {
		t.Fatal("must NOT migrate when fresh etcd has unit_file_sha256")
	}
	if fallback != "" {
		t.Errorf("disk matches fresh sha → no opinion; got %q", fallback)
	}
}

// TestShouldMigrate_FreshHasUnitFileSha256_DriftOnMismatch proves the
// helper reports unit_file_drift (not "") when disk disagrees with the
// fresh receipt. This is the drift path that the heartbeat should
// surface — not the migration path.
func TestShouldMigrate_FreshHasUnitFileSha256_DriftOnMismatch(t *testing.T) {
	fresh := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Kind: "SERVICE",
		Metadata: map[string]string{
			installreceipt.KeyUnitFileSha256: "AAAA000000000000AAAA000000000000AAAA000000000000AAAA000000000000",
		},
	}
	disk := "bbbb111111111111bbbb111111111111bbbb111111111111bbbb111111111111"
	proceed, fallback := shouldMigrateFromSidecar(fresh, disk)
	if proceed {
		t.Fatal("must NOT migrate; fresh has receipt")
	}
	if fallback != "unit_file_drift" {
		t.Errorf("expected unit_file_drift, got %q", fallback)
	}
}

// TestShouldMigrate_FreshHasInstalledByOnly_SkipsAsUnproven covers the
// transient window where the canonical install path has stamped
// installed_by but unit_file_sha256 is not yet populated. The helper
// must skip migration (otherwise we'd lose installed_by) and fail
// closed so the next heartbeat re-reads.
func TestShouldMigrate_FreshHasInstalledByOnly_SkipsAsUnproven(t *testing.T) {
	fresh := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Kind: "SERVICE",
		Metadata: map[string]string{
			installreceipt.KeyInstalledBy: "node-agent.installer-api",
		},
	}
	proceed, fallback := shouldMigrateFromSidecar(fresh, "any-disk-sha")
	if proceed {
		t.Fatal("must NOT migrate when fresh has installed_by")
	}
	if fallback != "installed_state_missing_or_unproven" {
		t.Errorf("expected installed_state_missing_or_unproven, got %q", fallback)
	}
}

// TestShouldMigrate_FreshHasMigrationSourceOnly_SkipsNoOpinion covers
// the case where a previous migration cycle already stamped
// migration_source for this unit but unit_file_sha256 was somehow not
// persisted. The helper must not re-stamp (idempotency safety): a
// second migration write would overwrite any field that another writer
// has since added. Return "" so authority 1 surfaces the drift on the
// next heartbeat once unit_file_sha256 is back.
func TestShouldMigrate_FreshHasMigrationSourceOnly_SkipsNoOpinion(t *testing.T) {
	fresh := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Kind: "SERVICE",
		Metadata: map[string]string{
			installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
		},
	}
	proceed, fallback := shouldMigrateFromSidecar(fresh, "any-disk-sha")
	if proceed {
		t.Fatal("must NOT re-stamp migration_source")
	}
	if fallback != "" {
		t.Errorf("expected empty fallback, got %q", fallback)
	}
}

// TestShouldMigrate_FreshHasNoReceipt_AllowsMigration proves the helper
// permits migration only when the fresh etcd row has NO receipt
// provenance — this is the legitimate first-time path for pre-refactor
// installs that have a sidecar on disk but no canonical receipt yet.
func TestShouldMigrate_FreshHasNoReceipt_AllowsMigration(t *testing.T) {
	fresh := &node_agentpb.InstalledPackage{
		Name:     "node-agent",
		Kind:     "SERVICE",
		Metadata: map[string]string{},
	}
	proceed, fallback := shouldMigrateFromSidecar(fresh, "disk-sha")
	if !proceed {
		t.Fatal("must allow migration when fresh has no receipt provenance")
	}
	if fallback != "" {
		t.Errorf("expected empty fallback when proceeding, got %q", fallback)
	}
}

// TestShouldMigrate_FreshHasNonReceiptMetadataOnly_AllowsMigration
// proves the helper does NOT mistake non-receipt fields like
// entrypoint_checksum for receipt provenance. Only the three receipt
// signals (unit_file_sha256, installed_by, migration_source) gate the
// migration.
func TestShouldMigrate_FreshHasNonReceiptMetadataOnly_AllowsMigration(t *testing.T) {
	fresh := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Kind: "SERVICE",
		Metadata: map[string]string{
			"entrypoint_checksum":   "abc123",
			"proof_on_disk_sha256":  "def456",
			"proof_binary_path":     "/usr/lib/globular/bin/node_agent_server",
		},
	}
	proceed, fallback := shouldMigrateFromSidecar(fresh, "disk-sha")
	if !proceed {
		t.Fatal("non-receipt metadata must NOT block migration")
	}
	if fallback != "" {
		t.Errorf("expected empty fallback when proceeding, got %q", fallback)
	}
}

// TestShouldMigrate_FreshIsNil_SkipsNoOpinion proves the helper backs
// off (no opinion) when the fresh read returns nil. Migration would
// have to invent a new record from stale snapshot evidence; better to
// stay silent and let the next cycle re-read.
func TestShouldMigrate_FreshIsNil_SkipsNoOpinion(t *testing.T) {
	proceed, fallback := shouldMigrateFromSidecar(nil, "disk-sha")
	if proceed {
		t.Fatal("must NOT migrate when fresh is nil")
	}
	if fallback != "" {
		t.Errorf("expected empty fallback for nil fresh, got %q", fallback)
	}
}

// TestShouldMigrate_FreshShaNormalization proves the sha comparison is
// tolerant of mixed-case / whitespace in the fresh receipt.
func TestShouldMigrate_FreshShaNormalization(t *testing.T) {
	const canonSha = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	fresh := &node_agentpb.InstalledPackage{
		Name: "node-agent",
		Kind: "SERVICE",
		Metadata: map[string]string{
			installreceipt.KeyUnitFileSha256: "  " + canonSha + "  ",
		},
	}
	proceed, fallback := shouldMigrateFromSidecar(fresh, canonSha)
	if proceed {
		t.Fatal("must NOT migrate; fresh has receipt")
	}
	if fallback != "" {
		t.Errorf("expected no opinion when shas match (modulo whitespace), got %q", fallback)
	}
}

// TestShouldMigrate_RejectFourKeyShapeReintroduction is the explicit
// regression for the live observation: even if a stale snapshot lacks
// installed_by AND lacks unit_file_sha256 (entirely empty receipt),
// the helper must still refuse migration when fresh etcd has either
// signal. This is the worst case for the wipe — a stale snapshot
// read minutes before any canonical stamp.
func TestShouldMigrate_RejectFourKeyShapeReintroduction(t *testing.T) {
	// Three different fresh shapes that should ALL block migration.
	cases := []struct {
		name string
		fresh *node_agentpb.InstalledPackage
	}{
		{
			"fresh has full canonical receipt",
			&node_agentpb.InstalledPackage{
				Metadata: map[string]string{
					installreceipt.KeyInstalledBy:    "node-agent.installer-api",
					installreceipt.KeyUnitFileSha256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
					installreceipt.KeyBinarySha256:   "111111111111111111111111111111111111111111111111111111111111aaaa",
				},
			},
		},
		{
			"fresh has installed_by only (canonical in flight)",
			&node_agentpb.InstalledPackage{
				Metadata: map[string]string{
					installreceipt.KeyInstalledBy: "node-agent.workflow.package_report_state",
				},
			},
		},
		{
			"fresh has prior legacy_sidecar migration",
			&node_agentpb.InstalledPackage{
				Metadata: map[string]string{
					installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar,
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			proceed, _ := shouldMigrateFromSidecar(c.fresh, "any-disk-sha")
			if proceed {
				t.Errorf("must NOT migrate when fresh has any receipt provenance (%s)", c.name)
			}
		})
	}
}
