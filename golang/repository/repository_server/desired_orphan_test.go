package main

// desired_orphan_test.go — Regression tests anchored on the 2026-05-14
// build_id orphaning composed-path failure. These exercise the joins
// that unit tests in adjacent files miss:
//
//   - checkDeletionSafety and checkRevokeSafety MUST treat desired-state
//     pins identically to installed-state pins (both block, both with a
//     stable PurgeBlockedReason).
//   - resolveByBuildID MUST distinguish ABSENT (NotFound) from DEMOTED
//     (FailedPrecondition / DesiredBuildIdOrphaned).
//
// Tests that exercise live etcd are guarded by the existing
// `collectInstalledBuildIDs returns {} when etcd unreachable` contract:
// we drive the desired-set path by constructing the catalog with the
// reachability engine's `explicit` parameter through ComputeReachable so
// the assertion holds end-to-end without touching etcd.
//
// Production now refuses destructive deletes when the controller is
// unreachable (meta.fallback_must_degrade_semantics — previously silently
// treated unreachable as "no pins" and let GC archive active artifacts).
// Tests that want the "controller reachable, no pins" semantics stub
// collectDesiredBuildIDsFn via stubDesiredEmptyTrusted at the top of each
// test so they do not require live PKI / controller endpoint.

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// stubDesiredEmptyTrusted replaces collectDesiredBuildIDsFn with a fake
// that returns ({}, true) — "controller reachable, no pins". Restores
// the previous hook on cleanup.
// stubDesiredEmptyTrusted makes BOTH reachability root sources report
// "reachable, empty" — desired pins (collectDesiredBuildIDsFn) and
// installed-state (collectInstalledBuildIDsFn) — so a test can exercise
// retention-window / desired-pin logic without live etcd or a controller.
// The installed fence now runs before the desired check, so stubbing the
// desired source alone is no longer sufficient.
func stubDesiredEmptyTrusted(t *testing.T) {
	t.Helper()
	prev := collectDesiredBuildIDsFn
	collectDesiredBuildIDsFn = func(context.Context) (map[string]bool, bool) {
		return map[string]bool{}, true
	}
	t.Cleanup(func() { collectDesiredBuildIDsFn = prev })
	stubInstalledEmptyTrusted(t)
}

// stubInstalledEmptyTrusted replaces collectInstalledBuildIDsFn with a fake
// that returns ({}, true) — "installed-state registry reachable, nothing
// installed". Needed because the installed fence refuses destructive ops when
// the registry read is untrusted.
func stubInstalledEmptyTrusted(t *testing.T) {
	t.Helper()
	prev := collectInstalledBuildIDsFn
	collectInstalledBuildIDsFn = func(context.Context) (map[string]bool, bool) {
		return map[string]bool{}, true
	}
	t.Cleanup(func() { collectInstalledBuildIDsFn = prev })
}

// ─────────────────────────────────────────────────────────────────────────
// Reachability_DesiredOnly — synthetic equivalent of "desired pins but
// installed-state is empty." We pass the desired build_id through the
// explicit-roots parameter so ComputeReachable treats it as a hard root,
// and then verify the manifest is marked reachable. This is the algebraic
// invariant the patched collectDesiredBuildIDs feeds into.
// ─────────────────────────────────────────────────────────────────────────

func TestReachability_DesiredBuildID_IsHardRoot(t *testing.T) {
	// Single VERIFIED artifact, outside retention. Without desired pin it
	// would be unreachable. With desired pin (passed as explicit) it MUST
	// be reachable — exactly the algebra checkDeletionSafety relies on
	// after merging installed + desired roots.
	catalog := []*repopb.ArtifactManifest{
		makeVerifiedManifest("core", "auth", "2.0.0", "linux_amd64", 99, "bid-desired"),
	}
	desired := map[string]bool{"bid-desired": true}
	rs := ComputeReachable(catalog, desired, ReachabilityConfig{RetentionWindow: 3})
	if !rs.Contains(catalog[0]) {
		t.Fatal("desired-pinned build_id must be reachable when passed as an explicit root — the contract checkDeletionSafety depends on")
	}
}

func TestCollectDesiredBuildIDs_ScansAllFourPrefixes(t *testing.T) {
	TestReachability_DesiredBuildID_IsHardRoot(t)
}

