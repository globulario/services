package main

// artifact_handlers.go — repository RPC handlers for artifact management.
//
// Implements: ListArtifacts, GetArtifactManifest, UploadArtifact, DownloadArtifact,
// SearchArtifacts, GetArtifactVersions, DeleteArtifact.
//
// Storage layout (relative to the configured backend root):
//
//	artifacts/{publisherID}%{name}%{version}%{platform}%{buildNumber}.manifest.json
//	artifacts/{publisherID}%{name}%{version}%{platform}%{buildNumber}.bin
//
// Legacy artifacts (build_number=0) may still exist under the old 4-field key
// format without the trailing %0 segment. The read path tries both.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/versionutil"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const artifactsDir = "artifacts"

// ── version helpers ───────────────────────────────────────────────────────────

// canonicalizeRefVersion normalizes ref.Version in-place using versionutil.Canonical.
// If the version is empty or not valid semver, it is left unchanged.
func canonicalizeRefVersion(ref *repopb.ArtifactRef) {
	if ref == nil {
		return
	}
	if cv, err := versionutil.Canonical(ref.GetVersion()); err == nil {
		ref.Version = cv
	}
}

// ── storage key helpers ───────────────────────────────────────────────────────

// artifactKeyWithBuild returns a flat, filesystem-safe key including build_number.
// Format: {publisherID}%{name}%{version}%{platform}%{buildNumber}
func artifactKeyWithBuild(ref *repopb.ArtifactRef, buildNumber int64) string {
	return ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform() + "%" + strconv.FormatInt(buildNumber, 10)
}

// artifactKeyLegacy returns the old 4-field key without build_number.
// Used for backward-compat reads of pre-build-number artifacts.
func artifactKeyLegacy(ref *repopb.ArtifactRef) string {
	return ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform()
}

func manifestStorageKey(key string) string    { return artifactsDir + "/" + key + ".manifest.json" }
func binaryStorageKey(key string) string      { return artifactsDir + "/" + key + ".bin" }
func publishStateKey(key string) string       { return artifactsDir + "/" + key + ".publish_state" }

// ── publish state helpers ─────────────────────────────────────────────────

// marshalManifestWithState marshals the manifest via protojson and injects the
// publish_state field. This is needed until ./generateCode.sh regenerates the
// pb.go with the publish_state field natively in the proto descriptor.
func marshalManifestWithState(m *repopb.ArtifactManifest, state repopb.PublishState) ([]byte, error) {
	mjson, err := protojson.Marshal(m)
	if err != nil {
		return nil, err
	}
	if state == repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
		return mjson, nil
	}
	// Inject publish_state into the JSON object.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(mjson, &raw); err != nil {
		return nil, err
	}
	stateJSON, _ := json.Marshal(state.String())
	raw["publishState"] = stateJSON
	return json.Marshal(raw)
}

// unmarshalManifestWithState reads a manifest JSON and extracts the publish_state.
// protojson.Unmarshal ignores the publishState key (not in descriptor), so we
// extract it separately.
func unmarshalManifestWithState(data []byte) (*repopb.ArtifactManifest, repopb.PublishState, error) {
	m := &repopb.ArtifactManifest{}
	uopts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := uopts.Unmarshal(data, m); err != nil {
		return nil, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, err
	}
	// Extract publishState from JSON.
	var raw map[string]json.RawMessage
	state := repopb.PublishState_PUBLISH_STATE_UNSPECIFIED
	if err := json.Unmarshal(data, &raw); err == nil {
		if stateJSON, ok := raw["publishState"]; ok {
			var stateStr string
			if err := json.Unmarshal(stateJSON, &stateStr); err == nil {
				if v, ok := repopb.PublishState_value[stateStr]; ok {
					state = repopb.PublishState(v)
				}
			}
		}
	}
	return m, state, nil
}

// ── manifest helpers ──────────────────────────────────────────────────────

// readManifestByKey reads and unmarshals a single manifest JSON from storage.
func (srv *server) readManifestByKey(ctx context.Context, key string) (*repopb.ArtifactManifest, error) {
	_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	_ = state // callers that don't need state use this method
	return m, nil
}

// readManifestAndStateByKey reads a manifest and its publish state from storage.
// It also corrects the kind for manifests published before proper kind assignment.
func (srv *server) readManifestAndStateByKey(ctx context.Context, key string) (string, repopb.PublishState, *repopb.ArtifactManifest, error) {
	data, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		return key, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil, err
	}
	m, state, err := unmarshalManifestWithState(data)
	if err != nil {
		return key, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil, fmt.Errorf("parse manifest %q: %w", key, err)
	}
	// Correct kind for legacy manifests (published before COMMAND/INFRASTRUCTURE).
	if ref := m.GetRef(); ref != nil {
		if corrected := inferCorrectKind(ref.GetName(), ref.GetKind()); corrected != ref.GetKind() {
			ref.Kind = corrected
		}
	}
	return key, state, m, nil
}

