package main

// resolver.go — Deterministic artifact resolver (PR 1).
//
// ResolveArtifact resolves a package reference to exactly one concrete build_id
// or returns a hard error. This is the canonical entry point for the controller
// planning phase. The reconciler MUST NOT call this at execution time — it reads
// build_id from desired state.
//
// Invariant E: resolver output is deterministic — exactly one build or a hard error.
//   - If zero artifacts match: NOT_FOUND with a descriptive reason.
//   - If multiple artifacts match after all filters: FAILED_PRECONDITION (ambiguous).
//   - Only PUBLISHED artifacts are eligible.
//   - DEV and CANDIDATE are never returned unless explicitly requested.
//   - "latest" resolution is valid here, but callers MUST persist build_id into
//     desired state immediately after this call.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResolveArtifact implements the deterministic resolver contract.
func (srv *server) ResolveArtifact(ctx context.Context, req *repopb.ResolveArtifactRequest) (*repopb.ResolveArtifactResponse, error) {
	if err := srv.requireCapability(CapRepoQuery); err != nil {
		return nil, err
	}

	publisher := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	platform := strings.TrimSpace(req.GetPlatform())
	if platform == "" {
		platform = "linux_amd64"
	}

	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// Determine target channel. Default to STABLE (reconciler-safe).
	targetChannel := req.GetChannel()
	if targetChannel == repopb.ArtifactChannel_CHANNEL_UNSET {
		targetChannel = repopb.ArtifactChannel_STABLE
	}

	// Short-circuit: if build_id is given, resolve directly.
	if buildID := strings.TrimSpace(req.GetBuildId()); buildID != "" {
		return srv.resolveByBuildID(ctx, buildID, publisher, name, platform, targetChannel)
	}

	targetVersion := strings.TrimSpace(req.GetVersion())

	// Scylla-first: use ledger as authoritative source. Direct call — no cache.
	// The resolver is an install-path entry point; stale lifecycle state (e.g. a
	// YANKED artifact still appearing PUBLISHED) must never reach the reconciler.
	// Falls back to the MinIO directory scan only when Scylla is nil.
	if srv.scylla != nil {
		rows, scyllaErr := srv.scylla.ListManifests(ctx)
		if scyllaErr != nil {
			return nil, status.Errorf(codes.Unavailable, "artifact ledger unavailable: %v", scyllaErr)
		}
		var candidates []*repopb.ArtifactManifest
		for _, row := range rows {
			// Column-level filters — fast, no JSON parsing.
			// isRowInstallable enforces BOTH publish_state==PUBLISHED AND
			// artifact_state ∈ {PUBLISHED, ""}. Non-empty intermediate or
			// broken pipeline states exclude the row from resolution.
			if !isRowInstallable(&row) {
				continue
			}
			if publisher != "" && !strings.EqualFold(row.PublisherID, publisher) {
				continue
			}
			if !strings.EqualFold(row.Name, name) {
				continue
			}
			if !strings.EqualFold(row.Platform, platform) {
				continue
			}
			// Channel — replicate effectiveChannel logic using stored column.
			rowChannel := repopb.ArtifactChannel_STABLE
			if v, ok := repopb.ArtifactChannel_value[row.Channel]; ok &&
				repopb.ArtifactChannel(v) != repopb.ArtifactChannel_CHANNEL_UNSET {
				rowChannel = repopb.ArtifactChannel(v)
			}
			if rowChannel != targetChannel {
				continue
			}
			// Kind filter using stored column.
			if req.GetKind() != repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
				rowKind := repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED
				if v, ok := repopb.ArtifactKind_value[row.Kind]; ok {
					rowKind = repopb.ArtifactKind(v)
				}
				if rowKind != req.GetKind() {
					continue
				}
			}
			// Version filter using stored column — avoids JSON parse for non-matches.
			if targetVersion != "" {
				cv, cvErr := versionutil.NormalizeExact(targetVersion)
				if cvErr != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid version %q: %v", targetVersion, cvErr)
				}
				refCV, refErr := versionutil.NormalizeExact(row.Version)
				if refErr != nil || refCV != cv {
					continue
				}
			}
			m, _, parseErr := manifestFromRow(row)
			if parseErr != nil {
				slog.Warn("resolver: skipping unreadable ledger row", "key", row.ArtifactKey, "err", parseErr)
				continue
			}
			candidates = append(candidates, m)
		}
		return srv.pickResolution(candidates, name, publisher, platform, targetVersion, targetChannel)
	}

	// Legacy / single-node fallback: scan MinIO directory.
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read artifact catalog: %v", err)
	}

	// Collect all candidates matching the request filters.
	var candidates []*repopb.ArtifactManifest

	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil {
			continue
		}

		// Only PUBLISHED artifacts with a coherent pipeline state are eligible.
		if !srv.isInstallableForRef(ctx, m.GetRef(), m.GetBuildNumber(), state) {
			continue
		}

		ref := m.GetRef()

		// Match publisher (if specified).
		if publisher != "" && !strings.EqualFold(ref.GetPublisherId(), publisher) {
			continue
		}
		// Match name.
		if !strings.EqualFold(ref.GetName(), name) {
			continue
		}
		// Match platform (if specified).
		if platform != "" && !strings.EqualFold(ref.GetPlatform(), platform) {
			continue
		}
		// Match kind (if specified).
		if req.GetKind() != repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED && ref.GetKind() != req.GetKind() {
			continue
		}
		// Channel must match target.
		if effectiveChannel(m) != targetChannel {
			continue
		}
		// Version filter (if specified).
		if targetVersion != "" {
			cv, cvErr := versionutil.NormalizeExact(targetVersion)
			if cvErr != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid version %q: %v", targetVersion, cvErr)
			}
			refCV, refErr := versionutil.NormalizeExact(ref.GetVersion())
			if refErr != nil {
				continue
			}
			if refCV != cv {
				continue
			}
		}

		candidates = append(candidates, m)
	}

	return srv.pickResolution(candidates, name, publisher, platform, targetVersion, targetChannel)
}

