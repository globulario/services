// @awareness namespace=globular.platform
// @awareness component=platform_repository.publish_reconciler
// @awareness file_role=background_retry_loop_promoting_stuck_verified_artifacts_to_published_with_bounded_attempts
// @awareness implements=globular.platform:intent.repository.publish_pipeline_is_ordered
// @awareness implements=globular.platform:intent.repository.lifecycle_state_machine
// @awareness enforces=globular.platform:invariant.repository.artifact.state_transitions_are_forward_only
// @awareness risk=high
package main

// publish_reconciler.go — closes the BLOB_VERIFIED stuck class.
// completePublish in UploadArtifact can fail (e.g. MinIO blip,
// network glitch) leaving an artifact in VERIFIED while the
// caller-side error path looks clean. This reconciler scans
// VERIFIED rows periodically and retries promotion, bounded by
// a per-artifact retry limit so a permanently-broken row does
// not loop forever.
//
// MUST keep the retry bound. Unbounded retries would hide a
// genuine completePublish failure behind "the reconciler keeps
// trying"; the bound forces escalation as a doctor finding so
// operators see what's stuck.

// publish_reconciler.go — Background goroutine that retries stuck VERIFIED artifacts.
//
// If completePublish fails during UploadArtifact, the artifact stays in VERIFIED
// state. This reconciler periodically scans for such artifacts and retries
// promotion, bounded by a per-artifact retry limit.

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

const (
	publishReconcileInterval = 60 * time.Second
	publishRetryThreshold    = 30 * time.Second // artifact must be older than this to retry
	publishMaxRetries        = 3
)

// publishReconciler retries promotion for artifacts stuck in VERIFIED state.
type publishReconciler struct {
	srv      *server
	interval time.Duration

	mu      sync.Mutex
	retries map[string]int // artifact key → retry count
}

func newPublishReconciler(srv *server) *publishReconciler {
	return &publishReconciler{
		srv:      srv,
		interval: publishReconcileInterval,
		retries:  make(map[string]int),
	}
}

// Start launches the reconciler as a background goroutine.
func (pr *publishReconciler) Start(ctx context.Context) {
	slog.Info("publish-reconciler: started", "interval", pr.interval,
		"sweeps", "VERIFIED→PUBLISHED retry + PUBLISHED_MISSING_BLOB mirror-heal")
	go func() {
		ticker := time.NewTicker(pr.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pr.reconcileOnce(ctx)
				pr.healPublishedMissingBlobsOnce(ctx)
			}
		}
	}()
}

func (pr *publishReconciler) reconcileOnce(ctx context.Context) {
	now := time.Now()
	verified := 0

	// Scylla-first: use ledger rows to find stuck VERIFIED artifacts.
	// Falls back to MinIO directory scan only when Scylla is nil.
	type candidate struct {
		key      string
		manifest *repopb.ArtifactManifest
	}
	var candidates []candidate

	if pr.srv.scylla != nil {
		rows, err := pr.srv.scylla.ListManifests(ctx)
		if err != nil {
			slog.Debug("publish-reconciler: Scylla list failed", "err", err)
			return
		}
		for _, row := range rows {
			if row.PublishState != repopb.PublishState_VERIFIED.String() {
				continue
			}
			m, _, parseErr := manifestFromRow(row)
			if parseErr != nil {
				continue
			}
			candidates = append(candidates, candidate{key: row.ArtifactKey, manifest: m})
		}
	} else {
		entries, err := pr.srv.Storage().ReadDir(ctx, artifactsDir)
		if err != nil {
			slog.Debug("publish-reconciler: ReadDir failed", "err", err)
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".manifest.json") {
				continue
			}
			key := strings.TrimSuffix(name, ".manifest.json")
			_, state, manifest, err := pr.srv.readManifestAndStateByKey(ctx, key)
			if err != nil || state != repopb.PublishState_VERIFIED {
				continue
			}
			candidates = append(candidates, candidate{key: key, manifest: manifest})
		}
	}

	for _, c := range candidates {
		verified++
		manifest := c.manifest

		// Only retry artifacts that have been stuck for longer than the threshold.
		modifiedAt := time.Unix(manifest.GetModifiedUnix(), 0)
		if modifiedAt.IsZero() || now.Sub(modifiedAt) < publishRetryThreshold {
			continue
		}

		// Check retry limit.
		pr.mu.Lock()
		count := pr.retries[c.key]
		if count >= publishMaxRetries {
			pr.mu.Unlock()
			continue
		}
		pr.retries[c.key] = count + 1
		pr.mu.Unlock()

		ref := manifest.GetRef()
		slog.Info("publish-reconciler: retrying stuck VERIFIED artifact",
			"key", c.key,
			"publisher", ref.GetPublisherId(),
			"name", ref.GetName(),
			"version", ref.GetVersion(),
			"attempt", count+1,
		)

		if err := pr.srv.completePublish(ctx, manifest, c.key, nil, nil); err != nil {
			slog.Error("publish-reconciler: retry failed",
				"key", c.key,
				"publisher", ref.GetPublisherId(),
				"name", ref.GetName(),
				"version", ref.GetVersion(),
				"attempt", count+1,
				"err", err,
			)
		} else {
			slog.Info("publish-reconciler: artifact promoted to PUBLISHED",
				"key", c.key,
				"name", ref.GetName(),
				"version", ref.GetVersion(),
			)
			pr.mu.Lock()
			delete(pr.retries, c.key)
			pr.mu.Unlock()
		}
	}

	if verified > 0 {
		slog.Info("publish-reconciler: scan complete", "verified", verified)
	}
}

