package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestResilienceIncidentSignals_EtcdFallback(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "n1"},
		},
		IngressNodeStatus:       map[string]map[string]interface{}{},
		ScyllaSchemaGuardStatus: map[string]map[string]interface{}{},
		DNSZoneReloadStatus:     map[string]interface{}{},
		ReconcileLaneStatus:     map[string]map[string]interface{}{},
	}

	// Incident signals:
	// - ingress spec missing
	// - scylla RF violation on critical keyspace
	// - dns serving last-known-good due to degraded reload
	// - critical reconcile lane blocked
	snap.IngressSpecPresent = false
	snap.ScyllaSchemaGuardStatus["dns"] = map[string]interface{}{
		"violation":   true,
		"current_rf":  float64(1),
		"required_rf": float64(3),
		"last_error":  "ALTER pending",
	}
	snap.DNSZoneReloadStatus = map[string]interface{}{
		"phase":                   "DEGRADED_RELOAD_FAILED",
		"serving_last_known_good": true,
		"last_error":              "LOCAL_QUORUM requires 1 alive 0",
	}
	snap.ReconcileLaneStatus["cluster_reconcile"] = map[string]interface{}{
		"phase":      "BLOCKED",
		"last_error": "previous run still active",
	}

	reg := NewRegistry(Config{})
	findings := reg.EvaluateAll(snap)

	assertInvariantPresent(t, findings, "ingress.spec_missing")
	assertInvariantPresent(t, findings, "scylla.keyspace.rf_policy_violation")
	assertInvariantPresent(t, findings, "dns.zone_reload_failed")
	assertInvariantPresent(t, findings, "dns.serving_last_known_good")
	assertInvariantPresent(t, findings, "reconcile.critical_lane_blocked")
}

func TestResilienceIncidentSignals_PrometheusPreferred(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"reconcile_lane_blocked_cluster":        1,
			"reconcile_lane_timeouts_cluster":       2,
			"reconcile_lane_blocked_projections":    0,
			"reconcile_lane_blocked_release_bridge": 0,
			"reconcile_lane_blocked_drift":          0,
		},
		ReconcileLaneStatus: map[string]map[string]interface{}{
			"cluster_reconcile": {
				"phase":      "BLOCKED",
				"last_error": "should be ignored when Prom metrics exist",
			},
		},
	}

	reg := NewRegistry(Config{})
	findings := reg.EvaluateAll(snap)

	assertInvariantPresent(t, findings, "reconcile.critical_lane_blocked")
	assertInvariantPresent(t, findings, "reconcile.lane_timeout")
	// Ensure etcd fallback rule did not duplicate with its own invariant.
	// It emits one of reconcile.* IDs too, so enforce count by invariant.
	if c := countInvariant(findings, "reconcile.critical_lane_blocked"); c != 1 {
		t.Fatalf("expected exactly 1 reconcile.critical_lane_blocked finding, got %d", c)
	}
}

func TestReconcileLaneSeverityMapping_FromEtcdFallback(t *testing.T) {
	snap := &collector.Snapshot{
		ReconcileLaneStatus: map[string]map[string]interface{}{
			"cluster_reconcile": {"phase": "BLOCKED", "last_error": "active"},
			"projections":       {"phase": "BLOCKED", "last_error": "active"},
			"release_bridge":    {"phase": "BLOCKED", "last_error": "active"},
			"drift_reconcile":   {"phase": "BLOCKED", "last_error": "active"},
		},
	}
	reg := NewRegistry(Config{})
	findings := reg.EvaluateAll(snap)

	if sev := invariantSeverity(findings, "reconcile.critical_lane_blocked", "cluster_reconcile"); sev != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("cluster_reconcile severity = %v, want CRITICAL", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_blocked", "projections"); sev != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("projections severity = %v, want WARN", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_blocked", "release_bridge"); sev != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("release_bridge severity = %v, want WARN", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_blocked", "drift_reconcile"); sev != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("drift_reconcile severity = %v, want WARN", sev)
	}
}

func TestReconcileLaneSeverityMapping_FromPrometheus(t *testing.T) {
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"reconcile_lane_blocked_cluster":        1,
			"reconcile_lane_timeouts_cluster":       1,
			"reconcile_lane_blocked_projections":    1,
			"reconcile_lane_blocked_release_bridge": 1,
			"reconcile_lane_blocked_drift":          1,
		},
	}
	reg := NewRegistry(Config{})
	findings := reg.EvaluateAll(snap)

	if sev := invariantSeverity(findings, "reconcile.critical_lane_blocked", "cluster_reconcile"); sev != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("cluster_reconcile blocked severity = %v, want CRITICAL", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_timeout", "cluster_reconcile"); sev != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("cluster_reconcile timeout severity = %v, want ERROR", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_blocked", "projections"); sev != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("projections blocked severity = %v, want WARN", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_blocked", "release_bridge"); sev != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("release_bridge blocked severity = %v, want WARN", sev)
	}
	if sev := invariantSeverity(findings, "reconcile.lane_blocked", "drift_reconcile"); sev != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("drift_reconcile blocked severity = %v, want WARN", sev)
	}
}

func assertInvariantPresent(t *testing.T, findings []Finding, invariant string) {
	t.Helper()
	for _, f := range findings {
		if f.InvariantID == invariant {
			return
		}
	}
	t.Fatalf("expected invariant %q not found", invariant)
}

func countInvariant(findings []Finding, invariant string) int {
	n := 0
	for _, f := range findings {
		if f.InvariantID == invariant {
			n++
		}
	}
	return n
}

func invariantSeverity(findings []Finding, invariant, entity string) cluster_doctorpb.Severity {
	for _, f := range findings {
		if f.InvariantID == invariant && f.EntityRef == entity {
			return f.Severity
		}
	}
	return cluster_doctorpb.Severity_SEVERITY_UNKNOWN
}
