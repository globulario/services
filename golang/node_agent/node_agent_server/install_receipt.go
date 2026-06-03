// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.install_receipt
// @awareness file_role=canonical_write_site_for_installed_state_receipt_metadata
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=high
//
// install_receipt.go — canonical write site for the "what did this install
// produce on disk?" proofs that live in installed_state.metadata.
//
// Authority contract:
//
//	The metadata keys defined here (receiptKey*) are the SOLE authority for
//	expected installed output. Sidecar files (/etc/systemd/system/<unit>.sha256)
//	are LEGACY and consumed only as a one-time migration seed by the heartbeat.
//	No code path may treat a sidecar as authoritative, and no NEW code path
//	may invent its own metadata-stamping scheme — every install writer must
//	call StampInstallReceipt before installed_state.WriteInstalledPackage.
//
// Why a chokepoint:
//
//	Pre-refactor, the system used two parallel authorities: installed_state
//	for binary identity (entrypoint_checksum) and `.sha256` sidecars for
//	unit-file integrity. Every install path had to keep both in sync; some
//	paths forgot one or the other, producing chronic hash_drift findings on
//	clusters that were otherwise converged. The chokepoint ensures every
//	writer goes through one function, computes the same set of hashes from
//	live disk evidence, and stamps the result into the canonical etcd
//	receipt. Forgetting to stamp surfaces as installed_state_missing_or
//	_unproven (fail closed) rather than silently leaving false drift.
//
// See docs/architecture/retire-systemd-sidecars.md for the full design.
package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/binhash"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Receipt metadata keys. Single source of truth — no other writer may
// invent keys that overlap with these. Heartbeat readers and doctor rules
// must reference these constants, not the raw strings.
const (
	receiptKeyUnitFilePath        = "unit_file_path"
	receiptKeyUnitFileSha256      = "unit_file_sha256"
	receiptKeyBinaryPath          = "binary_path"
	receiptKeyBinarySha256        = "binary_sha256"
	receiptKeyConfigPath          = "config_path"
	receiptKeyConfigSha256        = "config_sha256"
	receiptKeyEnvFilePath         = "env_file_path"
	receiptKeyEnvFileSha256       = "env_file_sha256"
	receiptKeyPackageSha256       = "package_sha256"
	receiptKeyArtifactDigest      = "artifact_digest"
	receiptKeyUnitRendererVersion = "unit_renderer_version"
	receiptKeyInstalledAt         = "installed_at"
	receiptKeyInstalledBy         = "installed_by"
	// receiptKeyMigrationSource is set ONLY by the legacy-sidecar migration
	// path in checkUnitHashDrift (see server.go). Production install paths
	// must NEVER write this key — its presence is forensic evidence that
	// the receipt was derived from a pre-refactor sidecar rather than from
	// the install action's first-hand observation.
	receiptKeyMigrationSource = "migration_source"
)

// ReceiptOpts declares the on-disk evidence the install path produced.
//
// Paths are absolute. An empty path means "this kind of evidence is not
// applicable for this install" and the corresponding metadata key is
// omitted — NOT cleared. Missing files at non-empty paths are an error;
// the caller must treat that as an install failure rather than commit a
// partial receipt (incomplete receipts cause heartbeat false positives).
type ReceiptOpts struct {
	UnitFilePath        string
	BinaryPath          string
	ConfigPath          string
	EnvFilePath         string
	PackageSha256       string // pre-computed package tarball sha (from manifest)
	ArtifactDigest      string // pre-computed artifact digest (from manifest)
	UnitRendererVersion string // version of the template renderer that produced UnitFilePath
	InstalledBy         string // defaults to "node-agent" when empty
}

