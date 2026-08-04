package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/remediation"
)

// TestVerificationSnapshotIsPostAction guards the defect that autonomous repairs
// were verified against a snapshot that could pre-date the repair.
//
// observeHealerOutcome originally called collector.GetSnapshot, which serves the
// cache — so the verification could read the very pre-repair snapshot the finding
// was raised from. The finding is still present in that data, so a repair that
// actually worked records FindingResolved=false. That is strictly worse than
// recording nothing: the learning loop accumulates support for the proposition
// that the repair does not work, and the promotion decision it governs is then
// made on inverted evidence.
//
// Forcing fresh alone does not close it. The collector can join an already
// in-flight fetch that started before the dispatch, which returns CacheHit=false
// while still describing pre-repair state. Freshness is therefore proven by
// ordering against the dispatch instant, which is what this pins.
func TestVerificationSnapshotIsPostAction(t *testing.T) {
	dispatchedAt := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		snap *collector.Snapshot
		want bool
		why  string
	}{
		{
			name: "snapshot generated after dispatch is usable",
			snap: &collector.Snapshot{GeneratedAt: dispatchedAt.Add(2 * time.Second)},
			want: true,
			why:  "this is the only case that observes the result of the action",
		},
		{
			name: "snapshot generated before dispatch is refused",
			snap: &collector.Snapshot{GeneratedAt: dispatchedAt.Add(-2 * time.Second)},
			want: false,
			why:  "the cached pre-repair snapshot — the original defect; it would record a successful repair as failed",
		},
		{
			name: "snapshot generated at the dispatch instant is refused",
			snap: &collector.Snapshot{GeneratedAt: dispatchedAt},
			want: false,
			why:  "same instant does not prove the snapshot observed the action's result",
		},
		{
			name: "nil snapshot is refused",
			snap: nil,
			want: false,
			why:  "nothing was collected, so nothing can be verified",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := verificationSnapshotIsPostAction(tc.snap, dispatchedAt); got != tc.want {
				t.Errorf("verificationSnapshotIsPostAction = %v, want %v — %s", got, tc.want, tc.why)
			}
		})
	}
}

// TestHealerOutcomeLineage_DispatchedAtIsStamped pins the half of the autonomous
// lineage that is now closed, and states the half that is not.
//
// An outcome is cited as verification evidence only when IsSuccess() AND
// LineageComplete() both hold (observation.FromRemediationOutcome); anything else
// is recorded as a deliberately unqualifiable diagnostic row. The healer's
// outcome previously carried neither DispatchedAt nor WorkflowRunID, so it
// scored two lineage defects and could never qualify.
//
// DispatchedAt is now stamped at the executor-accepted instant. WorkflowRunID is
// still absent by design — an autonomous repair belongs to no workflow run, and
// synthesising one would forge lineage for a dispatch that never had it. This
// test therefore asserts the exact remaining gap, so that closing it is a
// deliberate change to a failing assertion rather than a silent drift.
func TestHealerOutcomeLineage_DispatchedAtIsStamped(t *testing.T) {
	dispatchedAt := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	// The shape observeHealerOutcome now produces for a verified, cleared repair.
	o := remediation.Outcome{
		FindingID:       "f-unit-1",
		ActionCheckID:   "chk-1",
		Dispatched:      true,
		DispatchedAt:    dispatchedAt,
		Verified:        true,
		FindingResolved: true,
		VerifiedAt:      dispatchedAt.Add(3 * time.Second),
		ClusterID:       "cluster-1",
		InvariantID:     "node.systemd.units_running",
		EntityRef:       "node-1/globular-log.service",
		NodeID:          "node-1",
	}

	if !o.IsSuccess() {
		t.Fatalf("a dispatched, verified, cleared repair must be a success; status=%s", o.Status())
	}

	defects := o.LineageDefects()
	for _, d := range defects {
		if d == remediation.LineageMissingDispatchAt {
			t.Errorf("DispatchedAt must be stamped for an autonomous dispatch — %q means "+
				"post-action verification cannot be placed after the action", d)
		}
		if d == remediation.LineageVerifiedBefore {
			t.Errorf("verification must not pre-date dispatch; got defect %q", d)
		}
	}

	// The remaining, known gap. If this ever stops being the only defect, the
	// autonomous lineage question on #236 has been answered — update this test
	// deliberately rather than deleting the assertion.
	if len(defects) != 1 || defects[0] != remediation.LineageMissingWorkflowRun {
		t.Errorf("expected exactly one remaining lineage defect (%q), got %v.\n"+
			"An autonomous repair has no workflow run; until that is resolved this outcome "+
			"is recorded as a diagnostic row and cannot qualify as verification evidence.",
			remediation.LineageMissingWorkflowRun, defects)
	}
	if o.LineageComplete() {
		t.Error("LineageComplete must still be false — the autonomous dispatch identity is unresolved")
	}
}
