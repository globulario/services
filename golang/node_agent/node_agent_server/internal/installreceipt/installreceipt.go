// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.install_receipt
// @awareness file_role=canonical_install_receipt_helpers_shared_between_main_and_actions
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=high
//
// Package installreceipt is the canonical home for install-receipt
// helpers shared between the node_agent_server main package and the
// internal/actions sub-package. It encodes the contract documented in
// docs/architecture/retire-systemd-sidecars.md:
//
//	installed_state.metadata is the SOLE authority for expected
//	installed-output content (unit file, binary, config, env, etc).
//	Sidecar files (<unit>.sha256) are legacy and consumed only as a
//	one-time migration seed by the heartbeat.
//
// Before this package existed, the helpers lived in node_agent_server's
// main package, which the internal/actions sub-package cannot import.
// That left package_state.go's workflow action (packageReportStateAction)
// outside the receipt-preservation perimeter — every workflow report-state
// write could erase a freshly-stamped canonical receipt. Extracting to
// internal/installreceipt closes the loop.
package installreceipt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Receipt metadata keys. Single source of truth — every writer that
// stamps or preserves receipts MUST reference these constants instead
// of hardcoded strings. Doctor rules in cluster_doctor consume the
// metadata via these literal key names; consumer-side string literals
// are aligned with the values here.
const (
	KeyUnitFilePath        = "unit_file_path"
	KeyUnitFileSha256      = "unit_file_sha256"
	KeyBinaryPath          = "binary_path"
	KeyBinarySha256        = "binary_sha256"
	KeyConfigPath          = "config_path"
	KeyConfigSha256        = "config_sha256"
	KeyEnvFilePath         = "env_file_path"
	KeyEnvFileSha256       = "env_file_sha256"
	KeyPackageSha256       = "package_sha256"
	KeyArtifactDigest      = "artifact_digest"
	KeyUnitRendererVersion = "unit_renderer_version"
	KeyInstalledAt         = "installed_at"
	KeyInstalledBy         = "installed_by"
	// KeyMigrationSource is set ONLY by the legacy-sidecar migration
	// path. Production install writers must NEVER write this key; its
	// presence is forensic evidence that the receipt was derived from
	// a pre-refactor sidecar rather than from the install action's
	// first-hand observation.
	KeyMigrationSource = "migration_source"

	// MigrationSourceLegacySidecar is the only legitimate value for
	// KeyMigrationSource; constant-pinned so callers can't typo it.
	MigrationSourceLegacySidecar = "legacy_sidecar"

	// DefaultInstalledBy is the value used when a caller stamps a
	// receipt without an explicit InstalledBy. Useful as the test-time
	// expectation and as a stable identifier for general node-agent
	// writes that aren't from a more-specific code path.
	DefaultInstalledBy = "node-agent"
)

// receiptKeys is the canonical iteration order for preserve/clear
// operations. Exported via the Keys() helper so external readers don't
// have to enumerate the constants themselves.
var receiptKeys = []string{
	KeyUnitFilePath,
	KeyUnitFileSha256,
	KeyBinaryPath,
	KeyBinarySha256,
	KeyConfigPath,
	KeyConfigSha256,
	KeyEnvFilePath,
	KeyEnvFileSha256,
	KeyPackageSha256,
	KeyArtifactDigest,
	KeyUnitRendererVersion,
	KeyInstalledAt,
	KeyInstalledBy,
	KeyMigrationSource,
}

// Keys returns the canonical receipt metadata key list. The returned
// slice is a copy; callers may mutate without affecting other callers.
func Keys() []string {
	out := make([]string, len(receiptKeys))
	copy(out, receiptKeys)
	return out
}

// ReceiptOpts declares the on-disk evidence the install path produced.
//
// Paths are absolute. An empty path means "this kind of evidence is not
// applicable for this install" and the corresponding metadata key is
// omitted — NOT cleared. Missing files at non-empty paths are an error;
// the caller must treat that as install failure rather than commit a
// partial receipt.
type ReceiptOpts struct {
	UnitFilePath        string
	BinaryPath          string
	ConfigPath          string
	EnvFilePath         string
	PackageSha256       string // pre-computed package tarball sha (from manifest)
	ArtifactDigest      string // pre-computed artifact digest (from manifest)
	UnitRendererVersion string // version of the template renderer that produced UnitFilePath
	InstalledBy         string // defaults to DefaultInstalledBy when empty
}

