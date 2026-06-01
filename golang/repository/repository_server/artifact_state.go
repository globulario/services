package main

// artifact_state.go — durable repository pipeline state machine.
//
// Globular's repository must stop treating artifact publication as a loose
// set of side effects. ArtifactPipelineState makes the publish pipeline a
// durable, observable state machine that is independent of the public
// PublishState lifecycle gate.
//
// Core invariant — an artifact is INSTALLABLE only when:
//
//	pipeline_state == PUBLISHED
//	AND the binary blob exists at binaryStorageKey(artifactKeyWithBuild(...))
//	AND the blob size matches the recorded size_bytes (when known)
//	AND the blob digest matches the recorded checksum (when known)
//
// Intermediate pipeline states (DISCOVERED, DOWNLOADING, BLOB_WRITTEN,
// BLOB_VERIFIED, MANIFEST_WRITTEN, LEDGER_WRITTEN) are NEVER installable.
// Failure states (QUARANTINED, REVOKED, BROKEN_MISSING_BLOB,
// BROKEN_CHECKSUM_MISMATCH) are NEVER installable either.
//
// PublishState (the existing public lifecycle enum) and ArtifactPipelineState
// are kept SEPARATE on purpose:
//
//   - PublishState is the public lifecycle gate the resolver / RBAC / catalog
//     have always used (PUBLISHED, DEPRECATED, YANKED, QUARANTINED, REVOKED,
//     CORRUPTED, ARCHIVED). Existing code is not modified to look at
//     ArtifactPipelineState.
//
//   - ArtifactPipelineState is the durable repository pipeline tracker. It
//     records the FULL path an artifact takes through sync/publish so that
//     missing blobs, mid-pipeline crashes, and integrity drift all get an
//     explicit, queryable state.
//
// When an artifact that had been PUBLISHED is proven checksum-broken,
// markPipelineBrokenChecksum dual-stamps it: pipeline=BROKEN_CHECKSUM_MISMATCH
// AND publish_state=CORRUPTED — so existing resolver code (which only knows
// PublishState) also stops handing out the bad bytes.
//
// TODO(phase-b): attach artifact state transitions to workflow receipts /
// history so admin UI and AI executor can see per-artifact progress inside
// a workflow run.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/workflow"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// computeSHA256 returns the canonical "sha256:<hex>" form of data.
func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// ArtifactPipelineState is the durable pipeline state tracker.
//
type ArtifactPipelineState string

const (
	// PipelineUnspecified is the zero value. It also represents legacy rows
	// that were written before this enum existed — backfill / sync may
	// classify those into a concrete state once verified.
	PipelineUnspecified ArtifactPipelineState = ""

	PipelineDiscovered             ArtifactPipelineState = "DISCOVERED"
	PipelineDownloading            ArtifactPipelineState = "DOWNLOADING"
	PipelineBlobWritten            ArtifactPipelineState = "BLOB_WRITTEN"
	PipelineBlobVerified           ArtifactPipelineState = "BLOB_VERIFIED"
	PipelineManifestWritten        ArtifactPipelineState = "MANIFEST_WRITTEN"
	PipelineLedgerWritten          ArtifactPipelineState = "LEDGER_WRITTEN"
	PipelinePublished              ArtifactPipelineState = "PUBLISHED"
	PipelineQuarantined            ArtifactPipelineState = "QUARANTINED"
	PipelineRevoked                ArtifactPipelineState = "REVOKED"
	PipelineBrokenMissingBlob      ArtifactPipelineState = "BROKEN_MISSING_BLOB"
	PipelineBrokenChecksumMismatch ArtifactPipelineState = "BROKEN_CHECKSUM_MISMATCH"
)

// IsInstallable returns true only for PUBLISHED. Every other state — empty,
// intermediate, quarantined, revoked, or broken — is NOT installable.
func (s ArtifactPipelineState) IsInstallable() bool {
	return s == PipelinePublished
}

// IsTerminal returns true for end-of-life states that must not be auto-repaired.
func (s ArtifactPipelineState) IsTerminal() bool {
	return s == PipelineRevoked
}

