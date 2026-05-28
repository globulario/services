package main

// sync_from_upstream.go — Phase 3: Upstream source registry and sync.
//
// Implements four RPCs:
//   - RegisterUpstream  — store a named upstream source in etcd
//   - ListUpstreams     — enumerate all registered sources
//   - RemoveUpstream    — delete a named source
//   - SyncFromUpstream  — fetch a release index and import new artifacts
//
// Upstream sources live in etcd at /globular/repository/upstreams/<name>.
// All four RPCs require cluster-admin authorization (enforced by the authz
// interceptor via proto annotations).
//
// Sync identity model (from spec):
//   Artifact key = (publisher, name, version, platform)
//   Each key binds to exactly one immutable package_digest.
//   Same key + same digest → idempotent skip.
//   Same key + different digest → reject, emit audit event, do not store binary.
//
// Phase 1 constraints:
//   - release_tag is REQUIRED (empty tag returns InvalidArgument)
//   - "latest" discovery is not implemented (Phase 2)
//   - quarantine = QUARANTINED state (not installable)
//   - last_synced_tag advances only on a fully clean run (no rejected/failed)
//
// Dry-run invariant: dry_run=true NEVER writes to ScyllaDB, MinIO, or etcd.
// Preview results use SYNC_WOULD_* statuses exclusively.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/repository/upstream"
	"github.com/globulario/services/golang/workflow"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// ── etcd key schema ─────────────────────────────────────────────────────────

const upstreamEtcdPrefix = "/globular/repository/upstreams/"

func upstreamEtcdKey(name string) string { return upstreamEtcdPrefix + name }

// ── RegisterUpstream ─────────────────────────────────────────────────────────

func (srv *server) RegisterUpstream(ctx context.Context, req *repopb.RegisterUpstreamRequest) (*repopb.RegisterUpstreamResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}
	src := req.GetSource()
	if src == nil {
		return nil, status.Error(codes.InvalidArgument, "source is required")
	}
	name := strings.TrimSpace(src.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "source.name is required")
	}
	if src.GetIndexUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "source.index_url is required")
	}
	if err := upstream.ValidateIndexURLTemplate(src.GetIndexUrl()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "source.index_url invalid: %v", err)
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "etcd unavailable: %v", err)
	}

	data, err := protojson.Marshal(src)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal upstream source: %v", err)
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Put(tctx, upstreamEtcdKey(name), string(data)); err != nil {
		return nil, status.Errorf(codes.Internal, "etcd put upstream: %v", err)
	}

	slog.Info("upstream: registered", "name", name, "type", src.GetType(), "index_url", src.GetIndexUrl())
	return &repopb.RegisterUpstreamResponse{Source: src}, nil
}

// ── ListUpstreams ─────────────────────────────────────────────────────────────

func (srv *server) ListUpstreams(ctx context.Context, _ *repopb.ListUpstreamsRequest) (*repopb.ListUpstreamsResponse, error) {
	if err := srv.requireCapability(CapRepoQuery); err != nil {
		return nil, err
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "etcd unavailable: %v", err)
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, upstreamEtcdPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "etcd get upstreams: %v", err)
	}

	sources := make([]*repopb.UpstreamSource, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var src repopb.UpstreamSource
		if err := protojson.Unmarshal(kv.Value, &src); err != nil {
			slog.Warn("upstream: corrupt etcd entry", "key", string(kv.Key), "err", err)
			continue
		}
		// Redact credentials_ref in response — show presence but not the key path.
		if src.CredentialsRef != "" {
			src.CredentialsRef = "(set)"
		}
		sources = append(sources, &src)
	}
	return &repopb.ListUpstreamsResponse{Sources: sources}, nil
}

// ── RemoveUpstream ────────────────────────────────────────────────────────────

func (srv *server) RemoveUpstream(ctx context.Context, req *repopb.RemoveUpstreamRequest) (*repopb.RemoveUpstreamResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "etcd unavailable: %v", err)
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Delete(tctx, upstreamEtcdKey(name)); err != nil {
		return nil, status.Errorf(codes.Internal, "etcd delete upstream: %v", err)
	}
	slog.Info("upstream: removed", "name", name)
	return &repopb.RemoveUpstreamResponse{}, nil
}

// ── SyncFromUpstream ──────────────────────────────────────────────────────────
//
// The releaseIndex / releaseIndexEntry structs live in release_index.go —
// the canonical schema definition for release-index.json.

