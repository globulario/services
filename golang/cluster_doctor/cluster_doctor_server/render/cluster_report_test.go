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
