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
// (set of all build_ids installed on any cluster node, trusted). trusted is
// false only when the registry read failed — the empty map then means
// "unknown", NOT "nothing installed".
//
// Destructive callers MUST refuse to proceed when trusted=false. This is the
// asymmetric twin of collectDesiredBuildIDs's fence: treating a transient read
// failure as "not installed" lets an actively-installed artifact that is past
// its retention window be archived/deleted/revoked — the "build_id not found
// for name=" cascade the desired-state fence already prevents.
// (meta.absence_scope_must_be_explicit)
//
// Indirected through collectInstalledBuildIDsFn so tests can inject a trusted
// or untrusted installed set without live etcd — mirroring collectDesiredBuildIDs.
var collectInstalledBuildIDsFn = collectInstalledBuildIDsLive

func collectInstalledBuildIDs(ctx context.Context) (map[string]bool, bool) {
	return collectInstalledBuildIDsFn(ctx)
}

func collectInstalledBuildIDsLive(ctx context.Context) (map[string]bool, bool) {
	pkgs, err := installed_state.ListAllNodes(ctx, "", "")
	if err != nil {
		return map[string]bool{}, false
	}
	ids := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		if id := p.GetBuildId(); id != "" {
			ids[id] = true
		}
	}
	return ids, true
}

// collectDesiredBuildIDsFn is the package-level hook for test injection.
// Production wires it to collectDesiredBuildIDsLive (the controller-RPC
// implementation below). Tests can replace it to simulate "controller
// reachable but no pins" without needing live PKI material.
var collectDesiredBuildIDsFn = collectDesiredBuildIDsLive

// collectDesiredBuildIDs is the call site used by the reachability guard
// and the GC reconciler. Delegates to the injected hook so tests can stub
// the controller RPC.
func collectDesiredBuildIDs(ctx context.Context) (map[string]bool, bool) {
	return collectDesiredBuildIDsFn(ctx)
}

// collectDesiredBuildIDsLive returns (build_ids pinned by desired-state
// resources, trusted). The trusted bool is true only when we observed the
// controller's authoritative answer; false means we could not reach the
// controller and the empty map is "unknown", not "no pins". Callers MUST
// refuse destructive operations (archive/delete/revoke) when trusted=false
// — an absorbed TLS error or unreachable controller had previously cascaded
// into deleting active artifacts (meta.fallback_must_degrade_semantics).
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
func collectDesiredBuildIDsLive(ctx context.Context) (map[string]bool, bool) {
	ids := map[string]bool{}

	addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
	if addr == "" {
		slog.Warn("reachability_guard: controller endpoint unresolved — desired-state UNKNOWN, refusing destructive ops")
		return ids, false
	}
	target := config.ResolveDialTarget(addr)

	creds, err := repositoryClientTLSCreds(target.ServerName)
	if err != nil {
		slog.Warn("reachability_guard: TLS creds load failed — desired-state UNKNOWN, refusing destructive ops",
			"addr", target.Address, "err", err)
		return ids, false
	}

	conn, err := grpc.NewClient(target.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		slog.Warn("reachability_guard: dial controller failed — desired-state UNKNOWN, refusing destructive ops",
			"addr", target.Address, "err", err)
		return ids, false
	}
	defer func() { _ = conn.Close() }()

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	resp, err := client.ListDesiredBuildIDs(callCtx, &cluster_controllerpb.ListDesiredBuildIDsRequest{})
	if err != nil {
		slog.Warn("reachability_guard: ListDesiredBuildIDs failed — desired-state UNKNOWN, refusing destructive ops",
			"addr", target.Address, "err", err)
		return ids, false
	}

	for _, id := range resp.GetBuildIds() {
		if id != "" {
			ids[id] = true
		}
	}
	return ids, true
}

