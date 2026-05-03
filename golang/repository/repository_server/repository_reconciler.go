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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
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
		srv.reconcileScyllaFromLocalCAS(ctx)
		srv.reconcileLocalCASVsScylla(ctx)

		// Periodic reconciliation every hour.
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.reconcileLocalCASVsScylla(ctx)
			}
		}
	}()
}