// Stamp records the receipt fields into pkg.Metadata. The caller is
// responsible for then calling installed_state.WriteInstalledPackage to
// commit pkg to etcd. The split lets callers preserve their existing
// read-modify-write semantics for other metadata fields (entrypoint_
// checksum, proof_on_disk_sha256, etc.) without this package knowing
// about them.
//
// Atomicity: if any one hash computation fails, NO fields are written
// and the receipt is rejected. Better fail-closed than commit a partial
// installed_state that the heartbeat could misread.
//
// migration_source clearance: when Stamp succeeds, it deletes any prior
// KeyMigrationSource from pkg.Metadata. The legacy-sidecar marker is
// forensic evidence that the receipt came from a pre-refactor sidecar;
// once a first-hand install observation has been recorded, the marker
// becomes misleading and must go.
//
// The function MUTATES pkg.Metadata in place and never replaces the
// map. Caller-owned keys outside the receipt namespace are preserved.
func Stamp(pkg *node_agentpb.InstalledPackage, opts ReceiptOpts) error {
	if pkg == nil {
		return fmt.Errorf("installreceipt: pkg is nil")
	}

	type kv struct{ k, v string }
	var stamps []kv

	if opts.UnitFilePath != "" {
		sha, err := hashFile(opts.UnitFilePath)
		if err != nil {
			return fmt.Errorf("installreceipt: unit file %q: %w", opts.UnitFilePath, err)
		}
		stamps = append(stamps,
			kv{KeyUnitFilePath, opts.UnitFilePath},
			kv{KeyUnitFileSha256, sha},
		)
	}
	if opts.BinaryPath != "" {
		sha, err := hashFile(opts.BinaryPath)
		if err != nil {
			return fmt.Errorf("installreceipt: binary %q: %w", opts.BinaryPath, err)
		}
		stamps = append(stamps,
			kv{KeyBinaryPath, opts.BinaryPath},
			kv{KeyBinarySha256, sha},
		)
	}
	if opts.ConfigPath != "" {
		sha, err := hashFile(opts.ConfigPath)
		if err != nil {
			return fmt.Errorf("installreceipt: config %q: %w", opts.ConfigPath, err)
		}
		stamps = append(stamps,
			kv{KeyConfigPath, opts.ConfigPath},
			kv{KeyConfigSha256, sha},
		)
	}
	if opts.EnvFilePath != "" {
		sha, err := hashFile(opts.EnvFilePath)
		if err != nil {
			return fmt.Errorf("installreceipt: env file %q: %w", opts.EnvFilePath, err)
		}
		stamps = append(stamps,
			kv{KeyEnvFilePath, opts.EnvFilePath},
			kv{KeyEnvFileSha256, sha},
		)
	}
	if v := normalize(opts.PackageSha256); v != "" {
		stamps = append(stamps, kv{KeyPackageSha256, v})
	}
	if v := normalize(opts.ArtifactDigest); v != "" {
		stamps = append(stamps, kv{KeyArtifactDigest, v})
	}
	if v := strings.TrimSpace(opts.UnitRendererVersion); v != "" {
		stamps = append(stamps, kv{KeyUnitRendererVersion, v})
	}

	installedBy := strings.TrimSpace(opts.InstalledBy)
	if installedBy == "" {
		installedBy = DefaultInstalledBy
	}
	stamps = append(stamps,
		kv{KeyInstalledBy, installedBy},
		kv{KeyInstalledAt, strconv.FormatInt(time.Now().Unix(), 10)},
	)

	if pkg.Metadata == nil {
		pkg.Metadata = make(map[string]string, len(stamps))
	}
	delete(pkg.Metadata, KeyMigrationSource)
	for _, s := range stamps {
		pkg.Metadata[s.k] = s.v
	}
	return nil
}

