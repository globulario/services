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
	"strings"
	"time"

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