// healPublishedMissingBlobsOnce runs a separate sweep that heals the
// PUBLISHED_MISSING_BLOB split-brain: an artifact whose Scylla manifest
// shows publish_state=PUBLISHED but whose local POSIX CAS does not have
// the binary blob. The doctor finding
// repository.identity.missing_blob_for_published_manifest fires for these
// rows; the install pipeline cannot serve them.
//
// Healing policy (strict):
//
//   - Skip if local POSIX is already present (Stat succeeds). No work needed.
//   - Skip if no MinIO mirror is configured. We cannot manufacture bytes.
//   - Pull bytes from the mirror, recompute sha256, COMPARE against the
//     manifest's expected checksum. ONLY if the digest matches do we write
//     to local POSIX (atomic temp+rename). A checksum-mismatched mirror
//     blob is reported as a separate finding and never trusted.
//   - Preserve the architectural rule: MinIO is informational; the local
//     POSIX CAS remains the install-time authority. The mirror is used here
//     only as a recovery source under explicit cryptographic verification.
//
// Cases this sweep does NOT heal (left for operator action):
//
//   - Local POSIX missing + mirror missing: nothing to restore from. Doctor
//     keeps reporting MISSING_BLOB; operator must republish the artifact.
//   - Local POSIX missing + mirror checksum mismatches manifest: mirror is
//     poisoned. We refuse to copy and emit a slog.Error so operators see
//     the corruption rather than silently trusting it.
//   - publish_state != PUBLISHED: handled by reconcileOnce's VERIFIED retry
//     loop; the two sweeps are intentionally separate.
func (pr *publishReconciler) healPublishedMissingBlobsOnce(ctx context.Context) {
	if pr.srv.scylla == nil {
		// Cannot enumerate PUBLISHED set without Scylla; nothing to heal.
		return
	}
	if pr.srv.localStorage == nil {
		// No local CAS means we have no place to write recovered bytes.
		return
	}

	rows, err := pr.srv.scylla.ListManifests(ctx)
	if err != nil {
		slog.Debug("publish-reconciler.heal: Scylla list failed", "err", err)
		return
	}

	scanned := 0
	healed := 0
	skippedNoMirror := 0
	skippedChecksumMismatch := 0
	skippedNoMirrorBlob := 0
	for _, row := range rows {
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			continue
		}
		scanned++

		manifest, _, parseErr := manifestFromRow(row)
		if parseErr != nil || manifest == nil {
			continue
		}
		binKey := binaryStorageKey(row.ArtifactKey)

		// Skip if local is already present — no work needed.
		if _, statErr := pr.srv.localStorage.Stat(ctx, binKey); statErr == nil {
			continue
		}

		// Local POSIX missing. Try the mirror as a recovery source — but only
		// trust it under explicit checksum match.
		mirror := pr.srv.mirrorStorage
		if mirror == nil {
			skippedNoMirror++
			slog.Warn("publish-reconciler.heal: PUBLISHED blob missing locally and no mirror configured — operator republish required",
				"key", row.ArtifactKey,
				"publisher", manifest.GetRef().GetPublisherId(),
				"name", manifest.GetRef().GetName(),
				"version", manifest.GetRef().GetVersion(),
			)
			continue
		}
		data, mirrorErr := mirror.ReadFile(ctx, binKey)
		if mirrorErr != nil {
			skippedNoMirrorBlob++
			slog.Warn("publish-reconciler.heal: PUBLISHED blob missing locally and mirror read failed — operator republish required",
				"key", row.ArtifactKey,
				"name", manifest.GetRef().GetName(),
				"version", manifest.GetRef().GetVersion(),
				"err", mirrorErr,
			)
			continue
		}

		expected := manifest.GetChecksum()
		if expected == "" {
			// No expected checksum to verify against — refuse to write.
			// Trusting a mirror blob without a manifest-declared digest would
			// violate the invariant that mirror is informational only.
			skippedChecksumMismatch++
			slog.Error("publish-reconciler.heal: refusing to copy mirror blob — manifest carries no expected checksum",
				"key", row.ArtifactKey,
				"name", manifest.GetRef().GetName(),
				"version", manifest.GetRef().GetVersion(),
			)
			continue
		}

		// Use WriteFileAtomic with the expected checksum so the mirror bytes
		// are verified inside the write helper. A digest mismatch returns
		// error and the temp file is removed automatically.
		actual, writeErr := pr.srv.localStorage.WriteFileAtomic(ctx, binKey,
			bytes.NewReader(data), expected, int64(len(data)))
		if writeErr != nil {
			skippedChecksumMismatch++
			slog.Error("publish-reconciler.heal: mirror blob FAILED checksum verification — refusing to copy",
				"key", row.ArtifactKey,
				"name", manifest.GetRef().GetName(),
				"version", manifest.GetRef().GetVersion(),
				"expected", expected,
				"err", writeErr,
			)
			continue
		}

		healed++
		slog.Info("publish-reconciler.heal: restored PUBLISHED blob from mirror",
			"key", row.ArtifactKey,
			"name", manifest.GetRef().GetName(),
			"version", manifest.GetRef().GetVersion(),
			"checksum", actual,
			"size", len(data),
		)
	}

	if scanned > 0 && (healed > 0 || skippedNoMirror > 0 || skippedNoMirrorBlob > 0 || skippedChecksumMismatch > 0) {
		slog.Info("publish-reconciler.heal: scan complete",
			"scanned", scanned,
			"healed", healed,
			"skipped_no_mirror", skippedNoMirror,
			"skipped_no_mirror_blob", skippedNoMirrorBlob,
			"skipped_checksum_mismatch", skippedChecksumMismatch,
		)
	}
}