func (srv *server) SyncFromUpstream(ctx context.Context, req *repopb.SyncFromUpstreamRequest) (*repopb.SyncFromUpstreamResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}

	sourceName := strings.TrimSpace(req.GetSourceName())
	releaseTag := strings.TrimSpace(req.GetReleaseTag())
	dryRun := req.GetDryRun()

	resolveLatest := req.GetResolveLatest()

	if sourceName == "" {
		return nil, status.Error(codes.InvalidArgument, "source_name is required")
	}

	// ── Strict --latest semantics ─────────────────────────────────────────
	// tag set + resolve_latest=true → InvalidArgument
	if releaseTag != "" && resolveLatest {
		return nil, status.Error(codes.InvalidArgument, "cannot use both release_tag and resolve_latest — choose one")
	}
	// tag empty + resolve_latest=false → InvalidArgument
	if releaseTag == "" && !resolveLatest {
		return nil, status.Error(codes.InvalidArgument, "release_tag is required — specify an explicit tag (e.g. 'v1.0.17') or set resolve_latest=true")
	}

	// ── Load upstream source ────────────────────────────────────────────────
	src, err := srv.loadUpstreamSource(ctx, sourceName)
	if err != nil {
		return nil, err
	}
	if !src.GetEnabled() {
		return nil, status.Errorf(codes.FailedPrecondition, "upstream source %q is disabled", sourceName)
	}

	// ── Resolve credentials ──────────────────────────────────────────────
	var authToken string
	if credRef := src.GetCredentialsRef(); credRef != "" {
		tok, credErr := resolveCredentialFromEtcd(ctx, credRef)
		if credErr != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "resolve credentials_ref: %v", credErr)
		}
		authToken = tok
	}

	// ── Create provider via ReleaseSource abstraction ────────────────────
	sourceType := upstream.MapProtoType(int32(src.GetType()))
	provider, provErr := upstream.NewSource(sourceType)
	if provErr != nil {
		return nil, status.Errorf(codes.InvalidArgument, "upstream source %q: %v", sourceName, provErr)
	}
	opts := sourceOptsFromProto(src, authToken)

	// ── Discover latest release if requested ─────────────────────────────
	if resolveLatest {
		refs, listErr := provider.ListReleases(ctx, opts)
		if listErr != nil {
			return nil, status.Errorf(codes.Unavailable, "list releases from %q: %v", sourceName, listErr)
		}
		if len(refs) == 0 {
			return nil, status.Errorf(codes.NotFound, "no releases found in source %q", sourceName)
		}
		releaseTag = refs[0].Tag
		slog.Info("upstream: resolved latest release", "source", sourceName, "tag", releaseTag,
			"provider", provider.Type())
	}

	slog.Info("upstream: starting sync",
		"source", sourceName, "tag", releaseTag, "provider", provider.Type(), "dry_run", dryRun)

	// ── Start a workflow run for this sync (Phase B) ──────────────────────
	// The recorder is fire-and-forget. When the workflow service is not
	// reachable, RecordStep no-ops. Per-artifact transitions emit one
	// step each so the run history shows the full pipeline progression.
	var workflowRunID string
	if srv.workflowRec != nil && !dryRun {
		workflowRunID = srv.workflowRec.StartRun(ctx, &workflow.RunParams{
			ComponentName:    sourceName,
			ComponentKind:    workflow.KindService,
			ComponentVersion: releaseTag,
			ReleaseKind:      "RepositorySync",
			ReleaseObjectID:  fmt.Sprintf("%s/%s", sourceName, releaseTag),
			TriggerReason:    workflowpb.TriggerReason_TRIGGER_REASON_MANUAL,
			CorrelationID:    fmt.Sprintf("Sync/%s/%s", sourceName, releaseTag),
			WorkflowName:     "repository.sync.upstream",
		})
	}

	// ── Fetch release index via provider ─────────────────────────────────
	indexData, fetchErr := provider.GetReleaseIndex(ctx, opts, releaseTag)
	if fetchErr != nil {
		return nil, status.Errorf(codes.Unavailable, "fetch release index from %q: %v", sourceName, fetchErr)
	}
	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse release index: %v", err)
	}

	// ── Validate schema ───────────────────────────────────────────────────
	if err := ValidateReleaseIndex(idx); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid release index: %v", err)
	}

	// ── Filter by requested platform ───────────────────────────────────────
	targetPlatform := src.GetPlatform()
	if targetPlatform == "" {
		targetPlatform = "linux_amd64"
	}

	// ── Process each package ───────────────────────────────────────────────
	onlySet := make(map[string]bool)
	for _, n := range req.GetOnly() {
		onlySet[n] = true
	}

	results := make([]*repopb.UpstreamSyncResult, 0, len(idx.Packages))
	var imported, skipped, rejected, failed int32

	for _, entry := range idx.Packages {
		// Platform filter
		if entry.Platform != targetPlatform {
			continue
		}
		// Name filter
		if len(onlySet) > 0 && !onlySet[entry.Name] {
			continue
		}

		result := srv.processSyncEntry(ctx, entry, src, provider, opts, releaseTag, dryRun, workflowRunID)
		results = append(results, result)

		switch result.GetStatus() {
		case repopb.UpstreamSyncStatus_SYNC_IMPORTED, repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT:
			imported++
		case repopb.UpstreamSyncStatus_SYNC_SKIPPED, repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP:
			skipped++
		case repopb.UpstreamSyncStatus_SYNC_REJECTED, repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT:
			rejected++
		case repopb.UpstreamSyncStatus_SYNC_FAILED, repopb.UpstreamSyncStatus_SYNC_WOULD_FAIL:
			failed++
		}
	}

	// ── Update sync status on the upstream source record ──────────────────
	if !dryRun {
		syncStatus := "succeeded"
		var syncError string
		if rejected > 0 || failed > 0 {
			if imported > 0 {
				syncStatus = "partial"
			} else {
				syncStatus = "failed"
			}
			syncError = fmt.Sprintf("rejected=%d failed=%d", rejected, failed)
		}
		if updateErr := srv.updateSyncStatus(ctx, sourceName, releaseTag, syncStatus, syncError); updateErr != nil {
			slog.Warn("upstream: failed to update sync status (non-fatal)", "source", sourceName, "err", updateErr)
		}
	}

	mode := "real"
	if dryRun {
		mode = "dry-run"
	}
	slog.Info("upstream: sync complete",
		"source", sourceName, "tag", releaseTag, "mode", mode,
		"imported", imported, "skipped", skipped, "rejected", rejected, "failed", failed)

	if workflowRunID != "" && srv.workflowRec != nil {
		runStatus := workflow.Succeeded
		fc := workflow.NoFailure
		if failed > 0 || rejected > 0 {
			runStatus = workflow.Failed
			fc = workflowpb.FailureClass_FAILURE_CLASS_REPOSITORY
		}
		summary := fmt.Sprintf(
			"sync %s tag=%s imported=%d skipped=%d rejected=%d failed=%d",
			sourceName, releaseTag, imported, skipped, rejected, failed)
		var errMsg string
		if runStatus == workflow.Failed {
			errMsg = fmt.Sprintf("rejected=%d failed=%d", rejected, failed)
		}
		srv.workflowRec.FinishRun(ctx, workflowRunID, runStatus, summary, errMsg, fc)
	}

	return &repopb.SyncFromUpstreamResponse{
		Results:     results,
		Imported:    imported,
		Skipped:     skipped,
		Rejected:    rejected,
		Failed:      failed,
		DryRun:      dryRun,
		ResolvedTag: releaseTag,
		SourceName:  sourceName,
	}, nil
}

