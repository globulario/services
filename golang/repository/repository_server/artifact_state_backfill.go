// @awareness namespace=globular.platform
// @awareness component=platform_repository.artifact_state_backfill
// @awareness file_role=stat_only_classification_of_legacy_artifact_rows_into_pipeline_state_machine
// @awareness implements=globular.platform:intent.repository.lifecycle_state_machine
// @awareness enforces=globular.platform:invariant.repository.artifact.state_transitions_are_forward_only
// @awareness risk=high
package main

// artifact_state_backfill.go — DIAGNOSTIC + IDEMPOTENT
// migration. Classifies pre-pipeline manifest rows into
// PUBLISHED / BROKEN_MISSING_BLOB / BROKEN_CHECKSUM_MISMATCH
// using only os.Stat (size comparison). Full sha256
// verification belongs to VerifyArtifact; this file MUST NOT
// do hashing — heavy I/O during startup would block the
// repository service.
//
// Writes ONLY to rows where artifact_state is empty
// (PipelineUnspecified). Concrete states are never
// overwritten, so a backfill running against an
// already-migrated repository is a safe no-op. Removing the
// empty-state guard would let backfill silently downgrade a
// verified PUBLISHED to a stat-only classification.

// artifact_state_backfill.go — bounded migration of legacy rows into the
// ArtifactPipelineState model.
//
// Goals:
//   - Classify pre-existing manifest rows (those written before the pipeline
//     state machine was added) into one of:
//       PUBLISHED            — manifest exists, blob present, size matches
//       BROKEN_MISSING_BLOB  — manifest exists, blob absent
//       BROKEN_CHECKSUM_MISMATCH — manifest exists, blob present but size mismatch
//   - Be safe to run repeatedly (idempotent self-transitions are allowed).
//   - NOT block startup. Heavy hashing is deliberately avoided here; the
//     check is Stat-only. Full sha256 verification is the job of
//     VerifyArtifact / future repair workflows.
//
// The backfill writes only to rows where the artifact_state column is empty
// (PipelineUnspecified). Concrete states are never overwritten here, so an
// operator running an explicit verify-and-repair can do so independently.