// StampMigrationFromLegacySidecar records that the unit_file_sha256 was
// seeded from a pre-refactor sidecar. Used exclusively by the heartbeat's
// one-time migration path. After this stamp, the sidecar is never read
// again for this package.
//
// Does NOT clear other receipt keys. Other writers that wrote installed_
// state before migration are preserved; this helper only adds the
// migration marker + unit-file fields.
func StampMigrationFromLegacySidecar(pkg *node_agentpb.InstalledPackage, unitPath, sidecarSha string) {
	if pkg == nil {
		return
	}
	if pkg.Metadata == nil {
		pkg.Metadata = make(map[string]string)
	}
	pkg.Metadata[KeyUnitFilePath] = unitPath
	pkg.Metadata[KeyUnitFileSha256] = normalize(sidecarSha)
	pkg.Metadata[KeyMigrationSource] = MigrationSourceLegacySidecar
	pkg.Metadata[KeyInstalledAt] = strconv.FormatInt(time.Now().Unix(), 10)
}

// Preserve copies install-receipt fields from `existing` into
// `next.Metadata`. Non-install writers (heartbeat refresh, runtime
// proof writer, reconciliation paths, workflow report-state actions)
// MUST call this before installed_state.WriteInstalledPackage so receipt
// fields stamped by the canonical install path are not erased.
//
// Conflict resolution: NEXT wins. If a key is set in both `existing`
// and `next`, the value in `next` is kept. Canonical install writers
// invoke Stamp which populates next.Metadata with fresh values;
// calling this helper afterwards is a no-op for those keys.
//
// migration_source handling: if `next` already carries a non-empty
// installed_by (signalling a canonical install receipt is present),
// migration_source is NOT carried over from `existing`. This implements
// the rule "migration_source is preserved only when no canonical
// install receipt has replaced it yet."
//
// nil-safety: if either argument is nil, the function returns without
// effect. If existing.Metadata is nil there is nothing to preserve.
// If next.Metadata is nil but existing has receipt fields, a new map
// is allocated on next.
func Preserve(existing, next *node_agentpb.InstalledPackage) {
	if existing == nil || next == nil {
		return
	}
	if existing.Metadata == nil {
		return
	}
	// Canonical install detection: if next carries installed_by, a
	// canonical install writer has produced a fresh first-hand
	// receipt. The legacy_sidecar migration marker becomes misleading
	// and MUST NOT be carried over.
	canonicalInstallInNext := false
	if next.Metadata != nil {
		if v := strings.TrimSpace(next.Metadata[KeyInstalledBy]); v != "" {
			canonicalInstallInNext = true
		}
	}

	type kv struct{ k, v string }
	var carry []kv
	for _, k := range receiptKeys {
		if k == KeyMigrationSource && canonicalInstallInNext {
			continue
		}
		if next.Metadata != nil {
			if _, present := next.Metadata[k]; present {
				continue
			}
		}
		if v, ok := existing.Metadata[k]; ok && v != "" {
			carry = append(carry, kv{k, v})
		}
	}
	if len(carry) == 0 {
		return
	}
	if next.Metadata == nil {
		next.Metadata = make(map[string]string, len(carry))
	}
	for _, c := range carry {
		next.Metadata[c.k] = c.v
	}
}

// UnitFileSha256 returns the unit_file_sha256 recorded in the receipt
// for a package, or "" if absent. Heartbeat callers MUST use this
// rather than reading the metadata key directly — keeps the key
// constant private to this package.
func UnitFileSha256(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil || pkg.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(pkg.Metadata[KeyUnitFileSha256])
}

// UnitFilePath returns the unit_file_path recorded in the receipt for
// a package, or "" if absent.
func UnitFilePath(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil || pkg.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(pkg.Metadata[KeyUnitFilePath])
}

// hashFile is a local lightweight sha256 helper. Not using the binhash
// package directly because installreceipt must stay leaf-level
// (binhash imports nothing else from the project; including it here is
// fine, but inlining keeps the dependency graph minimal).
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// normalize lowercases, trims whitespace, and strips a leading "sha256:"
// prefix so receipt comparison is robust across writers.
func normalize(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	return strings.TrimPrefix(v, "sha256:")
}
