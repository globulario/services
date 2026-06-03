// @awareness namespace=globular.platform
// @awareness component=platform_repository.artifact_admin
// @awareness file_role=repair_quarantine_revoke_helpers_wrapping_artifact_state_machine_with_per_action_preconditions
// @awareness implements=globular.platform:intent.repository.repair_is_explicit
// @awareness implements=globular.platform:intent.repository.lifecycle_state_machine
// @awareness enforces=globular.platform:invariant.repository.artifact.state_transitions_are_forward_only
// @awareness risk=critical
package main

// artifact_admin.go — operator-initiated repair, quarantine,
// and revoke. Wraps the pipeline state machine in
// artifact_state.go with action-specific preconditions:
//
//   RepairArtifactFromUpstream — requires BROKEN_MISSING_BLOB
//     or BROKEN_CHECKSUM_MISMATCH, a usable UpstreamImport, and
//     a still-reachable source with a compatible policy
//   QuarantineArtifact / Unquarantine — reversible only after
//     a clean VerifyArtifact
//   RevokeArtifact — TERMINAL; sync cannot auto-repair afterward
//
// MUST NOT loosen the preconditions. Every repair path is
// explicit by design (intent: repair_is_explicit); auto-repair
// of revoked or quarantined artifacts re-opens supply-chain
// poisoning vectors that the state machine exists to prevent.

// artifact_admin.go — internal helpers for repair, quarantine, revoke.
//
// These wrap the artifact pipeline state machine (artifact_state.go) with
// the additional checks each operator action requires:
//
//   - RepairArtifactFromUpstream: rebuild a broken artifact's binary from
//     the upstream source recorded in its manifest. Requires:
//     • current pipeline state is BROKEN_MISSING_BLOB or BROKEN_CHECKSUM_MISMATCH
//       (or PUBLISHED in legacy/empty form — see force flag)
//     • a usable UpstreamImport on the manifest
//     • the source still exists, is enabled, and has a compatible policy
//
//   - QuarantineArtifact / UnquarantineArtifact: admin policy hold.
//     Quarantine is reversible only after a clean VerifyArtifact.
//
//   - RevokeArtifact: terminal lifecycle. Cannot be auto-repaired by sync.
//
// Public RPCs are deferred to a future pass; these helpers exist so admin
// flows have a stable internal surface and so tests can exercise the
// state-machine semantics independently of gRPC plumbing.