// processSyncEntry evaluates one release index entry and either imports it or
// produces a preview/skip/reject result. Never panics — all errors become
// SYNC_FAILED or SYNC_WOULD_FAIL results.
//
// Step 1: Normalize entry (resolve publisher, channel, build_number, build_id).
// Step 2: Check import policy against normalized identity.
// Step 3: Conflict detection using normalized identity.
// Step 4: Download, verify, import.
func (srv *server) processSyncEntry(
	ctx context.Context,
	entry *releaseIndexEntry,
	src *repopb.UpstreamSource,
	provider upstream.ReleaseSource,
	opts upstream.SourceOpts,
	releaseTag string,
	dryRun bool,
	workflowRunID string,
) *repopb.UpstreamSyncResult {

	// ── Step 1: Normalize ─────────────────────────────────────────────────
	n := normalizeReleaseEntry(entry, src)
	n.PlatformRelease = releaseTag // platform release = the release being synced

	result := &repopb.UpstreamSyncResult{
		Name:            n.Name,
		Version:         n.Version,
		BuildId:         n.BuildID,
		Platform:        n.Platform,
		PackageDigest:   n.Digest,
		Publisher:       n.Publisher,
		Kind:            n.Kind,
		Channel:         n.Channel,
		BuildNumber:     n.BuildNumber,
		ChecksumPresent: n.Digest != "",
	}

	// Populate local version from ledger if available.
	ledger := srv.readLedger(ctx, n.Publisher, n.Name)
	if ledger != nil && ledger.LatestVersion != "" {
		result.LocalVersion = ledger.LatestVersion
		// Find the latest build_number for this version.
		for _, r := range ledger.Releases {
			if r.Version == ledger.LatestVersion {
				// Parse build_number from ledger BuildID if numeric, else leave 0.
				// The ledger stores build_id, not build_number directly.
				result.LocalBuildNumber = 0
				break
			}
		}
	}

	// ── Step 2: Policy filtering ──────────────────────────────────────────
	if reason, rej := checkImportPolicy(n, src); rej {
		if dryRun {
			result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT
		} else {
			result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
			srv.publishAuditEvent(ctx, "upstream.policy_rejected", map[string]any{
				"source":    src.GetName(),
				"package":   n.Name,
				"version":   n.Version,
				"publisher": n.Publisher,
				"kind":      n.Kind,
				"channel":   n.Channel,
				"reason":    reason,
			})
		}
		result.Detail = reason
		result.Action = "blocked"
		result.BlockedReason = reason
		return result
	}

	// ── Step 3: Conflict detection + blob verification ────────────────────
	//
	// INVARIANT: An artifact is "up_to_date" only if ALL hold:
	//   - metadata exists (ledger row OR Scylla manifest)
	//   - the exact binary blob exists at binaryStorageKey(artifactKeyWithBuild(...))
	//   - the blob size matches the recorded size_bytes
	//   - the artifact_state is PUBLISHED (or empty for legacy rows)
	//
	// Metadata alone — even with matching digest — is NOT sufficient. No
	// skip path may bypass any of these checks.
	ref := &repopb.ArtifactRef{
		PublisherId: n.Publisher,
		Name:        n.Name,
		Version:     n.Version,
		Platform:    n.Platform,
	}
	artifactKey := artifactKeyWithBuild(ref, n.BuildNumber)

	// repairReason carries why we are reimporting despite metadata being present:
	// e.g. "missing_blob" or "size_mismatch". Empty when the artifact is genuinely
	// new/updated. Distinguishes "repair an existing record" from "first import".
	var repairReason string

	stateFields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(artifactKey),
		Checksum:    n.Digest,
		SizeBytes:   0, // populated post-download with actual size
		BuildID:     n.BuildID,
		BuildNumber: n.BuildNumber,
		PublisherID: n.Publisher,
		Name:        n.Name,
		Version:     n.Version,
		Platform:    n.Platform,
	}

	// Build-ID identity gate: same build_id is the same artifact identity.
	// Never create duplicate rows for an already-known build_id when the local
	// build_number is already equal or newer.
	if existingByID, stateByID, _, ok := srv.findExistingArtifactByBuildID(ctx, ref, n.BuildID); ok {
		if !digestEqual(existingByID.GetChecksum(), n.Digest) {
			result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
			result.Detail = fmt.Sprintf("build_id conflict: same build_id=%s but digest differs local=%s... upstream=%s...",
				n.BuildID, truncDigest(existingByID.GetChecksum()), truncDigest(n.Digest))
			result.Action = "conflict"
			return result
		}
		if existingByID.GetBuildNumber() >= n.BuildNumber {
			blobKey := blobKeyForRef(ref, existingByID.GetBuildNumber())
			blobPresent, blobReason := srv.artifactBlobStatus(ctx, ref, existingByID.GetBuildNumber(), existingByID.GetSizeBytes())
			if blobPresent {
				stateOK, currentState := srv.canSkipDueToExistingState(ctx, artifactKeyWithBuild(ref, existingByID.GetBuildNumber()))
				if !stateOK {
					logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
						n.BuildID, existingByID.GetBuildNumber(), n.Digest, blobKey,
						"reprocess", "pipeline_state_not_publishable:"+string(currentState))
				} else {
					logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
						n.BuildID, existingByID.GetBuildNumber(), n.Digest, blobKey,
						"skip", "same_build_id_blob_verified")
					result.LocalBuildNumber = existingByID.GetBuildNumber()
					if dryRun {
						result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
					} else {
						result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
					}
					result.Action = "up_to_date"
					result.Detail = fmt.Sprintf("same build_id already present at build_number=%d (upstream=%d, state=%s); blob verified",
						existingByID.GetBuildNumber(), n.BuildNumber, stateByID.String())
					return result
				}
			} else {
				repairReason = blobReason
				stateFields.SizeBytes = existingByID.GetSizeBytes()
				srv.markBrokenForReason(ctx, artifactKeyWithBuild(ref, existingByID.GetBuildNumber()), blobReason, workflowRunID, stateFields)
				logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
					n.BuildID, existingByID.GetBuildNumber(), n.Digest, blobKey, "repair_blob", blobReason)
			}
		}
		// existing build_id is older build_number; proceed to import at upstream
		// build_number so highest build_number is retained for this artifact.
	}

	if ledger != nil {
		for _, r := range ledger.Releases {
			if r.Version == n.Version && r.BuildID == n.BuildID && r.Platform == n.Platform {
				if digestEqual(r.Digest, n.Digest) {
					// Digest matches — verify the exact binary blob exists AND
					// matches the recorded size. Stat-only check using the same
					// key DownloadArtifact uses; never trust manifest/ledger alone.
					blobKey := blobKeyForRef(ref, n.BuildNumber)
					blobPresent, blobReason := srv.artifactBlobStatus(ctx, ref, n.BuildNumber, r.SizeBytes)

					if blobPresent {
						// Skip is only legal when artifact_state is also coherent.
						stateOK, currentState := srv.canSkipDueToExistingState(ctx, artifactKey)
						if !stateOK {
							logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
								n.BuildID, n.BuildNumber, n.Digest, blobKey,
								"reprocess", "pipeline_state_not_publishable:"+string(currentState))
							slog.Info("repository sync: blob present but artifact_state not publishable; reprocessing",
								"source", src.GetName(), "artifact_key", artifactKey,
								"artifact_state", string(currentState))
							// Carry forward through the import path so the
							// state machine resumes from the right place.
							repairReason = "pipeline_state:" + string(currentState)
							break
						}
						logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
							n.BuildID, n.BuildNumber, n.Digest, blobKey, "skip", "ledger_match_blob_verified")
						// Idempotent state stamp — Unspecified/PUBLISHED → PUBLISHED is allowed.
						stateFields.SizeBytes = r.SizeBytes
						_ = srv.transitionArtifactState(ctx, artifactKey, PipelinePublished,
							"sync_skip_idempotent", workflowRunID, stateFields)
						if dryRun {
							result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
						} else {
							result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
						}
						result.Detail = "already present with matching digest; blob verified"
						result.Action = "up_to_date"
						return result
					}

					// Metadata + digest matched but the binary is missing or
					// corrupted (size mismatch). Mark broken state, then force
					// re-import to repair.
					repairReason = blobReason
					stateFields.SizeBytes = r.SizeBytes
					srv.markBrokenForReason(ctx, artifactKey, blobReason, workflowRunID, stateFields)
					logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
						n.BuildID, n.BuildNumber, n.Digest, blobKey, "repair_blob", blobReason)
					slog.Info("repository sync: metadata exists but blob missing; re-importing",
						"source", src.GetName(), "publisher", n.Publisher,
						"name", n.Name, "version", n.Version, "platform", n.Platform,
						"build_id", n.BuildID, "build_number", n.BuildNumber,
						"digest", truncDigest(n.Digest), "blob_key", blobKey,
						"blob_status", blobReason)
					// Don't return — fall through to download/import path.
					break
				}
				detail := fmt.Sprintf("digest conflict: local=%s... upstream=%s...",
					truncDigest(r.Digest), truncDigest(n.Digest))
				if dryRun {
					result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT
					result.Detail = detail
				} else {
					result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
					result.Detail = detail
					srv.publishAuditEvent(ctx, "upstream.digest_conflict", map[string]any{
						"source":          src.GetName(),
						"release_tag":     releaseTag,
						"package_name":    n.Name,
						"version":         n.Version,
						"platform":        n.Platform,
						"local_digest":    r.Digest,
						"upstream_digest": n.Digest,
						"asset_url":       upstream.RedactAssetURL(n.AssetURL),
					})
				}
				result.Action = "conflict"
				return result
			}
		}
	}

	// Second metadata source: ScyllaDB manifest rows (or cached MinIO scan
	// when Scylla is unavailable). A digest match here also requires the exact
	// blob to be present at the matching build_number AND the artifact_state
	// to be PUBLISHED (or legacy-empty).
	if existing, state, _, ok := srv.findExistingArtifactByDigest(ctx, ref, n.Digest); ok {
		blobKey := blobKeyForRef(ref, existing.GetBuildNumber())
		blobPresent, blobReason := srv.artifactBlobStatus(ctx, ref, existing.GetBuildNumber(), existing.GetSizeBytes())
		// The Scylla branch may resolve a different build_number than n —
		// rebuild the artifact key for the state record at that build.
		existingKey := artifactKeyWithBuild(ref, existing.GetBuildNumber())
		if blobPresent {
			// Idempotent skip ONLY when the upstream build_number matches the
			// existing one. If they differ (e.g. bootstrap published build_number=1
			// before sync ran, but the BOM uses build_number=171), we must import
			// at n.BuildNumber so the controller can satisfy its requirement.
			if existing.GetBuildNumber() == n.BuildNumber {
				stateOK, currentState := srv.canSkipDueToExistingState(ctx, existingKey)
				if !stateOK {
					logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
						n.BuildID, existing.GetBuildNumber(), n.Digest, blobKey,
						"reprocess", "pipeline_state_not_publishable:"+string(currentState))
					slog.Info("repository sync: scylla blob present but artifact_state not publishable; reprocessing",
						"source", src.GetName(), "artifact_key", existingKey,
						"artifact_state", string(currentState))
					if repairReason == "" {
						repairReason = "pipeline_state:" + string(currentState)
					}
				} else {
					logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
						n.BuildID, existing.GetBuildNumber(), n.Digest, blobKey, "skip", "scylla_match_blob_verified")
					skipFields := stateFields
					skipFields.BlobKey = blobKey
					skipFields.SizeBytes = existing.GetSizeBytes()
					skipFields.BuildNumber = existing.GetBuildNumber()
					_ = srv.transitionArtifactState(ctx, existingKey, PipelinePublished,
						"sync_skip_idempotent", workflowRunID, skipFields)
					detail := fmt.Sprintf("already present with matching digest at build %d (%s); blob verified",
						existing.GetBuildNumber(), state.String())
					if dryRun {
						result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
					} else {
						result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
					}
					result.Detail = detail
					result.Action = "up_to_date"
					if err := srv.ensureReleaseBuildAlias(ctx, ref, releaseTag, n.BuildNumber, n.BuildID, existing.GetBuildId(), n.Digest, n.OriginRelease, src.GetName()); err != nil {
						if isAliasConflictError(err) {
							if dryRun {
								result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT
							} else {
								result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
							}
							result.Action = "conflict"
							result.Detail = fmt.Sprintf("alias conflict for %s build_number=%d: %v", releaseTag, n.BuildNumber, err)
							return result
						}
						slog.Warn("repository sync: failed to persist release/build alias", "err", err,
							"release_tag", releaseTag, "build_number", n.BuildNumber, "canonical_build_id", existing.GetBuildId())
					}
					return result
				}
			} else {
				// Same digest at a different build_number is a dedupe/alias case.
				// Canonical identity remains the existing build; do not duplicate
				// artifact rows solely to mirror upstream build_number locators.
				slog.Info("repository sync: same digest exists at different build_number — deduping to canonical local build",
					"source", src.GetName(), "publisher", n.Publisher,
					"name", n.Name, "version", n.Version, "platform", n.Platform,
					"existing_build_number", existing.GetBuildNumber(),
					"upstream_build_number", n.BuildNumber,
					"digest", truncDigest(n.Digest))
				detail := fmt.Sprintf("already present with matching digest at canonical build %d (upstream build_number=%d); deduped",
					existing.GetBuildNumber(), n.BuildNumber)
				if dryRun {
					result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
				} else {
					result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
				}
				result.Detail = detail
				result.Action = "up_to_date"
				if err := srv.ensureReleaseBuildAlias(ctx, ref, releaseTag, n.BuildNumber, n.BuildID, existing.GetBuildId(), n.Digest, n.OriginRelease, src.GetName()); err != nil {
					if isAliasConflictError(err) {
						if dryRun {
							result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT
						} else {
							result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
						}
						result.Action = "conflict"
						result.Detail = fmt.Sprintf("alias conflict for %s build_number=%d: %v", releaseTag, n.BuildNumber, err)
						return result
					}
					slog.Warn("repository sync: failed to persist release/build alias", "err", err,
						"release_tag", releaseTag, "build_number", n.BuildNumber, "canonical_build_id", existing.GetBuildId())
				}
				return result
			}
		} else {
			// Scylla/manifest record found but the blob is missing or wrong
			// size. Mark broken (publish_state→CORRUPTED if size mismatch and
			// row was PUBLISHED), then fall through to repair.
			if repairReason == "" {
				repairReason = blobReason
			}
			brokenFields := stateFields
			brokenFields.BlobKey = blobKey
			brokenFields.SizeBytes = existing.GetSizeBytes()
			brokenFields.BuildNumber = existing.GetBuildNumber()
			srv.markBrokenForReason(ctx, existingKey, blobReason, workflowRunID, brokenFields)
			logBlobSkipDecision(src.GetName(), n.Publisher, n.Name, n.Version, n.Platform,
				n.BuildID, existing.GetBuildNumber(), n.Digest, blobKey, "repair_blob", blobReason)
			slog.Info("repository sync: metadata exists but blob missing; re-importing",
				"source", src.GetName(), "publisher", n.Publisher,
				"name", n.Name, "version", n.Version, "platform", n.Platform,
				"build_id", n.BuildID, "build_number", existing.GetBuildNumber(),
				"digest", truncDigest(n.Digest), "blob_key", blobKey,
				"blob_status", blobReason, "match_source", "scylla")
		}
	}

	// Not found or blob missing → would import (or import).
	// Determine action: repair_blob when metadata existed but the binary did
	// not (or was corrupted). Otherwise classify as new/update/ahead.
	switch {
	case repairReason != "":
		result.Action = "repair_blob"
	case result.LocalVersion == "":
		result.Action = "new"
	case result.LocalVersion < n.Version:
		result.Action = "update"
	case result.LocalVersion > n.Version:
		result.Action = "ahead"
	default:
		result.Action = "new" // same version, different build
	}

	if dryRun {
		result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT
		loc := n.AssetURL
		if loc == "" {
			loc = n.AssetPath
		}
		if loc == "" {
			loc = n.Filename
		}
		if repairReason != "" {
			result.Detail = fmt.Sprintf("would re-import from %s to repair %s (build_number=%d)",
				upstream.RedactAssetURL(loc), repairReason, n.BuildNumber)
		} else {
			result.Detail = fmt.Sprintf("would download from %s (build_number=%d)",
				upstream.RedactAssetURL(loc), n.BuildNumber)
		}
		return result
	}

	// ── Step 4: Pipeline state transitions for the import path ───────────
	// DISCOVERED first if and only if the row is currently empty (legal
	// transitions ban PUBLISHED → DISCOVERED, and repair from BROKEN_X
	// transitions directly to DOWNLOADING).
	if curr := srv.readArtifactState(ctx, artifactKey); curr == PipelineUnspecified {
		_ = srv.transitionArtifactState(ctx, artifactKey, PipelineDiscovered,
			"observed_in_release_index:"+releaseTag, workflowRunID, stateFields)
	}
	_ = srv.transitionArtifactState(ctx, artifactKey, PipelineDownloading,
		"download_started", workflowRunID, stateFields)

	// ── Step 5: Download and verify via provider ─────────────────────────
	if !n.ChangedInRelease && n.OriginRelease != releaseTag {
		slog.Info("upstream: importing unchanged package from origin release",
			"name", n.Name, "version", n.Version, "origin", n.OriginRelease, "platform_release", releaseTag)
	}
	data, digest, err := downloadAndVerifyFromProvider(ctx, provider, opts, n)
	if err != nil {
		// State stays at DOWNLOADING with the error reason; sync surfaces SYNC_FAILED.
		result.Status = repopb.UpstreamSyncStatus_SYNC_FAILED
		result.Detail = fmt.Sprintf("download/verify failed: %v", err)
		slog.Warn("upstream: download failed", "name", n.Name, "err", err)
		return result
	}
	stateFields.Checksum = digest
	stateFields.SizeBytes = int64(len(data))

	if importErr := srv.importUpstreamArtifact(ctx, n, data, digest, src, releaseTag, stateFields, workflowRunID); importErr != nil {
		result.Status = repopb.UpstreamSyncStatus_SYNC_FAILED
		result.Detail = fmt.Sprintf("import failed: %v", importErr)
		slog.Error("upstream: import failed", "name", n.Name, "version", n.Version, "err", importErr)
		return result
	}

	result.Status = repopb.UpstreamSyncStatus_SYNC_IMPORTED
	switch {
	case repairReason != "":
		result.Action = "repair_blob"
		result.Detail = fmt.Sprintf(
			"metadata existed but binary blob was %s; re-imported from %s",
			repairReason, src.GetName())
	case strings.EqualFold(src.GetTrustPolicy(), "quarantine"):
		result.Detail = fmt.Sprintf("imported from %s (quarantined — requires manual promotion)", src.GetName())
	default:
		result.Detail = fmt.Sprintf("imported from %s", src.GetName())
	}
	slog.Info("upstream: imported",
		"source", src.GetName(), "publisher", n.Publisher,
		"name", n.Name, "version", n.Version, "platform", n.Platform,
		"build_number", n.BuildNumber, "build_id", n.BuildID,
		"digest", truncDigest(digest), "action", result.Action,
		"repair_reason", repairReason)
	return result
}