// IsBroken returns true for states that indicate concrete integrity damage
// (missing blob or checksum mismatch). Quarantined / revoked are policy
// states, not damage states.
func (s ArtifactPipelineState) IsBroken() bool {
	return s == PipelineBrokenMissingBlob || s == PipelineBrokenChecksumMismatch
}

// allowedTransitions enumerates every legal pipeline transition.
// Idempotent self-transitions (X → X) are always allowed for retry safety
// and are enforced separately in IsTransitionAllowed.
var allowedTransitions = map[ArtifactPipelineState]map[ArtifactPipelineState]bool{
	// Legacy / unspecified rows can be classified into any concrete state.
	// This is what lets backfill move pre-existing rows safely.
	PipelineUnspecified: {
		PipelineDiscovered:             true,
		PipelineDownloading:            true,
		PipelineBlobWritten:            true,
		PipelineBlobVerified:           true,
		PipelineManifestWritten:        true,
		PipelineLedgerWritten:          true,
		PipelinePublished:              true,
		PipelineQuarantined:            true,
		PipelineRevoked:                true,
		PipelineBrokenMissingBlob:      true,
		PipelineBrokenChecksumMismatch: true,
	},

	// Happy path forward edges.
	PipelineDiscovered:      {PipelineDownloading: true, PipelineQuarantined: true, PipelineRevoked: true},
	PipelineDownloading:     {PipelineBlobWritten: true, PipelineBrokenMissingBlob: true, PipelineBrokenChecksumMismatch: true},
	PipelineBlobWritten:     {PipelineBlobVerified: true, PipelineBrokenChecksumMismatch: true, PipelineBrokenMissingBlob: true},
	PipelineBlobVerified:    {PipelineManifestWritten: true, PipelineBrokenMissingBlob: true, PipelineBrokenChecksumMismatch: true},
	PipelineManifestWritten: {PipelineLedgerWritten: true, PipelineBrokenMissingBlob: true, PipelineBrokenChecksumMismatch: true},
	PipelineLedgerWritten:   {PipelinePublished: true, PipelineBrokenMissingBlob: true, PipelineBrokenChecksumMismatch: true},

	// PUBLISHED can degrade or be moved to admin states. It cannot move
	// "back up" through intermediate pipeline stages without going via a
	// broken state first.
	PipelinePublished: {
		PipelineBrokenMissingBlob:      true,
		PipelineBrokenChecksumMismatch: true,
		PipelineQuarantined:            true,
		PipelineRevoked:                true,
	},

	// Repair edges from broken states go through DOWNLOADING again.
	PipelineBrokenMissingBlob:      {PipelineDownloading: true, PipelineRevoked: true},
	PipelineBrokenChecksumMismatch: {PipelineDownloading: true, PipelineRevoked: true},

	// Admin states.
	PipelineQuarantined: {PipelinePublished: true, PipelineRevoked: true},
	PipelineRevoked:     {}, // terminal — no outbound transitions
}

// IsTransitionAllowed returns true if `from`→`to` is a legal pipeline edge.
// Idempotent self-transitions are always allowed (retry-safe).
func IsTransitionAllowed(from, to ArtifactPipelineState) bool {
	if from == to {
		return true
	}
	if next, ok := allowedTransitions[from]; ok {
		return next[to]
	}
	return false
}

// ArtifactStateFields are the per-artifact identity fields persisted alongside
// the pipeline state. Fields with zero values are not written.
type ArtifactStateFields struct {
	BlobKey     string
	Checksum    string
	SizeBytes   int64
	BuildID     string
	BuildNumber int64
	PublisherID string
	Name        string
	Version     string
	Platform    string
}

// artifactStateRecord is the in-memory mirror of the durable state row.
// Used as the test-mode fallback when srv.scylla == nil and as a per-process
// cache so DownloadArtifact does not need to round-trip Scylla on every call.
type artifactStateRecord struct {
	State         ArtifactPipelineState
	Reason        string
	UpdatedUnix   int64
	WorkflowRunID string
	Fields        ArtifactStateFields
}