import (
	"context"
	"fmt"
	"log/slog"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Repair ────────────────────────────────────────────────────────────────

// RepairOptions controls RepairArtifactFromUpstream's safety behavior.
type RepairOptions struct {
	// Force allows repair from any non-REVOKED state. Without Force, repair
	// is only attempted from BROKEN_MISSING_BLOB / BROKEN_CHECKSUM_MISMATCH
	// (or unspecified/empty for legacy rows).
	Force bool

	// WorkflowRunID, when non-empty, propagates into transition records and
	// any workflow receipts emitted during the repair.
	WorkflowRunID string

	// AllowQuarantineOverride permits repair of QUARANTINED artifacts. By
	// default QUARANTINED is policy-protected; auto-repair must not bypass.
	AllowQuarantineOverride bool
}

// RepairArtifactFromUpstream re-imports an artifact's binary from the
// upstream source recorded in its manifest. Walks the same pipeline as
// SyncFromUpstream — DOWNLOADING → BLOB_WRITTEN → BLOB_VERIFIED →
// MANIFEST_WRITTEN → LEDGER_WRITTEN → PUBLISHED — so the same invariants
// apply.
//
// Returns:
//   - nil  on success (artifact is now PUBLISHED + blob verified).
//   - error otherwise; pipeline state is left at the last successful step.
//
// REVOKED is terminal — RepairArtifactFromUpstream rejects unconditionally.
// QUARANTINED is rejected unless opts.AllowQuarantineOverride is set.
func (srv *server) RepairArtifactFromUpstream(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, opts RepairOptions) error {
	if ref == nil {
		return fmt.Errorf("RepairArtifactFromUpstream: nil ref")
	}
	key := artifactKeyWithBuild(ref, buildNumber)

	// State precondition. REVOKED is hard-no.
	current := srv.readArtifactState(ctx, key)
	switch current {
	case PipelineRevoked:
		return fmt.Errorf("repair: artifact %s is REVOKED — terminal, refusing", key)
	case PipelineQuarantined:
		if !opts.AllowQuarantineOverride {
			return fmt.Errorf("repair: artifact %s is QUARANTINED — refusing without explicit override", key)
		}
	case PipelinePublished:
		if !opts.Force {
			// Optimization: if PUBLISHED + blob verified, nothing to do.
			present, _ := srv.artifactBlobStatus(ctx, ref, buildNumber, 0)
			if present {
				slog.Info("repair: artifact already PUBLISHED with blob present — no-op",
					"artifact_key", key)
				return nil
			}
		}
	case PipelineBrokenMissingBlob, PipelineBrokenChecksumMismatch,
		PipelineUnspecified:
		// Expected repair-eligible states.
	default:
		if !opts.Force {
			return fmt.Errorf("repair: artifact %s is in pipeline state %s — refusing without Force",
				key, current)
		}
	}

	// Read the manifest to pull the UpstreamImport. Use the manifest as the
	// source of truth for source_name / asset_url / checksum since that's
	// what was recorded at original import.
	_, _, manifest, mErr := srv.readManifestAndStateByKey(ctx, key)
	if mErr != nil || manifest == nil {
		return fmt.Errorf("repair: read manifest %s: %w", key, mErr)
	}
	ui := manifest.GetUpstreamImport()
	if ui == nil || ui.GetSourceName() == "" {
		return fmt.Errorf("repair: artifact %s has no upstream_import — cannot repair from upstream", key)
	}

	// upstreamFallbackAllowed checks: source exists, is enabled, policy
	// permits, publisher/kind/channel match. Same gate DownloadArtifact's
	// refill path uses — fail closed.
	if !srv.upstreamFallbackAllowed(ctx, manifest) {
		return fmt.Errorf("repair: upstream fallback not allowed for %s (source policy)", key)
	}

	// refillBlobFromUpstream re-downloads + writes the .bin. After it
	// succeeds the blob is back. We do not piggyback the full state-machine
	// transitions through it because the live pipeline-state aware path
	// would loop; instead we drive transitions explicitly here.
	if current != PipelineBrokenMissingBlob && current != PipelineBrokenChecksumMismatch {
		// If we're forcing, mark broken first so the transition graph is honored.
		fields := ArtifactStateFields{
			BlobKey:     binaryStorageKey(key),
			Checksum:    manifest.GetChecksum(),
			SizeBytes:   manifest.GetSizeBytes(),
			BuildID:     manifest.GetBuildId(),
			BuildNumber: manifest.GetBuildNumber(),
			PublisherID: ref.GetPublisherId(),
			Name:        ref.GetName(),
			Version:     ref.GetVersion(),
			Platform:    ref.GetPlatform(),
		}
		srv.markPipelineMissingBlob(ctx, key, "repair_force_resync", opts.WorkflowRunID, fields)
	}

	stateFields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(key),
		Checksum:    manifest.GetChecksum(),
		SizeBytes:   manifest.GetSizeBytes(),
		BuildID:     manifest.GetBuildId(),
		BuildNumber: manifest.GetBuildNumber(),
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
	}
	if err := srv.transitionArtifactState(ctx, key, PipelineDownloading,
		"repair_started", opts.WorkflowRunID, stateFields); err != nil {
		return fmt.Errorf("repair: transition→DOWNLOADING: %w", err)
	}

	rc, refillErr := srv.refillBlobFromUpstream(ctx, key)
	if refillErr != nil {
		return fmt.Errorf("repair: refill from upstream: %w", refillErr)
	}
	defer rc.Close()
	// refillBlobFromUpstream wrote the .bin already; close the reader so
	// nothing leaks. Now drive the rest of the pipeline based on storage state.
	_ = srv.transitionArtifactState(ctx, key, PipelineBlobWritten,
		"repair_binary_persisted", opts.WorkflowRunID, stateFields)

	present, reason := srv.artifactBlobStatus(ctx, ref, buildNumber, manifest.GetSizeBytes())
	if !present {
		srv.markBrokenForReason(ctx, key, "repair_post_write_"+reason, opts.WorkflowRunID, stateFields)
		return fmt.Errorf("repair: post-write verification failed: %s", reason)
	}
	_ = srv.transitionArtifactState(ctx, key, PipelineBlobVerified,
		"repair_blob_verified", opts.WorkflowRunID, stateFields)

	// Manifest stays as-is — we did not change identity. Stamp transition
	// for completeness so workflow receipts show full sequence.
	_ = srv.transitionArtifactState(ctx, key, PipelineManifestWritten,
		"repair_manifest_unchanged", opts.WorkflowRunID, stateFields)

	// Re-append to the ledger. Idempotent — duplicate entry detection lives
	// inside appendToLedger.
	if ledgerErr := srv.appendToLedger(ctx, ref.GetPublisherId(), ref.GetName(),
		ref.GetVersion(), manifest.GetBuildId(), manifest.GetChecksum(),
		ref.GetPlatform(), manifest.GetSizeBytes()); ledgerErr != nil {
		return fmt.Errorf("repair: append to ledger: %w", ledgerErr)
	}
	_ = srv.transitionArtifactState(ctx, key, PipelineLedgerWritten,
		"repair_ledger_persisted", opts.WorkflowRunID, stateFields)

	// Re-elevate publish_state to PUBLISHED when it had been downgraded to
	// CORRUPTED by markPipelineBrokenChecksum. Other lifecycle states (e.g.
	// admin QUARANTINED with override) should not be auto-elevated here.
	if srv.scylla != nil {
		if state := srv.readPublishStateString(ctx, key); state == repopb.PublishState_CORRUPTED.String() {
			if err := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_PUBLISHED.String()); err != nil {
				slog.Warn("repair: publish_state→PUBLISHED restore failed",
					"artifact_key", key, "err", err)
			}
		}
	}

	if err := srv.transitionArtifactState(ctx, key, PipelinePublished,
		"repair_complete", opts.WorkflowRunID, stateFields); err != nil {
		return fmt.Errorf("repair: transition→PUBLISHED: %w", err)
	}
	slog.Info("repair: artifact repaired from upstream",
		"artifact_key", key, "publisher", ref.GetPublisherId(),
		"name", ref.GetName(), "version", ref.GetVersion(), "platform", ref.GetPlatform(),
		"build_number", buildNumber)
	return nil
}