// importUpstreamArtifact stores the downloaded artifact binary + manifest and
// appends it to the release ledger. Uses normalizedEntry for all identity
// fields and emits the BLOB_WRITTEN → BLOB_VERIFIED → MANIFEST_WRITTEN →
// LEDGER_WRITTEN → PUBLISHED pipeline transitions.
//
// Phase A contract change: ledger append is REQUIRED before PUBLISHED. If
// appendToLedger fails, the artifact is left in MANIFEST_WRITTEN with a
// reason and the function returns an error — the resolver will not see it.
func (srv *server) importUpstreamArtifact(
	ctx context.Context,
	n *normalizedEntry,
	data []byte,
	digest string,
	src *repopb.UpstreamSource,
	releaseTag string,
	stateFields ArtifactStateFields,
	workflowRunID string,
) error {
	ref := &repopb.ArtifactRef{
		PublisherId: n.Publisher,
		Name:        n.Name,
		Version:     n.Version,
		Platform:    n.Platform,
	}
	key := artifactKeyWithBuild(ref, n.BuildNumber)

	if existingByID, stateByID, _, ok := srv.findExistingArtifactByBuildID(ctx, ref, n.BuildID); ok {
		if !digestEqual(existingByID.GetChecksum(), digest) {
			return fmt.Errorf("build_id conflict: same build_id=%s has different digest local=%s upstream=%s",
				n.BuildID, existingByID.GetChecksum(), digest)
		}
		if existingByID.GetBuildNumber() >= n.BuildNumber {
			blobPresent, blobReason := srv.artifactBlobStatus(ctx, ref, existingByID.GetBuildNumber(), existingByID.GetSizeBytes())
			if blobPresent {
				{
					existingBuildKey := artifactKeyWithBuild(ref, existingByID.GetBuildNumber())
					curr := srv.readArtifactState(ctx, existingBuildKey)
					idemFields := stateFields
					idemFields.BuildNumber = existingByID.GetBuildNumber()
					idemFields.SizeBytes = existingByID.GetSizeBytes()
					switch curr {
					case PipelineManifestWritten:
						// Ledger append was interrupted on a prior sync; retry to complete
						// the pipeline. appendToLedger is idempotent for the same build_id.
						if ledgerErr := srv.appendToLedger(ctx, n.Publisher, n.Name, n.Version,
							n.BuildID, existingByID.GetChecksum(), n.Platform, existingByID.GetSizeBytes()); ledgerErr != nil {
							slog.Error("upstream: idempotent ledger retry failed — artifact stays at MANIFEST_WRITTEN",
								"key", existingBuildKey, "err", ledgerErr)
							return fmt.Errorf("append to ledger (retry): %w", ledgerErr)
						}
						_ = srv.transitionArtifactState(ctx, existingBuildKey, PipelineLedgerWritten,
							"ledger_retry_success", workflowRunID, idemFields)
						_ = srv.transitionArtifactState(ctx, existingBuildKey, PipelinePublished,
							"import_idempotent_resume", workflowRunID, idemFields)
					case PipelineLedgerWritten:
						_ = srv.transitionArtifactState(ctx, existingBuildKey, PipelinePublished,
							"import_idempotent_resume", workflowRunID, idemFields)
					default:
						_ = srv.transitionArtifactState(ctx, existingBuildKey, PipelinePublished,
							"import_idempotent_skip", workflowRunID, idemFields)
					}
				}
				slog.Info("upstream: same build_id already present at equal/higher build_number, skipping import",
					"source", src.GetName(), "publisher", n.Publisher,
					"name", n.Name, "version", n.Version, "platform", n.Platform,
					"build_id", n.BuildID,
					"existing_build_number", existingByID.GetBuildNumber(),
					"upstream_build_number", n.BuildNumber,
					"publish_state", stateByID.String())
				if err := srv.ensureReleaseBuildAlias(ctx, ref, releaseTag, n.BuildNumber, n.BuildID, existingByID.GetBuildId(), digest, n.OriginRelease, src.GetName()); err != nil {
					if isAliasConflictError(err) {
						return fmt.Errorf("alias conflict for %s build_number=%d: %w", releaseTag, n.BuildNumber, err)
					}
					slog.Warn("upstream: failed to persist release/build alias", "err", err,
						"release_tag", releaseTag, "build_number", n.BuildNumber, "canonical_build_id", existingByID.GetBuildId())
				}
				return nil
			}
			slog.Info("upstream: same build_id present but blob not healthy; continuing import for repair",
				"source", src.GetName(), "publisher", n.Publisher,
				"name", n.Name, "version", n.Version, "platform", n.Platform,
				"build_id", n.BuildID,
				"existing_build_number", existingByID.GetBuildNumber(),
				"upstream_build_number", n.BuildNumber,
				"blob_status", blobReason)
		}
	}

	if existing, state, _, ok := srv.findExistingArtifactByDigest(ctx, ref, digest); ok {
		// Verify the exact binary blob exists at the matching build_number AND
		// matches the recorded size. Otherwise fall through to re-create it.
		blobPresent, blobReason := srv.artifactBlobStatus(ctx, ref, existing.GetBuildNumber(), existing.GetSizeBytes())
		if blobPresent {
			// Idempotent skip ONLY when the build_number also matches. If the
			// upstream's build_number differs from the existing one (e.g. a local
			// bootstrap publish created build_number=1 before the sync ran, but the
			// BOM records build_number=171), we must still import at n.BuildNumber
			// so the controller can satisfy its build_number requirement. The blob
			// bytes are the same — only the metadata record differs.
			if existing.GetBuildNumber() == n.BuildNumber {
				existingKey := artifactKeyWithBuild(ref, existing.GetBuildNumber())
				{
					curr := srv.readArtifactState(ctx, existingKey)
					idemFields := stateFields
					idemFields.BuildNumber = existing.GetBuildNumber()
					idemFields.SizeBytes = existing.GetSizeBytes()
					switch curr {
					case PipelineManifestWritten:
						if ledgerErr := srv.appendToLedger(ctx, n.Publisher, n.Name, n.Version,
							existing.GetBuildId(), digest, n.Platform, existing.GetSizeBytes()); ledgerErr != nil {
							slog.Error("upstream: idempotent ledger retry (digest path) failed — artifact stays at MANIFEST_WRITTEN",
								"key", existingKey, "err", ledgerErr)
							return fmt.Errorf("append to ledger (retry): %w", ledgerErr)
						}
						_ = srv.transitionArtifactState(ctx, existingKey, PipelineLedgerWritten,
							"ledger_retry_success", workflowRunID, idemFields)
						_ = srv.transitionArtifactState(ctx, existingKey, PipelinePublished,
							"import_idempotent_resume", workflowRunID, idemFields)
					case PipelineLedgerWritten:
						_ = srv.transitionArtifactState(ctx, existingKey, PipelinePublished,
							"import_idempotent_resume", workflowRunID, idemFields)
					default:
						_ = srv.transitionArtifactState(ctx, existingKey, PipelinePublished,
							"import_idempotent_skip", workflowRunID, idemFields)
					}
				}
				slog.Info("upstream: identical artifact already exists (blob verified), skipping import",
						"source", src.GetName(), "publisher", n.Publisher,
						"name", n.Name, "version", n.Version, "platform", n.Platform,
						"build_number", existing.GetBuildNumber(), "build_id", existing.GetBuildId(),
						"digest", truncDigest(digest),
						"blob_key", blobKeyForRef(ref, existing.GetBuildNumber()),
						"publish_state", state.String())
					if err := srv.ensureReleaseBuildAlias(ctx, ref, releaseTag, n.BuildNumber, n.BuildID, existing.GetBuildId(), digest, n.OriginRelease, src.GetName()); err != nil {
						if isAliasConflictError(err) {
							return fmt.Errorf("alias conflict for %s build_number=%d: %w", releaseTag, n.BuildNumber, err)
						}
						slog.Warn("upstream: failed to persist release/build alias", "err", err,
							"release_tag", releaseTag, "build_number", n.BuildNumber, "canonical_build_id", existing.GetBuildId())
					}
					return nil
				}
			// Same digest but different build_number is dedupe/alias territory.
			// Keep canonical local artifact identity; do not duplicate rows only
			// to mirror an upstream locator.
			slog.Info("upstream: same digest exists at different build_number — deduping to canonical local build",
				"source", src.GetName(), "publisher", n.Publisher,
				"name", n.Name, "version", n.Version, "platform", n.Platform,
				"existing_build_number", existing.GetBuildNumber(),
				"upstream_build_number", n.BuildNumber,
				"digest", truncDigest(digest))
			if err := srv.ensureReleaseBuildAlias(ctx, ref, releaseTag, n.BuildNumber, n.BuildID, existing.GetBuildId(), digest, n.OriginRelease, src.GetName()); err != nil {
				if isAliasConflictError(err) {
					return fmt.Errorf("alias conflict for %s build_number=%d: %w", releaseTag, n.BuildNumber, err)
				}
				slog.Warn("upstream: failed to persist release/build alias", "err", err,
					"release_tag", releaseTag, "build_number", n.BuildNumber, "canonical_build_id", existing.GetBuildId())
			}
			return nil
		} else {
			slog.Info("repository sync: metadata exists but blob missing; re-importing",
				"source", src.GetName(), "publisher", n.Publisher,
				"name", n.Name, "version", n.Version, "platform", n.Platform,
				"build_number", existing.GetBuildNumber(), "build_id", existing.GetBuildId(),
				"digest", truncDigest(digest),
				"blob_key", blobKeyForRef(ref, existing.GetBuildNumber()),
				"blob_status", blobReason,
				"context", "importUpstreamArtifact")
		}
	}

	// Write binary atomically to the local POSIX CAS.
	// MaterializeArtifactToLocal: streams through sha256 verify, atomic rename,
	// and drives DOWNLOADING → BLOB_WRITTEN → BLOB_VERIFIED state transitions.
	matReq := ArtifactRequest{
		PublisherID:   n.Publisher,
		Name:          n.Name,
		Version:       n.Version,
		Platform:      n.Platform,
		BuildNumber:   n.BuildNumber,
		BuildID:       n.BuildID,
		Sha256:        digest,
		SizeBytes:     int64(len(data)),
		WorkflowRunID: workflowRunID,
	}
	matCandidate := &ArtifactCandidate{
		SourceName: src.GetName(),
		SourceType: upstream.MapProtoType(int32(src.GetType())),
		Reader:     io.NopCloser(bytes.NewReader(data)),
		SizeBytes:  int64(len(data)),
		Sha256:     digest,
	}
	if _, matErr := srv.MaterializeArtifactToLocal(ctx, matReq, matCandidate); matErr != nil {
		return fmt.Errorf("write binary: %w", matErr)
	}

	// Best-effort mirror write: populate MinIO so other nodes can read from the
	// mirror tier. Local POSIX CAS is already authoritative; mirror failure is
	// non-fatal and does not affect pipeline state.
	if srv.mirrorStorage != nil {
		if mirrorErr := srv.mirrorStorage.WriteFile(ctx, binaryStorageKey(key), data, 0o644); mirrorErr != nil {
			slog.Warn("upstream sync: mirror write failed — local CAS intact",
				"key", key, "err", mirrorErr)
		}
	}

	manifest := &repopb.ArtifactManifest{
		Ref:                ref,
		BuildNumber:        n.BuildNumber,
		BuildId:            n.BuildID,
		Checksum:           digest,
		SizeBytes:          int64(len(data)),
		EntrypointChecksum: n.EntrypointChecksum,
		Provisional:        false,
		Channel:            channelFromString(n.Channel),
		UpstreamImport: &repopb.UpstreamImportRecord{
			SourceName:             src.GetName(),
			ReleaseTag:             releaseTag,
			AssetUrl:               resolveProvenanceAssetURL(n),
			IndexUrl:               strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag),
			ImportedAt:             time.Now().Unix(),
			Publisher:              n.Publisher,
			Kind:                   n.Kind,
			Channel:                n.Channel,
			BuildNumber:            n.BuildNumber,
			Checksum:               digest,
			OriginRelease:          n.OriginRelease,
			ChangedInRelease:       n.ChangedInRelease,
			PlatformRelease:        n.PlatformRelease,
			PackageContractDigest:  n.PackageContractDigest,
		},
	}

	if kind, ok := kindFromArtifactKindString(n.Kind); ok {
		manifest.Ref.Kind = kind
	}

	// Enrich manifest from package.json inside the archive.
	if pkg := extractPackageManifest(data); pkg != nil {
		enrichManifestFromPackageJSON(manifest, pkg)
	}

	targetState := importTargetState(src)
	mjson, err := marshalManifestWithState(manifest, targetState)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	srv.syncManifestToScylla(ctx, key, manifest, targetState, mjson)
	_ = srv.transitionArtifactState(ctx, key, PipelineManifestWritten,
		"manifest_persisted", workflowRunID, stateFields)

	// Phase A: ledger write is REQUIRED before PUBLISHED. If it fails, the
	// artifact stays at MANIFEST_WRITTEN — resolver will not return it.
	if ledgerErr := srv.appendToLedger(ctx, n.Publisher, n.Name, n.Version,
		n.BuildID, digest, n.Platform, int64(len(data))); ledgerErr != nil {
		slog.Error("upstream: ledger append failed — leaving artifact at MANIFEST_WRITTEN",
			"name", n.Name, "err", ledgerErr)
		return fmt.Errorf("append to ledger: %w", ledgerErr)
	}
	_ = srv.transitionArtifactState(ctx, key, PipelineLedgerWritten,
		"ledger_persisted", workflowRunID, stateFields)

	// Phase F signature policy: if the source provider reports a non-LOCAL
	// origin AND policy requires a trusted signature for this publisher,
	// AND no valid signature is registered yet, transition to QUARANTINED
	// rather than PUBLISHED. Operators must register a trusted publisher
	// key + sign before the artifact becomes installable.
	providerType := ""
	if provType := src.GetType(); provType != repopb.UpstreamSourceType_UPSTREAM_TYPE_UNSPECIFIED {
		providerType = provType.String()
	}
	sigDec := srv.signaturePolicyDecision(ctx, ref, key, digest, providerType)
	if sigDec.Required && !sigDec.Allowed {
		quarantineReason := fmt.Sprintf("signature_policy:required+missing_or_invalid:%s", sigDec.Reason)
		// Don't go through transitionArtifactState directly — the artifact
		// is currently at LEDGER_WRITTEN; LEDGER_WRITTEN→QUARANTINED is not
		// a legal edge. Lift to PUBLISHED first (ledger is real), then
		// degrade to QUARANTINED via the dedicated helper that also stamps
		// publish_state=QUARANTINED.
		_ = srv.transitionArtifactState(ctx, key, PipelinePublished,
			"publish_pipeline_complete:awaiting_signature_quarantine", workflowRunID, stateFields)
		srv.markBrokenForReason(ctx, key, quarantineReason, workflowRunID, stateFields) // no-op if not size/checksum reason
		// Force QUARANTINED via the dedicated path so publish_state also moves.
		if err := srv.transitionArtifactState(ctx, key, PipelineQuarantined,
			quarantineReason, workflowRunID, stateFields); err == nil {
			if srv.scylla != nil {
				_ = srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_QUARANTINED.String())
			}
		}
		return nil
	}

	// Quarantine policy preserves PUBLISHED in the pipeline-state machine
	// (the artifact is fully present); QUARANTINED is a public-lifecycle
	// admin state captured by repopb.PublishState_QUARANTINED in the manifest.
	publishReason := "publish_pipeline_complete"
	if strings.EqualFold(src.GetTrustPolicy(), "quarantine") {
		publishReason = "publish_pipeline_complete:source_trust_policy=quarantine"
	}
	_ = srv.transitionArtifactState(ctx, key, PipelinePublished,
		publishReason, workflowRunID, stateFields)
	if err := srv.ensureReleaseBuildAlias(ctx, ref, releaseTag, n.BuildNumber, n.BuildID, n.BuildID, digest, n.OriginRelease, src.GetName()); err != nil {
		if isAliasConflictError(err) {
			return fmt.Errorf("alias conflict for %s build_number=%d: %w", releaseTag, n.BuildNumber, err)
		}
		slog.Warn("upstream: failed to persist release/build alias", "err", err,
			"release_tag", releaseTag, "build_number", n.BuildNumber, "canonical_build_id", n.BuildID)
	}

	// Materialize the .tgz into the local install package directory so
	// node-agent's local-only install path can find the exact-version
	// archive. Without this, install workflows hit findLocalPackage with
	// no exact match and either fall back to wildcard (silently installing
	// the wrong version — see installer.explicit_version_requires_exact_local_package)
	// or fail outright. Best-effort, idempotent, atomic via temp+rename.
	materializeLocalPackageArchive(n.Name, n.Version, n.Platform, n.Filename, data)

	return nil
}

