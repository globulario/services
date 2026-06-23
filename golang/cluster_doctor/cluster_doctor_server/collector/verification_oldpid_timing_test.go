package collector

import "testing"

// These tests pin the ApplyTime-vs-old_pid-boundary split in resolveInstallTimings
// (the collector-level coverage that was missing when 1dc77898 reintroduced the
// false old_pid_after_upgrade: the pure verifier tests only exercised a pre-set
// ApplyTime, never the collector's own timestamp resolution).

// Test 1 (resolution half): a no-op metadata reconcile bumps UpdatedUnix without
// a restart. ApplyTime must follow the RECENT timestamp (so the grace window
// still fires), but the old_pid boundary must stay pinned to the STABLE
// InstalledUnix so a healthy, un-restarted PID is not falsely aged.
func TestResolveInstallTimings_NoOpUpdatedBump_BoundaryStaysInstalled(t *testing.T) {
	const t1 = int64(1700000000)
	const t2 = t1 + 72 // UpdatedUnix bumped 72s later by a no-op reconcile (no restart)

	info, ok := resolveInstallTimings(t1, t2)
	if !ok {
		t.Fatal("resolveInstallTimings returned ok=false for valid timestamps")
	}
	if got := info.applyTime.Unix(); got != t2 {
		t.Errorf("applyTime = %d, want %d (max — recency drives the grace window)", got, t2)
	}
	if got := info.oldPidBoundary.Unix(); got != t1 {
		t.Errorf("oldPidBoundary = %d, want %d (stable InstalledUnix — must NOT follow the no-op UpdatedUnix bump)", got, t1)
	}
	// delta = 72s > 60s, so this is NOT classified as a same-operation first install.
	if info.isFirstInstall {
		t.Errorf("isFirstInstall = true, want false (72s gap exceeds the 60s same-operation window)")
	}
}

// Test 2: when InstalledUnix is absent (0), the old_pid boundary falls back to
// UpdatedUnix — the explicit fallback clause, so a record that only carries an
// update time is still usable rather than disabling the check.
func TestResolveInstallTimings_InstalledZero_BoundaryFallsBackToUpdated(t *testing.T) {
	const t2 = int64(1700000500)

	info, ok := resolveInstallTimings(0, t2)
	if !ok {
		t.Fatal("resolveInstallTimings returned ok=false")
	}
	if got := info.oldPidBoundary.Unix(); got != t2 {
		t.Errorf("oldPidBoundary = %d, want %d (fallback to UpdatedUnix when InstalledUnix==0)", got, t2)
	}
	if got := info.applyTime.Unix(); got != t2 {
		t.Errorf("applyTime = %d, want %d", got, t2)
	}
}

// Guard: neither timestamp set → ok=false so the caller falls through to the
// next kind or the release-level fallback rather than fabricating a zero apply.
func TestResolveInstallTimings_BothZero_NotOK(t *testing.T) {
	if _, ok := resolveInstallTimings(0, 0); ok {
		t.Error("resolveInstallTimings(0,0) returned ok=true; want false")
	}
}
