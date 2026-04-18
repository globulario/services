package main

// gc.go — Repository garbage collection (soft-delete via ARCHIVED state).
//
// ArchiveUnreachableArtifacts is the single entry point for GC. It scans the
// full artifact catalog, computes the reachable set using the shared
// reachability engine, and moves every unreachable artifact to ARCHIVED state.
//
// What GC archives:
//   • PUBLISHED artifacts outside the retention window (last N builds per series)
//     AND not in the etcd installed-state (no active deployment).
//   • DEPRECATED artifacts outside the retention window with no active deployment.
//   • VERIFIED/FAILED/ORPHANED artifacts with no active deployment
//     (abandoned uploads that never made it to PUBLISHED).
//
// What GC never touches:
//   • Reachable artifacts (inside retention window OR actively deployed).
//   • YANKED, QUARANTINED, REVOKED, CORRUPTED — these are moderation/security
//     states managed by humans. GC should not interfere.
//   • ARCHIVED artifacts (already done).
//
// ARCHIVED semantics:
//   • Hidden from catalog queries (IsDiscoveryHidden=true).
//   • Downloads are NOT blocked — owners/admins can still retrieve the binary.
//   • Binary is retained in MinIO; a future purge step can hard-delete.
//   • One-way: the only allowed transition out is REVOKED (admin only).
//
// dry_run=true previews without writing any state changes.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// gcEligibleStates are the publish states that GC may archive.
// Terminal/moderation states are intentionally excluded.
var gcEligibleStates = map[repopb.PublishState]string{
	repopb.PublishState_PUBLISHED:  "published_outside_retention",
	repopb.PublishState_DEPRECATED: "deprecated_outside_retention",
	repopb.PublishState_VERIFIED:   "abandoned_upload",
	repopb.PublishState_FAILED:     "failed_upload",
	repopb.PublishState_ORPHANED:   "orphaned_upload",
}

// ArchiveUnreachableArtifacts implements the GC RPC.
func (srv *server) ArchiveUnreachableArtifacts(
	ctx context.Context,
	req *repopb.ArchiveUnreachableArtifactsRequest,
) (*repopb.ArchiveUnreachableArtifactsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}

	dryRun := req.GetDryRun()

	// Load the full catalog (all states — reachability engine needs everything).
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list artifacts: %v", err)
	}

	// Build (catalog, key→state) maps in one pass.
	type entry struct {
		key   string
		state repopb.PublishState
		m     *repopb.ArtifactManifest
	}
	var all []entry
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".manifest.json")
		_, st, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil {
			continue
		}
		all = append(all, entry{key: key, state: st, m: m})
	}

	// Build catalog slice for the reachability engine.
	catalog := make([]*repopb.ArtifactManifest, len(all))
	for i, e := range all {
		catalog[i] = e.m
	}

	// Compute reachable set (retention-window + installed-state explicit roots).
	explicit := collectInstalledBuildIDs(ctx)
	rs := ComputeReachable(catalog, explicit, srv.reachabilityConfig())

	// Classify and archive.
	resp := &repopb.ArchiveUnreachableArtifactsResponse{}

	for _, e := range all {
		reason, eligible := gcEligibleStates[e.state]
		if !eligible {
			// Not a GC-eligible state — skip.
			resp.SkippedCount++
			continue
		}

		if rs.Contains(e.m) {
			// Reachable — protected.
			resp.ProtectedCount++
			continue
		}

		// Unreachable + eligible → archive.
		ref := e.m.GetRef()
		record := &repopb.ArchivedArtifactRecord{
			Key:       e.key,
			BuildId:   e.m.GetBuildId(),
			Name:      ref.GetName(),
			Version:   ref.GetVersion(),
			Publisher: ref.GetPublisherId(),
			Reason:    reason,
		}

		if !dryRun {
			if archiveErr := srv.archiveOne(ctx, e.key, e.m); archiveErr != nil {
				slog.Warn("GC: failed to archive artifact",
					"key", e.key, "err", archiveErr)
				resp.SkippedCount++
				continue
			}
			slog.Info("GC: archived artifact",
				"key", e.key,
				"build_id", e.m.GetBuildId(),
				"publisher", ref.GetPublisherId(),
				"name", ref.GetName(),
				"version", ref.GetVersion(),
				"reason", reason,
			)
		}

		resp.Archived = append(resp.Archived, record)
		resp.ArchivedCount++
	}

	mode := "dry-run"
	if !dryRun {
		mode = "executed"
	}
	slog.Info("GC run complete",
		"mode", mode,
		"archived", resp.ArchivedCount,
		"protected", resp.ProtectedCount,
		"skipped", resp.SkippedCount,
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "repository.gc", map[string]any{
		"dry_run":         dryRun,
		"archived_count":  resp.ArchivedCount,
		"protected_count": resp.ProtectedCount,
		"skipped_count":   resp.SkippedCount,
	})

	return resp, nil
}

// archiveOne transitions a single artifact to ARCHIVED state.
func (srv *server) archiveOne(ctx context.Context, key string, m *repopb.ArtifactManifest) error {
	mjson, err := marshalManifestWithState(m, repopb.PublishState_ARCHIVED)
	if err != nil {
		return fmt.Errorf("marshal archived manifest: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return fmt.Errorf("write archived manifest: %w", err)
	}
	srv.syncStateToScylla(ctx, key, repopb.PublishState_ARCHIVED)
	if srv.cache != nil {
		srv.cache.invalidateManifest(manifestStorageKey(key))
	}
	return nil
}
