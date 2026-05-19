package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestControllerStall_NonLeaderHeartbeatIsNotCritical verifies that the
// prometheus.controller_stalled finding is suppressed when Prometheus only
// scrapes a non-leader controller instance. Non-leaders never update
// loop_heartbeat_unix; their stale heartbeat must not fire a false CRITICAL.
func TestControllerStall_NonLeaderHeartbeatIsNotCritical(t *testing.T) {
	inv := promRuntime{}
	// Simulate: heartbeat stale by 12 000s, but this instance dropped 1 tick
	// for not_leader — so it is a follower, not stuck.
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"controller_loop_heartbeat_age":  12000,
			"reconcile_dropped_not_leader":   1,
		},
		PromTS: time.Now(),
	}
	for _, f := range inv.Evaluate(snap, Config{}) {
		if f.InvariantID == "prometheus.controller_stalled" {
			t.Fatalf("expected NO controller_stalled finding for non-leader instance, got: %s", f.Summary)
		}
	}
}

// TestControllerStall_LeaderHeartbeatStaleIsCritical verifies that a stale
// heartbeat on the leader (reconcile_dropped_not_leader == 0) still fires.
func TestControllerStall_LeaderHeartbeatStaleIsCritical(t *testing.T) {
	inv := promRuntime{}
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"controller_loop_heartbeat_age": 12000,
			"reconcile_dropped_not_leader":  0,
		},
		PromTS: time.Now(),
	}
	for _, f := range inv.Evaluate(snap, Config{}) {
		if f.InvariantID == "prometheus.controller_stalled" {
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
				t.Fatalf("severity=%v want SEVERITY_CRITICAL", f.Severity)
			}
			return
		}
	}
	t.Fatal("expected controller_stalled CRITICAL finding for leader with stale heartbeat, got none")
}

// TestControllerStall_NoDropMetric_StillFires verifies that a stale heartbeat
// fires even when the reconcile_dropped_not_leader metric is absent (metric
// not yet available — conservative: treat as potential stall).
func TestControllerStall_NoDropMetric_StillFires(t *testing.T) {
	inv := promRuntime{}
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"controller_loop_heartbeat_age": 12000,
			// reconcile_dropped_not_leader absent
		},
		PromTS: time.Now(),
	}
	for _, f := range inv.Evaluate(snap, Config{}) {
		if f.InvariantID == "prometheus.controller_stalled" {
			return // correctly fired
		}
	}
	t.Fatal("expected controller_stalled finding when drop metric absent, got none")
}

func TestPromRuntime_ControllerLeaderOutdatedFinding(t *testing.T) {
	inv := promRuntime{}
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"controller_leader_outdated": 1,
		},
		PromTS: time.Now(),
	}

	findings := inv.Evaluate(snap, Config{})
	var found *Finding
	for i := range findings {
		if findings[i].InvariantID == "controller_leader_outdated" {
			found = &findings[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected controller_leader_outdated finding, got %d findings", len(findings))
	}
	if found.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("severity=%v want SEVERITY_WARN", found.Severity)
	}
}

func TestPromRuntime_ControllerNoSafeSuccessorFinding(t *testing.T) {
	inv := promRuntime{}
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"controller_no_safe_successor": 1,
		},
		PromTS: time.Now(),
	}

	findings := inv.Evaluate(snap, Config{})
	var found *Finding
	for i := range findings {
		if findings[i].InvariantID == "controller_no_safe_successor" {
			found = &findings[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected controller_no_safe_successor finding, got %d findings", len(findings))
	}
	if found.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("severity=%v want SEVERITY_ERROR", found.Severity)
	}
}

func TestPromRuntime_ControllerLeaderSafetyZeroDoesNotFire(t *testing.T) {
	inv := promRuntime{}
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"controller_leader_outdated":   0,
			"controller_no_safe_successor": 0,
		},
		PromTS: time.Now(),
	}

	findings := inv.Evaluate(snap, Config{})
	for _, f := range findings {
		if f.InvariantID == "controller_leader_outdated" || f.InvariantID == "controller_no_safe_successor" {
			t.Fatalf("unexpected finding when gauges are zero: %s", f.InvariantID)
		}
	}
}
