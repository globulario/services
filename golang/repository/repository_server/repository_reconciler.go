package main

// repository_reconciler.go — Local CAS ↔ Scylla consistency reconciler.
//
// Truth layers (per CLAUDE.md):
//   Upstream/GitHub  = release authority
//   Local POSIX CAS  = installable local truth
//   ScyllaDB         = searchable metadata, state, audit, diagnostics
//   etcd             = desired cluster intent / source policy
//
// Reconciler invariant: every Scylla PUBLISHED row must have a verified local
// POSIX blob. If the blob is missing, the reconciler attempts repair from the
// source chain; if repair fails, the artifact is downgraded to BROKEN_MISSING_BLOB
// in both artifact_state and Scylla publish_state. The artifact stays VERIFIED
// (not BROKEN) if repair is in progress — terminal broken state means all sources
// were exhausted.
//
// Reverse pass: scan local POSIX receipt files → rebuild any missing Scylla rows.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	stagedPackageProjectionInterval = 10 * time.Second
	stagedPackageStableAge          = 2 * time.Second
)

// ── Forward pass: Scylla → local POSIX ───────────────────────────────────────

// reconcileLocalCASVsScylla verifies every PUBLISHED Scylla row has a local
// POSIX blob. Missing blobs are repaired from the source chain; if repair fails,
// the artifact is downgraded.
func (srv *server) reconcileLocalCASVsScylla(ctx context.Context) {
	if srv.scylla == nil || srv.localStorage == nil {
		return
	}
	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		slog.Warn("reconciler: scylla list failed", "err", err)
		return
	}

	var okCount, repaired, broken int
	for _, row := range rows {
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			continue
		}
		key := row.ArtifactKey
		binKey := binaryStorageKey(key)

		_, statErr := srv.localStorage.Stat(ctx, binKey)
		if statErr == nil {
			okCount++
			continue
		}

		// Local blob missing — try to repair from source chain.
		slog.Warn("reconciler: local blob missing, attempting repair",
			"key", key, "publisher", row.PublisherID, "name", row.Name)

		manifest, _, parseErr := manifestFromRow(row)
		if parseErr != nil {
			slog.Warn("reconciler: cannot parse manifest row", "key", key, "err", parseErr)
			broken++
			continue
		}
		req := artifactRequestFromManifest(manifest, row.BuildNumber)
		if _, resolveErr := srv.ResolveArtifactToLocal(ctx, req); resolveErr != nil {
			slog.Error("reconciler: repair failed — marking BROKEN_MISSING_BLOB",
				"key", key, "err", resolveErr)
			srv.downgradeToMissingBlob(ctx, key, manifest.GetRef(), row.BuildNumber)
			broken++
		} else {
			slog.Info("reconciler: local blob repaired", "key", key)
			repaired++
		}
	}
	slog.Info("reconciler: local-CAS-vs-scylla complete",
		"ok", okCount, "repaired", repaired, "broken", broken)
}

// downgradeToMissingBlob sets artifact_state=BROKEN_MISSING_BLOB in the etcd
// pipeline state machine. The Scylla publish_state is left as PUBLISHED so the
// manifest remains discoverable, but repository_findings.go will report
// REPO_FIND_PUBLISHED_MISSING_BLOB and VerifyArtifact will return BROKEN.
// Operators use `globular repository repair` to restore the blob.
func (srv *server) downgradeToMissingBlob(ctx context.Context, key string, ref *repopb.ArtifactRef, buildNumber int64) {
	fields := ArtifactStateFields{
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
		BuildNumber: buildNumber,
	}
	_ = srv.transitionArtifactState(ctx, key, PipelineBrokenMissingBlob,
		"reconciler_local_blob_missing", "", fields)
}

// ── Reverse pass: local POSIX receipts → Scylla ──────────────────────────────