// localInstallPackageDir is the directory where node-agent's local-only
// install path searches for .tgz archives by exact version. Repository sync
// materializes imported artifacts here so the install side can resolve
// <name>_<version>_<platform>.tgz without any further round-trip.
//
// Exposed as a var so tests can redirect to a temp dir.
var localInstallPackageDir = "/var/lib/globular/packages"

// materializeLocalPackageArchive writes the imported .tgz to the local
// install package directory atomically via temp file + rename.
//
// Contract:
//   - Idempotent: if a file with matching size+digest exists, no-op.
//   - Atomic: target file is either fully written with correct content or
//     not present at all. Partial writes leave only a temp file (then removed).
//   - Best-effort: filesystem failures are logged at WARN but do not surface
//     to the caller — the repository service's own storage still holds the
//     canonical artifact, and the install path can degrade gracefully.
//   - Filename: if upstream provided n.Filename it is preserved; otherwise
//     the canonical <name>_<version>_<platform>.tgz is used.
//
// See docs/intent/repository.sync_materializes_imported_package_archive.yaml.
func materializeLocalPackageArchive(name, version, platform, filename string, data []byte) {
	if len(data) == 0 {
		return
	}
	if filename == "" {
		filename = fmt.Sprintf("%s_%s_%s.tgz", name, version, platform)
	}
	dir := localInstallPackageDir
	target := filepath.Join(dir, filename)

	// Idempotency: skip if an existing file already matches the new bytes.
	if existing, err := os.ReadFile(target); err == nil && len(existing) == len(data) {
		if sha256.Sum256(existing) == sha256.Sum256(data) {
			return
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("materialize: mkdir failed; local install path will need other source",
			"dir", dir, "err", err)
		return
	}
	tmp, err := os.CreateTemp(dir, filename+".tmp-*")
	if err != nil {
		slog.Warn("materialize: create temp failed; local install path will need other source",
			"dir", dir, "err", err)
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		slog.Warn("materialize: write failed; partial temp removed",
			"target", target, "err", err)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		slog.Warn("materialize: close failed; partial temp removed",
			"target", target, "err", err)
		return
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		slog.Warn("materialize: atomic rename failed; partial temp removed",
			"from", tmpPath, "to", target, "err", err)
		return
	}
	slog.Info("materialize: wrote local install package archive",
		"path", target, "bytes", len(data))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// sourceOptsFromProto maps a proto UpstreamSource to a provider-neutral SourceOpts.
// This is the single place where proto fields are mapped to provider config.
func sourceOptsFromProto(src *repopb.UpstreamSource, authToken string) upstream.SourceOpts {
	owner := src.GetOwner()
	repo := src.GetRepo()
	// Backward compat: if owner/repo are empty but repo_url is set, parse it.
	if owner == "" && repo == "" && src.GetRepoUrl() != "" {
		if o, r, err := upstream.ParseRepoURL(src.GetRepoUrl()); err == nil {
			owner = o
			repo = r
		}
	}
	// GIT_INDEX cache directory: deterministic per-source path under repository data dir.
	cacheDir := ""
	if src.GetType() == repopb.UpstreamSourceType_GIT_INDEX {
		dataDir := config.GetDataDir()
		if dataDir != "" && src.GetName() != "" {
			cacheDir = dataDir + "/upstream-cache/" + src.GetName()
		}
	}
	return upstream.SourceOpts{
		IndexURL:           src.GetIndexUrl(),
		IndexPathTemplate:  src.GetIndexPathTemplate(),
		Platform:           src.GetPlatform(),
		AuthToken:          authToken,
		CredentialsRef:     src.GetCredentialsRef(),
		Owner:              owner,
		Repo:               repo,
		IncludePrereleases: src.GetIncludePrereleases(),
		RepoURL:            src.GetRepoUrl(),
		Branch:             src.GetBranch(),
		CacheDir:           cacheDir,
		ArtifactBaseURL:    src.GetArtifactBaseUrl(),
		LocalRoot:          src.GetLocalRoot(),
	}
}

// parseReleaseIndex unmarshals and validates a release index from raw bytes.
func parseReleaseIndex(data []byte) (*releaseIndex, error) {
	var idx releaseIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("decode index JSON: %w", err)
	}
	if err := ValidateReleaseIndex(&idx); err != nil {
		return nil, err
	}
	if err := ValidateReleaseIndexForInstall(&idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// resolveProvenanceAssetURL returns the best locator for provenance records.
// Prefers asset_url, falls back to "path:" + asset_path or filename.
func resolveProvenanceAssetURL(n *normalizedEntry) string {
	if n.AssetURL != "" {
		return n.AssetURL
	}
	if n.AssetPath != "" {
		return "path:" + n.AssetPath
	}
	if n.Filename != "" {
		return "file:" + n.Filename
	}
	return ""
}

// downloadAndVerifyFromProvider uses the provider.OpenArtifact interface to
// download an artifact, compute sha256, and verify it matches the expected digest.
// Works with all providers: HTTP, LOCAL_DIR, GIT_INDEX, GitHub.
//
// TODO(streaming): This function reads the full artifact into memory (up to
// maxArtifactBytes = 500 MiB). For large packages this causes memory pressure.
// Refactor to stream through a temp file:
//   1. provider.OpenArtifact → stream to os.CreateTemp while hashing
//   2. Verify sha256 after complete write
//   3. Pass temp file path to importUpstreamArtifact (which writes to MinIO)
//   4. Delete temp file after MinIO write
// This avoids holding 500 MiB in memory during import.
// Tracked as: https://github.com/globulario/services/issues/TBD
func downloadAndVerifyFromProvider(
	ctx context.Context,
	provider upstream.ReleaseSource,
	opts upstream.SourceOpts,
	n *normalizedEntry,
) ([]byte, string, error) {
	ref := upstream.ArtifactRef{
		AssetURL:      n.AssetURL,
		AssetPath:     n.AssetPath,
		Filename:      n.Filename,
		ReleaseTag:    n.ReleaseTag,
		OriginRelease: n.OriginRelease,
		Name:          n.Name,
		Version:       n.Version,
		Platform:      n.Platform,
		Sha256:        n.Digest,
	}

	rc, _, err := provider.OpenArtifact(ctx, opts, ref)
	if err != nil {
		return nil, "", fmt.Errorf("open artifact %s: %w", n.Name, err)
	}
	defer rc.Close()

	// Stream through sha256 with size limit.
	limited := io.LimitReader(rc, int64(maxArtifactBytes)+1)
	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(limited, h))
	if err != nil {
		return nil, "", fmt.Errorf("read artifact %s: %w", n.Name, err)
	}
	if len(data) > maxArtifactBytes {
		return nil, "", fmt.Errorf("artifact %s exceeds %d bytes", n.Name, maxArtifactBytes)
	}

	actual := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if n.Digest != "" && actual != n.Digest {
		return nil, "", fmt.Errorf("digest mismatch for %s: expected %s, got %s", n.Name, n.Digest, actual)
	}
	return data, actual, nil
}

// loadUpstreamSource reads a registered UpstreamSource from etcd.
func (srv *server) loadUpstreamSource(ctx context.Context, name string) (*repopb.UpstreamSource, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "etcd unavailable: %v", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, upstreamEtcdKey(name))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "etcd get upstream %q: %v", name, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, status.Errorf(codes.NotFound, "upstream source %q not found", name)
	}
	var src repopb.UpstreamSource
	if err := protojson.Unmarshal(resp.Kvs[0].Value, &src); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal upstream source: %v", err)
	}
	return &src, nil
}