// repositoryClientTLSCreds returns mTLS credentials for a repository →
// cluster_controller dial. Mirrors the pattern used by cluster_doctor's
// dialOptionsForInternalService: load the cluster CA, present the
// repository's own issued service certificate, and pin ServerName so
// SAN verification matches.
//
// Returns a non-nil error when the CA bundle or client cert/key cannot be
// loaded — this MUST propagate to the caller so it can refuse to dial
// rather than proceeding with a partial TLS config that produces a
// confusing handshake error far from the root cause. Previously these
// errors were silently absorbed, which cascaded into an empty
// "desired build_ids" set and let destructive GC decisions run under a
// hidden TLS failure (meta.connection_errors_must_not_be_absorbed +
// meta.fallback_must_degrade_semantics).
func repositoryClientTLSCreds(serverName string) (credentials.TransportCredentials, error) {
	if serverName == "" || serverName == "localhost" || serverName == "::1" {
		if h, err := os.Hostname(); err == nil && h != "" {
			serverName = h
		}
	}
	tlsCfg := &tls.Config{ServerName: serverName}

	caFile := config.GetTLSFile("", "", "ca.crt")
	if caFile == "" {
		return nil, fmt.Errorf("repositoryClientTLSCreds: cluster CA path not configured")
	}
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("repositoryClientTLSCreds: read CA %q: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("repositoryClientTLSCreds: CA bundle at %q contains no usable PEM blocks", caFile)
	}
	tlsCfg.RootCAs = pool

	// The controller requires the caller to present a cluster-issued
	// certificate for the desired-state read action. Missing identity is a
	// hard configuration error, not a degrade.
	const (
		clientCert = "/var/lib/globular/pki/issued/services/service.crt"
		clientKey  = "/var/lib/globular/pki/issued/services/service.key"
	)
	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("repositoryClientTLSCreds: load client keypair (%s, %s): %w", clientCert, clientKey, err)
	}
	tlsCfg.Certificates = []tls.Certificate{cert}
	return credentials.NewTLS(tlsCfg), nil
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
	installed, instTrusted := collectInstalledBuildIDs(ctx)
	if !instTrusted {
		// Cannot verify whether the build is installed anywhere — refuse rather
		// than risk deleting an actively-installed, past-retention artifact.
		return false, "installed-state unverifiable — registry read failed; retry when healthy", PurgeBlockedReferencedByInstalled
	}
	desired, trusted := collectDesiredBuildIDs(ctx)
	if !trusted {
		// Cannot verify whether the artifact is pinned by desired state —
		// refuse rather than proceed under partial knowledge. Enforces
		// repository.purge_must_not_delete_active_desired_builds when the
		// controller is unreachable.
		return false, "desired-state pin unverifiable — controller unreachable; retry when controller is healthy", PurgeBlockedReferencedByDesired
	}
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
	installed, instTrusted := collectInstalledBuildIDs(ctx)
	if !instTrusted {
		return true, fmt.Sprintf(
			"%s/%s build_id=%s installed status unverifiable — registry read failed. "+
				"Refusing to revoke under partial knowledge. Retry when healthy. "+
				"Administrators may revoke immediately.",
			target.GetRef().GetPublisherId(), target.GetRef().GetName(), buildID,
		), PurgeBlockedReferencedByInstalled
	}
	if installed[buildID] {
		return true, fmt.Sprintf(
			"%s/%s build_id=%s is currently installed on one or more cluster nodes. "+
				"Revoking it blocks downloads and may cause cluster repair loops. "+
				"Uninstall from all nodes first. Administrators may revoke immediately.",
			target.GetRef().GetPublisherId(), target.GetRef().GetName(), buildID,
		), PurgeBlockedReferencedByInstalled
	}
	desired, trusted := collectDesiredBuildIDs(ctx)
	if !trusted {
		return true, fmt.Sprintf(
			"%s/%s build_id=%s pinning status unverifiable — controller unreachable. "+
				"Refusing to revoke under partial knowledge. Retry when controller is healthy. "+
				"Administrators may revoke immediately.",
			target.GetRef().GetPublisherId(), target.GetRef().GetName(), buildID,
		), PurgeBlockedReferencedByDesired
	}
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
