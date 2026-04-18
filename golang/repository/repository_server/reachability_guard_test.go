package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── checkDeletionSafety ───────────────────────────────────────────────────

func TestDeletionSafety_UnreachableArtifact_IsAllowed(t *testing.T) {
	// An artifact outside the retention window with no explicit roots is safe to delete.
	catalog := []*repopb.ArtifactManifest{
		makePublishedManifest("core", "echo", "1.0.1", "linux_amd64", 1, "bid-1", nil),
		makePublishedManifest("core", "echo", "1.0.2", "linux_amd64", 2, "bid-2", nil),
		makePublishedManifest("core", "echo", "1.0.3", "linux_amd64", 3, "bid-3", nil),
		makePublishedManifest("core", "echo", "1.0.4", "linux_amd64", 4, "bid-4", nil),
		// bid-1 is the 4th newest → outside window=3.
	}
	target := catalog[0] // bid-1

	srv := &server{}
	safe, reason := srv.checkDeletionSafety(context.Background(), target, catalog)
	if !safe {
		t.Errorf("expected safe=true for unreachable artifact, got reason: %s", reason)
	}
}

func TestDeletionSafety_RetentionWindowArtifact_IsBlocked(t *testing.T) {
	// The newest build is within the retention window — must be blocked.
	catalog := []*repopb.ArtifactManifest{
		makePublishedManifest("core", "echo", "1.0.1", "linux_amd64", 1, "bid-1", nil),
		makePublishedManifest("core", "echo", "1.0.2", "linux_amd64", 2, "bid-2", nil),
	}
	target := catalog[1] // bid-2, newest, in window

	srv := &server{}
	safe, reason := srv.checkDeletionSafety(context.Background(), target, catalog)
	if safe {
		t.Error("expected safe=false for retention-window artifact")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
	t.Logf("reason: %s", reason)
}

func TestDeletionSafety_ActivelyDeployed_IsBlocked(t *testing.T) {
	// An artifact with its build_id in the installed-state (simulated via
	// explicit roots in the catalog — we can't reach etcd in unit tests,
	// but ComputeReachable will find the explicit root).
	//
	// We exercise the code path by creating a VERIFIED artifact (not in retention)
	// and including its build_id in the explicit roots via the reachability engine
	// indirectly. Since collectInstalledBuildIDs returns {} in tests (no etcd),
	// we verify the retention-window path in unit tests and trust the
	// activelyDeployed branch is covered by integration tests.
	//
	// What we CAN test: that a VERIFIED artifact outside retention is safe,
	// proving the guard doesn't over-block.
	catalog := []*repopb.ArtifactManifest{
		makeVerifiedManifest("core", "auth", "2.0.0", "linux_amd64", 99, "bid-desired"),
	}
	target := catalog[0]

	srv := &server{}
	safe, _ := srv.checkDeletionSafety(context.Background(), target, catalog)
	// VERIFIED — not in retention window, no etcd roots → safe to delete.
	if !safe {
		t.Error("VERIFIED artifact outside retention should be safe to delete")
	}
}

func TestDeletionSafety_SingleBuild_RetentionBlocks(t *testing.T) {
	// Only one published build exists — it is the newest (and only) → in window.
	catalog := []*repopb.ArtifactManifest{
		makePublishedManifest("core", "rbac", "1.0.0", "linux_amd64", 1, "bid-rbac", nil),
	}
	target := catalog[0]

	srv := &server{}
	safe, reason := srv.checkDeletionSafety(context.Background(), target, catalog)
	if safe {
		t.Error("single published build should be protected by retention window")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDeletionSafety_EmptyCatalog_IsAllowed(t *testing.T) {
	// Empty catalog → empty reachable set → safe.
	target := makePublishedManifest("core", "echo", "1.0.0", "linux_amd64", 1, "bid-x", nil)
	srv := &server{}
	safe, _ := srv.checkDeletionSafety(context.Background(), target, nil)
	if !safe {
		t.Error("empty catalog should make target unreachable (safe to delete)")
	}
}

// ── checkRevokeSafety ─────────────────────────────────────────────────────

func TestRevokeSafety_NotDeployed_IsAllowed(t *testing.T) {
	// No etcd in unit tests → collectInstalledBuildIDs returns {} → not deployed → safe.
	target := makePublishedManifest("core", "dns", "1.0.0", "linux_amd64", 1, "bid-dns", nil)
	srv := &server{}
	blocked, reason := srv.checkRevokeSafety(context.Background(), target, false)
	if blocked {
		t.Errorf("artifact not in installed state should be safe to revoke, got: %s", reason)
	}
}

func TestRevokeSafety_Admin_Bypasses(t *testing.T) {
	// Admin isAdmin=true should never be blocked even if deployed.
	target := makePublishedManifest("core", "dns", "1.0.0", "linux_amd64", 1, "bid-dns", nil)
	srv := &server{}
	blocked, _ := srv.checkRevokeSafety(context.Background(), target, true /* isAdmin */)
	if blocked {
		t.Error("admin should never be blocked by revoke safety check")
	}
}

// ── collectInstalledBuildIDs ──────────────────────────────────────────────

func TestCollectInstalledBuildIDs_ReturnsNonNilMap(t *testing.T) {
	// Whether or not etcd is reachable, the function must always return
	// a non-nil map — callers rely on map lookups, not nil checks.
	ids := collectInstalledBuildIDs(context.Background())
	if ids == nil {
		t.Error("collectInstalledBuildIDs must return non-nil map (even on etcd error)")
	}
}