// transitionArtifactState moves an artifact's pipeline state durably.
//
// Behavior:
//   - Validates from→to against allowedTransitions; illegal transitions
//     return an error and persist nothing. Self-transitions are allowed for
//     retry safety.
//   - Writes to ScyllaDB when available (authoritative); always updates the
//     in-memory cache as well.
//   - Emits a structured slog event repository.artifact.state_transition.
//   - Emits a best-effort audit event (publishAuditEvent is non-fatal).
//   - Manifest JSON is NOT updated — Scylla columns are authoritative.
//
// TODO(phase-b): attach this transition to the workflow run's receipt/history
// so admin UI and AI executor can observe per-artifact progress inside a
// repository.publish.artifact / repository.sync.upstream run.
func (srv *server) transitionArtifactState(
	ctx context.Context,
	artifactKey string,
	to ArtifactPipelineState,
	reason string,
	workflowRunID string,
	fields ArtifactStateFields,
) error {
	if artifactKey == "" {
		return fmt.Errorf("transitionArtifactState: empty artifact_key")
	}
	if to == PipelineUnspecified {
		return fmt.Errorf("transitionArtifactState: target state is unspecified")
	}

	from := srv.readArtifactState(ctx, artifactKey)
	if !IsTransitionAllowed(from, to) {
		return fmt.Errorf("transitionArtifactState: illegal transition %q → %q for %s",
			string(from), string(to), artifactKey)
	}

	now := time.Now().Unix()
	rec := artifactStateRecord{
		State:         to,
		Reason:        reason,
		UpdatedUnix:   now,
		WorkflowRunID: workflowRunID,
		Fields:        fields,
	}

	// In-memory cache — always written. Doubles as the fallback when no Scylla.
	srv.cacheArtifactState(artifactKey, rec)

	// Authoritative durable write — Scylla.
	if srv.scylla != nil {
		if err := srv.scylla.UpdateArtifactState(ctx, artifactKey, scyllaArtifactState{
			State:         string(to),
			Reason:        reason,
			UpdatedUnix:   now,
			WorkflowRunID: workflowRunID,
			BlobKey:       fields.BlobKey,
			Checksum:      fields.Checksum,
			SizeBytes:     fields.SizeBytes,
			BuildID:       fields.BuildID,
			BuildNumber:   fields.BuildNumber,
			PublisherID:   fields.PublisherID,
			Name:          fields.Name,
			Version:       fields.Version,
			Platform:      fields.Platform,
		}); err != nil {
			return fmt.Errorf("transitionArtifactState: persist %s → %s: %w", from, to, err)
		}
	}

	slog.Info("repository.artifact.state_transition",
		"artifact_key", artifactKey,
		"from", string(from),
		"to", string(to),
		"reason", reason,
		"workflow_run_id", workflowRunID,
		"blob_key", fields.BlobKey,
		"checksum", truncDigest(fields.Checksum),
		"size_bytes", fields.SizeBytes,
		"build_id", fields.BuildID,
		"build_number", fields.BuildNumber,
		"publisher", fields.PublisherID,
		"name", fields.Name,
		"version", fields.Version,
		"platform", fields.Platform,
	)

	if srv.artifactStateHook != nil {
		srv.artifactStateHook(artifactKey, from, to, reason, workflowRunID)
	}

	// Workflow receipt: one step per artifact state transition. Fire-and-forget;
	// the recorder no-ops when not connected. Skipped when no workflow run is
	// active for this transition.
	if workflowRunID != "" && srv.workflowRec != nil {
		details := map[string]any{
			"artifact_key":  artifactKey,
			"from":          string(from),
			"to":            string(to),
			"reason":        reason,
			"blob_key":      fields.BlobKey,
			"checksum":      fields.Checksum,
			"size_bytes":    fields.SizeBytes,
			"build_id":      fields.BuildID,
			"build_number":  fields.BuildNumber,
			"publisher":     fields.PublisherID,
			"name":          fields.Name,
			"version":       fields.Version,
			"platform":      fields.Platform,
		}
		var detailsJSON string
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = string(b)
		}
		seq := srv.workflowRec.RecordStep(ctx, workflowRunID, &workflow.StepParams{
			StepKey:     "repository.artifact." + strings.ToLower(string(to)),
			Title:       fmt.Sprintf("%s → %s (%s)", from, to, fields.Name),
			Actor:       workflow.ActorRepository,
			Phase:       workflow.PhasePublish,
			Status:      workflowpb.StepStatus_STEP_STATUS_SUCCEEDED,
			Message:     reason,
			DetailsJSON: detailsJSON,
		})
		// CompleteStep upserts duration metadata; for transition receipts we
		// don't track duration, so call CompleteStep with 0 — it's idempotent
		// against the SUCCEEDED status above and lets aggregations work.
		if seq > 0 {
			srv.workflowRec.CompleteStep(ctx, workflowRunID, seq, reason, 0)
		}
	}

	srv.publishAuditEvent(ctx, "repository.artifact.state_transition", map[string]any{
		"artifact_key":    artifactKey,
		"from":            string(from),
		"to":              string(to),
		"reason":          reason,
		"workflow_run_id": workflowRunID,
		"blob_key":        fields.BlobKey,
		"checksum":        fields.Checksum,
		"size_bytes":      fields.SizeBytes,
		"build_id":        fields.BuildID,
		"build_number":    fields.BuildNumber,
		"publisher":       fields.PublisherID,
		"name":            fields.Name,
		"version":         fields.Version,
		"platform":        fields.Platform,
	})

	return nil
}

