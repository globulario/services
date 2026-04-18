package main

// reachability_guard.go — Safety enforcement for destructive repository operations.
//
// All operations that remove or disable an artifact (DeleteArtifact, REVOKE,
// future archive/GC) MUST route through the appropriate guard here. This is the
// single enforcement point that wires the shared reachability engine to incoming
// RPCs.
//
// Safety semantics
// ────────────────
//
//   DeleteArtifact:
//     Blocked if the specific build is reachable — either:
//       a) It is within the retention window (one of the N newest PUBLISHED
//          builds for its publisher/name/platform series). These are kept for
//          rollback / re-download availability.
//       b) Its build_id appears in the etcd installed-state registry on any
//          cluster node. Deleting it would leave nodes unable to re-download
//          or verify the installed binary.
//     force=true bypasses both conditions.
//
//   SetArtifactState → REVOKED:
//     Blocked if the build_id is actively installed on any node. Revoking an
//     active artifact blocks downloads and may cause cluster repair loops.
//     Retention-window-only artifacts may be revoked freely — revoke is a
//     security / quality action that intentionally overrides availability.
//     Admin callers (subject="sa") can override the check for incident response.
//
// The guards are intentionally independent of DeleteArtifact/SetArtifactState
// business logic so the same checks can be reused by future GC / archive paths.

import (
	"context"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/installed_state"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── etcd helpers ──────────────────────────────────────────────────────────

// collectInstalledBuildIDs queries the etcd installed-state registry and returns
// the set of all build_ids currently installed on any cluster node.
// Best-effort: returns an empty (non-nil) map on error so callers degrade
// gracefully (safety checks become retention-window-only rather than failing).
func collectInstalledBuildIDs(ctx context.Context) map[string]bool {
	pkgs, err := installed_state.ListAllNodes(ctx, "", "")
	if err != nil {
		return map[string]bool{}
	}
	ids := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		if id := p.GetBuildId(); id != "" {
			ids[id] = true
		}
	}
	return ids
}

// loadAllManifests returns every manifest in the repository regardless of
// publish state. The reachability engine needs the full catalog — not just
// PUBLISHED — so that explicit roots (which may be VERIFIED) resolve correctly.
func (srv *server) loadAllManifests(ctx context.Context) []*repopb.ArtifactManifest {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil
	}
	var out []*repopb.ArtifactManifest
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".manifest.json")
		_, _, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ── delete guard ──────────────────────────────────────────────────────────

// checkDeletionSafety reports whether the given manifest may be safely removed.
//
// Returns (safe=true, "") when deletion is allowed.
// Returns (safe=false, reason) when the operation must be rejected or warned.
//
// The caller decides what to do with force=true (typically bypass).
func (srv *server) checkDeletionSafety(
	ctx context.Context,
	target *repopb.ArtifactManifest,
	catalog []*repopb.ArtifactManifest,
) (safe bool, reason string) {
	rCfg := srv.reachabilityConfig()
	explicit := collectInstalledBuildIDs(ctx)
	rs := ComputeReachable(catalog, explicit, rCfg)

	if !rs.Contains(target) {
		return true, ""
	}

	ref := target.GetRef()

	// Determine WHY the artifact is reachable — gives a better error message.
	if explicit[target.GetBuildId()] {
		return false, fmt.Sprintf(
			"%s/%s build_id=%s is currently installed on one or more cluster nodes "+
				"— uninstall from all nodes before deleting from the repository",
			ref.GetPublisherId(), ref.GetName(), target.GetBuildId(),
		)
	}

	// Reachable only via the retention window (not actively deployed).
	return false, fmt.Sprintf(
		"%s/%s@%s (build %d) is within the retention window "+
			"(the last %d published builds per series are protected for rollback). "+
			"Publish a newer version to push it out, or use force=true to override.",
		ref.GetPublisherId(), ref.GetName(), ref.GetVersion(),
		target.GetBuildNumber(), rCfg.RetentionWindow,
	)
}

// ── revoke guard ──────────────────────────────────────────────────────────

// checkRevokeSafety reports whether a REVOKE lifecycle transition is safe.
//
// Returns (blocked=true, reason) when the caller should be warned/blocked.
// Returns (false, "") when the revoke is safe to proceed.
//
// Revoking a retention-window-only artifact is always allowed — revoke is a
// security action that intentionally overrides availability guarantees.
// Revoking an actively-deployed artifact is blocked because it would:
//   - Block downloads for nodes that need to re-install or verify the binary.
//   - Potentially trigger infinite repair loops on affected nodes.
//
// Admin callers (subject="sa") bypass this check for security incident response.
func (srv *server) checkRevokeSafety(
	ctx context.Context,
	target *repopb.ArtifactManifest,
	isAdmin bool,
) (blocked bool, reason string) {
	if isAdmin {
		return false, "" // admin override for incident response
	}
	explicit := collectInstalledBuildIDs(ctx)
	if !explicit[target.GetBuildId()] {
		return false, "" // not actively deployed — revoke is fine
	}
	return true, fmt.Sprintf(
		"%s/%s build_id=%s is currently installed on one or more cluster nodes. "+
			"Revoking it blocks downloads and may cause cluster repair loops. "+
			"Uninstall from all nodes first. Administrators may revoke immediately.",
		target.GetRef().GetPublisherId(), target.GetRef().GetName(), target.GetBuildId(),
	)
}