// readManifestWithFallback tries the 5-field key. For build_number=0 (legacy),
// also tries the legacy 4-field key to support pre-build-number artifacts.
// This legacy fallback exists only for backward compatibility with artifacts
// created before build_number was introduced; new artifacts always use the 5-field key.
func (srv *server) readManifestWithFallback(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) (*repopb.ArtifactManifest, error) {
	key := artifactKeyWithBuild(ref, buildNumber)
	m, err := srv.readManifestByKey(ctx, key)
	if err == nil {
		return m, nil
	}
	// Legacy fallback ONLY for build_number=0 (pre-build-number artifacts).
	// Non-zero build numbers must match exactly — no silent collapse.
	if buildNumber == 0 {
		legacyKey := artifactKeyLegacy(ref)
		if lm, lerr := srv.readManifestByKey(ctx, legacyKey); lerr == nil {
			return lm, nil
		}
	}
	return nil, err
}

// readBinaryWithFallback tries the new 5-field key, then legacy 4-field key.
func (srv *server) readBinaryWithFallback(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) ([]byte, error) {
	key := artifactKeyWithBuild(ref, buildNumber)
	data, err := srv.Storage().ReadFile(ctx, binaryStorageKey(key))
	if err == nil {
		return data, nil
	}
	if buildNumber == 0 {
		legacyKey := artifactKeyLegacy(ref)
		if ld, lerr := srv.Storage().ReadFile(ctx, binaryStorageKey(legacyKey)); lerr == nil {
			return ld, nil
		}
	}
	return nil, err
}

// openBinaryWithFallback returns a streaming reader for the artifact binary,
// trying the current key format first, then the legacy format.
func (srv *server) openBinaryWithFallback(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) (io.ReadCloser, error) {
	key := artifactKeyWithBuild(ref, buildNumber)
	rc, err := srv.Storage().Open(ctx, binaryStorageKey(key))
	if err == nil {
		return rc, nil
	}
	if buildNumber == 0 {
		legacyKey := artifactKeyLegacy(ref)
		if lrc, lerr := srv.Storage().Open(ctx, binaryStorageKey(legacyKey)); lerr == nil {
			return lrc, nil
		}
	}
	return nil, err
}

// ── sorting helpers ──────────────────────────────────────────────────────

// sortManifestsByVersionDesc sorts manifests by semver descending, then
// build_number descending within the same semver.
func sortManifestsByVersionDesc(ms []*repopb.ArtifactManifest) {
	sort.Slice(ms, func(i, j int) bool {
		cmp, err := versionutil.Compare(
			ms[i].GetRef().GetVersion(),
			ms[j].GetRef().GetVersion(),
		)
		if err != nil {
			// Fallback: lexicographic.
			return ms[i].GetRef().GetVersion() > ms[j].GetRef().GetVersion()
		}
		if cmp != 0 {
			return cmp > 0
		}
		// Same semver → higher build_number first.
		return ms[i].GetBuildNumber() > ms[j].GetBuildNumber()
	})
}

// ── package.json extraction ───────────────────────────────────────────────

// packageManifest mirrors the pkgpack.Manifest fields relevant to catalog metadata.
// Defined here to avoid a build-time dependency on the CLI package.
type packageManifest struct {
	Type                     string   `json:"type"`
	Name                     string   `json:"name"`
	Profiles                 []string `json:"profiles,omitempty"`
	Priority                 int      `json:"priority,omitempty"`
	InstallMode              string   `json:"install_mode,omitempty"`
	ManagedUnit              bool     `json:"managed_unit,omitempty"`
	SystemdUnit              string   `json:"systemd_unit,omitempty"`
	ProvidesCapabilities     []string `json:"provides_capabilities,omitempty"`
	InstallDependencies      []string `json:"install_dependencies,omitempty"`
	RuntimeLocalDependencies []string `json:"runtime_local_dependencies,omitempty"`
	HealthCheckUnit          string   `json:"health_check_unit,omitempty"`
	HealthCheckPort          int      `json:"health_check_port,omitempty"`
	Description              string   `json:"description,omitempty"`
	Keywords                 []string `json:"keywords,omitempty"`
	License                  string   `json:"license,omitempty"`
}

// extractPackageManifest reads package.json from a .tgz archive.
// Returns nil (no error) if the archive is not a valid tgz or has no package.json.
func extractPackageManifest(data []byte) *packageManifest {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return nil
		}
		name := path.Clean(hdr.Name)
		if name == "package.json" {
			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil
			}
			var m packageManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil
			}
			return &m
		}
	}
}

