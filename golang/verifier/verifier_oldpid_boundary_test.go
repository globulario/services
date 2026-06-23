package verifier

import (
	"testing"
	"time"
)

// These tests cover the ApplyTime-vs-OldPidBoundary split at the verifier level
// (the integration the collector tests can't see). Helpers hashA, applyTimeT1,
// and makeProofStartedAt are shared from the other verifier tests.

// Test 1 (outcome half): the xds false-positive class. A no-op metadata reconcile
// bumped UpdatedUnix, so the collector set ApplyTime=T2 (recent, max) while
// OldPidBoundary stays at T1 (stable InstalledUnix). The PID is healthy
// (running==installed) and started ~T1. old_pid_after_upgrade must NOT fire:
// before the split it compared against ApplyTime (=T2) and a process at T1 looked
// "older than the apply"; now it compares against the stable boundary (T1).
func TestVerifyTarget_NoOpUpdatedBump_NoFalseOldPid(t *testing.T) {
	t1 := applyTimeT1
	t2 := t1.Add(72 * time.Second) // UpdatedUnix bump, no restart
	processStart := t1.Add(1 * time.Second)

	tgt := Target{
		Service:                   "xds",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.234",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 t2, // max(installed,updated) — recent, drives grace
		ApplyTimeSource:           "installed_package.updated_unix",
		OldPidBoundary:            t1, // stable InstalledUnix — drives old_pid
		IsFirstInstall:            false,
	}
	ev := Evidence{Proof: makeProofStartedAt("xds", hashA, processStart)}
	now := t2.Add(10 * time.Minute) // well past the restart-pending window

	v := VerifyTarget(tgt, ev, now)
	for _, f := range v.Findings {
		if f.ID == FindingOldPidAfterUpgrade || f.ID == FindingRestartPending {
			t.Errorf("unexpected %q (severity=%s): a no-op UpdatedUnix bump must not age a healthy PID (running==installed, started at the install boundary)", f.ID, f.Severity)
		}
	}
}

// Sanity counter-test: the split must NOT blind old_pid to a genuinely stale PID.
// When the process started BEFORE the stable boundary itself, the finding must
// still fire (running==installed, so it's the timing tell, escalating past the
// restart-pending window).
func TestVerifyTarget_ProcessOlderThanBoundary_StillFlagged(t *testing.T) {
	t1 := applyTimeT1
	processStart := t1.Add(-10 * time.Minute) // older than the boundary

	tgt := Target{
		Service:                   "xds",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.234",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 t1,
		ApplyTimeSource:           "installed_package.installed_unix",
		OldPidBoundary:            t1,
		IsFirstInstall:            false,
	}
	ev := Evidence{Proof: makeProofStartedAt("xds", hashA, processStart)}
	now := t1.Add(10 * time.Minute) // past restart-pending window

	v := VerifyTarget(tgt, ev, now)
	var flagged bool
	for _, f := range v.Findings {
		if f.ID == FindingOldPidAfterUpgrade {
			flagged = true
		}
	}
	if !flagged {
		t.Error("expected old_pid_after_upgrade for a PID that started before the apply boundary; the split must not blind the genuine case")
	}
}

// Test 3 (1dc77898 regression guard): after an upgrade the restarted process's
// proof is briefly nil. A RECENT ApplyTime (=max, bumped by the upgrade) must keep
// runtime_identity_unproven inside the Day-0/post-upgrade grace window (info) —
// even though OldPidBoundary is an old first-install time. This proves the split
// left the grace window reading ApplyTime, not the stable boundary.
func TestVerifyTarget_ProofPendingWithinGrace_StaysInfo(t *testing.T) {
	now := time.Unix(1800000000, 0)

	tgt := Target{
		Service:                   "xds",
		NodeID:                    "ryzen",
		DesiredVersion:            "1.2.234",
		DesiredEntrypointChecksum: hashA,
		RuntimeNeeded:             true,
		ApplyTime:                 now.Add(-30 * time.Second),         // upgrade just applied (recent max)
		ApplyTimeSource:           "installed_package.updated_unix",
		OldPidBoundary:            now.Add(-90 * 24 * time.Hour),      // old first-install anchor
		IsFirstInstall:            false,
	}
	ev := Evidence{Proof: nil} // proof pending during the restart

	v := VerifyTarget(tgt, ev, now)
	var found *Finding
	for i := range v.Findings {
		if v.Findings[i].ID == FindingRuntimeIdentityUnproven {
			found = &v.Findings[i]
		}
	}
	if found == nil {
		t.Fatal("expected runtime_identity_unproven when proof is nil")
	}
	if found.Severity != SeverityInfo {
		t.Errorf("severity = %s, want info: the recent ApplyTime must keep the grace window active; the old_pid boundary split must not weaken it", found.Severity)
	}
}
