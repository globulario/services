// @awareness namespace=globular.platform
// @awareness component=platform_repository.release_ledger
// @awareness file_role=per_package_release_ledger_in_scylla_with_minio_json_fallback_for_monotonic_version_authority
// @awareness implements=globular.platform:intent.repository.metadata_is_authority
// @awareness implements=globular.platform:intent.repository.version_allocation_is_exclusive
// @awareness implements=globular.platform:intent.repository.resolver_is_deterministic_or_errors
// @awareness enforces=globular.platform:invariant.repository.desired_build_id_must_resolve
// @awareness risk=critical
// @awareness failure_mode=repository.ledger_authority_inverted_minio_gates_scylla
package main

// release_ledger.go — durable record of every PUBLISHED release
// for a package. Enforces version MONOTONICITY (new RELEASED >
// latest) and provides deterministic version → build_id
// resolution.
//
// Two-tier storage: ScyllaDB (consistent, distributed) primary,
// MinIO JSON fallback when Scylla is unavailable. Reads MUST
// prefer Scylla — using MinIO as authority would let a stale
// snapshot win when Scylla recovers.
//
// Written on every promote-to-PUBLISHED transition; read by the
// release resolver for latest-version queries. Any code path
// that writes PUBLISHED state but skips the ledger write
// silently breaks rollback (rollback.go consults the ledger to
// validate candidate versions).

// release_ledger.go — Per-package release ledger.
//
// The release ledger is the persistent record of all PUBLISHED releases for a
// package. It provides:
//
//   - O(1) latest-release lookup (no directory scanning)
//   - Monotonic version enforcement (new RELEASED version must be > latest)
//   - Deterministic version → build_id resolution
//
// Storage: ScyllaDB table `repository.release_ledger` (distributed, consistent).
// Fallback: local POSIX JSON file at `ledger/{publisher}%{name}.json` when
// ScyllaDB is unavailable. (Packages never live in MinIO — the secondary copy
// is the local POSIX CAS, not a shared object store.)
//
// The ledger is written on every successful promote-to-PUBLISHED transition
// and read by the release resolver for latest-version queries.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"github.com/gocql/gocql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure repopb is used (for migration).
var _ = repopb.PublishState_PUBLISHED

// releaseLedgerEntry represents a single published release in the ledger.
type releaseLedgerEntry struct {
	Version    string `json:"version"`
	BuildID    string `json:"build_id"`
	Digest     string `json:"digest"`
	Platform   string `json:"platform"`
	SizeBytes  int64  `json:"size_bytes"`
	ReleasedAt string `json:"released_at"`
}

// releaseLedger is the per-package release history.
type releaseLedger struct {
	Publisher     string                `json:"publisher"`
	Name          string                `json:"name"`
	LatestVersion string                `json:"latest_version"`
	LatestBuildID string                `json:"latest_build_id"`
	Releases      []*releaseLedgerEntry `json:"releases"`
}

// ledgerStorageKey returns the MinIO key for a package's ledger.
func ledgerStorageKey(publisher, name string) string {
	return fmt.Sprintf("ledger/%s%%%s.json", publisher, name)
}

// ── Ledger read/write ───────────────────────────────────────────────────

// readLedger loads the release ledger for a package. Returns nil if no ledger
// exists (package has never been PUBLISHED).
func (srv *server) readLedger(ctx context.Context, publisher, name string) *releaseLedger {
	// Try ScyllaDB first.
	if srv.scylla != nil {
		if ledger := srv.readLedgerFromScylla(ctx, publisher, name); ledger != nil {
			return ledger
		}
	}

	// Fallback: local POSIX store.
	key := ledgerStorageKey(publisher, name)
	data, err := srv.Storage().ReadFile(ctx, key)
	if err != nil {
		return nil
	}
	var ledger releaseLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		slog.Warn("ledger: corrupt JSON", "publisher", publisher, "name", name, "err", err)
		return nil
	}
	return &ledger
}