// pickResolution applies the final selection logic to a candidate set that has
// already been filtered for PUBLISHED state, publisher, name, platform, channel,
// and version. Both the Scylla and MinIO paths converge here.
func (srv *server) pickResolution(
	candidates []*repopb.ArtifactManifest,
	name, publisher, platform, targetVersion string,
	targetChannel repopb.ArtifactChannel,
) (*repopb.ResolveArtifactResponse, error) {
	if len(candidates) == 0 {
		reason := fmt.Sprintf("no PUBLISHED artifact found: name=%q publisher=%q platform=%q channel=%s",
			name, publisher, platform, targetChannel.String())
		if targetVersion != "" {
			reason += fmt.Sprintf(" version=%q", targetVersion)
		}
		return nil, status.Error(codes.NotFound, reason)
	}

	// If version was specified, resolution must be deterministic and explicit.
	// More than one build for the same version is ambiguous by design; callers
	// must pin build_id (or an alias that resolves to build_id) before reconcile.
	if targetVersion != "" {
		if len(candidates) > 1 {
			top := pickHighestBuild(candidates)
			return nil, status.Errorf(codes.FailedPrecondition,
				"ambiguous resolution: multiple builds at %s/%s@%s (top build_number=%d) — specify build_id explicitly",
				name, platform, targetVersion, top.GetBuildNumber())
		}
		best := candidates[0]
		return &repopb.ResolveArtifactResponse{
			Manifest:         best,
			ResolutionSource: "exact-version",
		}, nil
	}

	// No version specified: resolve to the latest PUBLISHED version.
	// Sort candidates by semver desc, build_number desc.
	sortManifestsByVersionDesc(candidates)
	best := candidates[0]

	// Ambiguity check: if there are multiple artifacts at the same version and
	// build_number with different build_ids, that is a hard error.
	if len(candidates) > 1 {
		second := candidates[1]
		if sameVersionBuild(best, second) {
			return nil, status.Errorf(codes.FailedPrecondition,
				"ambiguous resolution: multiple builds at %s/%s@%s build_number=%d — specify build_id explicitly",
				name, platform, best.GetRef().GetVersion(), best.GetBuildNumber())
		}
	}

	return &repopb.ResolveArtifactResponse{
		Manifest:         best,
		ResolutionSource: fmt.Sprintf("latest-stable version=%s build_number=%d", best.GetRef().GetVersion(), best.GetBuildNumber()),
	}, nil
}

