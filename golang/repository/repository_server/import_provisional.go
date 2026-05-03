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
	"bytes"
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
	if err := srv.requireCapability(CapRepoWrite); err != nil {
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
				if digestEqual(entry.Digest, digest) {
					// Same digest in ledger — verify the blob still exists in
					// object storage before declaring idempotent.
					ref := &repopb.ArtifactRef{
						PublisherId: publisher,
						Name:        name,
						Version:     version,
						Platform:    platform,
					}
					// Resolve build_number from ScyllaDB manifest for this build_id.
					var buildNum int64
					if m, _, _, ok := srv.findExistingArtifactByDigest(ctx, ref, digest); ok {
						buildNum = m.GetBuildNumber()
					}
					if !srv.artifactBlobPresent(ctx, ref, buildNum, entry.SizeBytes) {
						slog.Info("import-provisional: blob missing from object store — allowing re-import",
							"name", name, "version", version, "build_id", entry.BuildID,
							"blob_key", blobKeyForRef(ref, buildNum))
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

		// Write binary to local POSIX CAS first — local CAS is the installability authority.
		binKey := binaryStorageKey(key)
		if _, err := srv.localStorage.WriteFileAtomic(ctx, binKey,
			bytes.NewReader(req.GetData()), digest, int64(len(req.GetData()))); err != nil {
			return nil, status.Errorf(codes.Internal, "write binary to local CAS: %v", err)
		}
		// Best-effort mirror write — local success is sufficient.
		if srv.mirrorStorage != nil {
			_ = srv.mirrorStorage.WriteFile(ctx, binKey, req.GetData(), 0o644)
		}

		// Build manifest and promote to PUBLISHED (verifies local CAS + writes Scylla + manifest).
		manifest := &repopb.ArtifactManifest{
			Ref:         ref,
			BuildNumber: buildNumber,
			BuildId:     confirmedBuildID,
			Checksum:    digest,
			SizeBytes:   int64(len(req.GetData())),
			Provisional: false, // imported = no longer provisional
		}
		if err := srv.promoteToPublished(ctx, key, manifest); err != nil {
			return nil, status.Errorf(codes.Internal, "promote to PUBLISHED: %v", err)
		}
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