// ── Quarantine / Revoke ───────────────────────────────────────────────────

// QuarantineArtifact moves the artifact to QUARANTINED in BOTH the public
// PublishState and ArtifactPipelineState. Resolver excludes QUARANTINED
// from installable. Quarantine is reversible via UnquarantineArtifact only
// after VerifyArtifact returns OK.
func (srv *server) QuarantineArtifact(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, reason, operator string) error {
	if ref == nil {
		return fmt.Errorf("QuarantineArtifact: nil ref")
	}
	if reason == "" {
		return fmt.Errorf("QuarantineArtifact: reason is required")
	}
	key := artifactKeyWithBuild(ref, buildNumber)

	current := srv.readArtifactState(ctx, key)
	if current == PipelineRevoked {
		return fmt.Errorf("quarantine: artifact %s is REVOKED — terminal, refusing", key)
	}

	fields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(key),
		BuildNumber: buildNumber,
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
	}
	auditReason := fmt.Sprintf("operator=%s reason=%s", operator, reason)
	if err := srv.transitionArtifactState(ctx, key, PipelineQuarantined, auditReason, "", fields); err != nil {
		return fmt.Errorf("quarantine: transition: %w", err)
	}
	if srv.scylla != nil {
		if err := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_QUARANTINED.String()); err != nil {
			slog.Warn("quarantine: publish_state update failed",
				"artifact_key", key, "err", err)
		}
	}
	slog.Info("artifact quarantined",
		"artifact_key", key, "operator", operator, "reason", reason)
	return nil
}

// UnquarantineArtifact restores QUARANTINED to PUBLISHED — but ONLY after
// VerifyArtifact returns OK. Refuses for any state other than QUARANTINED.
func (srv *server) UnquarantineArtifact(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, operator string) error {
	if ref == nil {
		return fmt.Errorf("UnquarantineArtifact: nil ref")
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	current := srv.readArtifactState(ctx, key)
	if current != PipelineQuarantined {
		return fmt.Errorf("unquarantine: artifact %s is %s, not QUARANTINED — refusing", key, current)
	}

	verification, err := srv.verifyArtifactIntegrity(ctx, ref, buildNumber)
	if err != nil {
		return fmt.Errorf("unquarantine: verify: %w", err)
	}
	if verification.Status != VerifyOK {
		return fmt.Errorf("unquarantine: verify returned %s (%s) — refusing",
			verification.Status, verification.Reason)
	}

	fields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(key),
		BuildNumber: buildNumber,
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
	}
	if err := srv.transitionArtifactState(ctx, key, PipelinePublished,
		"unquarantine_verified:operator="+operator, "", fields); err != nil {
		return fmt.Errorf("unquarantine: transition: %w", err)
	}
	if srv.scylla != nil {
		if err := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_PUBLISHED.String()); err != nil {
			slog.Warn("unquarantine: publish_state update failed",
				"artifact_key", key, "err", err)
		}
	}
	slog.Info("artifact un-quarantined", "artifact_key", key, "operator", operator)
	return nil
}

// RevokeArtifact moves the artifact to REVOKED — terminal. Cannot be
// auto-repaired by sync. publish_state is also stamped REVOKED so existing
// resolver code rejects it.
func (srv *server) RevokeArtifact(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, reason, operator string) error {
	if ref == nil {
		return fmt.Errorf("RevokeArtifact: nil ref")
	}
	if reason == "" {
		return fmt.Errorf("RevokeArtifact: reason is required")
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	fields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(key),
		BuildNumber: buildNumber,
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
	}
	auditReason := fmt.Sprintf("operator=%s reason=%s", operator, reason)
	if err := srv.transitionArtifactState(ctx, key, PipelineRevoked, auditReason, "", fields); err != nil {
		return fmt.Errorf("revoke: transition: %w", err)
	}
	if srv.scylla != nil {
		if err := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_REVOKED.String()); err != nil {
			slog.Warn("revoke: publish_state update failed",
				"artifact_key", key, "err", err)
		}
	}
	slog.Info("artifact revoked",
		"artifact_key", key, "operator", operator, "reason", reason)
	return nil
}
