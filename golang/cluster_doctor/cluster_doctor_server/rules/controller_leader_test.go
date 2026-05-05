package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestControllerLeaderPendingUpdate_NoRecord verifies that no findings are
// produced when the snapshot has no leader_pending_update record.
func TestControllerLeaderPendingUpdate_NoRecord(t *testing.T) {
	snap := &collector.Snapshot{}
	inv := controllerLeaderPendingUpdate{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("nil record → no findings, got %d", len(got))
	}
}

// TestControllerLeaderPendingUpdate_ZeroTimestamp verifies that a record with
// DetectedAtUnix == 0 is skipped (likely a write error, treat as no-record).
func TestControllerLeaderPendingUpdate_ZeroTimestamp(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "n1",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			DetectedAtUnix: 0,
		},
	}
	inv := controllerLeaderPendingUpdate{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("zero timestamp → no findings, got %d", len(got))
	}
}

// TestControllerLeaderPendingUpdate_StaleRecord verifies that a record older
// than pendingUpdateStaleness is silently ignored.
func TestControllerLeaderPendingUpdate_StaleRecord(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "n1",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			StuckSinceUnix: time.Now().Add(-30 * time.Minute).Unix(),
			DetectedAtUnix: time.Now().Add(-10 * time.Minute).Unix(), // older than 5-min staleness
		},
	}
	inv := controllerLeaderPendingUpdate{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("stale record → no findings, got %d", len(got))
	}
}

// TestControllerLeaderPendingUpdate_FreshWarning verifies that a fresh record
// where the leader has been stuck for less than pendingUpdateEscalateAfter
// produces a SEVERITY_WARN finding.
func TestControllerLeaderPendingUpdate_FreshWarning(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "node-ryzen",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			FollowersTotal: 2,
			StuckSinceUnix: time.Now().Add(-5 * time.Minute).Unix(), // under escalation threshold
			DetectedAtUnix: time.Now().Unix(),
		},
	}
	inv := controllerLeaderPendingUpdate{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "controller.leader_pending_update" {
		t.Errorf("InvariantID = %q, want controller.leader_pending_update", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("Severity = %v, want SEVERITY_WARN", f.Severity)
	}
}

// TestControllerLeaderPendingUpdate_EscalatesAfterThreshold verifies that when
// StuckSinceUnix indicates > pendingUpdateEscalateAfter, severity is ERROR.
func TestControllerLeaderPendingUpdate_EscalatesAfterThreshold(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "node-nuc",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			FollowersTotal: 2,
			StuckSinceUnix: time.Now().Add(-25 * time.Minute).Unix(), // exceeds 20-min threshold
			DetectedAtUnix: time.Now().Unix(),
		},
	}
	inv := controllerLeaderPendingUpdate{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("Severity = %v, want SEVERITY_ERROR after escalation threshold", findings[0].Severity)
	}
}

// TestControllerLeaderPendingUpdate_EntityRefIncludesLeaderNode verifies that
// the EntityRef encodes the leader node ID so operators can see which node
// owns the stuck control plane.
func TestControllerLeaderPendingUpdate_EntityRefIncludesLeaderNode(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "node-ryzen",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			StuckSinceUnix: time.Now().Add(-2 * time.Minute).Unix(),
			DetectedAtUnix: time.Now().Unix(),
		},
	}
	inv := controllerLeaderPendingUpdate{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if findings[0].EntityRef != "node-ryzen/cluster-controller" {
		t.Errorf("EntityRef = %q, want node-ryzen/cluster-controller", findings[0].EntityRef)
	}
}

// TestControllerLeaderPendingUpdate_RemediationStepsPresent verifies that the
// finding includes at least one remediation step.
func TestControllerLeaderPendingUpdate_RemediationStepsPresent(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "n1",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			StuckSinceUnix: time.Now().Add(-1 * time.Minute).Unix(),
			DetectedAtUnix: time.Now().Unix(),
		},
	}
	inv := controllerLeaderPendingUpdate{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if len(findings[0].Remediation) == 0 {
		t.Error("finding must include remediation steps")
	}
}

// TestControllerLeaderPendingUpdate_ZeroStuckSince verifies that when
// StuckSinceUnix is 0 (not set), severity stays at WARN (no escalation).
func TestControllerLeaderPendingUpdate_ZeroStuckSince(t *testing.T) {
	snap := &collector.Snapshot{
		LeaderPendingUpdate: &collector.LeaderPendingUpdateRecord{
			LeaderNodeID:   "n1",
			CurrentVersion: "1.0.83",
			TargetVersion:  "1.0.84+5",
			StuckSinceUnix: 0, // unset
			DetectedAtUnix: time.Now().Unix(),
		},
	}
	inv := controllerLeaderPendingUpdate{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("Severity = %v, want SEVERITY_WARN when stuck_since is 0", findings[0].Severity)
	}
}
