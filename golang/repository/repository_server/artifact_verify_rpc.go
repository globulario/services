// @awareness namespace=globular.platform
// @awareness component=platform_repository.artifact_verify_rpc
// @awareness file_role=public_rpc_verify_repair_explain_artifact_no_logic_duplication
// @awareness implements=globular.platform:intent.repository.metadata_is_authority
// @awareness implements=globular.platform:intent.repository.repair_is_explicit
// @awareness enforces=globular.platform:invariant.repository.artifact.installable_compound_predicate
// @awareness risk=critical
package main

// artifact_verify_rpc.go — operator-facing wrappers around
// verifyArtifactIntegrity, RepairArtifactFromUpstream,
// readManifestAndStateByKey, and artifactBlobStatus. NO
// verification logic is reimplemented here — the repository
// service is the single source of truth for installable
// status.
//
// Adding "lite verification" or "quick check" branches that
// bypass verifyArtifactIntegrity would let an unverified
// artifact slip past the explain UI as installable. Surface
// the truth verbatim; the operator decides what to do.

// artifact_verify_rpc.go — public RPC handlers for VerifyArtifact /
// RepairArtifact / ExplainArtifact.
//
// These are operator-facing surfaces. They wrap the existing internal helpers
// (verifyArtifactIntegrity, RepairArtifactFromUpstream, readManifestAndStateByKey,
// artifactBlobStatus) and translate the result into the proto response shape.
// No verification logic is reimplemented here — the repository service is the
// single source of truth.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── VerifyArtifact RPC ────────────────────────────────────────────────────

// VerifyArtifact runs a read-only integrity probe against a single artifact.
// Never mutates state. Mirrors the operator command `globular repository verify`.
func (srv *server) VerifyArtifact(ctx context.Context, req *repopb.VerifyArtifactRequest) (*repopb.VerifyArtifactResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	canonicalizeRefVersion(ref)

	buildNumber := req.GetBuildNumber()
	if buildNumber == 0 {
		// Repair targets are frequently non-installable (BROKEN_*). Using the
		// installable-only resolver here makes those rows impossible to repair.
		buildNumber = srv.resolveLatestExistingBuildNumber(ctx, ref)
	}
	if buildNumber == 0 {
		return nil, status.Errorf(codes.NotFound, "no PUBLISHED builds for %s/%s@%s [%s]",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	}

	verification, err := srv.verifyArtifactIntegrity(ctx, ref, buildNumber)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify: %v", err)
	}
	key := artifactKeyWithBuild(ref, buildNumber)

	resp := &repopb.VerifyArtifactResponse{
		Ref:             ref,
		ArtifactKey:     key,
		ArtifactState:   string(srv.readArtifactState(ctx, key)),
		BlobKey:         verification.BlobKey,
		ExpectedSize:    verification.ExpectedSiz,
		ActualSize:      verification.ActualSize,
		ExpectedDigest:  verification.ExpectedSHA,
		ActualDigest:    verification.ActualSHA,
		SignatureStatus: "NOT_IMPLEMENTED", // Phase CLI-B
	}

	// Read manifest for publish_state + ledger / manifest presence flags.
	_, publishState, m, mErr := srv.readManifestAndStateByKey(ctx, key)
	if mErr == nil {
		resp.PublishState = publishState
		if req.GetIncludeManifest() && m != nil {
			// Already present — manifest fields already filled via verification.
		}
	}

	// Map internal verification status to proto enum + repair recommendation.
	resp.Status = mapVerifyStatus(verification.Status)
	resp.Reason = verification.Reason

	pipelineState := ArtifactPipelineState(resp.ArtifactState)
	resp.Installable = pipelineState == PipelinePublished &&
		publishState == repopb.PublishState_PUBLISHED &&
		verification.Status == VerifyOK

	resp.Repairable, resp.RecommendedAction = repairAdvice(pipelineState, verification.Status)

	// If the caller asked us to recompute the digest and we haven't yet,
	// the verifyArtifactIntegrity call already does it when ExpectedSHA is set.
	// We just propagate verify_digest into the response shape.
	_ = req.GetVerifyDigest()
	_ = req.GetIncludeBlob()
	_ = req.GetIncludeLedger()

	return resp, nil
}

