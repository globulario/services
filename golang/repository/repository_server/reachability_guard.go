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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
// Authority boundary: the cluster_controller OWNS the
// /globular/resources/* prefix family. This function calls the
// controller's typed ListDesiredBuildIDs RPC (the single canonical
// answer to "which build_ids must I keep around?") instead of scanning
// etcd directly. Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	invariant:repository.desired_build_id_is_hard_reachability_root
//	invariant:repository.purge_must_not_delete_active_desired_builds
//	forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc
//
// Best-effort: returns an empty (non-nil) map on any error so the
// reachability guard degrades to "retention window + installed-state
// only" rather than failing destructive RPCs that may be safe. This
// matches the prior etcd-scan behaviour and the deletion-safety
// contract documented in checkDeletionSafety.
func collectDesiredBuildIDs(ctx context.Context) map[string]bool {
	ids := map[string]bool{}

	// Resolve controller endpoint from the service registry. An empty
	// address means we degrade — same as the prior etcd-Get failure
	// path.
	addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
	if addr == "" {
		slog.Debug("reachability_guard: controller endpoint unresolved — desired-state set empty")
		return ids
	}
	target := config.ResolveDialTarget(addr)

	conn, err := grpc.NewClient(target.Address, grpc.WithTransportCredentials(repositoryClientTLSCreds(target.ServerName)))
	if err != nil {
		slog.Warn("reachability_guard: dial controller failed — desired-state set empty",
			"addr", target.Address, "err", err)
		return ids
	}
	defer func() { _ = conn.Close() }()

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	resp, err := client.ListDesiredBuildIDs(callCtx, &cluster_controllerpb.ListDesiredBuildIDsRequest{})
	if err != nil {
		slog.Warn("reachability_guard: ListDesiredBuildIDs failed — desired-state set empty",
			"addr", target.Address, "err", err)
		return ids
	}

	for _, id := range resp.GetBuildIds() {
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

// repositoryClientTLSCreds returns mTLS credentials for a repository →
// cluster_controller dial. Mirrors the pattern used by cluster_doctor's
// dialOptionsForInternalService: load the cluster CA, present the
// repository's own issued service certificate, and pin ServerName so
// SAN verification matches.
func repositoryClientTLSCreds(serverName string) credentials.TransportCredentials {
	if serverName == "" || serverName == "localhost" || serverName == "::1" {
		if h, err := os.Hostname(); err == nil && h != "" {
			serverName = h
		}
	}
	tlsCfg := &tls.Config{ServerName: serverName}
	if caFile := config.GetTLSFile("", "", "ca.crt"); caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				tlsCfg.RootCAs = pool
			}
		}
	}
	// Best-effort mTLS — the controller requires the caller to present
	// a cluster-issued certificate for the desired-state read action.
	const (
		clientCert = "/var/lib/globular/pki/issued/services/service.crt"
		clientKey  = "/var/lib/globular/pki/issued/services/service.key"
	)
	if cert, err := tls.LoadX509KeyPair(clientCert, clientKey); err == nil {
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsCfg)
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
