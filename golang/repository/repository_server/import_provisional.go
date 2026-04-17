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

	slog.Info("import-provisional: starting",
		"publisher", publisher, "name", name, "version", version,
		"platform", platform, "digest", digest[:16]+"...",
		"provisional_build_id", req.GetProvisionalBuildId())

	// Check release ledger for this package.
	ledger := srv.readLedger(ctx, publisher, name)

	// Check if this version already exists in the ledger.
	if ledger != nil {
		for _, entry := range ledger.Releases {
			if entry.Version == version && entry.Platform == platform {
				// Same version exists — check digest.
				if entry.Digest == digest {
					// Idempotent: same content already released.
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
					Message: fmt.Sprintf("version %s already released with different digest (existing=%s, import=%s) — admin must resolve", version, entry.Digest[:16]+"...", digest[:16]+"..."),
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