// writeLedger persists the release ledger. Writes Scylla first (authoritative),
// then MinIO (mirror). Scylla failure is fatal — the ledger is not updated.
// MinIO failure is logged as WARNING but does not block the ledger append.
func (srv *server) writeLedger(ctx context.Context, ledger *releaseLedger) error {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ledger: %w", err)
	}

	// Write to ScyllaDB first — authoritative distributed ledger. If this fails
	// the caller must not treat the ledger as updated; meta.state_mutations_must_be_durably_committed_before_side_effects.
	if srv.scylla != nil {
		if err := srv.writeLedgerToScyllaErr(ctx, ledger, data); err != nil {
			return fmt.Errorf("write ledger to scylla (authoritative): %w", err)
		}
	}

	// Write to the local POSIX store — best-effort secondary for degraded /
	// single-node reads. Failure is non-fatal: Scylla already has the
	// authoritative ledger. (Never MinIO — packages and their ledgers live only
	// in the local POSIX CAS.)
	key := ledgerStorageKey(ledger.Publisher, ledger.Name)
	if err := srv.Storage().WriteFile(ctx, key, data, 0o644); err != nil {
		slog.Warn("ledger: local POSIX secondary write failed (scylla is authoritative, ledger is committed)",
			"publisher", ledger.Publisher, "name", ledger.Name, "err", err)
	}

	return nil
}

// ── ScyllaDB ledger operations ──────────────────────────────────────────

func (srv *server) readLedgerFromScylla(ctx context.Context, publisher, name string) *releaseLedger {
	if srv.scylla == nil {
		return nil
	}
	row, err := srv.scylla.GetManifest(ctx, fmt.Sprintf("ledger/%s/%s", publisher, name))
	if err != nil {
		// Distinguish "row does not exist" from a real storage error.
		// gocql.ErrNotFound means the ledger has never been written — treat as
		// an empty ledger (nil) without logging. Any other error is a genuine
		// store failure; log it so operators see transient Scylla failures.
		if !isGocqlNotFound(err) {
			slog.Warn("ledger: scylla read error (falling back to minio)",
				"publisher", publisher, "name", name, "err", err)
		}
		return nil
	}
	if row == nil {
		return nil
	}
	var ledger releaseLedger
	if err := json.Unmarshal(row.ManifestJSON, &ledger); err != nil {
		slog.Warn("ledger: corrupt JSON in scylla row",
			"publisher", publisher, "name", name, "err", err)
		return nil
	}
	return &ledger
}

// isGocqlNotFound returns true when err is gocql's "no rows returned" sentinel.
func isGocqlNotFound(err error) bool {
	return errors.Is(err, gocql.ErrNotFound)
}

// writeLedgerToScyllaErr writes the ledger row to Scylla and returns any error.
// Called by writeLedger as the authoritative first write. Errors here are fatal
// for the ledger append — the caller must not proceed.
func (srv *server) writeLedgerToScyllaErr(ctx context.Context, ledger *releaseLedger, data []byte) error {
	if srv.scylla == nil {
		return nil
	}
	row := manifestRow{
		ArtifactKey:  fmt.Sprintf("ledger/%s/%s", ledger.Publisher, ledger.Name),
		ManifestJSON: data,
		PublishState: "LEDGER",
		PublisherID:  ledger.Publisher,
		Name:         ledger.Name,
		Version:      ledger.LatestVersion,
		BuildNumber:  0,
		CreatedAt:    time.Now(),
	}
	return srv.scylla.PutManifest(ctx, row)
}

// ── Ledger operations ───────────────────────────────────────────────────

// ledgerMu serializes ledger writes to prevent concurrent modification.
var ledgerMu sync.Mutex

