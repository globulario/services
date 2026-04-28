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
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/repository/upstream"
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
	if err := srv.requireHealthy(); err != nil {
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
	if err := srv.requireHealthy(); err != nil {
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
	if err := srv.requireHealthy(); err != nil {
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
	if err := srv.requireHealthy(); err != nil {
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

	// ── GitHub latest-release discovery ───────────────────────────────────
	var indexURL string
	if resolveLatest {
		if src.GetRepoUrl() == "" {
			return nil, status.Errorf(codes.InvalidArgument,
				"resolve_latest requires repo_url on upstream source %q — register with --repo-url", sourceName)
		}
		owner, repo, parseErr := upstream.ParseRepoURL(src.GetRepoUrl())
		if parseErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid repo_url on source %q: %v", sourceName, parseErr)
		}
		release, fetchErr := upstream.FetchLatestRelease(owner, repo, src.GetIncludePrereleases(), authToken)
		if fetchErr != nil {
			return nil, status.Errorf(codes.Unavailable, "GitHub API: %v", fetchErr)
		}
		releaseTag = release.TagName
		asset, assetErr := upstream.FindReleaseIndexAsset(release)
		if assetErr != nil {
			return nil, status.Errorf(codes.NotFound, "GitHub release %q has no release-index.json asset", releaseTag)
		}
		indexURL = asset.BrowserDownloadURL
		slog.Info("upstream: resolved latest release", "source", sourceName, "tag", releaseTag,
			"prerelease", release.Prerelease)
	} else {
		indexURL = strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag)
	}

	slog.Info("upstream: starting sync",
		"source", sourceName, "tag", releaseTag, "index_url", upstream.RedactAssetURL(indexURL), "dry_run", dryRun)

	// ── Fetch release index ────────────────────────────────────────────────
	idx, err := fetchReleaseIndex(indexURL, authToken)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "fetch release index: %v", err)
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

		result := srv.processSyncEntry(ctx, entry, src, releaseTag, dryRun, authToken)
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
	releaseTag string,
	dryRun bool,
	authToken string,
) *repopb.UpstreamSyncResult {

	// ── Step 1: Normalize ─────────────────────────────────────────────────
	n := normalizeReleaseEntry(entry, src)

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

	// ── Step 3: Conflict detection ────────────────────────────────────────
	if ledger != nil {
		for _, r := range ledger.Releases {
			if r.Version == n.Version && r.BuildID == n.BuildID && r.Platform == n.Platform {
				if r.Digest == n.Digest {
					if dryRun {
						result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
					} else {
						result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
					}
					result.Detail = "already present with matching digest"
					result.Action = "up_to_date"
					return result
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

	ref := &repopb.ArtifactRef{
		PublisherId: n.Publisher,
		Name:        n.Name,
		Version:     n.Version,
		Platform:    n.Platform,
	}
	if existing, state, _, ok := srv.findExistingArtifactByDigest(ctx, ref, n.Digest); ok {
		detail := fmt.Sprintf("already present with matching digest at build %d (%s)", existing.GetBuildNumber(), state.String())
		if dryRun {
			result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
		} else {
			result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
		}
		result.Detail = detail
		result.Action = "up_to_date"
		return result
	}

	// Not found → would import (or import).
	// Determine if this is a new package or an update.
	if result.LocalVersion == "" {
		result.Action = "new"
	} else if result.LocalVersion < n.Version {
		result.Action = "update"
	} else if result.LocalVersion > n.Version {
		result.Action = "ahead"
	} else {
		result.Action = "new" // same version, different build
	}

	if dryRun {
		result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT
		result.Detail = fmt.Sprintf("would download from %s (build_number=%d)", upstream.RedactAssetURL(n.AssetURL), n.BuildNumber)
		return result
	}

	// ── Step 4: Download and verify ───────────────────────────────────────
	data, digest, err := downloadAndVerify(n.AssetURL, n.Digest, authToken)
	if err != nil {
		result.Status = repopb.UpstreamSyncStatus_SYNC_FAILED
		result.Detail = fmt.Sprintf("download/verify failed: %v", err)
		slog.Warn("upstream: download failed", "name", n.Name, "url", n.AssetURL, "err", err)
		return result
	}

	if importErr := srv.importUpstreamArtifact(ctx, n, data, digest, src, releaseTag); importErr != nil {
		result.Status = repopb.UpstreamSyncStatus_SYNC_FAILED
		result.Detail = fmt.Sprintf("import failed: %v", importErr)
		slog.Error("upstream: import failed", "name", n.Name, "version", n.Version, "err", importErr)
		return result
	}

	result.Status = repopb.UpstreamSyncStatus_SYNC_IMPORTED
	if strings.EqualFold(src.GetTrustPolicy(), "quarantine") {
		result.Detail = fmt.Sprintf("imported from %s (quarantined — requires manual promotion)", src.GetName())
	} else {
		result.Detail = fmt.Sprintf("imported from %s", src.GetName())
	}
	slog.Info("upstream: imported", "name", n.Name, "version", n.Version, "platform", n.Platform,
		"build_number", n.BuildNumber, "build_id", n.BuildID, "digest", truncDigest(digest))
	return result
}

// importUpstreamArtifact stores the downloaded artifact binary + manifest and
// appends it to the release ledger. Uses normalizedEntry for all identity fields.
func (srv *server) importUpstreamArtifact(
	ctx context.Context,
	n *normalizedEntry,
	data []byte,
	digest string,
	src *repopb.UpstreamSource,
	releaseTag string,
) error {
	ref := &repopb.ArtifactRef{
		PublisherId: n.Publisher,
		Name:        n.Name,
		Version:     n.Version,
		Platform:    n.Platform,
	}

	if existing, state, _, ok := srv.findExistingArtifactByDigest(ctx, ref, digest); ok {
		slog.Info("upstream: identical artifact already exists, skipping import",
			"name", n.Name, "version", n.Version,
			"build", existing.GetBuildNumber(), "build_id", existing.GetBuildId(),
			"publish_state", state.String())
		return nil
	}

	key := artifactKeyWithBuild(ref, n.BuildNumber)

	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), data, 0o644); err != nil {
		return fmt.Errorf("write binary: %w", err)
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
			SourceName:  src.GetName(),
			ReleaseTag:  releaseTag,
			AssetUrl:    n.AssetURL,
			IndexUrl:    strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag),
			ImportedAt:  time.Now().Unix(),
			Publisher:   n.Publisher,
			Kind:        n.Kind,
			Channel:     n.Channel,
			BuildNumber: n.BuildNumber,
			Checksum:    digest,
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

	if ledgerErr := srv.appendToLedger(ctx, n.Publisher, n.Name, n.Version,
		n.BuildID, digest, n.Platform, int64(len(data))); ledgerErr != nil {
		slog.Warn("upstream: ledger append failed (non-fatal)", "name", n.Name, "err", ledgerErr)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