// reconcileScyllaFromLocalCAS scans local POSIX receipts and rebuilds any
// missing Scylla rows. Used after Scylla data loss or schema migration.
func (srv *server) reconcileScyllaFromLocalCAS(ctx context.Context) {
	if srv.scylla == nil || srv.localStorage == nil {
		return
	}
	localRoot := srv.localStorage.LocalPath("packages")
	var rebuilt, skipped, errors int

	walkErr := filepath.Walk(localRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "/receipt.json") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			slog.Warn("reconciler: cannot read receipt", "path", path, "err", readErr)
			errors++
			return nil
		}
		var receipt ArtifactReceipt
		if jsonErr := json.Unmarshal(raw, &receipt); jsonErr != nil {
			slog.Warn("reconciler: cannot parse receipt", "path", path, "err", jsonErr)
			errors++
			return nil
		}

		ref := &repopb.ArtifactRef{
			PublisherId: receipt.PublisherID,
			Name:        receipt.Name,
			Version:     receipt.Version,
			Platform:    receipt.Platform,
		}
		key := artifactKeyWithBuild(ref, receipt.BuildNumber)

		// Check if Scylla already has this row.
		if _, _, _, rowErr := srv.readManifestAndStateByKey(ctx, key); rowErr == nil {
			skipped++
			return nil
		}

		// Scylla miss — try to read the manifest from local POSIX and rebuild the row.
		mKey := filepath.Join("packages",
			receipt.PublisherID, receipt.Name, receipt.Version, receipt.Platform,
			fmt.Sprintf("%d", receipt.BuildNumber), "manifest.json")
		mData, mErr := srv.localStorage.ReadFile(ctx, mKey)
		if mErr != nil {
			slog.Warn("reconciler: local manifest not found for receipt", "key", key, "err", mErr)
			errors++
			return nil
		}
		manifest, state, parseErr := unmarshalManifestWithState(mData)
		if parseErr != nil {
			slog.Warn("reconciler: cannot parse local manifest", "key", key, "err", parseErr)
			errors++
			return nil
		}
		mjson := mData
		srv.syncManifestToScylla(ctx, key, manifest, state, mjson)
		slog.Info("reconciler: rebuilt scylla row from local receipt", "key", key)
		rebuilt++
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		slog.Warn("reconciler: walk error", "err", walkErr)
	}
	slog.Info("reconciler: scylla-from-local-cas complete",
		"rebuilt", rebuilt, "skipped", skipped, "errors", errors)
}

// reconcileScyllaFromStagedPackages scans staged package archives under the
// node-local bootstrap/join package dirs and projects valid artifacts into the
// repository-owned authority path (UploadArtifact -> local CAS + Scylla).
//
// The staged directory is only evidence of bytes on disk. Publication still
// flows through repository-owned gates: package identity extraction, checksum
// idempotency, manifest sync, descriptor registration, and promotion to
// PUBLISHED. This keeps /var/lib/globular/packages as local-input truth while
// preserving Scylla + repository CAS as Layer 1 authority.
func (srv *server) reconcileScyllaFromStagedPackages(ctx context.Context, dirs []string) {
	if srv.scylla == nil || srv.localStorage == nil {
		return
	}

	var published, skipped, invalid, failures int
	seen := make(map[string]struct{})

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			slog.Warn("reconciler: cannot stat staged package dir", "dir", dir, "err", err)
			continue
		}
		if !info.IsDir() {
			continue
		}

		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				slog.Warn("reconciler: staged walk error", "dir", dir, "path", path, "err", err)
				failures++
				return nil
			}
			if info == nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".tgz") {
				return nil
			}
			if !isStableStagedPackage(info, time.Now()) {
				slog.Debug("reconciler: staged package still settling, skip until next projection",
					"path", path,
					"modified", info.ModTime(),
					"stable_age", stagedPackageStableAge.String())
				skipped++
				return nil
			}
			clean := filepath.Clean(path)
			if _, ok := seen[clean]; ok {
				return nil
			}
			seen[clean] = struct{}{}

			status, pubErr := srv.publishStagedArchive(ctx, clean)
			switch status {
			case stagedPublishPublished:
				published++
			case stagedPublishSkipped:
				skipped++
			case stagedPublishInvalid:
				invalid++
			default:
				failures++
			}
			if pubErr != nil {
				slog.Warn("reconciler: staged package publish failed", "path", clean, "status", status, "err", pubErr)
			}
			return nil
		})
		if walkErr != nil && !os.IsNotExist(walkErr) {
			slog.Warn("reconciler: staged package walk failed", "dir", dir, "err", walkErr)
		}
	}

	slog.Info("reconciler: staged-package projection complete",
		"dirs", len(dirs),
		"published", published,
		"skipped", skipped,
		"invalid", invalid,
		"failures", failures)
}

