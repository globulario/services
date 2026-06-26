package main

// Proof for convergence.identity_is_build_id at the reference-node acceptance
// gate (Path C): the cluster summary hash is a coarse version summary, so the
// AUTHORITY check that decides "is this reference node converged?" must compare
// build_id. referenceNodeBuildIDMismatch is that authority check; a same-version
// but different-build_id node must be REFUSED.

import "testing"

// (the bite) Same version, different build_id ⇒ refused.
func TestReferenceNodeBuildID_SameVersionDifferentBuildRefused(t *testing.T) {
	filtered := map[string]string{"dns": "1.2.3"} // same version on both sides
	desired := map[string]string{"dns": "build-A"}
	installed := map[string]string{"dns": "build-B"}
	svc, want, got, bad := referenceNodeBuildIDMismatch(filtered, desired, installed)
	if !bad {
		t.Fatal("same version + different build_id must be refused (the cluster hash cannot see this)")
	}
	if svc != "dns" || want != "build-A" || got != "build-B" {
		t.Fatalf("mismatch detail wrong: svc=%q want=%q got=%q", svc, want, got)
	}
}

// Same build_id ⇒ accepted.
func TestReferenceNodeBuildID_SameBuildAccepted(t *testing.T) {
	filtered := map[string]string{"dns": "1.2.3"}
	desired := map[string]string{"dns": "build-A"}
	installed := map[string]string{"dns": "build-A"}
	if _, _, _, bad := referenceNodeBuildIDMismatch(filtered, desired, installed); bad {
		t.Fatal("identical build_id must be accepted")
	}
}

// A node missing the build_id for a build-backed desired service is NOT proven
// converged ⇒ refused (empty installed != desired build_id).
func TestReferenceNodeBuildID_MissingInstalledBuildRefused(t *testing.T) {
	filtered := map[string]string{"dns": "1.2.3"}
	desired := map[string]string{"dns": "build-A"}
	installed := map[string]string{} // node reports no build_id
	if _, _, _, bad := referenceNodeBuildIDMismatch(filtered, desired, installed); !bad {
		t.Fatal("a desired build-backed service with no installed build_id must be refused")
	}
}

// Upstream-native / dev services (no resolved desired build_id) are skipped —
// build identity cannot be enforced where none exists; version convergence (the
// coarse hash) governs those.
func TestReferenceNodeBuildID_NoDesiredBuildIDSkipped(t *testing.T) {
	filtered := map[string]string{"etcd": "3.5.14"}
	desired := map[string]string{}                   // upstream: no build_id
	installed := map[string]string{"etcd": "ignored"} // not compared
	if _, _, _, bad := referenceNodeBuildIDMismatch(filtered, desired, installed); bad {
		t.Fatal("a service with no desired build_id must be skipped, not refused")
	}
}

// Only services in the per-node desired set are checked — a build_id mismatch on
// a service NOT desired on this node does not refuse it.
func TestReferenceNodeBuildID_OnlyFilteredServicesChecked(t *testing.T) {
	filtered := map[string]string{"dns": "1.2.3"}                       // only dns desired here
	desired := map[string]string{"dns": "build-A", "rbac": "build-X"}
	installed := map[string]string{"dns": "build-A", "rbac": "build-Y"} // rbac mismatches but isn't filtered
	if _, _, _, bad := referenceNodeBuildIDMismatch(filtered, desired, installed); bad {
		t.Fatal("a mismatch outside the per-node desired set must not refuse the node")
	}
}