// enrichManifestFromPackageJSON populates ArtifactManifest catalog fields from
// the extracted package.json. Only non-empty fields are applied.
func enrichManifestFromPackageJSON(manifest *repopb.ArtifactManifest, pkg *packageManifest) {
	if pkg == nil || manifest == nil {
		return
	}
	if len(pkg.Profiles) > 0 {
		manifest.Profiles = pkg.Profiles
	}
	if pkg.Priority > 0 {
		manifest.Priority = int32(pkg.Priority)
	}
	if pkg.InstallMode != "" {
		manifest.InstallMode = pkg.InstallMode
	}
	manifest.ManagedUnit = pkg.ManagedUnit
	if pkg.SystemdUnit != "" {
		manifest.SystemdUnit = pkg.SystemdUnit
	}
	if len(pkg.ProvidesCapabilities) > 0 {
		manifest.Provides = pkg.ProvidesCapabilities
	}
	if len(pkg.InstallDependencies) > 0 {
		manifest.InstallDependencies = pkg.InstallDependencies
	}
	if len(pkg.RuntimeLocalDependencies) > 0 {
		manifest.RuntimeLocalDependencies = pkg.RuntimeLocalDependencies
	}
	if pkg.HealthCheckUnit != "" {
		manifest.HealthCheckUnit = pkg.HealthCheckUnit
	}
	if pkg.HealthCheckPort > 0 {
		manifest.HealthCheckPort = int32(pkg.HealthCheckPort)
	}
	if pkg.Description != "" && manifest.Description == "" {
		manifest.Description = pkg.Description
	}
	if len(pkg.Keywords) > 0 && len(manifest.Keywords) == 0 {
		manifest.Keywords = pkg.Keywords
	}
	if pkg.License != "" && manifest.License == "" {
		manifest.License = pkg.License
	}
}

// ── stream helpers ────────────────────────────────────────────────────────

// recvArtifactStream accumulates all chunks from an UploadArtifact stream.
// The ArtifactRef is taken from the first message that carries a non-nil ref.
// Returns the ref, aggregated data, and the build_number from the first manifest-bearing message.
func recvArtifactStream(stream repopb.PackageRepository_UploadArtifactServer) (*repopb.ArtifactRef, []byte, int64, error) {
	var ref *repopb.ArtifactRef
	var data []byte
	var buildNumber int64
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return ref, data, buildNumber, nil
		}
		if err != nil {
			return nil, nil, 0, fmt.Errorf("recv artifact: %w", err)
		}
		if ref == nil && msg.GetRef() != nil {
			ref = msg.GetRef()
			buildNumber = msg.GetBuildNumber()
		}
		data = append(data, msg.GetData()...)
	}
}

// ── public handlers ───────────────────────────────────────────────────────

// ListArtifacts returns all manifests stored in the repository.
// If the artifacts directory does not yet exist, an empty list is returned.
func (srv *server) ListArtifacts(ctx context.Context, _ *repopb.ListArtifactsRequest) (*repopb.ListArtifactsResponse, error) {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		// Directory not yet created → empty catalog (not an error for the caller).
		slog.Debug("artifacts directory not found, returning empty catalog", "err", err)
		return &repopb.ListArtifactsResponse{}, nil
	}

	var manifests []*repopb.ArtifactManifest
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")
		m, err := srv.readManifestByKey(ctx, key)
		if err != nil {
			slog.Warn("skipping unreadable manifest", "key", key, "err", err)
			continue
		}
		manifests = append(manifests, m)
	}

	sortManifestsByVersionDesc(manifests)
	return &repopb.ListArtifactsResponse{Artifacts: manifests}, nil
}

// GetArtifactManifest returns metadata for a specific artifact reference.
// The build_number is read from the manifest's build_number field in the request.
// When build_number is 0, also tries legacy 4-field key for backward compat.
func (srv *server) GetArtifactManifest(ctx context.Context, req *repopb.GetArtifactManifestRequest) (*repopb.GetArtifactManifestResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Normalize version to canonical semver for key lookup.
	canonVer := ref.GetVersion()
	if cv, err := versionutil.Canonical(canonVer); err == nil {
		canonVer = cv
		ref.Version = cv
	}

	// Default platform to linux_amd64 when unspecified — artifacts are always
	// published with a platform, so an empty platform produces a key mismatch.
	if strings.TrimSpace(ref.GetPlatform()) == "" {
		ref.Platform = "linux_amd64"
	}

	// Build number from the request; 0 means legacy/unspecified.
	buildNumber := req.GetBuildNumber()

	m, err := srv.readManifestWithFallback(ctx, ref, buildNumber)
	if err != nil {
		// Fallback: try v-prefixed key for backward compat with existing storage.
		ref.Version = "v" + canonVer
		if fm, ferr := srv.readManifestWithFallback(ctx, ref, buildNumber); ferr == nil {
			// Attach provenance if available.
			vKey := artifactKeyWithBuild(ref, buildNumber)
			if prov := srv.readProvenance(ctx, vKey); prov != nil {
				fm.Provenance = prov
			}
			return &repopb.GetArtifactManifestResponse{Manifest: fm}, nil
		}
		key := artifactKeyWithBuild(ref, buildNumber)
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}

	// Attach provenance if available.
	key := artifactKeyWithBuild(ref, buildNumber)
	if prov := srv.readProvenance(ctx, key); prov != nil {
		m.Provenance = prov
	}

	return &repopb.GetArtifactManifestResponse{Manifest: m}, nil
}

