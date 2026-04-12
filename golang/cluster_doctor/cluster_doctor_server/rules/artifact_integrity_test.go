package rules

import (
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestArtifactIntegrity_NoReports_Silent verifies that nodes without
// integrity reports produce zero findings (best-effort, no false positives).
func TestArtifactIntegrity_NoReports_Silent(t *testing.T) {
	snap := &collector.Snapshot{
		IntegrityReports: map[string]*collector.IntegrityReport{},
	}
	got := artifactIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(got), got)
	}
}

// TestArtifactIntegrity_HealthyReport_NoFalsePositives verifies that a
// node whose VerifyPackageIntegrity returned a clean report produces no
// doctor findings.
func TestArtifactIntegrity_HealthyReport_NoFalsePositives(t *testing.T) {
	snap := &collector.Snapshot{
		IntegrityReports: map[string]*collector.IntegrityReport{
			"4c2b3cb3-d02a-56d3-93cf-4e2c8728e8a4": {
				NodeID:     "4c2b3cb3-d02a-56d3-93cf-4e2c8728e8a4",
				Checked:    48,
				Findings:   nil,
				Invariants: map[string]int{},
			},
		},
	}
	got := artifactIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 0 {
		t.Fatalf("healthy report should produce 0 findings, got %d", len(got))
	}
}

// TestArtifactIntegrity_DigestMismatch_Critical verifies that an ERROR-severity
// installed_digest_mismatch from the action is surfaced as a doctor
// SEVERITY_ERROR finding with the expected InvariantID.
func TestArtifactIntegrity_DigestMismatch_Critical(t *testing.T) {
	snap := &collector.Snapshot{
		IntegrityReports: map[string]*collector.IntegrityReport{
			"eb9a2dac-05b0-52ac-9002-99d8ffd35902": {
				NodeID:  "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
				Checked: 1,
				Findings: []collector.IntegrityFinding{
					{
						Invariant: "artifact.installed_digest_mismatch",
						Severity:  "ERROR",
						Package:   "event",
						Kind:      "SERVICE",
						Summary:   "installed_state checksum abcd differs from manifest ef01",
						Evidence: map[string]string{
							"installed_sha256": "abcd",
							"manifest_sha256":  "ef01",
						},
					},
				},
				Invariants: map[string]int{"artifact.installed_digest_mismatch": 1},
			},
		},
	}
	got := artifactIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	f := got[0]
	if f.InvariantID != "artifact.installed_digest_mismatch" {
		t.Fatalf("InvariantID: got %q, want artifact.installed_digest_mismatch", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("Severity: got %v, want SEVERITY_ERROR", f.Severity)
	}
	if f.Category != "artifact" {
		t.Fatalf("Category: got %q, want artifact", f.Category)
	}
	if len(f.Evidence) != 1 {
		t.Fatalf("expected 1 evidence block, got %d", len(f.Evidence))
	}
	if f.Evidence[0].GetKeyValues()["installed_sha256"] != "abcd" {
		t.Fatalf("evidence missing installed_sha256 entry")
	}
	if len(f.Remediation) < 1 {
		t.Fatalf("expected remediation steps, got none")
	}
}

// TestArtifactIntegrity_CacheMissing_Info surfaces informational findings
// for missing cache entries at INFO severity (not an error).
func TestArtifactIntegrity_CacheMissing_Info(t *testing.T) {
	snap := &collector.Snapshot{
		IntegrityReports: map[string]*collector.IntegrityReport{
			"node1": {
				NodeID: "node1",
				Findings: []collector.IntegrityFinding{
					{
						Invariant: "artifact.cache_missing",
						Severity:  "INFO",
						Package:   "search",
						Kind:      "SERVICE",
						Summary:   "manifest digest resolved but cache is absent",
					},
				},
				Invariants: map[string]int{"artifact.cache_missing": 1},
			},
		},
	}
	got := artifactIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Fatalf("cache_missing should be INFO, got %v", got[0].Severity)
	}
}

// TestArtifactIntegrity_WarnSeverities maps WARN-level invariants correctly.
func TestArtifactIntegrity_WarnSeverities(t *testing.T) {
	cases := []string{
		"artifact.desired_version_mismatch",
		"artifact.desired_build_mismatch",
		"artifact.cache_digest_mismatch",
	}
	for _, inv := range cases {
		snap := &collector.Snapshot{
			IntegrityReports: map[string]*collector.IntegrityReport{
				"n": {
					Findings: []collector.IntegrityFinding{
						{Invariant: inv, Severity: "WARN", Package: "p", Kind: "SERVICE", Summary: "x"},
					},
					Invariants: map[string]int{inv: 1},
				},
			},
		}
		got := artifactIntegrity{}.Evaluate(snap, Config{})
		if len(got) != 1 {
			t.Fatalf("%s: expected 1 finding, got %d", inv, len(got))
		}
		if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
			t.Fatalf("%s: expected SEVERITY_WARN, got %v", inv, got[0].Severity)
		}
		if got[0].InvariantID != inv {
			t.Fatalf("%s: InvariantID mismatch: got %q", inv, got[0].InvariantID)
		}
	}
}