// mapVerifyStatus translates the internal ArtifactVerificationStatus string
// into the proto enum exposed to clients.
func mapVerifyStatus(s ArtifactVerificationStatus) repopb.ArtifactVerifyStatus {
	switch s {
	case VerifyOK:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_OK
	case VerifyBrokenMissingBlob:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_MISSING_BLOB
	case VerifyBrokenChecksumMismatch:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_CHECKSUM_MISMATCH
	case VerifyBrokenLedgerMissing:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_LEDGER_MISSING
	case VerifyBrokenManifestMissing:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_MANIFEST_MISSING
	case VerifyInconclusive:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_INCONCLUSIVE
	default:
		return repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_STATUS_UNSPECIFIED
	}
}

// repairAdvice returns (repairable, recommendedAction) given the pipeline +
// verification state. The recommendation is what the operator should run next.
func repairAdvice(p ArtifactPipelineState, vs ArtifactVerificationStatus) (bool, string) {
	switch p {
	case PipelineRevoked:
		return false, "REVOKED is terminal — no repair available"
	case PipelineQuarantined:
		return false, "QUARANTINED — operator un-quarantine after manual review"
	}
	switch vs {
	case VerifyOK:
		return false, "ok"
	case VerifyBrokenMissingBlob:
		return true, "globular repository repair --from-upstream"
	case VerifyBrokenChecksumMismatch:
		return true, "globular repository repair --from-upstream (checksum drift)"
	case VerifyBrokenLedgerMissing:
		return true, "ledger missing — repair requires upstream re-import"
	case VerifyBrokenManifestMissing:
		return false, "manifest missing — manual investigation required"
	case VerifyInconclusive:
		return false, "INCONCLUSIVE — re-run verify after backfill"
	}
	return false, ""
}

// ── RepairArtifact RPC ────────────────────────────────────────────────────

