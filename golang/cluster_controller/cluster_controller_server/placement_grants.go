package main

import (
	"context"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// placement_grants.go — D1b: the grant term of the single placement predicate.
//
// D1 placement model (docs/design/placement-explicit-node-assignment-contract.md):
//
//	authorized(node, pkg) = profile_authorized(node, pkg) OR explicit_grant(node_id, pkg)
//
// A grant is ADDITIVE only (GRANT, no DENY). The component catalog remains the
// sole authority for package identity and profile placement — placementAllows /
// isOrphanedInstall stay profile-only and pure; the grant term only ADDS a
// package to a specific node. The service identity of a grant is the ENCLOSING
// ServiceRelease's ServiceName; a grant carries no package identity of its own
// (no second catalog, no second placement law).
//
// D1b builds and proves this machinery with SYNTHETIC ServiceReleases in tests.
// The write path still HARD-REJECTS any NodeAssignment (validateServiceReleaseSpec),
// so in production the granted set is ALWAYS empty until D1d lifts the reject —
// this changes no live behavior yet, it is the resolver half of the union.

// grantsNode reports whether assignments contain an explicit-placement GRANT for
// nodeID. A bare per-node version override (Placement=="") is NOT a grant.
func grantsNode(assignments []*cluster_controllerpb.NodeAssignment, nodeID string) bool {
	for _, a := range assignments {
		if a != nil && a.NodeID == nodeID && a.Placement == cluster_controllerpb.NodeAssignmentPlacementGrant {
			return true
		}
	}
	return false
}

// hasGrantIn reports whether any ServiceRelease additively GRANTs pkg to nodeID.
// Pure over a snapshot of releases so the predicate is unit-testable without a
// live resource store.
func hasGrantIn(releases []*cluster_controllerpb.ServiceRelease, pkg, nodeID string) bool {
	pkg = canonicalServiceName(pkg)
	nodeID = strings.TrimSpace(nodeID)
	if pkg == "" || nodeID == "" {
		return false
	}
	for _, rel := range releases {
		if rel == nil || rel.Spec == nil {
			continue
		}
		if canonicalServiceName(rel.Spec.ServiceName) != pkg {
			continue
		}
		if grantsNode(rel.Spec.NodeAssignments, nodeID) {
			return true
		}
	}
	return false
}

// grantedServicesForNode returns the set of services explicitly GRANTed to
// nodeID across all ServiceReleases. This is the controller-computed per-node
// grant list delivered to the node-agent as a join input — the node-agent is an
// executor, not the cluster brain, and does not read ServiceReleases itself
// (node_agent.is_executor_not_cluster_brain). Returns an empty (non-nil) set
// when there is no store or nodeID.
func (srv *server) grantedServicesForNode(ctx context.Context, nodeID string) (map[string]bool, error) {
	nodeID = strings.TrimSpace(nodeID)
	out := map[string]bool{}
	if srv.resources == nil || nodeID == "" {
		return out, nil
	}
	items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		return nil, err
	}
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok || rel.Spec == nil {
			continue
		}
		if grantsNode(rel.Spec.NodeAssignments, nodeID) {
			out[canonicalServiceName(rel.Spec.ServiceName)] = true
		}
	}
	return out, nil
}

// authorizedForNode is the SINGLE placement predicate (one law book): a package
// is authorized on a node if the catalog authorizes it by profile OR an explicit
// grant additively authorizes it. `grants` is the node's granted-service set
// (grantedServicesForNode), snapshotted once per reconcile pass so this stays
// pure and out of hot-loop I/O. Unknown-to-catalog packages are authorized by
// the profile term (placementAllows returns true for an empty catalog profile
// set), matching the existing isOrphanedInstall semantics.
func authorizedForNode(pkg string, nodeProfiles []string, grants map[string]bool) bool {
	var catProfiles []string
	if cat := CatalogByName(pkg); cat != nil {
		catProfiles = cat.Profiles
	}
	if placementAllows(catProfiles, nodeProfiles) {
		return true
	}
	return grants[canonicalServiceName(pkg)]
}

// isOrphanedInstallForNode is the grant-aware form of isOrphanedInstall: a
// cataloged package installed on a node whose profiles do not authorize it is an
// orphan UNLESS an explicit grant additively authorizes it (contract §7 — a
// granted install is never an orphan). Unknown-to-catalog packages remain "not a
// profile orphan" (isOrphanedInstall returns false for them).
func isOrphanedInstallForNode(name string, nodeProfiles []string, grants map[string]bool) bool {
	return isOrphanedInstall(name, nodeProfiles) && !grants[canonicalServiceName(name)]
}