// AppendToLedgerOpts carries optional behaviour switches for appendToLedger.
//
// RecoveryArtifactKey, when non-empty, asserts that the caller is completing
// a previously-partial import currently sitting at artifact_state =
// MANIFEST_WRITTEN. When the assertion is verified (state lookup confirms
// MANIFEST_WRITTEN), the monotonicity check is BYPASSED so that an older
// version that's already partially imported can finish its promotion to
// PUBLISHED even after a newer version was published in the meantime.
//
// All other ledger gates remain in force:
//   - version+platform immutability (no duplicate PUBLISHED identity)
//   - idempotent re-promote (same build_id already in ledger → no-op)
//   - LatestVersion/LatestBuildID anchor only advances on cmp > 0
//     (an older recovery never bumps "latest")
//
// If the state lookup returns anything other than MANIFEST_WRITTEN
// (PUBLISHED, REVOKED, QUARANTINED, BROKEN_*, UNSPECIFIED), the recovery
// claim is rejected silently and the normal monotonicity check applies —
// fail-closed by construction. This is what prevents the recovery path
// from being abused to publish a brand-new older version below latest:
// such a publish has no MANIFEST_WRITTEN row at the asserted key, so the
// bypass simply doesn't engage.
type AppendToLedgerOpts struct {
	RecoveryArtifactKey string
}

