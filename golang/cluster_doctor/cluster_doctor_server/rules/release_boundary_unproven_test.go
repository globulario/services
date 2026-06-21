package rules

import (
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/release_boundary"
)

func report(verdict release_boundary.Verdict, assertions ...release_boundary.AssertionReport) release_boundary.Report {
	return release_boundary.Report{
		ServiceName: "event",
		NodeName:    "globule-ryzen",
		BuildID:     "019eeaee-ffaa-7516-a438-40920785578a",
		Checksum:    "b4429a27015b20bf",
		Verdict:     verdict,
		Assertions:  assertions,
	}
}

func a(id release_boundary.AssertionID, v release_boundary.Verdict, reason string) release_boundary.AssertionReport {
	return release_boundary.AssertionReport{ID: id, Verdict: v, Reason: reason}
}

func snapWith(rb *collector.ReleaseBoundaryReport) *collector.Snapshot {
	m := map[string]*collector.ReleaseBoundaryReport{}
	if rb != nil {
		m[rb.Service+"@"+rb.Node] = rb
	}
	return &collector.Snapshot{ReleaseBoundaryReports: m}
}

func evidenceMap(t *testing.T, f Finding) map[string]string {
	t.Helper()
	if len(f.Evidence) == 0 {
		t.Fatal("finding has no evidence")
	}
	return f.Evidence[0].GetKeyValues()
}

// 1. PROVEN → no finding.
func TestReleaseBoundaryUnproven_Proven_NoFinding(t *testing.T) {
	snap := snapWith(&collector.ReleaseBoundaryReport{
		Service: "event", Node: "globule-ryzen",
		Report: report(release_boundary.VerdictProven,
			a("A0", release_boundary.VerdictProven, "ok"),
			a("A4", release_boundary.VerdictProven, "ok")),
	})
	if got := (releaseBoundaryUnproven{}).Evaluate(snap, Config{}); len(got) != 0 {
		t.Fatalf("PROVEN must emit no finding, got %d", len(got))
	}
}

// 2. FAILED → finding with INVARIANT_FAIL + failed assertion details.
func TestReleaseBoundaryUnproven_Failed(t *testing.T) {
	snap := snapWith(&collector.ReleaseBoundaryReport{
		Service: "event", Node: "globule-ryzen",
		Report: report(release_boundary.VerdictFailed,
			a("A2", release_boundary.VerdictFailed, "installed checksum does not match manifest"),
			a("A4", release_boundary.VerdictProven, "ok")),
	})
	got := (releaseBoundaryUnproven{}).Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("FAILED must emit one finding, got %d", len(got))
	}
	f := got[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("status = %v, want INVARIANT_FAIL", f.InvariantStatus)
	}
	if f.CheckError != "" {
		t.Errorf("FAILED must not set CheckError, got %q", f.CheckError)
	}
	em := evidenceMap(t, f)
	if em["A2"] != "FAILED" || em["A2_reason"] == "" {
		t.Errorf("finding missing A2 failed-assertion detail: %v", em)
	}
}

// 3. INDETERMINATE → finding with INVARIANT_UNKNOWN + CheckError + missing link.
func TestReleaseBoundaryUnproven_Indeterminate(t *testing.T) {
	snap := snapWith(&collector.ReleaseBoundaryReport{
		Service: "event", Node: "globule-ryzen",
		Report: report(release_boundary.VerdictIndeterminate,
			a("A0", release_boundary.VerdictIndeterminate, "repository verification evidence missing")),
	})
	got := (releaseBoundaryUnproven{}).Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("INDETERMINATE must emit one finding, got %d", len(got))
	}
	f := got[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("status = %v, want INVARIANT_UNKNOWN", f.InvariantStatus)
	}
	if f.CheckError == "" {
		t.Error("INDETERMINATE must set CheckError so it is not counted as FAIL")
	}
	if em := evidenceMap(t, f); em["A0_reason"] == "" {
		t.Errorf("finding missing A0 missing-link reason: %v", em)
	}
}

// 4. NOT_APPLICABLE → finding (never OK), INVARIANT_UNKNOWN + INFO.
func TestReleaseBoundaryUnproven_NotApplicable_NotOK(t *testing.T) {
	snap := snapWith(&collector.ReleaseBoundaryReport{
		Service: "keepalived", Node: "globule-ryzen",
		Report: report(release_boundary.VerdictNotApplicable),
	})
	got := (releaseBoundaryUnproven{}).Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("NOT_APPLICABLE must emit a finding (never OK), got %d", len(got))
	}
	f := got[0]
	if f.InvariantStatus == cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Error("NOT_APPLICABLE must never be marked OK/PASS")
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Errorf("severity = %v, want INFO", f.Severity)
	}
}

// 5. Collection errors appear in the finding evidence.
func TestReleaseBoundaryUnproven_CollectionErrorsSurfaced(t *testing.T) {
	snap := snapWith(&collector.ReleaseBoundaryReport{
		Service: "event", Node: "globule-ryzen",
		Report: report(release_boundary.VerdictIndeterminate,
			a("A0", release_boundary.VerdictIndeterminate, "repository verification evidence missing")),
		CollectionErrors: map[string]string{"verify": "VerifyArtifact: Unauthenticated"},
	})
	got := (releaseBoundaryUnproven{}).Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %d", len(got))
	}
	if em := evidenceMap(t, got[0]); em["collection_error.verify"] == "" {
		t.Errorf("collection error not surfaced in finding: %v", em)
	}
}

// Empty snapshot → no findings (no panic, no silent OK claim).
func TestReleaseBoundaryUnproven_EmptySnapshot(t *testing.T) {
	if got := (releaseBoundaryUnproven{}).Evaluate(&collector.Snapshot{}, Config{}); got != nil {
		t.Fatalf("empty snapshot must yield no findings, got %d", len(got))
	}
}