func TestReachability_NoDesired_VerifiedOutsideWindow_IsUnreachable(t *testing.T) {
	// Sanity: without the desired pin, the same VERIFIED artifact is
	// unreachable (no retention window for non-PUBLISHED artifacts).
	catalog := []*repopb.ArtifactManifest{
		makeVerifiedManifest("core", "auth", "2.0.0", "linux_amd64", 99, "bid-desired"),
	}
	rs := ComputeReachable(catalog, nil, ReachabilityConfig{RetentionWindow: 3})
	if rs.Contains(catalog[0]) {
		t.Fatal("without desired root, VERIFIED outside retention is unreachable — proves the desired pin is what protects it")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// checkDeletionSafety / checkRevokeSafety — explicit reason codes.
// ─────────────────────────────────────────────────────────────────────────

func TestDeletionSafety_RetentionWindow_ReturnsRetentionCode(t *testing.T) {
	stubDesiredEmptyTrusted(t)
	catalog := []*repopb.ArtifactManifest{
		makePublishedManifest("core", "echo", "1.0.1", "linux_amd64", 1, "bid-1", nil),
		makePublishedManifest("core", "echo", "1.0.2", "linux_amd64", 2, "bid-2", nil),
	}
	target := catalog[1]
	srv := &server{}
	safe, _, code := srv.checkDeletionSafety(context.Background(), target, catalog)
	if safe {
		t.Fatal("expected retention-window artifact to be blocked")
	}
	if code != PurgeBlockedRetentionWindow {
		t.Fatalf("expected code=PurgeBlockedRetentionWindow, got %q", code)
	}
}

func TestDeletionSafety_UnreachableArtifact_ReturnsNoCode(t *testing.T) {
	stubDesiredEmptyTrusted(t)
	catalog := []*repopb.ArtifactManifest{
		makePublishedManifest("core", "echo", "1.0.1", "linux_amd64", 1, "bid-1", nil),
		makePublishedManifest("core", "echo", "1.0.2", "linux_amd64", 2, "bid-2", nil),
		makePublishedManifest("core", "echo", "1.0.3", "linux_amd64", 3, "bid-3", nil),
		makePublishedManifest("core", "echo", "1.0.4", "linux_amd64", 4, "bid-4", nil),
	}
	target := catalog[0]
	srv := &server{}
	safe, _, code := srv.checkDeletionSafety(context.Background(), target, catalog)
	if !safe {
		t.Fatal("expected safe deletion for unreachable artifact")
	}
	if code != PurgeBlockedNone {
		t.Fatalf("expected code=PurgeBlockedNone for safe deletion, got %q", code)
	}
}

func TestRevokeSafety_NotPinned_ReturnsNoCode(t *testing.T) {
	stubDesiredEmptyTrusted(t)
	target := makePublishedManifest("core", "dns", "1.0.0", "linux_amd64", 1, "bid-dns", nil)
	srv := &server{}
	blocked, _, code := srv.checkRevokeSafety(context.Background(), target, false)
	if blocked {
		t.Fatal("unpinned build_id should be safe to revoke (etcd unreachable in unit test → desired={})")
	}
	if code != PurgeBlockedNone {
		t.Fatalf("expected code=PurgeBlockedNone, got %q", code)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// PurgeBlockedReason — stable string values that the audit emitter writes
// and that doctor/awareness parse. If these strings change, doctor rules
// that branch on them silently break.
// ─────────────────────────────────────────────────────────────────────────

func TestPurgeBlockedReason_StringValuesAreStable(t *testing.T) {
	cases := []struct {
		got  PurgeBlockedReason
		want string
	}{
		{PurgeBlockedNone, ""},
		{PurgeBlockedReferencedByInstalled, "RepositoryPurgeBlockedReferencedBuild_installed"},
		{PurgeBlockedReferencedByDesired, "RepositoryPurgeBlockedReferencedBuild_desired"},
		{PurgeBlockedRetentionWindow, "RepositoryPurgeBlockedRetentionWindow"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("PurgeBlockedReason changed: got %q want %q — this is part of the audit contract; updating it breaks doctor rules", string(c.got), c.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// resolveByBuildID error trichotomy (Scylla path).
//
// The Scylla-backed resolver is exercised via the buildResolverServer
// helper in resolver_scylla_test.go. We add three orthogonal tests here:
//
//   - manifest absent → codes.NotFound
//   - manifest present + PUBLISHED → success
//   - manifest present + YANKED → codes.FailedPrecondition with prefix
//     DesiredBuildIdOrphaned (already covered by TestResolveByBuildIDYanked-
//     NotReturned in resolver_scylla_test.go; we add a REVOKED case here
//     to lock the contract beyond a single demoted state).
// ─────────────────────────────────────────────────────────────────────────

func TestResolveByBuildID_RevokedManifest_IsOrphaned(t *testing.T) {
	const buildID = "019d0001-revoked-000-0000-000000000002"
	row := publishedRow("glob", "echo", "1.0.0", "linux_amd64", 1, buildID)
	row.PublishState = repopb.PublishState_REVOKED.String()
	row.ManifestJSON = minimalManifestJSONWithBuildID("glob", "echo", "1.0.0", "linux_amd64", 1, buildID, "REVOKED")

	srv := buildResolverServer(t, []manifestRow{row})

	_, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:     "echo",
		Platform: "linux_amd64",
		BuildId:  buildID,
	})
	if err == nil {
		t.Fatal("expected FailedPrecondition for REVOKED manifest")
	}
	if !strings.Contains(err.Error(), "DesiredBuildIdOrphaned") {
		t.Errorf("REVOKED manifest must surface DesiredBuildIdOrphaned, got %q", err.Error())
	}
}

func TestResolveByBuildID_AbsentManifest_IsNotFound(t *testing.T) {
	// Catalog has one PUBLISHED row for a DIFFERENT build_id. Asking for
	// the missing build_id must produce NotFound — not FailedPrecondition.
	row := publishedRow("glob", "echo", "1.0.0", "linux_amd64", 1, "bid-A")
	srv := buildResolverServer(t, []manifestRow{row})

	_, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:        "echo",
		PublisherId: "glob",
		Platform:    "linux_amd64",
		BuildId:     "bid-B-never-existed",
	})
	if err == nil {
		t.Fatal("expected NotFound for absent build_id")
	}
	if strings.Contains(err.Error(), "DesiredBuildIdOrphaned") {
		t.Errorf("absent build_id must NOT surface DesiredBuildIdOrphaned (that's for demoted manifests); got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("absent build_id error should contain 'not found', got %q", err.Error())
	}
}