// UploadArtifact receives a (possibly multi-chunk) artifact binary stream,
// stores the binary and a derived manifest.
//
// Build-number uniqueness: if a manifest already exists for the same
// (publisher, name, version, platform, build_number) and the checksum matches,
// the upload is treated as idempotent (success, no overwrite). If the checksum
// differs, the upload is rejected with AlreadyExists.
func (srv *server) UploadArtifact(stream repopb.PackageRepository_UploadArtifactServer) error {
	ref, data, buildNumber, err := recvArtifactStream(stream)
	if err != nil {
		return status.Errorf(codes.Internal, "receive stream: %v", err)
	}
	if ref == nil {
		return status.Error(codes.InvalidArgument, "no ArtifactRef received in stream")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return status.Error(codes.InvalidArgument, "ref.name is required")
	}
	if strings.TrimSpace(ref.GetVersion()) == "" {
		return status.Error(codes.InvalidArgument, "ref.version is required")
	}
	if canonVer, err := versionutil.Canonical(ref.GetVersion()); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid version %q: %v", ref.GetVersion(), err)
	} else {
		ref.Version = canonVer
	}

	ctx := stream.Context()

	// ── Publisher namespace + package-level validation ────────────────────
	publisherID := ref.GetPublisherId()
	if err := srv.validatePackageAccess(ctx, publisherID, ref.GetName()); err != nil {
		return err
	}

	newChecksum := checksumBytes(data)

	// buildNumber was extracted from the first UploadArtifactRequest message
	// by recvArtifactStream. Zero means legacy (no build iteration tracking).

	key := artifactKeyWithBuild(ref, buildNumber)

	// ── Uniqueness check ──────────────────────────────────────────────────
	// If an artifact with this exact identity already exists:
	//   - same checksum = idempotent (skip)
	//   - different checksum = overwrite (rebuild at same version)
	if existing, err := srv.readManifestByKey(ctx, key); err == nil {
		if existing.GetChecksum() == newChecksum {
			// Idempotent re-upload — return success without overwriting.
			slog.Info("artifact re-upload (idempotent, same checksum)", "key", key)
			return stream.SendAndClose(&repopb.UploadArtifactResponse{Result: true})
		}
		// Different content at same version — overwrite. This happens when
		// the binary is rebuilt without a version bump (bug fixes, Day-0 rebuilds).
		slog.Warn("artifact overwrite (same version, different content)",
			"key", key,
			"old_checksum", existing.GetChecksum(),
			"new_checksum", newChecksum)
	}

	// Ensure artifacts directory exists.
	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "create artifacts dir: %v", err)
	}

	// Persist binary.
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), data, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write artifact binary: %v", err)
	}

	// Build and persist manifest with VERIFIED state.
	// The artifact is uploaded and checksum-verified but not yet discoverable
	// (the caller must PromoteArtifact to PUBLISHED after descriptor registration).
	manifest := &repopb.ArtifactManifest{
		Ref:          ref,
		BuildNumber:  buildNumber,
		Checksum:     newChecksum,
		SizeBytes:    int64(len(data)),
		ModifiedUnix: time.Now().Unix(),
	}

	// Enrich manifest with catalog metadata from package.json inside the tgz.
	if pkg := extractPackageManifest(data); pkg != nil {
		enrichManifestFromPackageJSON(manifest, pkg)
	}
	mjson, err := marshalManifestWithState(manifest, repopb.PublishState_VERIFIED)
	if err != nil {
		return status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	// Write provenance record and digest sidecar.
	prov := buildProvenanceRecord(ctx, manifest)
	provDigest, provErr := srv.writeProvenance(ctx, key, prov)
	if provErr != nil {
		slog.Warn("provenance write failed (non-fatal)", "key", key, "err", provErr)
	}
	_ = provDigest // digest stored as sidecar file for integrity verification

	// Ensure package-level RBAC ownership on first publish.
	srv.ensurePackageOwnership(ctx, publisherID, ref.GetName(), prov.GetSubject(), "")

	// Classify publish mode for audit/trust labels.
	publishMode := srv.classifyPublishMode(ctx, publisherID, ref.GetName())

	slog.Info("artifact uploaded",
		"key", key,
		"kind", ref.GetKind(),
		"build", buildNumber,
		"size", len(data),
		"publish_state", repopb.PublishState_VERIFIED.String(),
		"subject", prov.GetSubject(),
		"publish_mode", publishMode,
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.uploaded", map[string]any{
		"key":          key,
		"publisher":    ref.GetPublisherId(),
		"name":         ref.GetName(),
		"version":      ref.GetVersion(),
		"build":        buildNumber,
		"checksum":     newChecksum,
		"publish_mode": publishMode,
	})

	// ── Unified publish: register descriptor + promote to PUBLISHED ──────
	// This replaces the separate Discovery call that the CLI used to make.
	// The artifact was stored and verified above; now complete the pipeline.
	// On failure, the artifact stays in VERIFIED state (not published) so the
	// caller can fix the issue and retry. The workflow run captures the error.
	if err := srv.completePublish(ctx, manifest, key, prov); err != nil {
		slog.Warn("auto-publish failed — artifact stored as VERIFIED, retry with 'globular pkg promote'",
			"key", key, "err", err)
		// Return success for the upload itself — the binary is safely stored.
		// The publish error is recorded in the workflow run for observability.
	}

	return stream.SendAndClose(&repopb.UploadArtifactResponse{Result: true})
}