// StampInstallReceipt records the receipt fields into pkg.Metadata. The
// caller is responsible for then calling installed_state.WriteInstalledPackage
// to commit pkg to etcd. The split lets callers preserve their existing
// read-modify-write semantics for other metadata fields (entrypoint_checksum,
// proof_on_disk_sha256, etc.) without StampInstallReceipt knowing about them.
//
// Atomicity: if any one hash computation fails, NO fields are written and
// the receipt is rejected. Better fail-closed than commit a partial
// installed_state that the heartbeat could misread.
//
// Idempotency: calling StampInstallReceipt twice with the same options
// produces the same metadata (modulo `installed_at`, which records the
// most recent receipt time). This is intentional — re-stamping on a
// successful re-install correctly advances the receipt.
//
// The function MUTATES pkg.Metadata in place and never replaces the map.
// Caller-owned keys outside the receipt namespace are preserved.
func StampInstallReceipt(pkg *node_agentpb.InstalledPackage, opts ReceiptOpts) error {
	if pkg == nil {
		return fmt.Errorf("install_receipt: pkg is nil")
	}

	// Compute every hash BEFORE mutating pkg.Metadata so partial failures
	// don't leak half-stamped state.
	type kv struct{ k, v string }
	var stamps []kv

	if opts.UnitFilePath != "" {
		sha, err := binhash.Hash(opts.UnitFilePath)
		if err != nil {
			return fmt.Errorf("install_receipt: unit file %q: %w", opts.UnitFilePath, err)
		}
		stamps = append(stamps,
			kv{receiptKeyUnitFilePath, opts.UnitFilePath},
			kv{receiptKeyUnitFileSha256, sha},
		)
	}
	if opts.BinaryPath != "" {
		sha, err := binhash.Hash(opts.BinaryPath)
		if err != nil {
			return fmt.Errorf("install_receipt: binary %q: %w", opts.BinaryPath, err)
		}
		stamps = append(stamps,
			kv{receiptKeyBinaryPath, opts.BinaryPath},
			kv{receiptKeyBinarySha256, sha},
		)
	}
	if opts.ConfigPath != "" {
		sha, err := binhash.Hash(opts.ConfigPath)
		if err != nil {
			return fmt.Errorf("install_receipt: config %q: %w", opts.ConfigPath, err)
		}
		stamps = append(stamps,
			kv{receiptKeyConfigPath, opts.ConfigPath},
			kv{receiptKeyConfigSha256, sha},
		)
	}
	if opts.EnvFilePath != "" {
		sha, err := binhash.Hash(opts.EnvFilePath)
		if err != nil {
			return fmt.Errorf("install_receipt: env file %q: %w", opts.EnvFilePath, err)
		}
		stamps = append(stamps,
			kv{receiptKeyEnvFilePath, opts.EnvFilePath},
			kv{receiptKeyEnvFileSha256, sha},
		)
	}
	if v := strings.TrimSpace(opts.PackageSha256); v != "" {
		stamps = append(stamps, kv{receiptKeyPackageSha256, binhash.Normalize(v)})
	}
	if v := strings.TrimSpace(opts.ArtifactDigest); v != "" {
		stamps = append(stamps, kv{receiptKeyArtifactDigest, binhash.Normalize(v)})
	}
	if v := strings.TrimSpace(opts.UnitRendererVersion); v != "" {
		stamps = append(stamps, kv{receiptKeyUnitRendererVersion, v})
	}

	installedBy := strings.TrimSpace(opts.InstalledBy)
	if installedBy == "" {
		installedBy = "node-agent"
	}
	stamps = append(stamps,
		kv{receiptKeyInstalledBy, installedBy},
		kv{receiptKeyInstalledAt, strconv.FormatInt(time.Now().Unix(), 10)},
	)

	// All hashes computed; commit to metadata.
	if pkg.Metadata == nil {
		pkg.Metadata = make(map[string]string, len(stamps))
	}
	// A fresh receipt SUPERSEDES any prior migration provenance — once the
	// install action has produced a first-hand receipt, the legacy sidecar
	// marker becomes misleading and must be removed.
	delete(pkg.Metadata, receiptKeyMigrationSource)
	for _, s := range stamps {
		pkg.Metadata[s.k] = s.v
	}
	return nil
}

// receiptUnitFileSha256 returns the unit_file_sha256 recorded in the
// receipt for a package, or "" if absent. Heartbeat callers MUST use this
// rather than reading the metadata key directly — keeps the key constant
// private to this file.
func receiptUnitFileSha256(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil || pkg.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(pkg.Metadata[receiptKeyUnitFileSha256])
}

// receiptUnitFilePath returns the unit_file_path recorded in the receipt
// for a package, or "" if absent. Used by the heartbeat to locate the
// authoritative unit file when computing live drift state.
func receiptUnitFilePath(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil || pkg.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(pkg.Metadata[receiptKeyUnitFilePath])
}

// stampMigrationFromLegacySidecar records that the unit_file_sha256 was
// seeded from a pre-refactor sidecar rather than from a first-hand install
// observation. Used exclusively by the heartbeat's one-time migration
// path. After stamping, the sidecar will never be consulted again for
// this package.
func stampMigrationFromLegacySidecar(pkg *node_agentpb.InstalledPackage, unitPath, sidecarSha string) {
	if pkg == nil {
		return
	}
	if pkg.Metadata == nil {
		pkg.Metadata = make(map[string]string)
	}
	pkg.Metadata[receiptKeyUnitFilePath] = unitPath
	pkg.Metadata[receiptKeyUnitFileSha256] = binhash.Normalize(sidecarSha)
	pkg.Metadata[receiptKeyMigrationSource] = "legacy_sidecar"
	pkg.Metadata[receiptKeyInstalledAt] = strconv.FormatInt(time.Now().Unix(), 10)
}