// appendToLedger adds a new release entry to the package's ledger.
// Called after successful promote-to-PUBLISHED.
// Returns an error if the version is not monotonically increasing.
//
// `repair` is the Phase 33 repair authorization. A nil value means no repair
// was requested — version+platform immutability is enforced absolutely (the
// default and correct behaviour for normal publishes). A non-nil + valid
// repair authorization is the ONLY way to legitimately REPLACE an existing
// ledger entry's build_id/digest/sizeBytes under the same (version, platform);
// this is the THIRD immutability gate after the seal (Phase 31) and the
// version resolver (Phase 32) that must be repair-aware for the repair lane
// to be resolver-visible end-to-end. See repair_authorization.go.
//
// `opts` (variadic, at most one element honoured) lets the sync retry path
// signal that this is a recovery of a stuck partial import. See
// AppendToLedgerOpts for the verification + bypass scope.
func (srv *server) appendToLedger(ctx context.Context, publisher, name, version, buildID, digest, platform string, sizeBytes int64, repair *RepairAuthorization, opts ...AppendToLedgerOpts) error {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()

	var recoveryKey string
	if len(opts) > 0 {
		recoveryKey = strings.TrimSpace(opts[0].RecoveryArtifactKey)
	}

	ledger := srv.readLedger(ctx, publisher, name)
	if ledger == nil {
		ledger = &releaseLedger{
			Publisher: publisher,
			Name:      name,
		}
	}

	// Verified-recovery detection: the caller asserted this is a completion
	// of a partial import. Trust the assertion ONLY when artifact_state for
	// the asserted key is exactly MANIFEST_WRITTEN. Any other state (or
	// missing row) falls through to the normal monotonicity check below.
	verifiedRecovery := false
	if recoveryKey != "" {
		if srv.readArtifactState(ctx, recoveryKey) == PipelineManifestWritten {
			verifiedRecovery = true
			slog.Info("release ledger: verified recovery of stuck partial import — bypassing monotonicity check",
				"publisher", publisher, "name", name, "version", version,
				"build_id", buildID, "artifact_key", recoveryKey,
				"latest_published_version", ledger.LatestVersion,
			)
		}
	}

	// Monotonic version enforcement: new version must be > latest.
	// Skipped for verified recoveries (see verifiedRecovery above). The
	// version+platform immutability check below still applies, so we cannot
	// create a duplicate PUBLISHED identity even on the recovery path.
	if !verifiedRecovery && ledger.LatestVersion != "" && version != ledger.LatestVersion {
		cmp, err := versionutil.Compare(version, ledger.LatestVersion)
		if err == nil && cmp < 0 {
			return fmt.Errorf("non-monotonic version: %s < latest %s for %s/%s",
				version, ledger.LatestVersion, publisher, name)
		}
	}

	// Version+platform immutability: once (version, platform) is in the ledger,
	// it is permanently bound to its first build_id. Re-publishing the same
	// (version, platform) under a new build_id creates duplicate artifacts that
	// cause build-id drift across the 4 layers — desired state updates to the
	// new build_id while nodes still carry the old one.
	//
	// Exceptions:
	//   1. True idempotent re-promote: same build_id already in ledger.
	//   2. Repair-unseal (Phase 33): same (version, platform) with a DIFFERENT
	//      build_id, but the caller presents a valid RepairAuthorization with
	//      a prior-digest that matches the existing ledger row's digest. In
	//      this case the ledger entry is REPLACED in place — version+platform
	//      binding preserved, build_id/digest/size point at the new bytes.
	//      The old build_number's manifest+blob remain in storage for
	//      forensics; only the ledger pointer flips.
	for i, r := range ledger.Releases {
		if r.BuildID == buildID {
			return nil // idempotent re-promote: exact same build already in ledger
		}
		if r.Version == version && r.Platform == platform {
			// Repair-unseal escape hatch.
			if repair != nil && repair.Requested {
				if strings.TrimSpace(repair.Reason) == "" {
					return status.Errorf(codes.InvalidArgument,
						"repair-unseal rejected at ledger gate: empty reason — provide --reason \"<why>\" describing why the published %s/%s@%s on %s is being repaired",
						publisher, name, version, platform)
				}
				if !digestsMatch(repair.PriorDigest, r.Digest) {
					return status.Errorf(codes.FailedPrecondition,
						"repair-unseal rejected at ledger gate: prior-digest mismatch — caller asserted prior=%s but the ledger row has %s. "+
							"Inspect via repository_explain_artifact and re-issue with the correct --prior-digest.",
						shortDigest(repair.PriorDigest), shortDigest(r.Digest))
				}
				// All gates passed. REPLACE the entry in place — preserving
				// version+platform binding while flipping build_id/digest/size
				// to the new bytes. This is the resolver-visible step: from
				// this moment on, getExactRelease and getPublishedDigest
				// return the new identity rather than the phantom.
				priorBuildID := r.BuildID
				priorDigest := r.Digest
				slog.Warn("release ledger repair authorized — replacing entry",
					"publisher", publisher, "name", name, "version", version, "platform", platform,
					"prior_build_id", priorBuildID, "new_build_id", buildID,
					"prior_digest", priorDigest, "new_digest", digest,
					"reason", repair.Reason,
				)
				repair.Used = true
				if repair.PriorBuildID == "" {
					repair.PriorBuildID = priorBuildID
				}
				ledger.Releases[i] = &releaseLedgerEntry{
					Version:    version,
					BuildID:    buildID,
					Digest:     digest,
					Platform:   platform,
					SizeBytes:  sizeBytes,
					ReleasedAt: time.Now().UTC().Format(time.RFC3339),
				}
				// If this entry was the LatestBuildID anchor, point it at
				// the new build_id so callers reading the ledger summary
				// see the new identity.
				if ledger.LatestBuildID == priorBuildID {
					ledger.LatestBuildID = buildID
				}
				return srv.writeLedger(ctx, ledger)
			}
			return fmt.Errorf("version %s is already published for %s/%s on %s (build_id=%s) — published versions are immutable; bump the version to release a new build, "+
				"or pass --unseal-official --reason \"<why>\" --prior-digest %s to repair a proven phantom",
				version, publisher, name, platform, r.BuildID, shortDigest(r.Digest))
		}
	}

	entry := &releaseLedgerEntry{
		Version:    version,
		BuildID:    buildID,
		Digest:     digest,
		Platform:   platform,
		SizeBytes:  sizeBytes,
		ReleasedAt: time.Now().UTC().Format(time.RFC3339),
	}
	ledger.Releases = append(ledger.Releases, entry)

	// Update latest only when strictly newer — same-version re-publish is now
	// rejected above, so cmp == 0 here means a different platform; don't
	// overwrite the canonical LatestBuildID with a platform-specific one.
	if ledger.LatestVersion == "" {
		ledger.LatestVersion = version
		ledger.LatestBuildID = buildID
	} else {
		cmp, err := versionutil.Compare(version, ledger.LatestVersion)
		if err == nil && cmp > 0 {
			ledger.LatestVersion = version
			ledger.LatestBuildID = buildID
		}
	}

	return srv.writeLedger(ctx, ledger)
}