func isStableStagedPackage(info os.FileInfo, now time.Time) bool {
	if info == nil {
		return false
	}
	if info.Size() <= 0 {
		return false
	}
	if info.ModTime().IsZero() {
		return true
	}
	return !info.ModTime().After(now) && now.Sub(info.ModTime()) >= stagedPackageStableAge
}

type stagedPublishStatus string

const (
	stagedPublishPublished stagedPublishStatus = "published"
	stagedPublishSkipped   stagedPublishStatus = "skipped"
	stagedPublishInvalid   stagedPublishStatus = "invalid"
	stagedPublishFailed    stagedPublishStatus = "failed"
)

func (srv *server) publishStagedArchive(ctx context.Context, archivePath string) (stagedPublishStatus, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return stagedPublishFailed, err
	}

	pkg, err := parseStagedPackageManifest(data)
	if err != nil {
		return stagedPublishInvalid, fmt.Errorf("parse package.json: %w", err)
	}

	kind := registryArtifactKind(pkg.Name, stagedPackageKind(pkg.Type))
	ref := &repopb.ArtifactRef{
		PublisherId: pkg.Publisher,
		Name:        pkg.Name,
		Version:     pkg.Version,
		Platform:    pkg.Platform,
		Kind:        kind,
	}

	if existing, _, _, ok := srv.findExistingArtifactByDigest(ctx, ref, checksumBytes(data)); ok {
		slog.Info("reconciler: staged archive already published", "path", archivePath, "build_id", existing.GetBuildId())
		return stagedPublishSkipped, nil
	}

	resp, err := srv.uploadArtifactInternally(ctx, ref, data)
	if err != nil {
		return stagedPublishFailed, err
	}
	if resp == nil || !resp.GetResult() {
		return stagedPublishFailed, fmt.Errorf("upload returned non-success response")
	}

	slog.Info("reconciler: staged archive published into repository authority",
		"path", archivePath,
		"publisher", ref.GetPublisherId(),
		"name", ref.GetName(),
		"version", ref.GetVersion(),
		"platform", ref.GetPlatform(),
		"build_id", resp.GetBuildId())
	return stagedPublishPublished, nil
}

func parseStagedPackageManifest(data []byte) (*packageManifest, error) {
	pkg := extractPackageManifest(data)
	if pkg == nil {
		return nil, fmt.Errorf("package.json missing or unreadable")
	}
	pkg.Name = strings.TrimSpace(pkg.Name)
	pkg.Version = strings.TrimSpace(pkg.Version)
	pkg.Platform = strings.TrimSpace(pkg.Platform)
	pkg.Publisher = strings.TrimSpace(pkg.Publisher)
	if pkg.Name == "" {
		return nil, fmt.Errorf("package.json missing name")
	}
	if pkg.Version == "" {
		return nil, fmt.Errorf("package.json missing version")
	}
	if pkg.Platform == "" {
		return nil, fmt.Errorf("package.json missing platform")
	}
	if pkg.Publisher == "" {
		return nil, fmt.Errorf("package.json missing publisher")
	}
	return pkg, nil
}