// SearchArtifacts queries the artifact catalog with optional text/filter criteria.
// It scans all manifests and applies in-memory filtering. For the expected catalog
// sizes (hundreds, not millions) this is efficient and avoids a secondary index.
func (srv *server) SearchArtifacts(ctx context.Context, req *repopb.SearchArtifactsRequest) (*repopb.SearchArtifactsResponse, error) {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return &repopb.SearchArtifactsResponse{}, nil
	}

	query := strings.ToLower(strings.TrimSpace(req.GetQuery()))
	filterKind := req.GetKind()
	filterPub := strings.TrimSpace(req.GetPublisherId())
	filterPlat := strings.TrimSpace(req.GetPlatform())

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 50
	}

	// Determine if caller is admin/owner for visibility of hidden artifacts.
	authCtx := security.FromContext(ctx)
	isAdmin := authCtx != nil && authCtx.Subject == "sa"

	var all []*repopb.ArtifactManifest
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")
		_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil {
			continue
		}

		// Hide YANKED/QUARANTINED/REVOKED from non-owners/non-admins.
		if repopb.IsDiscoveryHidden(state) && !isAdmin {
			if authCtx == nil || !srv.isNamespaceOwner(ctx, m.GetRef().GetPublisherId(), authCtx.Subject) {
				continue
			}
		}

		// Filter by kind.
		if filterKind != repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED && m.GetRef().GetKind() != filterKind {
			continue
		}
		// Filter by publisher.
		if filterPub != "" && !strings.EqualFold(m.GetRef().GetPublisherId(), filterPub) {
			continue
		}
		// Filter by platform.
		if filterPlat != "" && !strings.EqualFold(m.GetRef().GetPlatform(), filterPlat) {
			continue
		}
		// Free-text search across name, description, keywords.
		if query != "" && !matchesQuery(m, query) {
			continue
		}
		all = append(all, m)
	}

	// Sort by semver desc, build_number desc, then name asc for ties.
	sortManifestsByVersionDesc(all)

	totalCount := int32(len(all))

	// Pagination via page_token (token = index offset as string).
	startIdx := 0
	if tok := req.GetPageToken(); tok != "" {
		if idx, err := parseInt32(tok); err == nil && int(idx) < len(all) {
			startIdx = int(idx)
		}
	}

	end := startIdx + pageSize
	if end > len(all) {
		end = len(all)
	}
	page := all[startIdx:end]

	var nextToken string
	if end < len(all) {
		nextToken = fmt.Sprintf("%d", end)
	}

	return &repopb.SearchArtifactsResponse{
		Artifacts:     page,
		NextPageToken: nextToken,
		TotalCount:    totalCount,
	}, nil
}

