package main

// import_provisional.go — Phase 6: Day-0 provisional artifact import.
//
// When a cluster bootstraps (day-0), packages are installed without a
// repository. These packages carry provisional identity (locally-generated
// build_id, unconfirmed version). When the repository becomes available,
// the node-agent calls ImportProvisionalArtifact to confirm each package.
//
// The import flow:
//   1. Check if version already exists in the release ledger
//      - YES + same digest → link to existing release (idempotent)
//      - YES + different digest → REJECT (conflict)
//      - NO → accept as new release
//   2. Assign repository-issued build_id
//   3. Add to release ledger
//   4. Return confirmed identity
//
// INV-9: Day-0 artifacts are provisional until imported.
// INV-10: Repair never silently rewrites history.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func truncDigest(d string) string {
	if len(d) > 16 {
		return d[:16]
	}
	return d
}

// ImportProvisionalArtifact handles the day-0 → day-1 transition for a
// provisionally installed package.
func (srv *server) ImportProvisionalArtifact(ctx context.Context, req *repopb.ImportProvisionalRequest) (*repopb.ImportProvisionalResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}

	publisher := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	version := strings.TrimSpace(req.GetVersion())
	platform := strings.TrimSpace(req.GetPlatform())
	digest := strings.TrimSpace(req.GetDigest())

	if publisher == "" {
		publisher = "core@globular.io"
	}
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if version == "" {
		return nil, status.Error(codes.InvalidArgument, "version is required")
	}
	if platform == "" {
		platform = "linux_amd64"
	}
	if digest == "" {
		return nil, status.Error(codes.InvalidArgument, "digest is required (content fingerprint)")
	}

	// Normalize version.
	if cv, err := versionutil.Canonical(version); err == nil {
		version = cv
	}

	digestLog := digest
	if len(digestLog) > 16 {
		digestLog = digestLog[:16] + "..."
	}
	slog.Info("import-provisional: starting",
		"publisher", publisher, "name", name, "version", version,
		"platform", platform, "digest", digestLog,
		"provisional_build_id", req.GetProvisionalBuildId())

	// Check release ledger for this package.
	ledger := srv.readLedger(ctx, publisher, name)

	// Check if this version already exists in the ledger.
	if ledger != nil {
		for _, entry := range ledger.Releases {
			if entry.Version == version && entry.Platform == platform {
				// Same version exists — check digest.
				if entry.Digest == digest {
					// Same digest in ledger — verify the blob still exists in
					// object storage before declaring idempotent. If the blob was
					// lost (e.g. MinIO standalone→distributed transition), allow
					// re-import so the blob is recreated.
					if blobMissing := srv.isArtifactBlobMissing(ctx, entry.BuildID); blobMissing {
						slog.Info("import-provisional: blob missing from object store — allowing re-import",
							"name", name, "version", version, "build_id", entry.BuildID)
						// Fall through to normal import path — blob will be recreated.
						break
					}
					// Idempotent: same content already released and blob exists.
					slog.Info("import-provisional: idempotent — same digest already released",
						"name", name, "version", version, "build_id", entry.BuildID)
					return &repopb.ImportProvisionalResponse{
						Ok:               true,
						ConfirmedBuildId: entry.BuildID,
						ConfirmedVersion: version,
						State:            "RELEASED",
						Message:          "already released with matching digest",
					}, nil
				}
				// Conflict: same version, different digest.
				return &repopb.ImportProvisionalResponse{
					Ok:      false,
					Message: fmt.Sprintf("version %s already released with different digest (existing=%s..., import=%s...) — admin must resolve", version, truncDigest(entry.Digest), truncDigest(digest)),
				}, nil
			}
		}

		// Check monotonicity.
		if ledger.LatestVersion != "" {
			cmp, err := versionutil.Compare(version, ledger.LatestVersion)
			if err == nil && cmp < 0 {
				return &repopb.ImportProvisionalResponse{
					Ok:      false,
					Message: fmt.Sprintf("version %s < latest released %s — non-monotonic import rejected", version, ledger.LatestVersion),
				}, nil
			}
		}
	}

	// Accept: new version or first release for this package.
	confirmedBuildID := uuid.Must(uuid.NewV7()).String()

	// If artifact data was provided, store it in the repository.
	if len(req.GetData()) > 0 {
		// Store binary.
		ref := &repopb.ArtifactRef{
			PublisherId: publisher,
			Name:        name,
			Version:     version,
			Platform:    platform,
		}
		buildNumber := int64(0)
		if ledger != nil {
			// Count existing releases at this version for build_number.
			for _, e := range ledger.Releases {
				if e.Version == version {
					buildNumber++
				}
			}
		}
		key := artifactKeyWithBuild(ref, buildNumber)

		if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
			return nil, status.Errorf(codes.Internal, "create artifacts dir: %v", err)
		}
		if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), req.GetData(), 0o644); err != nil {
			return nil, status.Errorf(codes.Internal, "write binary: %v", err)
		}

		// Build and store manifest.
		manifest := &repopb.ArtifactManifest{
			Ref:          ref,
			BuildNumber:  buildNumber,
			BuildId:      confirmedBuildID,
			Checksum:     digest,
			SizeBytes:    int64(len(req.GetData())),
			Provisional:  false, // imported = no longer provisional
		}
		mjson, err := marshalManifestWithState(manifest, repopb.PublishState_PUBLISHED)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal manifest: %v", err)
		}
		if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
			return nil, status.Errorf(codes.Internal, "write manifest: %v", err)
		}
		srv.syncManifestToScylla(ctx, key, manifest, repopb.PublishState_PUBLISHED, mjson)
	}

	// Add to release ledger.
	if err := srv.appendToLedger(ctx, publisher, name, version, confirmedBuildID, digest, platform, int64(len(req.GetData()))); err != nil {
		slog.Warn("import-provisional: ledger append failed", "name", name, "err", err)
		// Non-fatal: import still succeeds, ledger can be rebuilt.
	}

	slog.Info("import-provisional: success",
		"name", name, "version", version,
		"confirmed_build_id", confirmedBuildID,
		"provisional_build_id", req.GetProvisionalBuildId())

	return &repopb.ImportProvisionalResponse{
		Ok:               true,
		ConfirmedBuildId: confirmedBuildID,
		ConfirmedVersion: version,
		State:            "RELEASED",
		Message:          fmt.Sprintf("imported %s@%s as RELEASED", name, version),
	}, nil
}

// isArtifactBlobMissing checks whether the binary blob for a given build_id
// exists in object storage. Returns true when the blob is confirmed missing.
// Returns false when the blob exists or when the check cannot be performed
// (MinIO unavailable, ScyllaDB unavailable — assume blob exists to be safe).
func (srv *server) isArtifactBlobMissing(ctx context.Context, buildID string) bool {
	if srv.scylla == nil {
		return false // can't check without ScyllaDB
	}

	// Find the storage key for this build_id from ScyllaDB manifest.
	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		return false // can't verify, assume exists
	}

	for _, row := range rows {
		m, _, parseErr := manifestFromRow(row)
		if parseErr != nil || m.GetBuildId() != buildID {
			continue
		}
		// Found the manifest — check if the binary blob exists in storage.
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		blobKey := binaryStorageKey(artifactKeyWithBuild(ref, m.GetBuildNumber()))

		_, readErr := srv.Storage().Stat(ctx, blobKey)
		if readErr != nil {
			return true // blob missing
		}
		return false // blob exists
	}

	return false // build_id not found in manifests, can't determine
}
