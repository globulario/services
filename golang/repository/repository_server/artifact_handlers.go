package main

// artifact_handlers.go — repository RPC handlers for artifact management.
//
// Implements: ListArtifacts, GetArtifactManifest, UploadArtifact, DownloadArtifact.
//
// Storage layout (relative to the configured backend root):
//
//	artifacts/{publisherID}%{name}%{version}%{platform}.manifest.json  — protojson manifest
//	artifacts/{publisherID}%{name}%{version}%{platform}.bin            — raw binary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/plan/versionutil"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	resourcepb "github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const artifactsDir = "artifacts"

// ── storage key helpers ───────────────────────────────────────────────────────

// artifactKey returns a flat, filesystem-safe key component from an ArtifactRef.
// Format: {publisherID}%{name}%{version}%{platform}
func artifactKey(ref *repopb.ArtifactRef) string {
	return ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform()
}

func manifestStorageKey(key string) string { return artifactsDir + "/" + key + ".manifest.json" }
func binaryStorageKey(key string) string   { return artifactsDir + "/" + key + ".bin" }

// ── manifest helpers ──────────────────────────────────────────────────────────

// readManifestByKey reads and unmarshals a single manifest JSON from storage.
func (srv *server) readManifestByKey(ctx context.Context, key string) (*repopb.ArtifactManifest, error) {
	data, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		return nil, err
	}
	m := &repopb.ArtifactManifest{}
	if err := protojson.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", key, err)
	}
	return m, nil
}

// ── stream helpers ────────────────────────────────────────────────────────────

// recvArtifactStream accumulates all chunks from an UploadArtifact stream.
// The ArtifactRef is taken from the first message that carries a non-nil ref.
func recvArtifactStream(stream repopb.PackageRepository_UploadArtifactServer) (*repopb.ArtifactRef, []byte, error) {
	var ref *repopb.ArtifactRef
	var data []byte
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return ref, data, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("recv artifact: %w", err)
		}
		if ref == nil && msg.GetRef() != nil {
			ref = msg.GetRef()
		}
		data = append(data, msg.GetData()...)
	}
}

// ── public handlers ───────────────────────────────────────────────────────────

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
	return &repopb.ListArtifactsResponse{Artifacts: manifests}, nil
}

// GetArtifactManifest returns metadata for a specific artifact reference.
func (srv *server) GetArtifactManifest(ctx context.Context, req *repopb.GetArtifactManifestRequest) (*repopb.GetArtifactManifestResponse, error) {
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Normalize version to canonical semver for key lookup.
	if canonVer, err := versionutil.Canonical(ref.GetVersion()); err == nil {
		ref.Version = canonVer
	}

	key := artifactKey(ref)
	m, err := srv.readManifestByKey(ctx, key)
	if err != nil {
		// Fallback: try v-prefixed key for backward compat with existing storage.
		ref.Version = "v" + ref.GetVersion()
		fallbackKey := artifactKey(ref)
		if fm, ferr := srv.readManifestByKey(ctx, fallbackKey); ferr == nil {
			return &repopb.GetArtifactManifestResponse{Manifest: fm}, nil
		}
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}
	return &repopb.GetArtifactManifestResponse{Manifest: m}, nil
}