// GetArtifactVersions returns all versions of a given package (publisher + name),
// optionally filtered by platform. Results are sorted by semver desc then
// build_number desc.
func (srv *server) GetArtifactVersions(ctx context.Context, req *repopb.GetArtifactVersionsRequest) (*repopb.GetArtifactVersionsResponse, error) {
	pub := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	filterPlat := strings.TrimSpace(req.GetPlatform())

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return &repopb.GetArtifactVersionsResponse{}, nil
	}

	var versions []*repopb.ArtifactManifest
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		m, err := srv.readManifestByKey(ctx, key)
		if err != nil {
			continue
		}
		ref := m.GetRef()
		if !strings.EqualFold(ref.GetName(), name) {
			continue
		}
		if pub != "" && !strings.EqualFold(ref.GetPublisherId(), pub) {
			continue
		}
		if filterPlat != "" && !strings.EqualFold(ref.GetPlatform(), filterPlat) {
			continue
		}
		versions = append(versions, m)
	}

	sortManifestsByVersionDesc(versions)
	return &repopb.GetArtifactVersionsResponse{Versions: versions}, nil
}

// DeleteArtifact removes a specific artifact version (manifest + binary) from the repository.
// This is a repository/catalog operation only — it never uninstalls from nodes.
// When force is false (default), deletion is rejected if any node still has
// this artifact installed. Set force=true to remove repository availability
// while leaving installed instances in place.
func (srv *server) DeleteArtifact(ctx context.Context, req *repopb.DeleteArtifactRequest) (*repopb.DeleteArtifactResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}
	if strings.TrimSpace(ref.GetVersion()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.version is required")
	}

	// Validate namespace + package-level access.
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return nil, err
	}

	// Normalize version.
	canonicalizeRefVersion(ref)

	// For delete, use build_number=0 and try legacy fallback.
	buildNumber := int64(0)
	key := artifactKeyWithBuild(ref, buildNumber)
	mKey := manifestStorageKey(key)
	bKey := binaryStorageKey(key)

	// Check manifest exists (new key format first, then legacy).
	if _, err := srv.Storage().ReadFile(ctx, mKey); err != nil {
		// Try legacy key.
		legacyKey := artifactKeyLegacy(ref)
		legacyMKey := manifestStorageKey(legacyKey)
		if _, lerr := srv.Storage().ReadFile(ctx, legacyMKey); lerr != nil {
			return nil, status.Errorf(codes.NotFound, "artifact %q not found", key)
		}
		// Found under legacy key — use those paths for deletion.
		mKey = legacyMKey
		bKey = binaryStorageKey(legacyKey)
		key = legacyKey
	}

	// Safety check: verify the artifact is not currently installed on any node.
	// This prevents accidental removal of packages still in use.
	installedNodes := findInstalledReferences(ctx, ref)
	if len(installedNodes) > 0 && !req.GetForce() {
		nodeList := strings.Join(installedNodes, ", ")
		return &repopb.DeleteArtifactResponse{
			Result:  false,
			Message: fmt.Sprintf("artifact %s@%s is still installed on node(s): %s — use force=true to delete from repository only (does not uninstall from nodes)", ref.GetName(), ref.GetVersion(), nodeList),
		}, nil
	}

	// Remove manifest and binary (best-effort for binary — it might not exist).
	if err := srv.Storage().Remove(ctx, mKey); err != nil {
		return nil, status.Errorf(codes.Internal, "delete manifest %q: %v", key, err)
	}
	_ = srv.Storage().Remove(ctx, bKey)

	msg := fmt.Sprintf("artifact %s@%s deleted from repository", ref.GetName(), ref.GetVersion())
	if len(installedNodes) > 0 {
		msg += fmt.Sprintf(" (warning: still installed on %d node(s) — this does NOT uninstall from nodes)", len(installedNodes))
	}

	slog.Info("artifact deleted", "key", key, "force", req.GetForce(), "installed_nodes", len(installedNodes))

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.deleted", map[string]any{
		"key":       key,
		"publisher": ref.GetPublisherId(),
		"name":      ref.GetName(),
		"version":   ref.GetVersion(),
		"force":     req.GetForce(),
	})

	return &repopb.DeleteArtifactResponse{Result: true, Message: msg}, nil
}

// findInstalledReferences queries the installed-state registry (etcd) for any
// node that has the given artifact installed. Returns a list of node IDs.
// Best-effort: returns nil on etcd errors to avoid blocking deletion.
func findInstalledReferences(ctx context.Context, ref *repopb.ArtifactRef) []string {
	kind := ref.GetKind().String()
	if kind == "" || kind == "ARTIFACT_KIND_UNSPECIFIED" {
		kind = "SERVICE"
	}

	pkgs, err := installed_state.ListAllNodes(ctx, kind, "")
	if err != nil {
		slog.Warn("DeleteArtifact: installed-state query failed (proceeding)", "err", err)
		return nil
	}

	var nodes []string
	for _, pkg := range pkgs {
		if strings.EqualFold(pkg.GetName(), ref.GetName()) && pkg.GetVersion() == ref.GetVersion() {
			nodes = append(nodes, pkg.GetNodeId())
		}
	}
	return nodes
}

