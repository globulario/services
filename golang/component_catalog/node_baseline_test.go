package component_catalog

import (
	"testing"
)

// The NODE_BASELINE placement class: packages every admitted node needs in
// order to participate at all, independent of the work its profiles assign it.
//
// These tests protect BOTH directions. The failure this fixes was
// under-authorization (compute nodes permanently orphaned for envoy/gateway/xds
// even though the join installs them by design). The failure it must not
// introduce is over-authorization — "everything installed is authorized" —
// which would make placement.installed_package_orphaned unable to fire at all.

// TestNodeBaselineAuthorizedOnEveryProfile is the positive control: a node with
// any single profile, and a node with none, must be authorized for the baseline.
//
// Written over ProfileNames() rather than a hardcoded list precisely so that
// ADDING A NEW PROFILE CANNOT SILENTLY RECREATE THE BUG. A future gpu/edge/
// ai-worker profile inherits this assertion without anyone remembering to
// repeat the trio.
func TestNodeBaselineAuthorizedOnEveryProfile(t *testing.T) {
	for _, profile := range ProfileNames() {
		authorized := map[string]bool{}
		for _, p := range PackagesForProfiles([]string{profile}) {
			authorized[p] = true
		}
		for _, b := range NodeBaseline {
			if !authorized[b] {
				t.Errorf("profile %q does not authorize NODE_BASELINE package %q — "+
					"a node with this profile would report it as an orphaned install "+
					"even though the join installs it unconditionally", profile, b)
			}
		}
	}

	// A node with NO resolvable profile must still return empty. That empty
	// result is a load-bearing signal ("no profile matched") which Day-0 treats
	// as fatal; handing back the baseline would silently turn "this node has no
	// valid profiles" into "this node has three packages". The baseline belongs
	// to ADMITTED nodes, and an unresolvable node is not one.
	for _, bad := range [][]string{nil, {}, {"", " "}, {"nonexistent-profile"}} {
		if got := PackagesForProfiles(bad); len(got) != 0 {
			t.Errorf("PackagesForProfiles(%v) = %v, want empty: NODE_BASELINE must not "+
				"mask the no-profile-matched signal", bad, got)
		}
	}
}

// TestNodeBaselineIsNotAnAmnesty is the negative control that matters most.
// Widening authorization must not make orphan detection impossible: a package
// that is genuinely not authorized for a profile must still come back
// unauthorized.
func TestNodeBaselineIsNotAnAmnesty(t *testing.T) {
	// scylladb is a storage/control-plane concern and must NOT be authorized by
	// a bare compute profile.
	authorized := map[string]bool{}
	for _, p := range PackagesForProfiles([]string{"compute"}) {
		authorized[p] = true
	}
	if authorized["scylladb"] {
		t.Error("compute now authorizes scylladb — NODE_BASELINE has leaked into a " +
			"general amnesty and orphaned-install detection can no longer fire")
	}
	if authorized["cluster-controller"] {
		t.Error("compute now authorizes cluster-controller — the control plane must " +
			"not be placeable on a compute-only node")
	}

	// The baseline must stay SMALL and explicit. A growing baseline is the same
	// amnesty arriving slowly.
	if len(NodeBaseline) > 5 {
		t.Errorf("NodeBaseline has grown to %d packages (%v). It is the node substrate, "+
			"not a dumping ground for anything awkward to place; re-justify before "+
			"raising this bound", len(NodeBaseline), NodeBaseline)
	}
}

// TestProfilesForPackageStaysThePureInverse pins a boundary I initially crossed.
// It is tempting to make ProfilesForPackage report "every profile" for a
// baseline package, so the doctor never prints "catalog requires one of
// [control-plane gateway]" about something required everywhere. But that
// function is the declared inverse of ProfilePackages, other callers rely on
// that meaning, and the misleading message is unreachable anyway: the
// orphaned-install rule resolves through PackagesForProfiles, which now
// authorizes the baseline, so the finding never fires for these packages.
// Fix the authorization, not the inverse map.
func TestProfilesForPackageStaysThePureInverse(t *testing.T) {
	for _, b := range NodeBaseline {
		got := ProfilesForPackage(b)
		if len(got) == len(ProfileNames()) {
			t.Errorf("ProfilesForPackage(%q) now returns every profile; it must remain "+
				"the pure inverse of ProfilePackages", b)
		}
		if len(got) == 0 {
			t.Errorf("ProfilesForPackage(%q) returned nothing; baseline packages are "+
				"still catalog-tracked", b)
		}
	}
}

// TestNonBaselinePackageStillReportsItsProfiles guards the inverse of the
// message change: only baseline packages get the "every profile" answer.
func TestNonBaselinePackageStillReportsItsProfiles(t *testing.T) {
	got := ProfilesForPackage("scylladb")
	if len(got) == 0 {
		t.Fatal("scylladb resolved to no profiles; it is catalog-tracked and should " +
			"report the profiles that place it")
	}
	if len(got) == len(ProfileNames()) {
		t.Errorf("scylladb reports every profile (%v) — the NODE_BASELINE short-circuit "+
			"is matching non-baseline packages", got)
	}
}
