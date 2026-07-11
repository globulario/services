package main

// D1b — the single grant-aware placement predicate: authorized = profile ∪ grant.
// Proven with synthetic ServiceReleases / grant sets (the write path still
// rejects any NodeAssignment, so tests construct grants directly, never via RPC).

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestHasGrantIn covers the grant term alone (no catalog): explicit-only,
// unknown-node, version-override-is-not-a-grant, no-assignments, no-release.
func TestHasGrantIn(t *testing.T) {
	const nodeA = "681710ee-6966-5df3-b155-3cef8b4e1a96"
	const nodeB = "aaaa1111-2222-3333-4444-555566667777"

	mk := func(svc string, a ...*cluster_controllerpb.NodeAssignment) *cluster_controllerpb.ServiceRelease {
		return &cluster_controllerpb.ServiceRelease{Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: svc, NodeAssignments: a}}
	}
	grant := func(nid string) *cluster_controllerpb.NodeAssignment {
		return &cluster_controllerpb.NodeAssignment{NodeID: nid, Placement: cluster_controllerpb.NodeAssignmentPlacementGrant}
	}
	verOverride := func(nid string) *cluster_controllerpb.NodeAssignment {
		return &cluster_controllerpb.NodeAssignment{NodeID: nid, Version: "1.2.272"} // Placement=="" — NOT a grant
	}

	releases := []*cluster_controllerpb.ServiceRelease{
		mk("torrent", grant(nodeA)),
		mk("title", verOverride(nodeA)),
		mk("media"), // no assignments
	}

	if !hasGrantIn(releases, "torrent", nodeA) {
		t.Errorf("explicit-only: torrent is granted to node-a")
	}
	if hasGrantIn(releases, "torrent", nodeB) {
		t.Errorf("unknown-node: torrent grant targets node-a only, not node-b")
	}
	if hasGrantIn(releases, "title", nodeA) {
		t.Errorf("a bare per-node version override is NOT a placement grant")
	}
	if hasGrantIn(releases, "media", nodeA) {
		t.Errorf("media has no assignments — no grant")
	}
	if hasGrantIn(releases, "nonexistent", nodeA) {
		t.Errorf("no release for the package — no grant")
	}
}

// TestPlacementPredicate_ProfileUnionGrant covers the union at the predicate
// level: profile-only, explicit-only, union, removal, unknown package. Uses
// isOrphanedInstall (catalog-authoritative) as ground truth and skips if the
// catalog no longer classifies the sample packages as expected.
func TestPlacementPredicate_ProfileUnionGrant(t *testing.T) {
	node := []string{"control-plane", "core", "storage"} // deliberately no media-server/compute

	// A cataloged package that IS a profile orphan on this node.
	orphanPkg := "torrent"
	if !isOrphanedInstall(orphanPkg, node) {
		t.Skipf("catalog changed: %q is no longer a profile orphan on %v", orphanPkg, node)
	}
	noGrants := map[string]bool{}
	granted := map[string]bool{canonicalServiceName(orphanPkg): true}

	// profile-only, no grant → NOT authorized, IS orphan.
	if authorizedForNode(orphanPkg, node, noGrants) {
		t.Errorf("%s must not be authorized without a grant (profile-unauthorized)", orphanPkg)
	}
	if !isOrphanedInstallForNode(orphanPkg, node, noGrants) {
		t.Errorf("%s must be an orphan without a grant", orphanPkg)
	}

	// explicit-only (grant) → authorized, NOT orphan.
	if !authorizedForNode(orphanPkg, node, granted) {
		t.Errorf("explicit-only: %s must be authorized when granted", orphanPkg)
	}
	if isOrphanedInstallForNode(orphanPkg, node, granted) {
		t.Errorf("a granted install is never an orphan")
	}

	// removal: drop the grant → orphan again.
	if !isOrphanedInstallForNode(orphanPkg, node, noGrants) {
		t.Errorf("removal: dropping the grant returns %s to orphan", orphanPkg)
	}

	// union: a profile-authorized package stays authorized regardless of grants.
	profilePkg := "dns"
	if isOrphanedInstall(profilePkg, node) {
		t.Skipf("catalog changed: %q is not profile-placed on %v", profilePkg, node)
	}
	if !authorizedForNode(profilePkg, node, noGrants) {
		t.Errorf("union: %s must be authorized by profile", profilePkg)
	}
	if isOrphanedInstallForNode(profilePkg, node, noGrants) {
		t.Errorf("%s is profile-placed, not an orphan", profilePkg)
	}

	// unknown package: not in catalog → not a profile orphan.
	if isOrphanedInstallForNode("totally-unknown-pkg-xyz", node, noGrants) {
		t.Errorf("unknown-to-catalog package must not be a profile orphan")
	}
}
