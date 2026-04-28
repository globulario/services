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
//   - "latest" discovery is not implemented
//   - quarantine = audit event only (no quarantine storage tier)
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
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
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

	if sourceName == "" {
		return nil, status.Error(codes.InvalidArgument, "source_name is required")
	}
	// Phase 1: explicit tag is mandatory — no "latest" discovery.
	if releaseTag == "" {
		return nil, status.Error(codes.InvalidArgument, "release_tag is required in phase 1 — specify an explicit tag (e.g. 'v1.0.17')")
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

	// ── Build index URL ────────────────────────────────────────────────────
	indexURL := strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag)

	slog.Info("upstream: starting sync",
		"source", sourceName, "tag", releaseTag, "index_url", indexURL, "dry_run", dryRun)

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
		Results:  results,
		Imported: imported,
		Skipped:  skipped,
		Rejected: rejected,
		Failed:   failed,
		DryRun:   dryRun,
	}, nil
}

// processSyncEntry evaluates one release index entry and either imports it or
// produces a preview/skip/reject result. Never panics — all errors become
// SYNC_FAILED or SYNC_WOULD_FAIL results.
func (srv *server) processSyncEntry(
	ctx context.Context,
	entry *releaseIndexEntry,
	src *repopb.UpstreamSource,
	releaseTag string,
	dryRun bool,
	authToken string,
) *repopb.UpstreamSyncResult {

	publisher := entry.Publisher
	if publisher == "" {
		if dp := src.GetDefaultPublisherId(); dp != "" {
			publisher = dp
		} else {
			publisher = "core@globular.io"
		}
	}

	result := &repopb.UpstreamSyncResult{
		Name:          entry.Name,
		Version:       entry.Version,
		BuildId:       entry.BuildID,
		Platform:      entry.Platform,
		PackageDigest: entry.PackageDigest,
	}

	// ── Policy filtering ──────────────────────────────────────────────────
	if reason, rejected := checkImportPolicy(entry, publisher, src); rejected {
		if dryRun {
			result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT
		} else {
			result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
			srv.publishAuditEvent(ctx, "upstream.policy_rejected", map[string]any{
				"source":     src.GetName(),
				"package":    entry.Name,
				"version":    entry.Version,
				"publisher":  publisher,
				"kind":       entry.Kind,
				"reason":     reason,
			})
		}
		result.Detail = reason
		return result
	}

	// ── Check local ledger ─────────────────────────────────────────────────
	ledger := srv.readLedger(ctx, publisher, entry.Name)
	if ledger != nil {
		for _, r := range ledger.Releases {
			// Artifact key = (name, version, build_id, platform). All four must
			// match to be the same artifact. Different build_id → distinct artifact.
			if r.Version == entry.Version && r.BuildID == entry.BuildID && r.Platform == entry.Platform {
				// Same key — compare digest (the content binding).
				if r.Digest == entry.PackageDigest {
					// Same key + same digest → idempotent skip.
					if dryRun {
						result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
						result.Detail = "already present with matching digest"
					} else {
						result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
						result.Detail = "already present with matching digest"
					}
					return result
				}
				// Same key + different digest → reject.
				detail := fmt.Sprintf(
					"digest conflict: local=%s... upstream=%s...",
					truncDigest(r.Digest), truncDigest(entry.PackageDigest),
				)
				if dryRun {
					result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT
					result.Detail = detail
				} else {
					result.Status = repopb.UpstreamSyncStatus_SYNC_REJECTED
					result.Detail = detail
					// Quarantine = audit event only (phase 1).
					srv.publishAuditEvent(ctx, "upstream.digest_conflict", map[string]any{
						"source":          src.GetName(),
						"release_tag":     releaseTag,
						"package_name":    entry.Name,
						"version":         entry.Version,
						"platform":        entry.Platform,
						"local_digest":    r.Digest,
						"upstream_digest": entry.PackageDigest,
						"asset_url":       entry.AssetURL,
					})
				}
				return result
			}
		}
	}

	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        entry.Name,
		Version:     entry.Version,
		Platform:    entry.Platform,
	}
	if existing, state, _, ok := srv.findExistingArtifactByDigest(ctx, ref, entry.PackageDigest); ok {
		detail := fmt.Sprintf("already present with matching digest at build %d (%s)", existing.GetBuildNumber(), state.String())
		if dryRun {
			result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP
		} else {
			result.Status = repopb.UpstreamSyncStatus_SYNC_SKIPPED
		}
		result.Detail = detail
		return result
	}

	// Not found → would import (or import).
	if dryRun {
		result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT
		result.Detail = fmt.Sprintf("would download from %s", entry.AssetURL)
		return result
	}

	// ── Download and verify ────────────────────────────────────────────────
	data, digest, err := downloadAndVerify(entry.AssetURL, entry.PackageDigest, authToken)
	if err != nil {
		result.Status = repopb.UpstreamSyncStatus_SYNC_FAILED
		result.Detail = fmt.Sprintf("download/verify failed: %v", err)
		slog.Warn("upstream: download failed", "name", entry.Name, "url", entry.AssetURL, "err", err)
		return result
	}

	// ── Parse build_number from release index build_id ────────────────────
	var buildNumber int64
	if n, parseErr := strconv.ParseInt(entry.BuildID, 10, 64); parseErr == nil {
		buildNumber = n
	}

	// ── Store artifact ─────────────────────────────────────────────────────
	if importErr := srv.importUpstreamArtifact(ctx, entry, publisher, data, digest, buildNumber, src, releaseTag); importErr != nil {
		result.Status = repopb.UpstreamSyncStatus_SYNC_FAILED
		result.Detail = fmt.Sprintf("import failed: %v", importErr)
		slog.Error("upstream: import failed", "name", entry.Name, "version", entry.Version, "err", importErr)
		return result
	}

	result.Status = repopb.UpstreamSyncStatus_SYNC_IMPORTED
	if strings.EqualFold(src.GetTrustPolicy(), "quarantine") {
		result.Detail = fmt.Sprintf("imported from %s (quarantined — requires manual promotion)", src.GetName())
	} else {
		result.Detail = fmt.Sprintf("imported from %s", src.GetName())
	}
	slog.Info("upstream: imported", "name", entry.Name, "version", entry.Version, "platform", entry.Platform, "digest", truncDigest(digest))
	return result
}

