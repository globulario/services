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

	"github.com/google/uuid"
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

// releaseIndexEntry mirrors the release-index.json package entry schema.
type releaseIndexEntry struct {
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Publisher          string `json:"publisher"`
	Version            string `json:"version"`
	BuildID            string `json:"build_id"`
	Platform           string `json:"platform"`
	Filename           string `json:"filename"`
	PackageDigest      string `json:"package_digest"`
	EntrypointChecksum string `json:"entrypoint_checksum"`
	AssetURL           string `json:"asset_url"`
	ReleaseTag         string `json:"release_tag"`
	PublishedAt        string `json:"published_at"`
}

type releaseIndex struct {
	SchemaVersion   int                  `json:"schema_version"`
	ReleaseTag      string               `json:"release_tag"`
	GlobularVersion string               `json:"globular_version"`
	Publisher       string               `json:"publisher"`
	Packages        []*releaseIndexEntry `json:"packages"`
}

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

	// ── Build index URL ────────────────────────────────────────────────────
	indexURL := strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag)

	slog.Info("upstream: starting sync",
		"source", sourceName, "tag", releaseTag, "index_url", indexURL, "dry_run", dryRun)

	// ── Fetch release index ────────────────────────────────────────────────
	idx, err := fetchReleaseIndex(indexURL)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "fetch release index: %v", err)
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

		result := srv.processSyncEntry(ctx, entry, src, releaseTag, dryRun)
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

	// ── Advance last_synced_tag only on a fully clean run ─────────────────
	// Rule: advance only when dry_run=false AND rejected==0 AND failed==0.
	if !dryRun && rejected == 0 && failed == 0 {
		if updateErr := srv.updateLastSyncedTag(ctx, sourceName, releaseTag); updateErr != nil {
			slog.Warn("upstream: failed to update last_synced_tag (non-fatal)", "source", sourceName, "err", updateErr)
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
) *repopb.UpstreamSyncResult {

	publisher := entry.Publisher
	if publisher == "" {
		publisher = "core@globular.io"
	}

	result := &repopb.UpstreamSyncResult{
		Name:          entry.Name,
		Version:       entry.Version,
		BuildId:       entry.BuildID,
		Platform:      entry.Platform,
		PackageDigest: entry.PackageDigest,
	}

	// ── Check local ledger ─────────────────────────────────────────────────
	ledger := srv.readLedger(ctx, publisher, entry.Name)
	if ledger != nil {
		for _, r := range ledger.Releases {
			if r.Version == entry.Version && r.Platform == entry.Platform {
				// Version+platform exists — compare digest (the content binding).
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

	// Not found → would import (or import).
	if dryRun {
		result.Status = repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT
		result.Detail = fmt.Sprintf("would download from %s", entry.AssetURL)
		return result
	}

	// ── Download and verify ────────────────────────────────────────────────
	data, digest, err := downloadAndVerify(entry.AssetURL, entry.PackageDigest)
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
	result.Detail = fmt.Sprintf("imported from %s", src.GetName())
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

	key := artifactKeyWithBuild(ref, buildNumber)

	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), data, 0o644); err != nil {
		return fmt.Errorf("write binary: %w", err)
	}

	confirmedBuildID := uuid.Must(uuid.NewV7()).String()
	manifest := &repopb.ArtifactManifest{
		Ref:                ref,
		BuildNumber:        buildNumber,
		BuildId:            confirmedBuildID,
		Checksum:           digest,
		SizeBytes:          int64(len(data)),
		EntrypointChecksum: entry.EntrypointChecksum,
		Provisional:        false,
		UpstreamImport: &repopb.UpstreamImportRecord{
			SourceName:  src.GetName(),
			ReleaseTag:  releaseTag,
			AssetUrl:    entry.AssetURL,
			IndexUrl:    strings.ReplaceAll(src.GetIndexUrl(), "{tag}", releaseTag),
			ImportedAt:  time.Now().Unix(),
		},
	}

	// Attempt to extract kind from the archive.
	if kind, ok := kindFromArtifactKindString(entry.Kind); ok {
		manifest.Ref.Kind = kind
	}

	mjson, err := marshalManifestWithState(manifest, repopb.PublishState_PUBLISHED)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	srv.syncManifestToScylla(ctx, key, manifest, repopb.PublishState_PUBLISHED, mjson)

	// Append to release ledger.
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

// updateLastSyncedTag writes the new last_synced_tag back to etcd.
func (srv *server) updateLastSyncedTag(ctx context.Context, sourceName, tag string) error {
	src, err := srv.loadUpstreamSource(ctx, sourceName)
	if err != nil {
		return err
	}
	src.LastSyncedTag = tag

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

// fetchReleaseIndex fetches and parses the release-index.json from indexURL.
func fetchReleaseIndex(indexURL string) (*releaseIndex, error) {
	resp, err := http.Get(indexURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", indexURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", indexURL, resp.StatusCode)
	}
	var idx releaseIndex
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode index JSON: %w", err)
	}
	return &idx, nil
}

// downloadAndVerify downloads the artifact from assetURL, computes its sha256,
// and verifies it matches expectedDigest ("sha256:<hex>").
// Returns the raw bytes and the verified digest string.
func downloadAndVerify(assetURL, expectedDigest string) ([]byte, string, error) {
	resp, err := http.Get(assetURL) //nolint:noctx
	if err != nil {
		return nil, "", fmt.Errorf("GET %s: %w", assetURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s returned %d", assetURL, resp.StatusCode)
	}

	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(resp.Body, h))
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
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
