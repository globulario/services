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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	clientv3 "go.etcd.io/etcd/client/v3"
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

// collectDesiredBuildIDs returns build_ids pinned by desired-state resources.
// Best-effort: callers combine this with installed-state roots so destructive
// repository operations never archive/delete/revoke a build that the controller
// still intends to roll out.
//
// All four desired-state etcd prefixes are scanned:
//
//   - /globular/resources/ServiceDesiredVersion/   (workload services)
//   - /globular/resources/InfrastructureRelease/   (infrastructure releases)
//   - /globular/resources/DesiredService/          (legacy / per-service intent)
//   - /globular/resources/ServiceRelease/          (release tracking — resolved build_id)
//
// Missing any of these prefixes was the original bug: a build_id pinned by
// ServiceRelease.Status.ResolvedBuildID could be archived even though the
// controller still attempts to install it, producing the
// "build_id not found for name=…" cascade observed in production.
func collectDesiredBuildIDs(ctx context.Context) map[string]bool {
	ids := map[string]bool{}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return ids
	}

	// Common shape: every desired-state record may pin a build_id in either
	// Spec.BuildID or Status.ResolvedBuildID. We parse both and accept either.
	type genericSpec struct {
		BuildID string `json:"build_id"`
	}
	type genericStatus struct {
		ResolvedBuildID string `json:"resolved_build_id"`
		BuildID         string `json:"build_id"`
	}
	type genericRec struct {
		Spec   *genericSpec   `json:"spec"`
		Status *genericStatus `json:"status"`
	}

	collect := func(prefix string) {
		resp, getErr := cli.Get(ctx, prefix,
			clientv3.WithPrefix(), clientv3.WithLimit(500))
		if getErr != nil {
			return
		}
		for _, kv := range resp.Kvs {
			var rec genericRec
			if json.Unmarshal(kv.Value, &rec) != nil {
				continue
			}
			if rec.Status != nil {
				if rec.Status.ResolvedBuildID != "" {
					ids[rec.Status.ResolvedBuildID] = true
				}
				if rec.Status.BuildID != "" {
					ids[rec.Status.BuildID] = true
				}
			}
			if rec.Spec != nil && rec.Spec.BuildID != "" {
				ids[rec.Spec.BuildID] = true
			}
		}
	}

	collect("/globular/resources/ServiceDesiredVersion/")
	collect("/globular/resources/InfrastructureRelease/")
	collect("/globular/resources/DesiredService/")
	collect("/globular/resources/ServiceRelease/")

	return ids
}