// importUpstreamArtifact stores the downloaded artifact binary + manifest and
// appends it to the release ledger. Mirrors the ImportProvisionalArtifact path
// but also sets upstream_import provenance on the manifest.
func (srv *server) importUpstreamArtifact(
	ctx context.Context,
	entry *releaseIndexEntry,
	publisher string,
	data []byte,
	digest string,
	buildNumber int64,
	src *repopb.UpstreamSource,
	releaseTag string,
) error {
	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        entry.Name,
		Version:     entry.Version,
		Platform:    entry.Platform,
	}

	if existing, state, _, ok := srv.findExistingArtifactByDigest(ctx, ref, digest); ok {
		slog.Info("upstream: identical artifact already exists, skipping import",
			"name", entry.Name,
			"version", entry.Version,
			"build", existing.GetBuildNumber(),
			"build_id", existing.GetBuildId(),
			"publish_state", state.String(),
		)
		return nil
	}

	key := artifactKeyWithBuild(ref, buildNumber)

	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), data, 0o644); err != nil {
		return fmt.Errorf("write binary: %w", err)
	}

	// Deterministic build identity: preserve upstream build_id if present.
	// Otherwise derive from (publisher, name, version, platform, sha256).
	// Never generate random UUIDv7 for imported artifacts.
	confirmedBuildID := entry.BuildID
	if confirmedBuildID == "" {
		confirmedBuildID = deriveUpstreamBuildID(publisher, entry.Name, entry.Version, entry.Platform, digest)
	}
	manifest := &repopb.ArtifactManifest{
		Ref:                ref,
		BuildNumber:        buildNumber,
		BuildId:            confirmedBuildID,
		Checksum:           digest,
		SizeBytes:          int64(len(data)),
		EntrypointChecksum: entry.EntrypointChecksum,
		Provisional:        false,
		UpstreamImport: &repopb.UpstreamImportRecord{
			SourceName: src.GetName(),
			ReleaseTag: releaseTag,
			AssetUrl:   entry.AssetURL,
			IndexUrl:   strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag),
			ImportedAt: time.Now().Unix(),
		},
	}

	// Attempt to extract kind from the release index.
	if kind, ok := kindFromArtifactKindString(entry.Kind); ok {
		manifest.Ref.Kind = kind
	}

	// Enrich manifest from package.json inside the archive.
	// Populates profiles, deps, provides/requires, health config, etc.
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

	// Append to release ledger using confirmedBuildID (same as manifest).
	// With deterministic build identity, manifest and ledger are now in sync.
	if ledgerErr := srv.appendToLedger(ctx, publisher, entry.Name, entry.Version,
		confirmedBuildID, digest, entry.Platform, int64(len(data))); ledgerErr != nil {
		slog.Warn("upstream: ledger append failed (non-fatal)", "name", entry.Name, "err", ledgerErr)
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
// Advances last_synced_tag only on fully clean runs (status == "succeeded").
func (srv *server) updateSyncStatus(ctx context.Context, sourceName, tag, syncStatus, syncError string) error {
	src, err := srv.loadUpstreamSource(ctx, sourceName)
	if err != nil {
		return err
	}

	// Only advance last_synced_tag on a fully clean run.
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
// The token value is never logged.
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

// upstreamHTTPClient returns an http.Client with sensible timeouts for
// upstream fetches. The 30s timeout covers connection + TLS + response.
func upstreamHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// fetchReleaseIndex fetches and parses the release-index.json from indexURL.
// Enforces a 30s timeout and 10 MiB size limit to prevent hangs and OOM.
// authToken is applied as Bearer authorization when non-empty.
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
	// Content-Length pre-check when available.
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

// downloadAndVerify downloads the artifact from assetURL, computes its sha256,
// and verifies it matches expectedDigest ("sha256:<hex>").
// Enforces a 30s timeout and 500 MiB size limit.
// authToken is applied as Bearer authorization when non-empty.
// Returns the raw bytes and the verified digest string.
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
	// Content-Length pre-check when available.
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
// Returns nil if not present (non-fatal for sync — the index provides metadata).
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

// checkImportPolicy validates a release index entry against the upstream source's
// import policy. Returns (reason, true) if rejected, ("", false) if allowed.
func checkImportPolicy(entry *releaseIndexEntry, publisher string, src *repopb.UpstreamSource) (string, bool) {
	// allowed_publishers: if set, reject publishers not in the list.
	if pubs := src.GetAllowedPublishers(); len(pubs) > 0 {
		if !containsFold(pubs, publisher) {
			return fmt.Sprintf("publisher %q not in allowed_publishers", publisher), true
		}
	}

	// allowed_kinds: if set, reject kinds not in the list.
	if kinds := src.GetAllowedKinds(); len(kinds) > 0 {
		if entry.Kind != "" && !containsFold(kinds, entry.Kind) {
			return fmt.Sprintf("kind %q not in allowed_kinds", entry.Kind), true
		}
	}

	// allowed_channels: if set, reject channels not in the list.
	if channels := src.GetAllowedChannels(); len(channels) > 0 {
		// Entries without a channel field default to "stable".
		ch := src.GetChannel()
		if ch == "" {
			ch = "stable"
		}
		if !containsFold(channels, ch) {
			return fmt.Sprintf("channel %q not in allowed_channels", ch), true
		}
	}

	// require_checksum: reject entries with empty sha256.
	if src.GetRequireChecksum() && entry.PackageDigest == "" {
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

// deriveUpstreamBuildID produces a deterministic build_id from the package
// identity and content hash. Used only when the upstream release index has no
// build_id. The result is a sha256-based string (not a UUIDv7) that is stable
// across repeated imports of the same artifact.
func deriveUpstreamBuildID(publisher, name, version, platform, digest string) string {
	h := sha256.New()
	for _, s := range []string{publisher, name, version, platform, digest} {
		h.Write([]byte(s))
		h.Write([]byte{0}) // separator
	}
	return "upstream:" + hex.EncodeToString(h.Sum(nil))[:32]
}

// importTargetState returns the publish state to use when importing.
// "quarantine" trust_policy → QUARANTINED (recorded but not installable).
// Default ("import" or empty) → PUBLISHED.
func importTargetState(src *repopb.UpstreamSource) repopb.PublishState {
	if strings.EqualFold(src.GetTrustPolicy(), "quarantine") {
		return repopb.PublishState_QUARANTINED
	}
	return repopb.PublishState_PUBLISHED
}
