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
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResolveArtifact implements the deterministic resolver contract.
func (srv *server) ResolveArtifact(ctx context.Context, req *repopb.ResolveArtifactRequest) (*repopb.ResolveArtifactResponse, error) {
	if err := srv.requireHealthy(); err != nil {
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

	// Load all manifest entries from storage.
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read artifact catalog: %v", err)
	}

	// Collect all candidates matching the request filters.
	var candidates []*repopb.ArtifactManifest
	targetVersion := strings.TrimSpace(req.GetVersion())

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

		// Only PUBLISHED artifacts are eligible.
		if state != repopb.PublishState_PUBLISHED {
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
			cv, cvErr := versionutil.Canonical(targetVersion)
			if cvErr != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid version %q: %v", targetVersion, cvErr)
			}
			refCV, refErr := versionutil.Canonical(ref.GetVersion())
			if refErr != nil {
				continue
			}
			if refCV != cv {
				continue
			}
		}

		candidates = append(candidates, m)
	}

	if len(candidates) == 0 {
		reason := fmt.Sprintf("no PUBLISHED artifact found: name=%q publisher=%q platform=%q channel=%s",
			name, publisher, platform, targetChannel.String())
		if targetVersion != "" {
			reason += fmt.Sprintf(" version=%q", targetVersion)
		}
		return nil, status.Error(codes.NotFound, reason)
	}

	// If version was specified and multiple builds exist for that version,
	// pick the highest build_number (most recent build of that version).
	if targetVersion != "" {
		best := pickHighestBuild(candidates)
		return &repopb.ResolveArtifactResponse{
			Manifest:         best,
			ResolutionSource: fmt.Sprintf("exact-version build_number=%d", best.GetBuildNumber()),
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
func (srv *server) resolveByBuildID(ctx context.Context, buildID, publisher, name, platform string, targetChannel repopb.ArtifactChannel) (*repopb.ResolveArtifactResponse, error) {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read artifact catalog: %v", err)
	}
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil || state != repopb.PublishState_PUBLISHED {
			continue
		}
		if m.GetBuildId() != buildID {
			continue
		}
		ref := m.GetRef()
		if publisher != "" && !strings.EqualFold(ref.GetPublisherId(), publisher) {
			return nil, status.Errorf(codes.FailedPrecondition,
				"build_id %s belongs to publisher %q, not %q", buildID, ref.GetPublisherId(), publisher)
		}
		if !strings.EqualFold(ref.GetName(), name) {
			return nil, status.Errorf(codes.FailedPrecondition,
				"build_id %s belongs to artifact %q, not %q", buildID, ref.GetName(), name)
		}
		if effectiveChannel(m) != targetChannel {
			return nil, status.Errorf(codes.FailedPrecondition,
				"build_id %s is on channel %s, not %s", buildID, effectiveChannel(m).String(), targetChannel.String())
		}
		return &repopb.ResolveArtifactResponse{
			Manifest:         m,
			ResolutionSource: "exact-build_id",
		}, nil
	}
	return nil, status.Errorf(codes.NotFound, "build_id %q not found or not PUBLISHED", buildID)
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