// updateSyncStatus updates the upstream source record in etcd with sync results.
func (srv *server) updateSyncStatus(ctx context.Context, sourceName, tag, syncStatus, syncError string) error {
	src, err := srv.loadUpstreamSource(ctx, sourceName)
	if err != nil {
		return err
	}
	if syncStatus == "succeeded" {
		src.LastSyncedTag = tag
	}
	src.LastSyncUnix = time.Now().Unix()
	src.LastSyncStatus = syncStatus
	src.LastSyncError = syncError

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	data, err := protojson.Marshal(src)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = cli.Put(tctx, upstreamEtcdKey(sourceName), string(data))
	return err
}

// resolveCredentialFromEtcd reads a token from an etcd key. Only accepts
// keys under /globular/credentials/ to prevent arbitrary etcd reads.
func resolveCredentialFromEtcd(ctx context.Context, credRef string) (string, error) {
	const allowedPrefix = "/globular/credentials/"
	if !strings.HasPrefix(credRef, allowedPrefix) {
		return "", fmt.Errorf("credentials_ref must start with %q (got %q)", allowedPrefix, credRef)
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return "", fmt.Errorf("etcd unavailable: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, credRef)
	if err != nil {
		return "", fmt.Errorf("etcd get credential: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("credential key %q not found in etcd", credRef)
	}
	return strings.TrimSpace(string(resp.Kvs[0].Value)), nil
}

// upstreamHTTPClient returns an http.Client with sensible timeouts.
func upstreamHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// fetchReleaseIndex fetches and parses the release-index.json from indexURL.
func fetchReleaseIndex(indexURL, authToken string) (*releaseIndex, error) {
	req, err := http.NewRequest(http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request %s: %w", indexURL, err)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := upstreamHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", indexURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", indexURL, resp.StatusCode)
	}
	if resp.ContentLength > int64(maxReleaseIndexBytes) {
		return nil, fmt.Errorf("release index too large: %d bytes (max %d)", resp.ContentLength, maxReleaseIndexBytes)
	}
	limited := io.LimitReader(resp.Body, int64(maxReleaseIndexBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read index body: %w", err)
	}
	if len(data) > maxReleaseIndexBytes {
		return nil, fmt.Errorf("release index exceeds %d bytes", maxReleaseIndexBytes)
	}
	var idx releaseIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("decode index JSON: %w", err)
	}
	return &idx, nil
}

// downloadAndVerify downloads the artifact, computes sha256, verifies.
// Streams through a temp file to avoid holding full artifact in memory.
func downloadAndVerify(assetURL, expectedDigest, authToken string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request %s: %w", assetURL, err)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := upstreamHTTPClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("GET %s: %w", assetURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s returned %d", assetURL, resp.StatusCode)
	}
	if resp.ContentLength > int64(maxArtifactBytes) {
		return nil, "", fmt.Errorf("artifact too large: %d bytes (max %d)", resp.ContentLength, maxArtifactBytes)
	}

	limited := io.LimitReader(resp.Body, int64(maxArtifactBytes)+1)
	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(limited, h))
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}
	if len(data) > maxArtifactBytes {
		return nil, "", fmt.Errorf("artifact exceeds %d bytes", maxArtifactBytes)
	}

	actual := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if actual != expectedDigest {
		return nil, "", fmt.Errorf("digest mismatch: expected %s, got %s", expectedDigest, actual)
	}
	return data, actual, nil
}