// resolveByBuildID finds an artifact by its exact build_id. Validates publisher/name/channel.
// Scylla-first: when Scylla is available it scans the ledger directly — one round-trip
// instead of N MinIO GetObject calls. Falls back to MinIO only when Scylla is nil.
//
// Error semantics — the caller (node-agent, controller, doctor, awareness) MUST
// be able to tell these three outcomes apart, because each has a different fix:
//
//  1. SUCCESS                     — manifest returned.
//  2. codes.FailedPrecondition    — manifest EXISTS but the row is not installable
//                                   (ARCHIVED/YANKED/REVOKED/QUARANTINED/CORRUPTED
//                                   /broken pipeline state). The build is "orphaned"
//                                   in the sense that desired-state still points at
//                                   it but the repository has demoted it. Error
//                                   carries the stable prefix `DesiredBuildIdOrphaned`
//                                   and includes the live publish_state /
//                                   artifact_state so doctor can classify the cause.
//                                   Conflating this with NotFound is what produced
//                                   the production "build_id not found" install storm.
//  3. codes.NotFound              — manifest does not exist for this build_id.
//                                   Legitimate missing artifact / never-published /
//                                   purged-without-being-pinned.
//
// The node-agent's fallback path consumes these codes: FailedPrecondition means
// "do NOT silently install a local pinned tarball — the repository has explicitly
// demoted this build", and the install must surface as a structured release
// blocker, not a quiet local install.
func (srv *server) resolveByBuildID(ctx context.Context, buildID, publisher, name, platform string, targetChannel repopb.ArtifactChannel) (*repopb.ResolveArtifactResponse, error) {
	if srv.scylla != nil {
		rows, scyllaErr := srv.scylla.ListManifests(ctx)
		if scyllaErr != nil {
			return nil, status.Errorf(codes.Unavailable, "artifact ledger unavailable: %v", scyllaErr)
		}
		var orphanRow *manifestRow
		for i, row := range rows {
			m, _, parseErr := manifestFromRow(row)
			if parseErr != nil {
				continue
			}
			if m.GetBuildId() != buildID {
				continue
			}
			if !matchesResolveIdentity(m, publisher, name, platform, targetChannel) {
				continue
			}
			if !isRowInstallable(&row) {
				// Manifest exists for this identity but the row is not installable.
				// Remember the first such row so we can raise a precise orphan error
				// after we've finished scanning (in case a later row *is* installable
				// for the same identity).
				if orphanRow == nil {
					orphanRow = &rows[i]
				}
				continue
			}
			return &repopb.ResolveArtifactResponse{
				Manifest:         m,
				ResolutionSource: "exact-build_id",
			}, nil
		}
		if orphanRow != nil {
			return nil, status.Errorf(codes.FailedPrecondition,
				"DesiredBuildIdOrphaned: build_id %q for name=%q publisher=%q platform=%q channel=%s exists in repository but is not installable (publish_state=%s, artifact_state=%s) — the build was demoted while desired state still pins it; roll desired state forward or run repair-index",
				buildID, name, publisher, platform, targetChannel.String(),
				strings.TrimSpace(orphanRow.PublishState), strings.TrimSpace(orphanRow.ArtifactState))
		}
		return nil, status.Errorf(codes.NotFound,
			"build_id %q not found for name=%q publisher=%q platform=%q channel=%s",
			buildID, name, publisher, platform, targetChannel.String())
	}

	// Legacy / single-node fallback: scan MinIO directory.
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read artifact catalog: %v", err)
	}
	var orphanState repopb.PublishState
	orphanFound := false
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil {
			continue
		}
		if m.GetBuildId() != buildID {
			continue
		}
		if !matchesResolveIdentity(m, publisher, name, platform, targetChannel) {
			continue
		}
		// Identity matches. Is it installable?
		if state == repopb.PublishState_PUBLISHED && srv.isInstallableForRef(ctx, m.GetRef(), m.GetBuildNumber(), state) {
			return &repopb.ResolveArtifactResponse{
				Manifest:         m,
				ResolutionSource: "exact-build_id",
			}, nil
		}
		// Manifest exists for the identity but is demoted.
		if !orphanFound {
			orphanState = state
			orphanFound = true
		}
	}
	if orphanFound {
		return nil, status.Errorf(codes.FailedPrecondition,
			"DesiredBuildIdOrphaned: build_id %q for name=%q publisher=%q platform=%q channel=%s exists in repository but is not installable (publish_state=%s) — the build was demoted while desired state still pins it; roll desired state forward or run repair-index",
			buildID, name, publisher, platform, targetChannel.String(), orphanState.String())
	}
	return nil, status.Errorf(codes.NotFound,
		"build_id %q not found for name=%q publisher=%q platform=%q channel=%s",
		buildID, name, publisher, platform, targetChannel.String())
}

func matchesResolveIdentity(m *repopb.ArtifactManifest, publisher, name, platform string, targetChannel repopb.ArtifactChannel) bool {
	ref := m.GetRef()
	if ref == nil {
		return false
	}
	if publisher != "" && !strings.EqualFold(ref.GetPublisherId(), publisher) {
		return false
	}
	if name != "" && !strings.EqualFold(ref.GetName(), name) {
		return false
	}
	if platform != "" && !strings.EqualFold(ref.GetPlatform(), platform) {
		return false
	}
	return effectiveChannel(m) == targetChannel
}

// pickHighestBuild returns the manifest with the highest build_number among candidates.
func pickHighestBuild(candidates []*repopb.ArtifactManifest) *repopb.ArtifactManifest {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.GetBuildNumber() > best.GetBuildNumber() {
			best = c
		}
	}
	return best
}

// sameVersionBuild returns true if two manifests have identical version and build_number.
func sameVersionBuild(a, b *repopb.ArtifactManifest) bool {
	return a.GetRef().GetVersion() == b.GetRef().GetVersion() &&
		a.GetBuildNumber() == b.GetBuildNumber()
}