// markPipelineBrokenChecksum transitions to BROKEN_CHECKSUM_MISMATCH AND, if
// the artifact had previously been PUBLISHED, also downgrades the public
// PublishState to CORRUPTED so existing resolver code (which only reads
// PublishState) also stops handing out the bad bytes.
func (srv *server) markPipelineBrokenChecksum(ctx context.Context, artifactKey, reason, workflowRunID string, fields ArtifactStateFields) {
	if err := srv.transitionArtifactState(ctx, artifactKey, PipelineBrokenChecksumMismatch, reason, workflowRunID, fields); err != nil {
		slog.Warn("artifact-state: broken-checksum transition failed",
			"artifact_key", artifactKey, "err", err)
	}
	if srv.scylla != nil {
		// Only downgrade publish_state if it was PUBLISHED — never elevate.
		if state := srv.readPublishStateString(ctx, artifactKey); state == repopb.PublishState_PUBLISHED.String() {
			if err := srv.scylla.UpdatePublishState(ctx, artifactKey, repopb.PublishState_CORRUPTED.String()); err != nil {
				slog.Warn("artifact-state: publish_state→CORRUPTED failed",
					"artifact_key", artifactKey, "err", err)
			}
		}
	}
}

// markPipelineMissingBlob transitions to BROKEN_MISSING_BLOB. PublishState is
// left unchanged for compatibility — but artifact_state is authoritative for
// installability decisions in sync/skip paths and the DownloadArtifact gate.
func (srv *server) markPipelineMissingBlob(ctx context.Context, artifactKey, reason, workflowRunID string, fields ArtifactStateFields) {
	if err := srv.transitionArtifactState(ctx, artifactKey, PipelineBrokenMissingBlob, reason, workflowRunID, fields); err != nil {
		slog.Warn("artifact-state: missing-blob transition failed",
			"artifact_key", artifactKey, "err", err)
	}
}

// markBrokenForReason maps a blob-status reason ("missing_blob",
// "size_mismatch", or anything else carrying "checksum"/"sha256") to the
// right BROKEN_X transition, applying the dual-state stamp where required.
func (srv *server) markBrokenForReason(ctx context.Context, artifactKey, reason, workflowRunID string, fields ArtifactStateFields) {
	r := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(r, "size") || strings.Contains(r, "checksum") || strings.Contains(r, "sha256"):
		srv.markPipelineBrokenChecksum(ctx, artifactKey, reason, workflowRunID, fields)
	default:
		srv.markPipelineMissingBlob(ctx, artifactKey, reason, workflowRunID, fields)
	}
}

