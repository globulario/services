package render

import (
	"testing"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
)

// TestToProtofindingsFiltersPass verifies that INVARIANT_PASS findings
// (e.g. service.bootstrap_ordering_skew and package.wraps_upstream_binary
// emitted as info-only by the verifier) are dropped from the report output.
// PASS findings must not clutter the findings list; they are internal markers
// used by the incident scanner's "skip PASS" filter, not operator-facing signals.
func TestToProtofindingsFiltersPass(t *testing.T) {
	findings := []rules.Finding{
		{
			FindingID:       "pass-finding",
			InvariantID:     "service.bootstrap_ordering_skew",
			Severity:        cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:        "diagnostic.runtime",
			Summary:         "bootstrap_ordering_skew — info only",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
		},
		{
			FindingID:       "pass-finding-2",
			InvariantID:     "package.wraps_upstream_binary",
			Severity:        cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:        "diagnostic.runtime",
			Summary:         "wraps_upstream_binary — info only",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
		},
		{
			FindingID:       "fail-finding",
			InvariantID:     "node.reachable",
			Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:        "availability",
			Summary:         "node unreachable",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		},
	}

	proto := toProtoFindings(findings)

	if len(proto) != 1 {
		t.Fatalf("toProtoFindings: got %d findings, want 1 (PASS findings must be suppressed)", len(proto))
	}
	if proto[0].GetFindingId() != "fail-finding" {
		t.Errorf("toProtoFindings: got finding %q, want %q", proto[0].GetFindingId(), "fail-finding")
	}
}

// TestClusterReportPassFindingsNotInOutput is an end-to-end variant: PASS
// findings must not appear in ClusterReport.Findings even though they are
// present in the rules.Finding slice (they are set as PASS by
// runtime_verification.go for info-severity verifier findings so the
// incident scanner's "skip PASS" filter drops them internally).
func TestClusterReportPassFindingsNotInOutput(t *testing.T) {
	snap := &collector.Snapshot{
		SnapshotID:  "test",
		GeneratedAt: time.Now(),
	}
	findings := []rules.Finding{
		{
			FindingID:       "pass-skew",
			InvariantID:     "service.bootstrap_ordering_skew",
			Severity:        cluster_doctorpb.Severity_SEVERITY_INFO,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
		},
		{
			FindingID:       "fail-wf",
			InvariantID:     "workflow.reachable",
			Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		},
	}

	report := ClusterReport(snap, findings, "v0.0.0-test", Freshness{})

	for _, f := range report.GetFindings() {
		if f.GetInvariantStatus() == cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
			t.Errorf("ClusterReport.Findings contains PASS finding %q — must be suppressed", f.GetFindingId())
		}
	}
	if got := len(report.GetFindings()); got != 1 {
		t.Errorf("ClusterReport.Findings: got %d, want 1", got)
	}
}

// TestClusterReport_PlacementOrphanVisibleButHealthy is the D2 regression at the
// rollup: a placement orphan (SEVERITY_WARN + INVARIANT_PENDING) is a
// NON-BLOCKING warning, so it must (1) REMAIN VISIBLE in ClusterReport.Findings
// (PENDING is not dropped like PASS) and (2) NOT degrade the cluster verdict —
// the report stays CLUSTER_HEALTHY. The overallStatus control proves the fix is
// load-bearing: the SAME finding as INVARIANT_FAIL would degrade to
// CLUSTER_DEGRADED (the pre-D2 behavior we corrected).
func TestClusterReport_PlacementOrphanVisibleButHealthy(t *testing.T) {
	snap := &collector.Snapshot{SnapshotID: "test", GeneratedAt: time.Now()}
	orphan := rules.Finding{
		FindingID:       "placement.installed_package_orphaned:node-a:torrent",
		InvariantID:     "placement.installed_package_orphaned",
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:        "placement",
		Summary:         "orphaned install (test)",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PENDING,
	}

	report := ClusterReport(snap, []rules.Finding{orphan}, "v0.0.0-test", Freshness{})

	// (1) VISIBLE: a PENDING orphan is not filtered like PASS.
	if got := len(report.GetFindings()); got != 1 {
		t.Fatalf("placement orphan (INVARIANT_PENDING) must remain visible in the report: got %d findings, want 1", got)
	}
	if st := report.GetFindings()[0].GetInvariantStatus(); st != cluster_doctorpb.InvariantStatus_INVARIANT_PENDING {
		t.Fatalf("visible orphan must carry INVARIANT_PENDING, got %v", st)
	}

	// (2) HEALTHY: a non-blocking orphan must not degrade the cluster verdict.
	if got := report.GetOverallStatus(); got != cluster_doctorpb.ClusterStatus_CLUSTER_HEALTHY {
		t.Fatalf("placement orphan must not degrade the cluster verdict: got %v, want CLUSTER_HEALTHY", got)
	}

	// Control: the SAME WARN finding as INVARIANT_FAIL WOULD degrade — proving
	// the INVARIANT_PENDING classification is precisely what keeps it healthy.
	failing := []*cluster_doctorpb.Finding{{
		InvariantId:     "placement.installed_package_orphaned",
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
	if got := overallStatus(failing, false); got != cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED {
		t.Fatalf("control: an INVARIANT_FAIL WARN finding must degrade: got %v, want CLUSTER_DEGRADED", got)
	}
}

// TestOverallStatus_DataIncompleteDegrades pins
// meta.fallback_must_degrade_semantics for the cluster status rollup. When
// the snapshot is marked DataIncomplete (collectors couldn't reach every
// node), an empty findings list does NOT mean the cluster is healthy — it
// means we don't know. The badge must read DEGRADED so consumers see the
// uncertainty, even though the header already carries DataIncomplete.
func TestOverallStatus_DataIncompleteDegrades(t *testing.T) {
	got := overallStatus(nil, true)
	if got != cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED {
		t.Fatalf("overallStatus(nil, dataIncomplete=true) = %v, want CLUSTER_DEGRADED", got)
	}
}

// TestOverallStatus_CriticalEscalatesOverIncomplete confirms that a real
// CRITICAL finding still escalates the status past DEGRADED when the data
// is also incomplete — the dataIncomplete floor must not mask higher-severity
// findings, only lift HEALTHY → DEGRADED in their absence.
func TestOverallStatus_CriticalEscalatesOverIncomplete(t *testing.T) {
	findings := []*cluster_doctorpb.Finding{
		{
			FindingId:       "crit",
			Severity:        cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		},
	}
	got := overallStatus(findings, true)
	if got != cluster_doctorpb.ClusterStatus_CLUSTER_CRITICAL {
		t.Fatalf("overallStatus(critical+incomplete) = %v, want CLUSTER_CRITICAL", got)
	}
}

// TestOverallStatus_CompleteHealthy is the unchanged happy path: no
// findings AND no incomplete-data marker → healthy.
func TestOverallStatus_CompleteHealthy(t *testing.T) {
	got := overallStatus(nil, false)
	if got != cluster_doctorpb.ClusterStatus_CLUSTER_HEALTHY {
		t.Fatalf("overallStatus(nil, false) = %v, want CLUSTER_HEALTHY", got)
	}
}
