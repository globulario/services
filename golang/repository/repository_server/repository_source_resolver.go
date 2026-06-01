// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream
// @awareness file_role=upstream_source_resolver
// @awareness risk=medium
package main

// repository_source_resolver.go — Source chain resolution and local materialization.
//
// ResolveArtifactToLocal tries each source in priority order.
// MaterializeArtifactToLocal streams a candidate to the local POSIX CAS atomically.
//
// Law: Install/download always happens from local verified POSIX storage.
//      The resolver ensures every remote hit is materialized and verified before
//      returning a ResolutionResult.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	uppkg "github.com/globulario/services/golang/repository/upstream"
)

// ── Result types ──────────────────────────────────────────────────────────────

// ResolutionResult describes a successful local materialization.
type ResolutionResult struct {
	LocalKey      string // storage key (binaryStorageKey output)
	LocalPath     string // absolute filesystem path
	Sha256        string
	SizeBytes     int64
	SourceName    string
	SourceType    string
	ProvenanceURL string
	Diagnostics   []SourceAttempt
}

// SourceAttempt records what happened when the resolver tried one source.
type SourceAttempt struct {
	SourceName string
	SourceType string
	Status     string // HIT, MISS, UNAVAILABLE, CHECKSUM_MISMATCH, ERROR
	Reason     string
	DurationMs int64
}

// ── Resolver ──────────────────────────────────────────────────────────────────

// ResolveArtifactToLocal tries each source in the chain until the artifact is
// present in the local POSIX CAS.
func (srv *server) ResolveArtifactToLocal(
	ctx context.Context,
	req ArtifactRequest,
) (*ResolutionResult, error) {
	sources := srv.buildSourceChain(ctx, req.SourceName)
	policy := srv.loadSourcePolicy(ctx)
	return srv.resolveFromSources(ctx, req, sources, policy)
}

// resolveFromSources is the testable inner loop for ResolveArtifactToLocal.
// Callers that need etcd-free testing inject sources and policy directly.
func (srv *server) resolveFromSources(
	ctx context.Context,
	req ArtifactRequest,
	sources []RepositorySource,
	policy SourcePolicy,
) (*ResolutionResult, error) {
	var diag []SourceAttempt

	for _, src := range sources {
		attempt := SourceAttempt{SourceName: src.Name(), SourceType: src.Type()}
		t0 := time.Now()

		h := src.Health(ctx)
		if !h.Available {
			attempt.Status = "UNAVAILABLE"
			attempt.Reason = h.Reason
			attempt.DurationMs = time.Since(t0).Milliseconds()
			diag = append(diag, attempt)
			continue
		}

		if src.Type() == "MINIO_MIRROR" && !policy.AllowMinioMirror {
			attempt.Status = "UNAVAILABLE"
			attempt.Reason = "minio_mirror disabled by source policy"
			attempt.DurationMs = time.Since(t0).Milliseconds()
			diag = append(diag, attempt)
			continue
		}

		if policy.RequireChecksum && req.Sha256 == "" && src.Type() != "LOCAL_POSIX" {
			attempt.Status = "UNAVAILABLE"
			attempt.Reason = "require_checksum=true but request has no sha256"
			attempt.DurationMs = time.Since(t0).Milliseconds()
			diag = append(diag, attempt)
			continue
		}

		candidate, openErr := src.Open(ctx, req)
		if openErr != nil {
			switch {
			case errors.Is(openErr, ErrArtifactNotFound):
				attempt.Status = "MISS"
			case errors.Is(openErr, ErrSourceUnavailable):
				attempt.Status = "UNAVAILABLE"
			default:
				attempt.Status = "ERROR"
			}
			attempt.Reason = openErr.Error()
			attempt.DurationMs = time.Since(t0).Milliseconds()
			diag = append(diag, attempt)
			continue
		}

		result, matErr := srv.MaterializeArtifactToLocal(ctx, req, candidate)
		if matErr != nil {
			if errors.Is(matErr, errChecksumMismatch) {
				attempt.Status = "CHECKSUM_MISMATCH"
			} else {
				attempt.Status = "ERROR"
			}
			attempt.Reason = matErr.Error()
			attempt.DurationMs = time.Since(t0).Milliseconds()
			diag = append(diag, attempt)
			slog.Warn("repository-resolver: materialization failed", "source", src.Name(), "err", matErr)
			continue
		}

		attempt.Status = "HIT"
		attempt.DurationMs = time.Since(t0).Milliseconds()
		diag = append(diag, attempt)
		result.Diagnostics = diag
		slog.Info("repository-resolver: resolved",
			"source", src.Name(), "key", result.LocalKey, "sha256", truncDigest(result.Sha256))
		return result, nil
	}

	return nil, fmt.Errorf("artifact unavailable: no source could provide %s/%s/%s — %s",
		req.PublisherID, req.Name, req.Version, formatDiagnostics(diag))
}