// UploadArtifact receives a (possibly multi-chunk) artifact binary stream,
// stores the binary and a derived manifest.
func (srv *server) UploadArtifact(stream repopb.PackageRepository_UploadArtifactServer) error {
	ref, data, err := recvArtifactStream(stream)
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
	key := artifactKey(ref)

	// Ensure artifacts directory exists.
	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "create artifacts dir: %v", err)
	}

	// Persist binary.
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), data, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write artifact binary: %v", err)
	}

	// Build and persist manifest.
	manifest := &repopb.ArtifactManifest{
		Ref:          ref,
		Checksum:     checksumBytes(data),
		SizeBytes:    int64(len(data)),
		ModifiedUnix: time.Now().Unix(),
	}
	mjson, err := protojson.Marshal(manifest)
	if err != nil {
		return status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	slog.Info("artifact uploaded",
		"key", key,
		"kind", ref.GetKind(),
		"size", len(data),
	)
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

	var all []*repopb.ArtifactManifest
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")
		m, err := srv.readManifestByKey(ctx, key)
		if err != nil {
			continue
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

	// Sort by published_unix desc (newest first), then name asc.
	sort.Slice(all, func(i, j int) bool {
		if all[i].GetPublishedUnix() != all[j].GetPublishedUnix() {
			return all[i].GetPublishedUnix() > all[j].GetPublishedUnix()
		}
		return all[i].GetRef().GetName() < all[j].GetRef().GetName()
	})

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
// optionally filtered by platform.
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

	// Sort by version descending (newest first).
	sort.Slice(versions, func(i, j int) bool {
		cmp, err := versionutil.Compare(versions[i].GetRef().GetVersion(), versions[j].GetRef().GetVersion())
		if err != nil {
			return versions[i].GetRef().GetVersion() > versions[j].GetRef().GetVersion()
		}
		return cmp > 0
	})

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

	// Normalize version.
	if cv, err := versionutil.Canonical(ref.GetVersion()); err == nil {
		ref.Version = cv
	}

	key := artifactKey(ref)
	mKey := manifestStorageKey(key)
	bKey := binaryStorageKey(key)

	// Check manifest exists.
	if _, err := srv.Storage().ReadFile(ctx, mKey); err != nil {
		return nil, status.Errorf(codes.NotFound, "artifact %q not found", key)
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

	pkgs, err := installed_state.ListAllNodes(ctx, kind)
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

// DownloadArtifact streams the binary for a stored artifact.
func (srv *server) DownloadArtifact(req *repopb.DownloadArtifactRequest, stream repopb.PackageRepository_DownloadArtifactServer) error {
	ref := req.GetRef()
	if ref == nil {
		return status.Error(codes.InvalidArgument, "ref is required")
	}

	// Normalize version to canonical semver for key lookup.
	if canonVer, err := versionutil.Canonical(ref.GetVersion()); err == nil {
		ref.Version = canonVer
	}

	key := artifactKey(ref)
	canonVer := ref.GetVersion() // already normalized above
	data, err := srv.Storage().ReadFile(stream.Context(), binaryStorageKey(key))
	if err != nil {
		// Fallback 1: try v-prefixed key for backward compat with existing storage.
		ref.Version = "v" + canonVer
		fallbackKey := artifactKey(ref)
		if fdata, ferr := srv.Storage().ReadFile(stream.Context(), binaryStorageKey(fallbackKey)); ferr == nil {
			data = fdata
			err = nil
		}
		ref.Version = canonVer // restore
	}
	if err != nil {
		// Fallback 2: try the legacy bundle storage path (packages-repository/{UUID}.tar.gz).
		// Bundles uploaded via UploadBundle use a different key scheme.
		desc := &resourcepb.PackageDescriptor{
			PublisherID: ref.GetPublisherId(),
			Name:        ref.GetName(),
			Version:     canonVer,
		}
		desc.Id = descriptorID(desc)
		bID := bundleID(desc, ref.GetPlatform())
		bundleKey := "packages-repository/" + bID + ".tar.gz"
		if fdata, ferr := srv.Storage().ReadFile(stream.Context(), bundleKey); ferr == nil {
			data = fdata
			err = nil
			slog.Info("artifact served from legacy bundle path", "key", key, "bundle_key", bundleKey)
		}
	}
	if err != nil {
		return status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}

	const chunk = 32 * 1024
	for off := 0; off < len(data); off += chunk {
		end := off + chunk
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&repopb.DownloadArtifactResponse{Data: data[off:end]}); err != nil {
			return fmt.Errorf("send artifact chunk: %w", err)
		}
	}
	return nil
}