// extractPackageJSON reads ./package.json from a .tgz archive.
func extractPackageJSON(data []byte) map[string]any {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return nil
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Name == "./package.json" || hdr.Name == "package.json" {
			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil
			}
			var m map[string]any
			if json.Unmarshal(raw, &m) != nil {
				return nil
			}
			return m
		}
	}
	return nil
}

// kindFromArtifactKindString maps a string kind name to the proto enum.
func kindFromArtifactKindString(s string) (repopb.ArtifactKind, bool) {
	switch strings.ToUpper(s) {
	case "SERVICE":
		return repopb.ArtifactKind_SERVICE, true
	case "APPLICATION":
		return repopb.ArtifactKind_APPLICATION, true
	case "INFRASTRUCTURE":
		return repopb.ArtifactKind_INFRASTRUCTURE, true
	case "COMMAND":
		return repopb.ArtifactKind_COMMAND, true
	case "AGENT":
		return repopb.ArtifactKind_AGENT, true
	case "SUBSYSTEM":
		return repopb.ArtifactKind_SUBSYSTEM, true
	case "AWARENESS_BUNDLE":
		return repopb.ArtifactKind_AWARENESS_BUNDLE, true
	default:
		return repopb.ArtifactKind_SERVICE, false
	}
}

// checkImportPolicy validates a normalized entry against the upstream source's
// import policy. Returns (reason, true) if rejected, ("", false) if allowed.
func checkImportPolicy(n *normalizedEntry, src *repopb.UpstreamSource) (string, bool) {
	if pubs := src.GetAllowedPublishers(); len(pubs) > 0 {
		if !containsFold(pubs, n.Publisher) {
			return fmt.Sprintf("publisher %q not in allowed_publishers", n.Publisher), true
		}
	}
	if kinds := src.GetAllowedKinds(); len(kinds) > 0 {
		if n.Kind != "" && !containsFold(kinds, n.Kind) {
			return fmt.Sprintf("kind %q not in allowed_kinds", n.Kind), true
		}
	}
	// Channel policy: check the normalized entry channel, not source channel.
	if channels := src.GetAllowedChannels(); len(channels) > 0 {
		if !containsFold(channels, n.Channel) {
			return fmt.Sprintf("channel %q not in allowed_channels", n.Channel), true
		}
	}
	if src.GetRequireChecksum() && n.Digest == "" {
		return "require_checksum is set but entry has no package_digest", true
	}
	return "", false
}