// isRowInstallable is the central installability filter for repository
// resolver / list paths. An artifact is installable iff:
//
//   - publish_state == PUBLISHED (existing public lifecycle gate), AND
//   - artifact_state is either PUBLISHED (post-state-machine) or empty
//     (legacy row that predates the state machine — backfill will lift it)
//
// Any non-empty artifact_state other than PUBLISHED — DOWNLOADING,
// MANIFEST_WRITTEN, BROKEN_*, QUARANTINED, REVOKED — excludes the row.
//
// The legacy-empty fallthrough is the soft mode for the transition period:
// once cluster-wide backfill has run, every PUBLISHED row will also have
// artifact_state == PUBLISHED and this clause becomes a no-op. Removing it
// later is a breaking-change one-liner.
func isRowInstallable(row *manifestRow) bool {
	if row == nil {
		return false
	}
	if row.PublishState != repopb.PublishState_PUBLISHED.String() {
		return false
	}
	switch ArtifactPipelineState(strings.ToUpper(strings.TrimSpace(row.ArtifactState))) {
	case PipelinePublished, PipelineUnspecified:
		return true
	default:
		return false
	}
}

// isRowInstallableWithSignaturePolicy is the ctx-aware variant of
// isRowInstallable. Use it at install paths that have a context (resolver,
// DownloadArtifact gate, candidate enumeration). Adds the signature policy
// gate to the existing publish_state + artifact_state checks.
func (srv *server) isRowInstallableWithSignaturePolicy(ctx context.Context, row *manifestRow) bool {
	if !isRowInstallable(row) {
		return false
	}
	ref := &repopb.ArtifactRef{
		PublisherId: row.PublisherID,
		Name:        row.Name,
		Version:     row.Version,
		Platform:    row.Platform,
	}
	dec := srv.signaturePolicyDecision(ctx, ref, row.ArtifactKey, row.Checksum, "")
	return dec.Allowed
}

