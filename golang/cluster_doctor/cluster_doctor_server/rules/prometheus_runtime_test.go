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

// Backend pressure must NOT fire just because the lifetime counter is
// non-zero — a transient blip during a workflow restart can leave the
// counter at 1 forever and the finding would never auto-clear. Only the
// rate-over-window metrics drive the finding now.
func TestPromRuntime_BackendPressure_LifetimeCounterAloneDoesNotFire(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"workflow_dispatch_rejected":          5, // historical blip
			"workflow_dispatch_rejected_rate_5m":  0, // nothing recent
			"workflow_dispatch_rejected_rate_15m": 0,
		},
		PromTS: time.Now(),
	}
	for _, f := range (promRuntime{}).Evaluate(snap, Config{}) {
		if f.InvariantID == "workflow.backend_pressure" {
			t.Fatalf("transient historical rejections must NOT fire backend_pressure; got %+v", f)
		}
	}
}

// Recent rejections below the sustained threshold = advisory only
// (INFO + INVARIANT_PASS) so the incident scanner's "skip PASS" gate
// drops them.
func TestPromRuntime_BackendPressure_TransientIsAdvisoryPass(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"workflow_dispatch_rejected_rate_5m":  2,
			"workflow_dispatch_rejected_rate_15m": 2,
		},
		PromTS: time.Now(),
	}
	var got *Finding
	for _, f := range (promRuntime{}).Evaluate(snap, Config{}) {
		if f.InvariantID == "workflow.backend_pressure" {
			f := f
			got = &f
		}
	}
	if got == nil {
		t.Fatal("expected a transient backend_pressure finding")
	}
	if got.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Errorf("transient pressure must be INVARIANT_PASS (advisory); got %v", got.InvariantStatus)
	}
	if got.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Errorf("transient pressure must be INFO; got %v", got.Severity)
	}
}

// Sustained pressure (rate_15m > threshold) elevates to WARN + FAIL so
// it opens an OPEN incident operators can act on.
func TestPromRuntime_BackendPressure_SustainedElevatesToFail(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"workflow_dispatch_rejected_rate_5m":  20,
			"workflow_dispatch_rejected_rate_15m": 50, // well above threshold (5)
		},
		PromTS: time.Now(),
	}
	var got *Finding
	for _, f := range (promRuntime{}).Evaluate(snap, Config{}) {
		if f.InvariantID == "workflow.backend_pressure" {
			f := f
			got = &f
		}
	}
	if got == nil {
		t.Fatal("expected a sustained backend_pressure finding")
	}
	if got.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("sustained pressure must be INVARIANT_FAIL; got %v", got.InvariantStatus)
	}
	if got.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("sustained pressure must be WARN; got %v", got.Severity)
	}
}

// release.blocked_workflow_unavailable: a low value (1-2) on a sticky
// gauge after a flap is advisory; only N+ stuck releases elevate.
func TestPromRuntime_ReleaseBlocked_LowValueIsAdvisory(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"release_transient_blocked": 1,
		},
		PromTS: time.Now(),
	}
	var got *Finding
	for _, f := range (promRuntime{}).Evaluate(snap, Config{}) {
		if f.InvariantID == "release.blocked_workflow_unavailable" {
			f := f
			got = &f
		}
	}
	if got == nil {
		t.Fatal("expected a release.blocked_workflow_unavailable finding")
	}
	if got.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Errorf("low stuck count must be INVARIANT_PASS (advisory); got %v", got.InvariantStatus)
	}
}

// xds_config_applied_total only increments when the controller injects a
// globular-xds.service restart action because the rendered config hash
// changed. xDS itself pushes snapshots over gRPC independently of this
// counter, so applied_total=0 with events_total>0 is normal on a stable
// cluster. Must be INVARIANT_PASS so no incident is opened.
func TestPromRuntime_XdsNoApplies_AdvisoryNotFailure(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"xds_config_events_total":  8,
			"xds_config_applied_total": 0,
		},
		PromTS: time.Now(),
	}
	var got *Finding
	for _, f := range (promRuntime{}).Evaluate(snap, Config{}) {
		if f.InvariantID == "xds.no_applies" {
			f := f
			got = &f
		}
	}
	if got == nil {
		t.Fatal("expected an xds.no_applies advisory finding")
	}
	if got.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Errorf("xds.no_applies must be advisory PASS (counter-mismatch is normal); got %v", got.InvariantStatus)
	}
	if got.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Errorf("xds.no_applies must be INFO severity; got %v", got.Severity)
	}
}

func TestPromRuntime_ReleaseBlocked_HighValueElevates(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"release_transient_blocked": 7,
		},
		PromTS: time.Now(),
	}
	var got *Finding
	for _, f := range (promRuntime{}).Evaluate(snap, Config{}) {
		if f.InvariantID == "release.blocked_workflow_unavailable" {
			f := f
			got = &f
		}
	}
	if got == nil {
		t.Fatal("expected a release.blocked_workflow_unavailable finding")
	}
	if got.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("high stuck count must be INVARIANT_FAIL; got %v", got.InvariantStatus)
	}
}