// containsFold checks if any element in the slice matches s (case-insensitive).
func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// importTargetState returns the publish state to use when importing.
func importTargetState(src *repopb.UpstreamSource) repopb.PublishState {
	if strings.EqualFold(src.GetTrustPolicy(), "quarantine") {
		return repopb.PublishState_QUARANTINED
	}
	return repopb.PublishState_PUBLISHED
}

// ── Upstream fallback policy ────────────────────────────────────────────────

// upstreamFallbackAllowed checks whether DownloadArtifact should attempt
// upstream refill for a given manifest+source. This is server-policy driven —
// old clients benefit without setting allow_upstream_fallback.
func (srv *server) upstreamFallbackAllowed(ctx context.Context, manifest *repopb.ArtifactManifest) bool {
	ui := manifest.GetUpstreamImport()
	if ui == nil || ui.GetAssetUrl() == "" {
		return false
	}
	if manifest.GetChecksum() == "" {
		return false
	}

	// Source must exist, be enabled, and have a compatible trust policy.
	src, err := srv.loadUpstreamSource(ctx, ui.GetSourceName())
	if err != nil {
		return false
	}
	if !src.GetEnabled() {
		return false
	}
	// Quarantine-only sources should not auto-refill (would bypass quarantine intent).
	if strings.EqualFold(src.GetTrustPolicy(), "quarantine") {
		return false
	}

	// Validate publisher/kind/channel against source policy.
	ref := manifest.GetRef()
	if pubs := src.GetAllowedPublishers(); len(pubs) > 0 {
		if !containsFold(pubs, ref.GetPublisherId()) {
			return false
		}
	}
	if kinds := src.GetAllowedKinds(); len(kinds) > 0 {
		if !containsFold(kinds, ref.GetKind().String()) {
			return false
		}
	}
	if channels := src.GetAllowedChannels(); len(channels) > 0 {
		ch := effectiveChannel(manifest).String()
		if !containsFold(channels, ch) {
			return false
		}
	}

	return true
}
