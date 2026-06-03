// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.install_receipt
// @awareness file_role=main_package_install_receipt_glue
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=high
//
// install_receipt.go — main-package glue around the install-receipt
// helpers that now live in internal/installreceipt. This file holds:
//
//   - Re-exported aliases for the keys/types so existing main-package
//     callers (apply_package_release.go, heartbeat.go, server.go,
//     installer_api.go, self_hosted_runtime_proof_writer.go,
//     process_fingerprint.go, minio_systemd_reconcile.go) compile
//     unchanged.
//   - stampReceiptForInstalledPackage, the main-package helper that
//     derives unit/binary paths from package conventions using
//     installedBinaryPath() (a main-package symbol that the sub-package
//     cannot import).
//
// The actual receipt logic — Stamp, Preserve, StampMigrationFrom
// LegacySidecar, key constants — lives in the sub-package so that
// internal/actions/package_state.go can also wire through the same
// chokepoint without duplicating helpers. See
// docs/architecture/retire-systemd-sidecars.md.
package main

import (
	"log"
	"os"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ── Re-exports (back-compat for existing main-package callers) ─────────────

// ReceiptOpts is an alias for installreceipt.ReceiptOpts.
type ReceiptOpts = installreceipt.ReceiptOpts

// StampInstallReceipt delegates to installreceipt.Stamp.
func StampInstallReceipt(pkg *node_agentpb.InstalledPackage, opts ReceiptOpts) error {
	return installreceipt.Stamp(pkg, opts)
}

// PreserveInstallReceiptMetadata delegates to installreceipt.Preserve.
func PreserveInstallReceiptMetadata(existing, next *node_agentpb.InstalledPackage) {
	installreceipt.Preserve(existing, next)
}

// stampMigrationFromLegacySidecar delegates to
// installreceipt.StampMigrationFromLegacySidecar.
func stampMigrationFromLegacySidecar(pkg *node_agentpb.InstalledPackage, unitPath, sidecarSha string) {
	installreceipt.StampMigrationFromLegacySidecar(pkg, unitPath, sidecarSha)
}

// receiptUnitFileSha256 delegates to installreceipt.UnitFileSha256.
func receiptUnitFileSha256(pkg *node_agentpb.InstalledPackage) string {
	return installreceipt.UnitFileSha256(pkg)
}

// receiptUnitFilePath delegates to installreceipt.UnitFilePath.
func receiptUnitFilePath(pkg *node_agentpb.InstalledPackage) string {
	return installreceipt.UnitFilePath(pkg)
}

// Receipt-key constants — re-exported under their original names so
// existing test files compile without rewrites. New code should prefer
// the installreceipt.Key* names.
const (
	receiptKeyUnitFilePath        = installreceipt.KeyUnitFilePath
	receiptKeyUnitFileSha256      = installreceipt.KeyUnitFileSha256
	receiptKeyBinaryPath          = installreceipt.KeyBinaryPath
	receiptKeyBinarySha256        = installreceipt.KeyBinarySha256
	receiptKeyConfigPath          = installreceipt.KeyConfigPath
	receiptKeyConfigSha256        = installreceipt.KeyConfigSha256
	receiptKeyEnvFilePath         = installreceipt.KeyEnvFilePath
	receiptKeyEnvFileSha256       = installreceipt.KeyEnvFileSha256
	receiptKeyPackageSha256       = installreceipt.KeyPackageSha256
	receiptKeyArtifactDigest      = installreceipt.KeyArtifactDigest
	receiptKeyUnitRendererVersion = installreceipt.KeyUnitRendererVersion
	receiptKeyInstalledAt         = installreceipt.KeyInstalledAt
	receiptKeyInstalledBy         = installreceipt.KeyInstalledBy
	receiptKeyMigrationSource     = installreceipt.KeyMigrationSource
)

// receiptMetadataKeys exposed for the existing test that asserts the
// canonical key set. New code should use installreceipt.Keys().
var receiptMetadataKeys = installreceipt.Keys()

// ── Main-package-only helper ──────────────────────────────────────────────

// stampReceiptForInstalledPackage is the helper every install-complete
// site in node-agent must call before installed_state.WriteInstalledPackage.
// It derives unit/binary paths from package conventions and delegates
// to installreceipt.Stamp.
//
// Conventions:
//
//	unit file path : /etc/systemd/system/globular-<pkg.Name>.service
//	binary path    : installedBinaryPath(pkg.Name, pkg.Kind)
//	package digest : pkg.Checksum (when non-empty)
//
// Missing files at conventional paths are silently skipped (a COMMAND
// package may have no systemd unit; an INFRASTRUCTURE wrapper may have
// no binary in /usr/lib/globular/bin). The chokepoint's atomicity rule
// only fires on declared-but-unreadable paths.
//
// Lives in the main package (not in installreceipt) because it uses
// installedBinaryPath() from apply_package_release.go, which the
// internal/actions sub-package and the installreceipt sub-package
// cannot import.
func stampReceiptForInstalledPackage(pkg *node_agentpb.InstalledPackage, installedBy string, binPath string) {
	if pkg == nil || pkg.GetName() == "" {
		return
	}
	opts := installreceipt.ReceiptOpts{
		InstalledBy:    installedBy,
		PackageSha256:  pkg.GetChecksum(),
		ArtifactDigest: pkg.GetChecksum(),
	}
	unitPath := "/etc/systemd/system/globular-" + pkg.GetName() + ".service"
	if fi, err := os.Stat(unitPath); err == nil && !fi.IsDir() {
		opts.UnitFilePath = unitPath
	}
	if binPath != "" {
		if fi, err := os.Stat(binPath); err == nil && !fi.IsDir() {
			opts.BinaryPath = binPath
		}
	}
	if err := installreceipt.Stamp(pkg, opts); err != nil {
		log.Printf("install_receipt: receipt skipped for %s/%s: %v", pkg.GetKind(), pkg.GetName(), err)
	}
}