// isInstallableForRef checks installability when the caller has a manifest
// (not a manifestRow). The artifact_state column is read live, and the
// signature-policy gate is consulted last (so an artifact that's PUBLISHED
// but signed-required-and-missing fails closed).
func (srv *server) isInstallableForRef(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, publishState repopb.PublishState) bool {
	if publishState != repopb.PublishState_PUBLISHED {
		return false
	}
	if ref == nil {
		return false
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	switch srv.readArtifactState(ctx, key) {
	case PipelinePublished, PipelineUnspecified:
		// fall through to signature policy gate
	default:
		return false
	}
	// Signature policy: refuse if a signature is required for this
	// artifact and not present / invalid / from a revoked key.
	expectedDigest := ""
	if _, _, m, _ := srv.readManifestAndStateByKey(ctx, key); m != nil {
		expectedDigest = m.GetChecksum()
	}
	dec := srv.signaturePolicyDecision(ctx, ref, key, expectedDigest, "")
	return dec.Allowed
}

// canSkipDueToExistingState returns (true, currentState) if the artifact's
// pipeline state allows a sync skip (i.e. the row is genuinely PUBLISHED, or
// it's a legacy row with empty state that we'll classify as PUBLISHED on the
// idempotent skip stamp). Returns (false, currentState) for any state that
// must trigger reprocessing — DOWNLOADING, BLOB_WRITTEN, BROKEN_*,
// QUARANTINED, REVOKED, etc.
//
// Skip eligibility ≠ installability. Skip means "the upstream import does not
// need to do work"; that is true for both PUBLISHED and legacy-empty rows
// (the latter will be stamped PUBLISHED by the caller as part of backfill).
func (srv *server) canSkipDueToExistingState(ctx context.Context, artifactKey string) (bool, ArtifactPipelineState) {
	s := srv.readArtifactState(ctx, artifactKey)
	switch s {
	case PipelinePublished, PipelineUnspecified:
		// PUBLISHED and Unspecified-but-blob-verified rows are both nominally
		// eligible for skip — but only if the manifest is actually present.
		// A row with NULL manifest_json (skeleton row from an interrupted sync)
		// would otherwise be stamped PUBLISHED and silently kept broken, with
		// the controller hitting "proto: syntax error" forever on lookup.
		// See docs/intent/repository.metadata_is_authority.yaml.
		if !srv.manifestJSONPresent(ctx, artifactKey) {
			slog.Warn("artifact-state: row exists but manifest_json is missing — refusing skip, forcing full re-import",
				"artifact_key", artifactKey, "state", string(s))
			return false, s
		}
		if s == PipelineUnspecified {
			slog.Info("artifact-state: legacy artifact_state missing — treating as eligible for idempotent PUBLISHED stamp",
				"artifact_key", artifactKey)
		}
		return true, s
	default:
		return false, s
	}
}

// manifestJSONPresent reports whether the Scylla manifests row for artifactKey
// has a non-empty manifest_json column. A row created only via
// UpdateArtifactState (UPSERT without manifest_json) appears in the table but
// has manifest_json=NULL — it must NOT be treated as "manifest present".
func (srv *server) manifestJSONPresent(ctx context.Context, artifactKey string) bool {
	if srv.scylla == nil {
		// No ledger available — fall back to legacy behavior (assume present).
		return true
	}
	row, err := srv.scylla.GetManifest(ctx, artifactKey)
	if err != nil || row == nil {
		return false
	}
	return len(row.ManifestJSON) > 0
}

// readArtifactState returns the current pipeline state for an artifact key.
// Reads from Scylla when available (authoritative); falls back to the
// in-memory cache otherwise. Returns PipelineUnspecified for legacy rows
// that have no artifact_state set yet.
func (srv *server) readArtifactState(ctx context.Context, artifactKey string) ArtifactPipelineState {
	if srv.scylla != nil {
		if state, err := srv.scylla.GetArtifactState(ctx, artifactKey); err == nil {
			return ArtifactPipelineState(strings.ToUpper(strings.TrimSpace(state)))
		}
	}
	if rec, ok := srv.lookupArtifactStateCache(artifactKey); ok {
		return rec.State
	}
	return PipelineUnspecified
}

// readPublishStateString returns the publish_state column as a string, or
// "" if Scylla is unavailable / row missing. Used by markPipelineBrokenChecksum
// to decide whether to downgrade publish_state.
func (srv *server) readPublishStateString(ctx context.Context, artifactKey string) string {
	if srv.scylla == nil {
		return ""
	}
	row, err := srv.scylla.GetManifest(ctx, artifactKey)
	if err != nil || row == nil {
		return ""
	}
	return row.PublishState
}

// ── In-memory cache (also serves as test fallback when scylla=nil) ─────────

func (srv *server) cacheArtifactState(artifactKey string, rec artifactStateRecord) {
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if srv.artifactStateCache == nil {
		srv.artifactStateCache = map[string]artifactStateRecord{}
	}
	srv.artifactStateCache[artifactKey] = rec
}

func (srv *server) lookupArtifactStateCache(artifactKey string) (artifactStateRecord, bool) {
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	rec, ok := srv.artifactStateCache[artifactKey]
	return rec, ok
}

// artifactStateMutex / artifactStateCache fields are declared on the server
// struct in server.go. Declared here as a fence to fail the build if removed.
var _ = sync.Mutex{}

// ── VerifyArtifact (read-only integrity probe) ─────────────────────────────

// ArtifactVerificationStatus enumerates the outcomes of VerifyArtifact.
type ArtifactVerificationStatus string

const (
	VerifyOK                     ArtifactVerificationStatus = "OK"
	VerifyBrokenMissingBlob      ArtifactVerificationStatus = "BROKEN_MISSING_BLOB"
	VerifyBrokenChecksumMismatch ArtifactVerificationStatus = "BROKEN_CHECKSUM_MISMATCH"
	VerifyBrokenLedgerMissing    ArtifactVerificationStatus = "BROKEN_LEDGER_MISSING"
	VerifyBrokenManifestMissing  ArtifactVerificationStatus = "BROKEN_MANIFEST_MISSING"
	VerifyInconclusive           ArtifactVerificationStatus = "INCONCLUSIVE"
)

// ArtifactVerification is the read-only result of VerifyArtifact.
type ArtifactVerification struct {
	Status      ArtifactVerificationStatus
	Reason      string
	BlobKey     string
	ExpectedSHA string
	ActualSHA   string
	ExpectedSiz int64
	ActualSize  int64
}

// verifyArtifactIntegrity checks an artifact's integrity without mutating
// state. Caller decides whether to act on the result (e.g. transition to
// BROKEN_X). Internal helper — the public RPC handler is in artifact_handlers.go
// (server.VerifyArtifact). Both share this same routine.
//
// Checks (in order, short-circuit on hard failure):
//  1. Manifest exists and decodes.
//  2. Blob exists at binaryStorageKey(artifactKeyWithBuild(ref, buildNumber)).
//  3. Blob size matches manifest.SizeBytes (when SizeBytes > 0).
//  4. Blob sha256 matches manifest.Checksum (only computed when checksum is
//     present AND size matches — full hash is expensive).
//  5. Release ledger has a row with the same digest+build_id.
//
// Returns INCONCLUSIVE for legacy/unknown shapes; BROKEN_X for concrete
// failures; OK when all checks pass.
func (srv *server) verifyArtifactIntegrity(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) (*ArtifactVerification, error) {
	if ref == nil {
		return &ArtifactVerification{Status: VerifyInconclusive, Reason: "nil_ref"}, nil
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	blobKey := binaryStorageKey(key)

	res := &ArtifactVerification{BlobKey: blobKey}

	_, _, manifest, manifestErr := srv.readManifestAndStateByKey(ctx, key)
	if manifestErr != nil || manifest == nil {
		res.Status = VerifyBrokenManifestMissing
		res.Reason = "manifest_not_found"
		if manifestErr != nil {
			res.Reason = manifestErr.Error()
		}
		return res, nil
	}
	res.ExpectedSHA = manifest.GetChecksum()
	res.ExpectedSiz = manifest.GetSizeBytes()

	// Blob presence check: local POSIX CAS only.
	// MinIO mirror presence must NOT make integrity verification pass — if the
	// blob is absent from local POSIX CAS the artifact is not installable.
	if srv.localStorage == nil {
		res.Status = VerifyInconclusive
		res.Reason = "local_storage_not_initialized"
		return res, nil
	}
	fi, statErr := srv.localStorage.Stat(ctx, blobKey)
	if statErr != nil {
		res.Status = VerifyBrokenMissingBlob
		res.Reason = "stat: " + statErr.Error()
		return res, nil
	}
	res.ActualSize = fi.Size()
	if res.ExpectedSiz > 0 && res.ActualSize != res.ExpectedSiz {
		res.Status = VerifyBrokenChecksumMismatch
		res.Reason = "size_mismatch"
		return res, nil
	}

	if res.ExpectedSHA != "" {
		localPath := srv.localStorage.LocalPath(blobKey)
		actual, readErr := checksumLocalFile(localPath)
		if readErr != nil {
			res.Status = VerifyBrokenMissingBlob
			res.Reason = "read: " + readErr.Error()
			return res, nil
		}
		res.ActualSHA = actual
		if !digestEqual(actual, res.ExpectedSHA) {
			res.Status = VerifyBrokenChecksumMismatch
			res.Reason = "sha256_mismatch"
			return res, nil
		}
	}

	// Ledger check — best-effort. Treat absence as INCONCLUSIVE not BROKEN
	// because some flows (early uploads) do not yet have ledger entries.
	if manifest.GetChecksum() != "" {
		ledger := srv.readLedger(ctx, ref.GetPublisherId(), ref.GetName())
		if ledger == nil {
			res.Status = VerifyInconclusive
			res.Reason = "ledger_missing_for_publisher"
			return res, nil
		}
		found := false
		for _, r := range ledger.Releases {
			if r.Version == ref.GetVersion() && r.Platform == ref.GetPlatform() &&
				digestEqual(r.Digest, manifest.GetChecksum()) {
				found = true
				break
			}
		}
		if !found {
			res.Status = VerifyBrokenLedgerMissing
			res.Reason = "no_ledger_row_with_matching_digest"
			return res, nil
		}
	}

	res.Status = VerifyOK
	res.Reason = "ok"
	return res, nil
}