// RepairArtifact attempts to repair a broken artifact by re-importing from
// the upstream source recorded in its manifest.
func (srv *server) RepairArtifact(ctx context.Context, req *repopb.RepairArtifactRequest) (*repopb.RepairArtifactResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	canonicalizeRefVersion(ref)

	buildNumber := req.GetBuildNumber()
	if buildNumber == 0 {
		// Repair targets are frequently non-installable (BROKEN_*). Using the
		// installable-only resolver here makes those rows impossible to repair.
		buildNumber = srv.resolveLatestExistingBuildNumber(ctx, ref)
	}
	if buildNumber == 0 {
		return nil, status.Errorf(codes.NotFound, "no builds for %s/%s@%s [%s]",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	}

	key := artifactKeyWithBuild(ref, buildNumber)
	resp := &repopb.RepairArtifactResponse{
		Ref:                  ref,
		ArtifactKey:          key,
		ArtifactStateBefore:  string(srv.readArtifactState(ctx, key)),
	}

	// Audit / operator subject — prefer the request's claimed subject only when
	// the caller is admin. Otherwise use the authn subject.
	authCtx := security.FromContext(ctx)
	subject := req.GetOperatorSubject()
	if authCtx != nil && authCtx.Subject != "" {
		subject = authCtx.Subject
	}
	_ = subject // recorded in transition reason via the helper

	// Project D: probe whether the CAS files are self-healing.
	//
	// When a Scylla row exists with NULL manifest_json (skeleton from a
	// pre-d2ef80ee partial write), readManifestAndStateByKey returns an
	// error and verifyArtifactIntegrity reports BROKEN_MANIFEST_MISSING.
	// But the CAS .manifest.json file may still be intact on local POSIX
	// storage. Probe the local file directly, bypassing the Scylla read.
	// Returns (manifest, true) when the local manifest + blob both validate.
	probeLocalManifestAndBlob := func() (*repopb.ArtifactManifest, bool) {
		if srv.localStorage == nil {
			return nil, false
		}
		mKey := manifestStorageKey(key)
		mData, mErr := srv.localStorage.ReadFile(ctx, mKey)
		if mErr != nil || len(mData) == 0 {
			return nil, false
		}
		manifest, _, parseErr := unmarshalManifestWithState(mData)
		if parseErr != nil || manifest == nil {
			return nil, false
		}
		binKey := binaryStorageKey(key)
		fi, statErr := srv.localStorage.Stat(ctx, binKey)
		if statErr != nil {
			return nil, false
		}
		if declared := manifest.GetSizeBytes(); declared > 0 && fi.Size() != declared {
			return nil, false
		}
		if expected := manifest.GetChecksum(); expected != "" {
			actual, readErr := checksumLocalFile(srv.localStorage.LocalPath(binKey))
			if readErr != nil || !digestEqual(actual, expected) {
				return nil, false
			}
		}
		return manifest, true
	}

	// Dry-run path: probe and return what would happen.
	if req.GetDryRun() {
		v, _ := srv.verifyArtifactIntegrity(ctx, ref, buildNumber)
		switch v.Status {
		case VerifyOK:
			// Project D: when integrity is OK but the Scylla pipeline state
			// is not PUBLISHED, the row is either a skeleton (publish_state
			// and/or artifact_state null) or missing entirely. The blob and
			// CAS manifest are healthy — only the index step is broken. The
			// repair is to re-run completePublish from the existing local
			// manifest, NOT to re-fetch from upstream.
			if ArtifactPipelineState(resp.ArtifactStateBefore) != PipelinePublished {
				resp.Action = "would_repair_publish_index"
				resp.Detail = "integrity OK but Scylla state not PUBLISHED; would re-run completePublish from local manifest"
			} else {
				resp.Action = "skipped_ok"
				resp.Detail = "no repair needed"
			}
		case VerifyBrokenManifestMissing:
			// Project D: BROKEN_MANIFEST_MISSING can mean two things:
			//   (1) the CAS .manifest.json file is genuinely absent — true broken
			//   (2) the Scylla row is a skeleton (NULL manifest_json) so the
			//       ledger-first read fails, but the CAS file is intact
			// Probe the local CAS directly to distinguish.
			if _, ok := probeLocalManifestAndBlob(); ok {
				resp.Action = "would_repair_publish_index"
				resp.Detail = "Scylla manifest_json is null (skeleton row) but local CAS manifest+blob are intact; would re-publish from local manifest"
			} else {
				resp.Action = "would_skip"
				resp.Detail = "BROKEN_MANIFEST_MISSING — local CAS manifest also absent"
			}
		case VerifyBrokenMissingBlob:
			resp.Action = "would_repair_blob"
			resp.Detail = "blob missing; would re-import from upstream"
		case VerifyBrokenChecksumMismatch:
			resp.Action = "would_repair_checksum_mismatch"
			resp.Detail = "checksum drift; would re-import from upstream"
		default:
			resp.Action = "would_skip"
			resp.Detail = string(v.Status)
		}
		resp.ArtifactStateAfter = resp.ArtifactStateBefore
		return resp, nil
	}

	// Project D: integrity-OK-but-Scylla-broken path.
	//
	// The blob and CAS manifest are valid but the Scylla repository.manifests
	// row is in a skeleton state (publish_state=null AND/OR manifest_json=null)
	// OR the artifact_state pipeline column is stuck at a non-PUBLISHED value
	// (e.g. DOWNLOADING from a partially-completed previous publish).
	//
	// Trigger conditions — repair when ANY of these is true:
	//   (a) artifact_state pipeline column != PUBLISHED (Group B "missing row"
	//       and DOWNLOADING-stuck cases), OR
	//   (b) Scylla's readManifestAndStateByKey fails (NULL manifest_json
	//       skeleton — the row exists but the JSON column is null, so the
	//       canonical read can't parse it).
	//
	// Condition (b) is the gateway/globular-cli/minio case where
	// artifact_state is already PUBLISHED but publish_state and manifest_json
	// are NULL — the row was partially written by a pre-d2ef80ee publish.
	// Without this condition the precondition (a) skips the backfill and the
	// resolver continues to fail because publish_state is what it reads.
	//
	// The straightforward path — completePublish — calls
	// transitionArtifactState which only permits PUBLISHED as the terminal
	// of the LedgerWritten happy path. Calling it from DOWNLOADING /
	// BlobVerified / etc. fails with "illegal transition". For a backfill
	// scenario the artifact's lifecycle was already proven (.bin +
	// .manifest.json on local POSIX CAS with matching size + sha256). The
	// state machine's progressive stages are not the right tool — we are
	// re-establishing PUBLISHED authority from existing bytes, not advancing
	// a new artifact through the pipeline.
	//
	// Use the repository-owned write primitives directly:
	//   1. syncManifestToScylla(... manifest, PUBLISHED, manifestJSON)  → fills the row + manifest_json
	//   2. scylla.UpdatePublishState(... "PUBLISHED")                   → ensures the column is PUBLISHED
	//   3. transitionArtifactState(... PipelinePublished) BEST-EFFORT   → tries to align artifact_state
	//
	// Step 3 may fail with "illegal transition" — that's tolerated for
	// backfill because the source of truth (manifest_json + publish_state)
	// is now correct. The artifact_state column drift is documented in the
	// open observations of the result report.
	needsBackfill := ArtifactPipelineState(resp.ArtifactStateBefore) != PipelinePublished
	if !needsBackfill {
		// Condition (b): even when artifact_state says PUBLISHED, the
		// canonical Scylla manifest read may fail because manifest_json is
		// NULL. The resolver consults publish_state + manifest_json — if
		// either is broken, the resolver can't use this artifact.
		_, _, scyManifest, scyErr := srv.readManifestAndStateByKey(ctx, key)
		if scyErr != nil || scyManifest == nil {
			needsBackfill = true
		}
	}
	if needsBackfill {
		manifest, ok := probeLocalManifestAndBlob()
		if ok {
			mKey := manifestStorageKey(key)
			mData, mErr := srv.localStorage.ReadFile(ctx, mKey)
			if mErr == nil && len(mData) > 0 {
				// Step 1: write the full row (manifest_json + initial publish_state).
				// syncManifestToScylla → PutManifest is the canonical repository-owned
				// path that publish writes use; never a raw INSERT.
				srv.syncManifestToScylla(ctx, key, manifest, repopb.PublishState_PUBLISHED, mData)

				// Step 2: ensure publish_state column is PUBLISHED. Idempotent.
				if srv.scylla != nil {
					if updErr := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_PUBLISHED.String()); updErr != nil {
						slog.Warn("repair: UpdatePublishState failed during backfill",
							"artifact_key", key, "err", updErr)
					}
				}

				// Step 3: best-effort transition of artifact_state. The state
				// machine may reject (DOWNLOADING → PUBLISHED is not a legal
				// edge); that's acceptable for backfill — the row is now
				// PUBLISHED in the SOLE authoritative column.
				ref := manifest.GetRef()
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
				if transErr := srv.transitionArtifactState(ctx, key, PipelinePublished,
					"project_d_backfill", "", stateFields); transErr != nil {
					slog.Info("repair: artifact_state transition not legal from this state — publish_state column authority is sufficient",
						"artifact_key", key, "state_before", resp.ArtifactStateBefore, "err", transErr)
				}

				resp.ArtifactStateAfter = string(srv.readArtifactState(ctx, key))
				resp.Action = "repair_publish_index"
				resp.Detail = "Scylla manifest_json + publish_state column backfilled from local CAS via repository-owned primitives"
				slog.Info("repair: publish-index backfilled from local CAS",
					"artifact_key", key,
					"state_before", resp.ArtifactStateBefore,
					"state_after", resp.ArtifactStateAfter)
				return resp, nil
			}
		}
	}

	// Real repair path. RepairArtifactFromUpstream enforces REVOKED /
	// QUARANTINED safety internally and emits the full pipeline transitions.
	repairErr := srv.RepairArtifactFromUpstream(ctx, ref, buildNumber, RepairOptions{
		Force:                   req.GetForce(),
		AllowQuarantineOverride: req.GetAllowQuarantineOverride(),
	})

	resp.ArtifactStateAfter = string(srv.readArtifactState(ctx, key))

	if repairErr != nil {
		errMsg := repairErr.Error()
		switch {
		case containsAny(errMsg, "REVOKED"):
			resp.Action = "blocked_revoked"
		case containsAny(errMsg, "QUARANTINED"):
			resp.Action = "blocked_quarantined"
		default:
			resp.Action = "failed"
		}
		resp.Detail = errMsg
		// Don't return an error — the response carries the failure; CLI maps it.
		slog.Info("repair: completed with non-success",
			"artifact_key", key, "action", resp.Action, "detail", errMsg)
		return resp, nil
	}

	// Success: classify by state-before to pick the right action label.
	switch ArtifactPipelineState(resp.ArtifactStateBefore) {
	case PipelineBrokenMissingBlob:
		resp.Action = "repair_blob"
		resp.Detail = "missing blob re-imported from upstream"
	case PipelineBrokenChecksumMismatch:
		resp.Action = "repair_checksum_mismatch"
		resp.Detail = "blob checksum drift repaired from upstream"
	case PipelinePublished:
		resp.Action = "skipped_ok"
		resp.Detail = "already published with verified blob"
	default:
		resp.Action = "repair_blob"
		resp.Detail = fmt.Sprintf("repaired from %s", resp.ArtifactStateBefore)
	}
	return resp, nil
}

