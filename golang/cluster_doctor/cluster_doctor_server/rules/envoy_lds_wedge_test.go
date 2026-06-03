package rules

// envoy_lds_wedge_test.go — Phase 28.
//
// Pins the diagnostic behaviour of envoyLDSWedge documented in:
//   invariant: envoy.lds_progress_required_for_http_mesh_readiness
//   failure_mode: envoy.lds_update_attempt_zero_despite_cds_progress
//
// These tests cover the four possible PromMetrics shapes the rule
// must classify correctly — plus the "metrics absent" no-op.

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestEnvoyLDSWedge_CDSProgress_LDSZero_FiresCritical(t *testing.T) {
	// The Phase 24 anchor scenario: Envoy has applied several CDS
	// updates but LDS update_attempt is still 0. Mesh is wedged; port
	// 443 will not be bound. Doctor MUST surface this as CRITICAL.
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"envoy_cds_update_success":  4,
			"envoy_lds_update_attempt":  0,
			"envoy_lds_update_success":  0,
			"envoy_lds_update_rejected": 0,
		},
		PromTS: time.Now(),
	}
	fs := envoyLDSWedge{}.Evaluate(snap, Config{})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d", len(fs))
	}
	f := fs[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("severity=%v want CRITICAL", f.Severity)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("status=%v want INVARIANT_FAIL", f.InvariantStatus)
	}
	if f.InvariantID != "envoy.lds_progress_required_for_http_mesh_readiness" {
		t.Errorf("invariant_id=%q does not match the anchored invariant", f.InvariantID)
	}
}

func TestEnvoyLDSWedge_LDSHealthy_FiresInfoPass(t *testing.T) {
	// Both CDS and LDS have progressed. Surface as INFO + PASS so the
	// healthy state is recorded in the snapshot ledger, but no
	// incident is opened.
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"envoy_cds_update_success":  10,
			"envoy_lds_update_attempt":  10,
			"envoy_lds_update_success":  10,
			"envoy_lds_update_rejected": 0,
		},
		PromTS: time.Now(),
	}
	fs := envoyLDSWedge{}.Evaluate(snap, Config{})
	if len(fs) != 1 {
		t.Fatalf("want 1 PASS info finding, got %d", len(fs))
	}
	f := fs[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Errorf("severity=%v want INFO", f.Severity)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Errorf("status=%v want INVARIANT_PASS", f.InvariantStatus)
	}
}

func TestEnvoyLDSWedge_ColdInit_NoFinding(t *testing.T) {
	// CDS hasn't progressed yet either — Envoy is still in cold init.
	// Firing now would just be noise during the normal startup window.
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"envoy_cds_update_success": 0,
			"envoy_lds_update_attempt": 0,
		},
		PromTS: time.Now(),
	}
	if fs := (envoyLDSWedge{}).Evaluate(snap, Config{}); len(fs) != 0 {
		t.Fatalf("expected no finding during cold init, got %d", len(fs))
	}
}

func TestEnvoyLDSWedge_LDSAttemptedButNotSucceeded_NoFire(t *testing.T) {
	// LDS update_attempt > 0 but success still 0 (e.g. snapshot
	// rejected, or in-flight). The specific wedge condition from the
	// invariant anchor (update_attempt == 0) is no longer present, so
	// this rule stays silent and lets the more specific rejection
	// rules (future work) own the diagnosis. Critically, doctor must
	// NOT mis-classify a transient handshake as wedged.
	snap := &collector.Snapshot{
		PromMetrics: map[string]float64{
			"envoy_cds_update_success":  4,
			"envoy_lds_update_attempt":  3,
			"envoy_lds_update_success":  0,
			"envoy_lds_update_rejected": 3,
		},
		PromTS: time.Now(),
	}
	if fs := (envoyLDSWedge{}).Evaluate(snap, Config{}); len(fs) != 0 {
		t.Fatalf("expected no finding when LDS attempted (rejected path is owned elsewhere), got %d", len(fs))
	}
}

func TestEnvoyLDSWedge_MetricsAbsent_NoOp(t *testing.T) {
	// PromMetrics nil OR the specific keys missing must produce
	// nothing. Cluster doctor runs everywhere; not every cluster has
	// Prometheus, and during partial scrape windows the keys may
	// transiently be absent. A doctor rule must NEVER produce a
	// finding from absent evidence — that's how false-positive
	// incidents get manufactured.
	cases := []*collector.Snapshot{
		{PromMetrics: nil},
		{PromMetrics: map[string]float64{}},
		{PromMetrics: map[string]float64{"envoy_cds_update_success": 4}}, // lds key absent
		{PromMetrics: map[string]float64{"envoy_lds_update_attempt": 0}}, // cds key absent
	}
	for i, snap := range cases {
		fs := envoyLDSWedge{}.Evaluate(snap, Config{})
		if len(fs) != 0 {
			t.Fatalf("case %d: expected no finding for absent metrics, got %d", i, len(fs))
		}
	}
}

func TestEnvoyLDSWedge_RegisteredInDefaultRegistry(t *testing.T) {
	// Regression guard: NewRegistry must include envoyLDSWedge.
	// Without registration, the rule's Evaluate is never called and
	// the doctor sweep silently skips the wedge classification — the
	// exact "rule appears to work but is silently a no-op" trap that
	// registry.go's own header comment warns about.
	reg := NewRegistry(Config{})
	for _, inv := range reg.invariants {
		if inv.ID() == "envoy.lds_wedge" {
			return
		}
	}
	t.Fatal("envoyLDSWedge is not registered in NewRegistry — doctor sweeps will silently skip the LDS-wedge classification")
}