// ── Materialization ───────────────────────────────────────────────────────────

var errChecksumMismatch = errors.New("checksum mismatch")

// MaterializeArtifactToLocal writes a candidate blob into the local POSIX CAS
// atomically, verifies checksum and size, and transitions artifact state.
//
// If the candidate is already local (LocalPath set), verify and return immediately.
// Otherwise stream Reader through sha256 verification into a temp file, then rename.
func (srv *server) MaterializeArtifactToLocal(
	ctx context.Context,
	req ArtifactRequest,
	candidate *ArtifactCandidate,
) (*ResolutionResult, error) {
	ref := artifactRequestToRef(req)
	key := artifactKeyWithBuild(ref, req.BuildNumber)
	binKey := binaryStorageKey(key)

	localStorage := srv.localStorage
	if localStorage == nil {
		return nil, fmt.Errorf("local store not initialized — cannot materialize")
	}

	// Fast path: candidate is already on the local filesystem.
	if candidate.LocalPath != "" {
		fi, err := os.Stat(candidate.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("local candidate stat failed: %w", err)
		}
		if req.SizeBytes > 0 && fi.Size() != req.SizeBytes {
			return nil, fmt.Errorf("%w: local blob size %d != expected %d",
				errChecksumMismatch, fi.Size(), req.SizeBytes)
		}
		// Verify sha256 on the fast path — catches silent corruption of a local blob
		// that passes size check but has wrong content.
		actualSha256 := candidate.Sha256
		if req.Sha256 != "" {
			computed, computeErr := checksumLocalFile(candidate.LocalPath)
			if computeErr != nil {
				return nil, fmt.Errorf("fast-path checksum read failed: %w", computeErr)
			}
			if !digestEqual(computed, req.Sha256) {
				return nil, fmt.Errorf("%w: local blob sha256 %s != expected %s",
					errChecksumMismatch, computed, req.Sha256)
			}
			actualSha256 = computed
		}
		result := &ResolutionResult{
			LocalKey:      binKey,
			LocalPath:     candidate.LocalPath,
			Sha256:        actualSha256,
			SizeBytes:     fi.Size(),
			SourceName:    candidate.SourceName,
			SourceType:    candidate.SourceType,
			ProvenanceURL: candidate.ProvenanceURL,
		}
		srv.writeLocalReceipt(req, result, "")
		return result, nil
	}

	if candidate.Reader == nil {
		return nil, fmt.Errorf("candidate has neither LocalPath nor Reader")
	}
	defer candidate.Reader.Close()

	// Transition to DOWNLOADING before writing anything.
	stateFields := ArtifactStateFields{
		BlobKey:     binKey,
		Checksum:    req.Sha256,
		SizeBytes:   req.SizeBytes,
		BuildID:     req.BuildID,
		BuildNumber: req.BuildNumber,
		PublisherID: req.PublisherID,
		Name:        req.Name,
		Version:     req.Version,
		Platform:    req.Platform,
	}
	wfID := req.WorkflowRunID
	// Only advance to DOWNLOADING if not already there. The sync path sets
	// DOWNLOADING before calling us; a self-transition would pollute the
	// audit trail without adding information.
	if srv.readArtifactState(ctx, key) != PipelineDownloading {
		_ = srv.transitionArtifactState(ctx, key, PipelineDownloading, "resolver_materialize_start", wfID, stateFields)
	}

	if err := localStorage.MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir artifacts dir: %w", err)
	}

	actual, writeErr := localStorage.WriteFileAtomic(ctx, binKey, candidate.Reader, req.Sha256, req.SizeBytes)
	if writeErr != nil {
		if strings.Contains(writeErr.Error(), "mismatch") {
			_ = srv.transitionArtifactState(ctx, key, PipelineBrokenChecksumMismatch,
				"materialize_checksum_mismatch", wfID, stateFields)
			return nil, fmt.Errorf("%w: %v", errChecksumMismatch, writeErr)
		}
		return nil, fmt.Errorf("atomic write failed: %w", writeErr)
	}

	stateFields.Checksum = actual
	_ = srv.transitionArtifactState(ctx, key, PipelineBlobWritten, "blob_persisted", wfID, stateFields)

	fi, statErr := localStorage.Stat(ctx, binKey)
	if statErr != nil {
		return nil, fmt.Errorf("post-write stat failed: %w", statErr)
	}
	stateFields.SizeBytes = fi.Size()
	_ = srv.transitionArtifactState(ctx, key, PipelineBlobVerified, "blob_stat_size_match", wfID, stateFields)

	result := &ResolutionResult{
		LocalKey:      binKey,
		LocalPath:     localStorage.LocalPath(binKey),
		Sha256:        actual,
		SizeBytes:     fi.Size(),
		SourceName:    candidate.SourceName,
		SourceType:    candidate.SourceType,
		ProvenanceURL: candidate.ProvenanceURL,
	}
	srv.writeLocalReceipt(req, result, candidate.SourceName)
	return result, nil
}