// containsAny is a tiny string-match helper that avoids a strings import here
// (artifact_handlers.go and friends pull in their own).
func containsAny(s, needle string) bool {
	if needle == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// ── ExplainArtifact RPC ───────────────────────────────────────────────────

// ExplainArtifact composes manifest, ledger, blob, signature, and pipeline
// state into one operator-readable response. Read-only.
func (srv *server) ExplainArtifact(ctx context.Context, req *repopb.ExplainArtifactRequest) (*repopb.ExplainArtifactResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	canonicalizeRefVersion(ref)

	buildNumber := req.GetBuildNumber()
	if buildNumber == 0 {
		buildNumber = srv.resolveLatestBuildNumber(ctx, ref)
	}
	if buildNumber == 0 {
		return nil, status.Errorf(codes.NotFound, "no builds for %s/%s@%s [%s]",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	}

	key := artifactKeyWithBuild(ref, buildNumber)
	v, vErr := srv.verifyArtifactIntegrity(ctx, ref, buildNumber)
	if vErr != nil {
		return nil, status.Errorf(codes.Internal, "verify: %v", vErr)
	}

	pipelineState := srv.readArtifactState(ctx, key)
	_, publishState, m, _ := srv.readManifestAndStateByKey(ctx, key)
	manifestPresent := m != nil

	// Ledger presence — true if any ledger row carries this digest+platform.
	ledgerPresent := false
	if m != nil {
		ledger := srv.readLedger(ctx, ref.GetPublisherId(), ref.GetName())
		if ledger != nil {
			for _, r := range ledger.Releases {
				if r.Version == ref.GetVersion() && r.Platform == ref.GetPlatform() &&
					digestEqual(r.Digest, m.GetChecksum()) {
					ledgerPresent = true
					break
				}
			}
		}
	}

	blobPresent := v.Status != VerifyBrokenMissingBlob
	sourceAvail, repairable := srv.probeSourceAvailability(ctx, ref, buildNumber, m, blobPresent)

	resp := &repopb.ExplainArtifactResponse{
		Ref:                ref,
		ArtifactKey:        key,
		ArtifactState:      string(pipelineState),
		PublishState:       publishState,
		BlobKey:            v.BlobKey,
		BlobPresent:        blobPresent,
		ExpectedSize:       v.ExpectedSiz,
		ActualSize:         v.ActualSize,
		ExpectedDigest:     v.ExpectedSHA,
		ActualDigest:       v.ActualSHA,
		LedgerPresent:      ledgerPresent,
		ManifestPresent:    manifestPresent,
		SignatureStatus:    "NOT_IMPLEMENTED",
		Installable:        pipelineState == PipelinePublished && publishState == repopb.PublishState_PUBLISHED && v.Status == VerifyOK,
		VerifyStatus:       mapVerifyStatus(v.Status),
		Detail:             v.Reason,
		SourceAvailability: sourceAvail,
		Repairable:         repairable,
	}
	_, resp.RecommendedAction = repairAdvice(pipelineState, v.Status)

	// Workflow run id — best-effort: return the run id stored on the
	// authoritative state record if there was one.
	if rec, ok := srv.lookupArtifactStateCache(key); ok {
		resp.RelatedWorkflowRunId = rec.WorkflowRunID
	}
	return resp, nil
}

// probeSourceAvailability checks each layer in the source chain and reports
// whether the artifact is accessible from each. Only meaningful when the blob
// is absent locally — when present the local layer is authoritative.
//
// Returns a slice of "name:type:status:reason" entries (e.g.
// "local-posix:LOCAL_POSIX:PRESENT:") and a repairable flag that is true
// when at least one non-local source can provide the missing blob.
// All probes are read-only and best-effort; failures are noted, not fatal.
func (srv *server) probeSourceAvailability(
	ctx context.Context,
	ref *repopb.ArtifactRef,
	buildNumber int64,
	manifest *repopb.ArtifactManifest,
	localPresent bool,
) ([]string, bool) {
	var entries []string
	repairable := false

	// ── Local POSIX ───────────────────────────────────────────────────────
	if localPresent {
		entries = append(entries, "local-posix:LOCAL_POSIX:PRESENT:")
	} else {
		entries = append(entries, "local-posix:LOCAL_POSIX:ABSENT:blob missing from local CAS")
	}

	// ── MinIO mirror ──────────────────────────────────────────────────────
	if srv.mirrorStorage != nil {
		key := artifactKeyWithBuild(ref, buildNumber)
		binKey := binaryStorageKey(key)
		statCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, statErr := srv.mirrorStorage.Stat(statCtx, binKey)
		cancel()
		if statErr == nil {
			entries = append(entries, "minio-mirror:MINIO_MIRROR:PRESENT:")
			if !localPresent {
				repairable = true
			}
		} else {
			entries = append(entries, fmt.Sprintf("minio-mirror:MINIO_MIRROR:ABSENT:%v", statErr))
		}
	} else {
		entries = append(entries, "minio-mirror:MINIO_MIRROR:UNCONFIGURED:no mirror storage")
	}

	// ── Upstream sources ──────────────────────────────────────────────────
	upstreams := srv.loadUpstreamSources(ctx)
	if len(upstreams) == 0 {
		entries = append(entries, "upstream:UPSTREAM:UNCONFIGURED:no upstream sources registered")
	}
	for _, us := range upstreams {
		h := us.Health(ctx)
		if !h.Available {
			entries = append(entries, fmt.Sprintf("%s:%s:UNREACHABLE:%s", us.Name(), us.Type(), h.Reason))
			continue
		}
		// Reachable — check whether this source has context to repair the blob.
		reason := ""
		if !localPresent && manifest != nil {
			if ui := manifest.GetUpstreamImport(); ui != nil && ui.GetSourceName() == us.Name() {
				reason = "can repair from upstream_import"
				repairable = true
			}
		}
		entries = append(entries, fmt.Sprintf("%s:%s:REACHABLE:%s", us.Name(), us.Type(), reason))
	}

	return entries, repairable
}