// getExactRelease returns the build_id for an exact (name, version, platform)
// tuple in the PUBLISHED ledger, or "" if not found. Used by AllocateUpload to
// enforce version immutability — if a build_id is returned, the version is
// already published and cannot be re-allocated.
func (srv *server) getExactRelease(ctx context.Context, publisher, name, version, platform string) string {
	ledger := srv.readLedger(ctx, publisher, name)
	if ledger == nil {
		return ""
	}
	for _, r := range ledger.Releases {
		if (r.Platform == platform || platform == "") && r.Version == version {
			return r.BuildID
		}
	}
	return ""
}

// getLatestRelease returns the latest PUBLISHED build_id for a package on a
// specific platform. Returns ("", "") if no release exists.
//
// Selects by maximum SemVer version (via versionutil.Compare), NOT by
// insertion order. Pre-recovery-bypass the two were the same because the
// monotonicity check guaranteed insertion-order == version-order. The
// MANIFEST_WRITTEN recovery path (see AppendToLedgerOpts) can append an
// older version AFTER a newer one, so insertion-order-reverse would be
// wrong. The explicit ledger.LatestVersion anchor remains authoritative;
// this function uses Compare to honour it across platform-filtered queries
// (where the anchor's platform may differ from the caller's).
func (srv *server) getLatestRelease(ctx context.Context, publisher, name, platform string) (version, buildID string) {
	ledger := srv.readLedger(ctx, publisher, name)
	if ledger == nil {
		return "", ""
	}

	for _, r := range ledger.Releases {
		if r.Platform != platform && platform != "" {
			continue
		}
		if version == "" {
			version, buildID = r.Version, r.BuildID
			continue
		}
		cmp, err := versionutil.Compare(r.Version, version)
		if err == nil && cmp > 0 {
			version, buildID = r.Version, r.BuildID
		}
	}
	return version, buildID
}

// ── Ledger migration ────────────────────────────────────────────────────

const ledgerMigrationMarker = "ledger/.migration-complete"

// MigrateReleaseLedger builds the release ledger from existing PUBLISHED
// artifacts. Idempotent — skips if marker exists.
func (srv *server) MigrateReleaseLedger(ctx context.Context) {
	if _, err := srv.Storage().ReadFile(ctx, ledgerMigrationMarker); err == nil {
		slog.Debug("release ledger migration already complete")
		return
	}

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		slog.Debug("no artifacts directory, skipping ledger migration")
		return
	}

	built := 0
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil || m == nil {
			continue
		}
		if state != repopb.PublishState_PUBLISHED {
			continue
		}
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		buildID := m.GetBuildId()
		if buildID == "" {
			continue // can't add to ledger without build_id
		}

		err := srv.appendToLedger(ctx, ref.GetPublisherId(), ref.GetName(),
			ref.GetVersion(), buildID, m.GetChecksum(),
			ref.GetPlatform(), m.GetSizeBytes(), nil)
		if err != nil {
			slog.Warn("ledger migration: skip", "key", key, "err", err)
			continue
		}
		built++
	}

	// Write marker.
	marker, _ := json.Marshal(map[string]any{
		"migrated_at": time.Now().UTC().Format(time.RFC3339),
		"entries":     built,
	})
	_ = srv.Storage().WriteFile(ctx, ledgerMigrationMarker, marker, 0o644)

	slog.Info("release ledger migration complete", "entries", built)
}
