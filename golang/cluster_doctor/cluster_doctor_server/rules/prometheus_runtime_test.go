package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

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