// ── Source chain builder ──────────────────────────────────────────────────────

// buildSourceChain constructs the ordered source chain.
func (srv *server) buildSourceChain(ctx context.Context, sourceName string) []RepositorySource {
	var chain []RepositorySource

	if srv.localStorage != nil {
		chain = append(chain, newLocalPOSIXSource(srv.localStorage, srv.localStoreRoot()))
	}

	if sourceName != "" {
		if src, err := srv.loadUpstreamSource(ctx, sourceName); err == nil && src.GetEnabled() {
			provType := uppkg.MapProtoType(int32(src.GetType()))
			if provider, pErr := uppkg.NewSource(provType); pErr == nil {
				var authToken string
				if credRef := src.GetCredentialsRef(); credRef != "" {
					if tok, _ := resolveCredentialFromEtcd(ctx, credRef); tok != "" {
						authToken = tok
					}
				}
				chain = append(chain, newUpstreamSource(srv, src, provider, sourceOptsFromProto(src, authToken)))
			}
		}
	} else {
		for _, us := range srv.loadUpstreamSources(ctx) {
			chain = append(chain, us)
		}
	}

	policy := srv.loadSourcePolicy(ctx)
	if policy.AllowMinioMirror && srv.mirrorStorage != nil {
		chain = append(chain, newMinIOSource(srv.mirrorStorage))
	}

	sort.Slice(chain, func(i, j int) bool {
		return chain[i].Priority() < chain[j].Priority()
	})
	return chain
}

// ── etcd helpers ─────────────────────────────────────────────────────────────

// scanAllUpstreamSources reads all upstream sources from etcd (including credentials_ref intact).
func (srv *server) scanAllUpstreamSources(ctx context.Context) ([]*repopb.UpstreamSource, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd unavailable: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, upstreamEtcdPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get upstreams: %w", err)
	}
	out := make([]*repopb.UpstreamSource, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var src repopb.UpstreamSource
		if err := protojson.Unmarshal(kv.Value, &src); err != nil {
			slog.Warn("repository-source: corrupt upstream etcd entry", "key", string(kv.Key), "err", err)
			continue
		}
		out = append(out, &src)
	}
	return out, nil
}

// ── Utility helpers ───────────────────────────────────────────────────────────

// artifactRequestToRef converts an ArtifactRequest to a proto ArtifactRef.
func artifactRequestToRef(req ArtifactRequest) *repopb.ArtifactRef {
	return &repopb.ArtifactRef{
		PublisherId: req.PublisherID,
		Name:        req.Name,
		Version:     req.Version,
		Platform:    req.Platform,
	}
}