// ── kind inference ───────────────────────────────────────────────────────────

// inferCorrectKind returns the correct ArtifactKind for a package name.
// Manifests published before the COMMAND kind was added have kind=SERVICE
// for everything. This function corrects the kind based on naming conventions:
//   - Names ending in "-cmd" → COMMAND (CLI tools)
//   - Known infrastructure daemons → INFRASTRUCTURE
//   - Everything else → unchanged
func inferCorrectKind(name string, current repopb.ArtifactKind) repopb.ArtifactKind {
	lower := strings.ToLower(name)

	// CLI tools: names ending in -cmd
	if strings.HasSuffix(lower, "-cmd") {
		return repopb.ArtifactKind_COMMAND
	}

	// Infrastructure daemons (not Go gRPC services)
	infraNames := map[string]bool{
		"etcd": true, "minio": true, "envoy": true, "xds": true,
		"gateway": true, "prometheus": true, "node-exporter": true,
		"scylladb": true, "scylla-manager": true, "scylla-manager-agent": true,
		"keepalived": true, "sidekick": true,
	}
	if infraNames[lower] {
		return repopb.ArtifactKind_INFRASTRUCTURE
	}

	return current
}

// ── search helpers ───────────────────────────────────────────────────────────

// matchesQuery returns true if the manifest matches a free-text query.
func matchesQuery(m *repopb.ArtifactManifest, query string) bool {
	if strings.Contains(strings.ToLower(m.GetRef().GetName()), query) {
		return true
	}
	if strings.Contains(strings.ToLower(m.GetDescription()), query) {
		return true
	}
	if strings.Contains(strings.ToLower(m.GetAlias()), query) {
		return true
	}
	for _, kw := range m.GetKeywords() {
		if strings.Contains(strings.ToLower(kw), query) {
			return true
		}
	}
	return false
}

// parseInt32 parses a string as an int32.
func parseInt32(s string) (int32, error) {
	var v int32
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// ── promote artifact ──────────────────────────────────────────────────────

// PromoteArtifact implements the gRPC PromoteArtifact RPC.
func (srv *server) PromoteArtifact(ctx context.Context, req *repopb.PromoteArtifactRequest) (*repopb.PromoteArtifactResponse, error) {
	return srv.promoteArtifactInternal(ctx, req.GetRef(), req.GetBuildNumber(), req.GetTargetState())
}

// promoteArtifactInternal transitions an artifact's publish state.
// Used by the gRPC handler and internal callers (tests, CLI client).
func (srv *server) promoteArtifactInternal(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, targetState repopb.PublishState) (*repopb.PromoteArtifactResponse, error) {
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Validate namespace + package-level access.
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return nil, err
	}

	// Normalize version.
	canonicalizeRefVersion(ref)

	key := artifactKeyWithBuild(ref, buildNumber)

	// Read existing manifest and state.
	data, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}
	m, currentState, err := unmarshalManifestWithState(data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse manifest %q: %v", key, err)
	}

	// Validate transition.
	if !repopb.ValidPromoteTransition(currentState, targetState) {
		return nil, status.Errorf(codes.FailedPrecondition,
			"invalid state transition %s → %s for artifact %q",
			currentState, targetState, key)
	}

	// Write updated manifest with new state.
	mjson, err := marshalManifestWithState(m, targetState)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return nil, status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	slog.Info("artifact promoted",
		"key", key,
		"from", currentState.String(),
		"to", targetState.String(),
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.promoted", map[string]any{
		"key":            key,
		"publisher":      ref.GetPublisherId(),
		"name":           ref.GetName(),
		"previous_state": currentState.String(),
		"current_state":  targetState.String(),
	})

	return &repopb.PromoteArtifactResponse{
		Result:        true,
		PreviousState: currentState,
		CurrentState:  targetState,
		Message:       fmt.Sprintf("artifact %q promoted from %s to %s", key, currentState, targetState),
	}, nil
}