func mergeBuildIDRoots(sets ...map[string]bool) map[string]bool {
	merged := map[string]bool{}
	for _, set := range sets {
		for id := range set {
			if id != "" {
				merged[id] = true
			}
		}
	}
	return merged
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

// PurgeBlockedReason identifies why a destructive repository operation was
// blocked. Each value maps 1:1 to a structured error/finding so doctor and
// awareness can reason about the failure without parsing strings.
type PurgeBlockedReason string

const (
	PurgeBlockedNone                  PurgeBlockedReason = ""
	PurgeBlockedReferencedByInstalled PurgeBlockedReason = "RepositoryPurgeBlockedReferencedBuild_installed"
	PurgeBlockedReferencedByDesired   PurgeBlockedReason = "RepositoryPurgeBlockedReferencedBuild_desired"
	PurgeBlockedRetentionWindow       PurgeBlockedReason = "RepositoryPurgeBlockedRetentionWindow"
)

// checkDeletionSafety reports whether the given manifest may be safely removed.
//
// Returns (safe=true, "", "") when deletion is allowed.
// Returns (safe=false, reason, code) when the operation must be rejected.
// `code` is a stable PurgeBlockedReason so callers (doctor/awareness/CLI)
// can branch without parsing the human reason.
//
// A build is treated as referenced when:
//
//   - its build_id is in the **installed** registry on any node (Layer 3), OR
//   - its build_id is in the **desired** state (Layer 2) — pinned by an active
//     ServiceDesiredVersion / InfrastructureRelease, even if not yet installed
//     anywhere, OR
//   - it is in the retention window (rollback safety).
//
// Desired references must block deletion: removing a desired-pinned build is
// what produced the "build_id not found for name=…" cascade — the controller
// keeps trying to install something the repository forgot.
//
// The caller decides what to do with force=true (typically bypass).
func (srv *server) checkDeletionSafety(
	ctx context.Context,
	target *repopb.ArtifactManifest,
	catalog []*repopb.ArtifactManifest,
) (safe bool, reason string, code PurgeBlockedReason) {
	rCfg := srv.reachabilityConfig()
	installed := collectInstalledBuildIDs(ctx)
	desired := collectDesiredBuildIDs(ctx)
	explicit := mergeBuildIDRoots(installed, desired)
	rs := ComputeReachable(catalog, explicit, rCfg)

	if !rs.Contains(target) {
		return true, "", PurgeBlockedNone
	}

	ref := target.GetRef()
	buildID := target.GetBuildId()

	// Determine WHY the artifact is reachable — gives a better error message
	// AND a stable structured code so doctor / CLI can react.
	if installed[buildID] {
		return false, fmt.Sprintf(
			"%s/%s build_id=%s is currently installed on one or more cluster nodes "+
				"— uninstall from all nodes before deleting from the repository",
			ref.GetPublisherId(), ref.GetName(), buildID,
		), PurgeBlockedReferencedByInstalled
	}

	if desired[buildID] {
		return false, fmt.Sprintf(
			"%s/%s build_id=%s is pinned by active desired state "+
				"(ServiceDesiredVersion or InfrastructureRelease). "+
				"Deleting it would orphan the build_id and break installs. "+
				"Roll desired state forward first, or use force=true (will trigger orphan finding).",
			ref.GetPublisherId(), ref.GetName(), buildID,
		), PurgeBlockedReferencedByDesired
	}

	// Reachable only via the retention window (not actively deployed).
	return false, fmt.Sprintf(
		"%s/%s@%s (build %d) is within the retention window "+
			"(the last %d published builds per series are protected for rollback). "+
			"Publish a newer version to push it out, or use force=true to override.",
		ref.GetPublisherId(), ref.GetName(), ref.GetVersion(),
		target.GetBuildNumber(), rCfg.RetentionWindow,
	), PurgeBlockedRetentionWindow
}

// ── revoke guard ──────────────────────────────────────────────────────────

// checkRevokeSafety reports whether a REVOKE lifecycle transition is safe.
//
// Returns (blocked=true, reason, code) when the caller should be warned/blocked.
// Returns (false, "", "") when the revoke is safe to proceed.
//
// Revoking a retention-window-only artifact is always allowed — revoke is a
// security action that intentionally overrides availability guarantees.
// Revoking an actively-deployed OR desired-pinned artifact is blocked because:
//   - It would block downloads for nodes that need to re-install or verify.
//   - It would orphan the desired build_id and trigger install-storms / repair
//     loops on every node still waiting to converge to that build (this is
//     exactly the cascade we are fixing).
//
// Admin callers (subject="sa") bypass this check for security incident response.
func (srv *server) checkRevokeSafety(
	ctx context.Context,
	target *repopb.ArtifactManifest,
	isAdmin bool,
) (blocked bool, reason string, code PurgeBlockedReason) {
	if isAdmin {
		return false, "", PurgeBlockedNone
	}
	buildID := target.GetBuildId()
	installed := collectInstalledBuildIDs(ctx)
	if installed[buildID] {
		return true, fmt.Sprintf(
			"%s/%s build_id=%s is currently installed on one or more cluster nodes. "+
				"Revoking it blocks downloads and may cause cluster repair loops. "+
				"Uninstall from all nodes first. Administrators may revoke immediately.",
			target.GetRef().GetPublisherId(), target.GetRef().GetName(), buildID,
		), PurgeBlockedReferencedByInstalled
	}
	desired := collectDesiredBuildIDs(ctx)
	if desired[buildID] {
		return true, fmt.Sprintf(
			"%s/%s build_id=%s is pinned by active desired state. "+
				"Revoking it would orphan the build_id and break installs cluster-wide. "+
				"Roll desired state forward first. Administrators may revoke immediately.",
			target.GetRef().GetPublisherId(), target.GetRef().GetName(), buildID,
		), PurgeBlockedReferencedByDesired
	}
	return false, "", PurgeBlockedNone
}