// artifactRequestFromManifest builds an ArtifactRequest from a stored manifest.
// Used by DownloadArtifact to trigger repair.
func artifactRequestFromManifest(m *repopb.ArtifactManifest, buildNumber int64) ArtifactRequest {
	ref := m.GetRef()
	req := ArtifactRequest{
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
		BuildNumber: buildNumber,
		BuildID:     m.GetBuildId(),
		Sha256:      m.GetChecksum(),
		SizeBytes:   m.GetSizeBytes(),
	}
	if ui := m.GetUpstreamImport(); ui != nil {
		req.SourceName = ui.GetSourceName()
		req.ReleaseTag = ui.GetReleaseTag()
		req.AssetURL = ui.GetAssetUrl()
	}
	return req
}

func formatDiagnostics(diag []SourceAttempt) string {
	if len(diag) == 0 {
		return "no sources configured"
	}
	parts := make([]string, 0, len(diag))
	for _, d := range diag {
		parts = append(parts, fmt.Sprintf("%s:%s(%s)", d.SourceName, d.Status, d.Reason))
	}
	return strings.Join(parts, ", ")
}

// ── Local receipt ─────────────────────────────────────────────────────────────

// ArtifactReceipt is the JSON structure written beside each verified CAS blob.
// It records provenance so the reconciler can rebuild Scylla from local POSIX.
type ArtifactReceipt struct {
	PublisherID        string `json:"publisher_id"`
	Name               string `json:"name"`
	Version            string `json:"version"`
	Platform           string `json:"platform"`
	BuildNumber        int64  `json:"build_number"`
	BuildID            string `json:"build_id,omitempty"`
	CASKey             string `json:"cas_key"`
	LocalPath          string `json:"local_path"`
	Sha256             string `json:"sha256"`
	SizeBytes          int64  `json:"size_bytes"`
	SourceUsed         string `json:"source_used"`
	SourceType         string `json:"source_type"`
	MirrorStatus       string `json:"mirror_status,omitempty"`
	VerificationResult string `json:"verification_result"`
	VerifiedAt         string `json:"verified_at"`
}

// receiptLocalKey returns the path within localStorage for the receipt JSON.
func receiptLocalKey(req ArtifactRequest) string {
	return filepath.Join("packages",
		req.PublisherID, req.Name, req.Version, req.Platform,
		fmt.Sprintf("%d", req.BuildNumber),
		"receipt.json")
}

// writeLocalReceipt persists a receipt JSON beside the CAS blob.
// Best-effort: errors are logged but never returned to the caller.
func (srv *server) writeLocalReceipt(req ArtifactRequest, result *ResolutionResult, sourceUsed string) {
	if srv.localStorage == nil {
		return
	}
	if sourceUsed == "" {
		sourceUsed = result.SourceName
	}
	mirrorStatus := "not_synced"
	if srv.mirrorStorage != nil {
		mirrorStatus = "pending"
	}
	receipt := ArtifactReceipt{
		PublisherID:        req.PublisherID,
		Name:               req.Name,
		Version:            req.Version,
		Platform:           req.Platform,
		BuildNumber:        req.BuildNumber,
		BuildID:            req.BuildID,
		CASKey:             result.LocalKey,
		LocalPath:          result.LocalPath,
		Sha256:             result.Sha256,
		SizeBytes:          result.SizeBytes,
		SourceUsed:         sourceUsed,
		SourceType:         result.SourceType,
		MirrorStatus:       mirrorStatus,
		VerificationResult: "ok",
		VerifiedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		slog.Warn("writeLocalReceipt: marshal failed", "key", result.LocalKey, "err", err)
		return
	}
	rKey := receiptLocalKey(req)
	if _, writeErr := srv.localStorage.WriteFileAtomic(context.Background(), rKey,
		strings.NewReader(string(data)), "", 0); writeErr != nil {
		slog.Warn("writeLocalReceipt: write failed", "key", result.LocalKey, "err", writeErr)
	}
}
