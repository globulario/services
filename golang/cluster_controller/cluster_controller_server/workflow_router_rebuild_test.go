package main

import "testing"

// TestRebuildReleaseRouterFromInputs pins the restart-safe rebuild contract:
// a release callback that self-describes its generation rebuilds a per-run
// router; one that cannot recover its generation is refused (fail-safe, so the
// caller never writes through a guard-disabled router); a non-release callback
// passes through to the default router.
func TestRebuildReleaseRouterFromInputs(t *testing.T) {
	srv := &server{}

	t.Run("release with generation → rebuilt", func(t *testing.T) {
		r, isRelease := srv.rebuildReleaseRouterFromInputs(
			"ServiceRelease/core@globular.io/echo",
			`{"release_name":"core@globular.io/echo","package_kind":"SERVICE","dispatch_generation":3}`)
		if !isRelease {
			t.Fatal("expected isReleaseCallback=true")
		}
		if r == nil {
			t.Fatal("expected a rebuilt router when release_name + dispatch_generation present")
		}
	})

	t.Run("release without generation → refuse (nil,true)", func(t *testing.T) {
		r, isRelease := srv.rebuildReleaseRouterFromInputs(
			"ServiceRelease/core@globular.io/echo",
			`{"release_name":"core@globular.io/echo","package_kind":"SERVICE"}`)
		if !isRelease {
			t.Fatal("expected isReleaseCallback=true for a release callback")
		}
		if r != nil {
			t.Fatal("expected NO router (refuse) when dispatch_generation is unrecoverable — must not write guard-less")
		}
	})

	t.Run("not a release callback → passthrough (nil,false)", func(t *testing.T) {
		r, isRelease := srv.rebuildReleaseRouterFromInputs("some-run", `{"package_kind":"SERVICE"}`)
		if isRelease {
			t.Fatal("expected isReleaseCallback=false when no release_name")
		}
		if r != nil {
			t.Fatal("expected nil router for a non-release callback")
		}
	})

	t.Run("empty/invalid inputs → passthrough (nil,false)", func(t *testing.T) {
		if r, isRelease := srv.rebuildReleaseRouterFromInputs("x", ""); r != nil || isRelease {
			t.Fatal("empty inputs must be (nil,false)")
		}
		if r, isRelease := srv.rebuildReleaseRouterFromInputs("x", "not json"); r != nil || isRelease {
			t.Fatal("invalid inputs must be (nil,false)")
		}
	})
}