func stagedPackageKind(pkgType string) repopb.ArtifactKind {
	switch strings.ToLower(strings.TrimSpace(pkgType)) {
	case "application":
		return repopb.ArtifactKind_APPLICATION
	case "infrastructure":
		return repopb.ArtifactKind_INFRASTRUCTURE
	case "command":
		return repopb.ArtifactKind_COMMAND
	default:
		return repopb.ArtifactKind_SERVICE
	}
}

type internalUploadArtifactStream struct {
	ctx  context.Context
	reqs []*repopb.UploadArtifactRequest
	idx  int
	resp *repopb.UploadArtifactResponse
}

func (s *internalUploadArtifactStream) Recv() (*repopb.UploadArtifactRequest, error) {
	if s.idx >= len(s.reqs) {
		return nil, io.EOF
	}
	req := s.reqs[s.idx]
	s.idx++
	return proto.Clone(req).(*repopb.UploadArtifactRequest), nil
}

func (s *internalUploadArtifactStream) SendAndClose(resp *repopb.UploadArtifactResponse) error {
	s.resp = proto.Clone(resp).(*repopb.UploadArtifactResponse)
	return nil
}

func (s *internalUploadArtifactStream) Context() context.Context       { return s.ctx }
func (s *internalUploadArtifactStream) SendMsg(_ any) error            { return nil }
func (s *internalUploadArtifactStream) RecvMsg(_ any) error            { return nil }
func (s *internalUploadArtifactStream) SetHeader(_ metadata.MD) error  { return nil }
func (s *internalUploadArtifactStream) SendHeader(_ metadata.MD) error { return nil }
func (s *internalUploadArtifactStream) SetTrailer(_ metadata.MD)       {}

func (srv *server) uploadArtifactInternally(ctx context.Context, ref *repopb.ArtifactRef, data []byte) (*repopb.UploadArtifactResponse, error) {
	stream := &internalUploadArtifactStream{
		ctx: ctx,
		reqs: []*repopb.UploadArtifactRequest{{
			Ref:  proto.Clone(ref).(*repopb.ArtifactRef),
			Data: append([]byte(nil), data...),
		}},
	}
	if err := srv.UploadArtifact(stream); err != nil {
		return nil, err
	}
	if stream.resp == nil {
		return nil, fmt.Errorf("upload returned no response")
	}
	return stream.resp, nil
}

// ── Scheduled loop ────────────────────────────────────────────────────────────

// startReconcilerLoop runs the reconciler on startup and then periodically.
// This is a best-effort background task; errors are logged but never fatal.
func (srv *server) startReconcilerLoop(ctx context.Context) {
	// Initial startup run — rebuild any missing Scylla rows from local receipts
	// and verify all PUBLISHED rows have local blobs.
	go func() {
		// Wait a moment for Scylla to be ready after startup.
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		srv.reconcileScyllaFromStagedPackages(ctx, stagedPackageDirs)
		srv.reconcileScyllaFromLocalCAS(ctx)
		srv.reconcileLocalCASVsScylla(ctx)
		srv.sweepOrphanTempBlobs(ctx, time.Now())

		// Near-real-time staged-package projection keeps
		// /var/lib/globular/packages reflected into repository Layer 1 without
		// making Day-0 remember a one-shot publish ceremony. This still only
		// publishes repository authority; desired state and runtime restarts stay
		// owned by controller/workflow/node-agent.
		stagedTicker := time.NewTicker(stagedPackageProjectionInterval)
		defer stagedTicker.Stop()

		// Full integrity reconciliation remains lower-frequency because it can
		// scan all Scylla rows and local CAS receipts.
		fullTicker := time.NewTicker(time.Hour)
		defer fullTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stagedTicker.C:
				srv.reconcileScyllaFromStagedPackages(ctx, stagedPackageDirs)
			case <-fullTicker.C:
				srv.reconcileScyllaFromStagedPackages(ctx, stagedPackageDirs)
				srv.reconcileLocalCASVsScylla(ctx)
				srv.sweepOrphanTempBlobs(ctx, time.Now())
			}
		}
	}()
}
