package rules

import (
	"fmt"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/config"
)

// ── pki.ca_not_published ────────────────────────────────────────────────────

func TestPKICANotPublished_NoNodes_NoFinding(t *testing.T) {
	// Empty cluster: no findings even if CA is missing.
	snap := &collector.Snapshot{
		CAMetadata: nil,
		Nodes:      nil,
	}
	findings := (pkiCANotPublished{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty cluster, got %d", len(findings))
	}
}

func TestPKICANotPublished_CAPresent_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		CAMetadata: &config.CAMetadata{Fingerprint: "abc123", Generation: 1},
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "n1"},
		},
	}
	findings := (pkiCANotPublished{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when CA is present, got %d", len(findings))
	}
}

func TestPKICANotPublished_CAMissing_WithNodes_FindingFired(t *testing.T) {
	snap := &collector.Snapshot{
		CAMetadata: nil,
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "n1"},
		},
	}
	findings := (pkiCANotPublished{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when CA absent and nodes exist, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "pki.ca_not_published" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN severity, got %v", f.Severity)
	}
}

// ── pki.ca_expiry_warning ────────────────────────────────────────────────────

func TestPKICAExpiry_Healthy_NoFinding(t *testing.T) {
	notAfter := time.Now().Add(90 * 24 * time.Hour).UTC().Format(time.RFC3339)
	snap := &collector.Snapshot{
		CAMetadata: &config.CAMetadata{Fingerprint: "abc", NotAfter: notAfter},
	}
	findings := (pkiCAExpiryWarning{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for 90-day healthy CA, got %d", len(findings))
	}
}

func TestPKICAExpiry_30Days_IsWarn(t *testing.T) {
	notAfter := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	snap := &collector.Snapshot{
		CAMetadata: &config.CAMetadata{Fingerprint: "abc", NotAfter: notAfter},
	}
	findings := (pkiCAExpiryWarning{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for 30-day CA, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN for 30-day expiry, got %v", findings[0].Severity)
	}
}

func TestPKICAExpiry_7Days_IsError(t *testing.T) {
	notAfter := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	snap := &collector.Snapshot{
		CAMetadata: &config.CAMetadata{Fingerprint: "abc", NotAfter: notAfter},
	}
	findings := (pkiCAExpiryWarning{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for 7-day CA, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR for 7-day expiry, got %v", findings[0].Severity)
	}
}

func TestPKICAExpiry_Expired_IsCritical(t *testing.T) {
	notAfter := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	snap := &collector.Snapshot{
		CAMetadata: &config.CAMetadata{Fingerprint: "abc", NotAfter: notAfter},
	}
	findings := (pkiCAExpiryWarning{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for expired CA, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL for expired CA, got %v", findings[0].Severity)
	}
}

func TestPKICAExpiry_NilCAMetadata_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{CAMetadata: nil}
	findings := (pkiCAExpiryWarning{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil CA, got %d", len(findings))
	}
}

func TestPKICAExpiry_SummaryContainsDaysLeft(t *testing.T) {
	notAfter := time.Now().Add(45 * 24 * time.Hour).UTC().Format(time.RFC3339)
	snap := &collector.Snapshot{
		CAMetadata: &config.CAMetadata{Fingerprint: "abc", NotAfter: notAfter, Generation: 3},
	}
	findings := (pkiCAExpiryWarning{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for 45-day CA, got %d", len(findings))
	}
	// Summary should mention the remaining days.
	if !containsStr(findings[0].Summary, "45") && !containsStr(findings[0].Summary, "44") {
		t.Errorf("expected days-left in summary, got: %s", findings[0].Summary)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || fmt.Sprintf("%s", s) != "" && len(sub) > 0 &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