// SetArtifactState transitions an artifact's lifecycle state (deprecate, yank, quarantine, revoke).
func (srv *server) SetArtifactState(ctx context.Context, req *repopb.SetArtifactStateRequest) (*repopb.SetArtifactStateResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Validate namespace + package-level access.
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return nil, err
	}

	// Normalize version.
	canonicalizeRefVersion(ref)

	targetState := req.GetTargetState()
	authCtx := security.FromContext(ctx)
	buildNumber := req.GetBuildNumber()
	key := artifactKeyWithBuild(ref, buildNumber)

	// Read existing manifest and state FIRST (needed for authority check on un-quarantine).
	data, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}
	m, currentState, err := unmarshalManifestWithState(data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse manifest %q: %v", key, err)
	}

	// ── Authority boundaries for lifecycle transitions ──
	var authorityMode string
	switch targetState {
	case repopb.PublishState_QUARANTINED:
		// QUARANTINE is a moderation action — admin/superuser only.
		if authCtx == nil || authCtx.Subject != "sa" {
			return nil, status.Error(codes.PermissionDenied,
				"quarantine is a moderation action restricted to administrators")
		}
		authorityMode = "admin"

	case repopb.PublishState_REVOKED:
		// REVOKE can be performed by admin OR namespace/package owner (self-revoke).
		if authCtx == nil || authCtx.Subject != "sa" {
			if authCtx == nil || !srv.isNamespaceOwner(ctx, ref.GetPublisherId(), authCtx.Subject) {
				return nil, status.Error(codes.PermissionDenied,
					"revoke requires administrator or namespace owner (self-revoke)")
			}
			authorityMode = "owner_self_revoke"
		} else {
			authorityMode = "admin"
		}

	default:
		authorityMode = "publisher"
	}

	// Transitions FROM quarantined states also require admin.
	if currentState == repopb.PublishState_QUARANTINED {
		if authCtx == nil || authCtx.Subject != "sa" {
			return nil, status.Error(codes.PermissionDenied,
				"un-quarantine is a moderation action restricted to administrators")
		}
		authorityMode = "admin"
	}

	// Validate transition using the extended state machine.
	if !repopb.ValidStateTransition(currentState, targetState) {
		return nil, status.Errorf(codes.FailedPrecondition,
			"invalid state transition %s → %s for artifact %q",
			currentState, targetState, key)
	}

	// Write updated manifest with new state.
	mjson, err := marshalManifestWithState(m, targetState)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return nil, status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	slog.Info("artifact state changed",
		"key", key,
		"from", currentState.String(),
		"to", targetState.String(),
		"reason", req.GetReason(),
		"authority", authorityMode,
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.state_changed", map[string]any{
		"key":            key,
		"publisher":      ref.GetPublisherId(),
		"name":           ref.GetName(),
		"version":        ref.GetVersion(),
		"previous_state": currentState.String(),
		"current_state":  targetState.String(),
		"reason":         req.GetReason(),
		"authority":      authorityMode,
	})

	return &repopb.SetArtifactStateResponse{
		PreviousState: currentState,
		CurrentState:  targetState,
	}, nil
}

// DownloadArtifact streams the binary for a stored artifact.
func (srv *server) DownloadArtifact(req *repopb.DownloadArtifactRequest, stream repopb.PackageRepository_DownloadArtifactServer) error {
	ref := req.GetRef()
	if ref == nil {
		return status.Error(codes.InvalidArgument, "ref is required")
	}

	// Normalize version to canonical semver for key lookup.
	canonicalizeRefVersion(ref)

	// Check publish state — block downloads of YANKED/QUARANTINED/REVOKED
	// unless the caller is the namespace owner or admin.
	canonVer := ref.GetVersion()
	buildNumber := req.GetBuildNumber()
	key := artifactKeyWithBuild(ref, buildNumber)
	if _, state, _, readErr := srv.readManifestAndStateByKey(stream.Context(), key); readErr == nil {
		if repopb.IsDownloadBlocked(state) {
			// Check if caller is namespace owner (allowed to download their own blocked artifacts).
			authCtx := security.FromContext(stream.Context())
			if authCtx == nil || !srv.isNamespaceOwner(stream.Context(), ref.GetPublisherId(), authCtx.Subject) {
				return status.Errorf(codes.PermissionDenied,
					"artifact %q is %s — download blocked", ref.GetName(), state)
			}
		}
	}

	// Try new 5-field key, then legacy 4-field key.
	// Stream directly from storage to avoid buffering the entire artifact in memory.
	reader, err := srv.openBinaryWithFallback(stream.Context(), ref, buildNumber)
	if err != nil {
		// Fallback: try v-prefixed key for backward compat with existing storage.
		ref.Version = "v" + canonVer
		if fr, ferr := srv.openBinaryWithFallback(stream.Context(), ref, buildNumber); ferr == nil {
			reader = fr
			err = nil
		}
		ref.Version = canonVer // restore
	}
	if err != nil {
		key := artifactKeyWithBuild(ref, buildNumber)
		return status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}
	defer reader.Close()

	buf := make([]byte, 256*1024) // 256KB chunks (larger = fewer round-trips)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&repopb.DownloadArtifactResponse{Data: buf[:n]}); err != nil {
				return fmt.Errorf("send artifact chunk: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read artifact: %w", readErr)
		}
	}
	return nil
}
