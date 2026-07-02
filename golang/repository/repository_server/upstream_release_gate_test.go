package main

// upstream_release_gate_test.go — required tests for the in-cluster release
// authority gate on the upstream-sync ingestion path (P2).
//
// These prove the channel CI stamps in release-index.json is NOT trusted: a
// STABLE import survives only with proven RBAC release authority, and FEDERATION
// alone never grants STABLE (package.forge_binding_is_not_authorization).

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

//  1. Only STABLE (the convergeable channel) is subject to the gate; a DEV claim
//     is never touched, regardless of federation/authorization state.
func TestUpstreamRelease_NonStableClaimPassesThrough(t *testing.T) {
	for _, ch := range []string{"dev", "candidate", "canary"} {
		if final, downgraded := upstreamReleaseDecision(ch, true, false, false); downgraded || final != ch {
			t.Fatalf("channel %q must pass through unchanged; got (%q, downgraded=%v)", ch, final, downgraded)
		}
	}
}

//  2. Safe rollout: an unmanaged namespace (no registered trusted publishers) is
//     never downgraded, even for a STABLE claim with no authority.
func TestUpstreamRelease_UnmanagedNamespaceUntouched(t *testing.T) {
	if final, downgraded := upstreamReleaseDecision("stable", false, false, false); downgraded || final != "stable" {
		t.Fatalf("unmanaged namespace must be untouched; got (%q, downgraded=%v)", final, downgraded)
	}
	// Empty channel normalizes to STABLE and must behave identically.
	if final, downgraded := upstreamReleaseDecision("", false, false, false); downgraded {
		t.Fatalf("unmanaged empty/STABLE claim must be untouched; got (%q, downgraded=%v)", final, downgraded)
	}
}

//  3. THE core contract: in a managed namespace, a federated-but-NOT-authorized
//     publisher is downgraded to DEV. Federation (a trusted-publisher binding) is
//     NOT authorization — package.forge_binding_is_not_authorization.
func TestUpstreamRelease_FederationWithoutAuthorizationIsDowngraded(t *testing.T) {
	final, downgraded := upstreamReleaseDecision("stable", true /*managed*/, true /*federated*/, false /*authorized*/)
	if !downgraded || final != "dev" {
		t.Fatalf("federated-but-unauthorized STABLE must downgrade to DEV; got (%q, downgraded=%v)", final, downgraded)
	}
}

// 4. A managed namespace with neither federation nor authorization downgrades.
func TestUpstreamRelease_UnfederatedStableIsDowngraded(t *testing.T) {
	final, downgraded := upstreamReleaseDecision("stable", true, false, false)
	if !downgraded || final != "dev" {
		t.Fatalf("unfederated STABLE must downgrade to DEV; got (%q, downgraded=%v)", final, downgraded)
	}
	// Empty channel (normalizes to STABLE) must downgrade too.
	if final, downgraded := upstreamReleaseDecision("", true, false, false); !downgraded || final != "dev" {
		t.Fatalf("empty/STABLE claim must downgrade in a managed ns; got (%q, downgraded=%v)", final, downgraded)
	}
}

//  5. Both steps satisfied → STABLE survives. This is the only path to a
//     convergeable upstream import.
func TestUpstreamRelease_FederatedAndAuthorizedKeepsStable(t *testing.T) {
	if final, downgraded := upstreamReleaseDecision("stable", true, true, true); downgraded || final != "stable" {
		t.Fatalf("federated+authorized STABLE must survive; got (%q, downgraded=%v)", final, downgraded)
	}
}

// 6. upstreamForgeSubject precedence: owner → repo → name.
func TestUpstreamForgeSubject_Precedence(t *testing.T) {
	cases := []struct {
		owner, repo, name string
		want              string
	}{
		{"globulario", "services", "gh-src", "globulario"},
		{"", "services", "gh-src", "services"},
		{"", "", "gh-src", "gh-src"},
		{"  ", "  ", "gh-src", "gh-src"},
	}
	for _, c := range cases {
		src := &repopb.UpstreamSource{Owner: c.owner, Repo: c.repo, Name: c.name}
		if got := upstreamForgeSubject(src); got != c.want {
			t.Fatalf("forge subject for (owner=%q repo=%q name=%q) = %q, want %q",
				c.owner, c.repo, c.name, got, c.want)
		}
	}
}