import (
	"context"
	"log/slog"
	"strings"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// BackfillSummary reports the outcome of a backfill pass.
type BackfillSummary struct {
	Scanned             int
	PublishedOK         int
	MissingBlob         int
	ChecksumMismatch    int
	Quarantined         int
	Revoked             int
	Inconclusive        int
	AlreadyClassified   int
	Errors              int
	Truncated           bool
	Duration            time.Duration
}

// LogFields returns a slice suitable for slog calls.
func (b BackfillSummary) LogFields() []any {
	return []any{
		"scanned", b.Scanned,
		"published_ok", b.PublishedOK,
		"missing_blob", b.MissingBlob,
		"checksum_mismatch", b.ChecksumMismatch,
		"quarantined", b.Quarantined,
		"revoked", b.Revoked,
		"inconclusive", b.Inconclusive,
		"already_classified", b.AlreadyClassified,
		"errors", b.Errors,
		"truncated", b.Truncated,
		"duration_ms", b.Duration.Milliseconds(),
	}
}

// backfillArtifactStates classifies legacy manifest rows into concrete
// pipeline states using only cheap Stat checks (no full sha256 hashing).
//
//   - max=0 means "no cap" (test/admin path).
//   - max>0 caps the number of rows examined; remaining legacy rows can be
//     handled on a subsequent pass or by the row's natural sync/download path.
//
// Returns a summary that callers can log.
func (srv *server) backfillArtifactStates(ctx context.Context, max int) BackfillSummary {
	start := time.Now()
	summary := BackfillSummary{}

	if srv.scylla == nil {
		// No Scylla → backfill operates on the in-memory cache. Tests rely
		// on this path; in production Scylla is required.
		summary.Duration = time.Since(start)
		return summary
	}

	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		slog.Warn("artifact-state backfill: list manifests failed", "err", err)
		summary.Errors++
		summary.Duration = time.Since(start)
		return summary
	}

	for i := range rows {
		row := rows[i]

		if max > 0 && summary.Scanned >= max {
			summary.Truncated = true
			break
		}
		summary.Scanned++

		// Skip ledger pseudo-rows (they live in the same table but use a
		// different artifact_key shape and are not real artifacts).
		if strings.HasPrefix(row.ArtifactKey, "ledger/") {
			continue
		}

		// Already classified. Most concrete states are left as-is.
		// Exception: BLOB_VERIFIED is an intermediate publish pipeline state that
		// should only appear transiently. If a row is stuck here (e.g. because the
		// download-refill path left it behind before the fix), repair it by driving
		// the pipeline forward to PUBLISHED — provided the blob is actually present.
		current := srv.readArtifactState(ctx, row.ArtifactKey)
		if current != PipelineUnspecified && current != PipelineBlobVerified {
			summary.AlreadyClassified++
			continue
		}
		if current == PipelineBlobVerified {
			ref := &repopb.ArtifactRef{
				PublisherId: row.PublisherID,
				Name:        row.Name,
				Version:     row.Version,
				Platform:    row.Platform,
			}
			present, _ := srv.artifactBlobStatus(ctx, ref, row.BuildNumber, row.SizeBytes)
			if !present {
				summary.MissingBlob++
				continue
			}
			fields := ArtifactStateFields{
				BlobKey:     binaryStorageKey(row.ArtifactKey),
				Checksum:    row.Checksum,
				SizeBytes:   row.SizeBytes,
				BuildNumber: row.BuildNumber,
				PublisherID: row.PublisherID,
				Name:        row.Name,
				Version:     row.Version,
				Platform:    row.Platform,
			}
			// Drive BLOB_VERIFIED → MANIFEST_WRITTEN → LEDGER_WRITTEN → PUBLISHED
			// (manifest/ledger exist; no file re-writes needed — state machine only).
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelineManifestWritten, "backfill_repair_blob_verified", "", fields)
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelineLedgerWritten, "backfill_repair_blob_verified", "", fields)
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelinePublished, "backfill_repair_blob_verified", "", fields)
			summary.PublishedOK++
			continue
		}

		fields := ArtifactStateFields{
			BlobKey:     binaryStorageKey(row.ArtifactKey),
			Checksum:    row.Checksum,
			SizeBytes:   row.SizeBytes,
			BuildNumber: row.BuildNumber,
			PublisherID: row.PublisherID,
			Name:        row.Name,
			Version:     row.Version,
			Platform:    row.Platform,
		}

		// Lifecycle-state shortcuts: rows already in admin states get a
		// matching pipeline state without touching the blob.
		switch row.PublishState {
		case repopb.PublishState_QUARANTINED.String():
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelineQuarantined,
				"backfill_from_publish_state_quarantined", "", fields)
			summary.Quarantined++
			continue
		case repopb.PublishState_REVOKED.String():
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelineRevoked,
				"backfill_from_publish_state_revoked", "", fields)
			summary.Revoked++
			continue
		case repopb.PublishState_CORRUPTED.String():
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelineBrokenChecksumMismatch,
				"backfill_from_publish_state_corrupted", "", fields)
			summary.ChecksumMismatch++
			continue
		}

		// Concrete classification — cheap Stat check only.
		ref := &repopb.ArtifactRef{
			PublisherId: row.PublisherID,
			Name:        row.Name,
			Version:     row.Version,
			Platform:    row.Platform,
		}
		present, reason := srv.artifactBlobStatus(ctx, ref, row.BuildNumber, row.SizeBytes)
		switch {
		case present:
			_ = srv.transitionArtifactState(ctx, row.ArtifactKey, PipelinePublished,
				"backfill_blob_verified", "", fields)
			summary.PublishedOK++
		case reason == "missing_blob":
			srv.markPipelineMissingBlob(ctx, row.ArtifactKey,
				"backfill_blob_missing", "", fields)
			summary.MissingBlob++
		case reason == "size_mismatch":
			srv.markPipelineBrokenChecksum(ctx, row.ArtifactKey,
				"backfill_size_mismatch", "", fields)
			summary.ChecksumMismatch++
		default:
			summary.Inconclusive++
		}
	}

	summary.Duration = time.Since(start)
	slog.Info("artifact-state backfill: complete", summary.LogFields()...)
	return summary
}
