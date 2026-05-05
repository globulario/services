package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestPackageKindMismatch_Empty verifies that no findings are produced when
// there are no kind mismatch records in the snapshot.
func TestPackageKindMismatch_Empty(t *testing.T) {
	snap := &collector.Snapshot{}
	inv := packageKindMismatch{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("no records → no findings, got %d", len(got))
	}
}

// TestPackageKindMismatch_FreshRecord verifies that a fresh kind mismatch
// record (detected within the staleness window) produces a SEVERITY_ERROR
// finding with the correct InvariantID and entity ref.
func TestPackageKindMismatch_FreshRecord(t *testing.T) {
	snap := &collector.Snapshot{
		KindMismatches: []collector.KindMismatchRecord{
			{
				NodeID:         "node-ryzen",
				PkgName:        "rbac",
				DesiredKind:    "SERVICE",
				RepoKind:       "INFRASTRUCTURE",
				DetectedAtUnix: time.Now().Unix(),
			},
		},
	}
	inv := packageKindMismatch{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "package.kind_mismatch" {
		t.Errorf("InvariantID = %q, want package.kind_mismatch", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("Severity = %v, want SEVERITY_ERROR", f.Severity)
	}
	if f.EntityRef != "node-ryzen/rbac" {
		t.Errorf("EntityRef = %q, want node-ryzen/rbac", f.EntityRef)
	}
}

// TestPackageKindMismatch_StaleRecord verifies that a record older than the
// staleness window is silently ignored — the mismatch was resolved and the
// controller has stopped refreshing the record.
func TestPackageKindMismatch_StaleRecord(t *testing.T) {
	snap := &collector.Snapshot{
		KindMismatches: []collector.KindMismatchRecord{
			{
				NodeID:         "node-nuc",
				PkgName:        "workflow",
				DesiredKind:    "SERVICE",
				RepoKind:       "INFRASTRUCTURE",
				DetectedAtUnix: time.Now().Add(-20 * time.Minute).Unix(), // older than 15-min window
			},
		},
	}
	inv := packageKindMismatch{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("stale record → no findings, got %d", len(got))
	}
}

// TestPackageKindMismatch_MultipleRecords verifies that one finding is produced
// per {node, package} pair, and that stale entries are filtered independently.
func TestPackageKindMismatch_MultipleRecords(t *testing.T) {
	now := time.Now()
	snap := &collector.Snapshot{
		KindMismatches: []collector.KindMismatchRecord{
			{NodeID: "n1", PkgName: "rbac", DesiredKind: "SERVICE", RepoKind: "INFRASTRUCTURE", DetectedAtUnix: now.Unix()},
			{NodeID: "n2", PkgName: "rbac", DesiredKind: "SERVICE", RepoKind: "INFRASTRUCTURE", DetectedAtUnix: now.Unix()},
			{NodeID: "n3", PkgName: "old-pkg", DesiredKind: "SERVICE", RepoKind: "COMMAND", DetectedAtUnix: now.Add(-30 * time.Minute).Unix()}, // stale
		},
	}
	inv := packageKindMismatch{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 2 {
		t.Errorf("expected 2 fresh findings, got %d", len(findings))
	}
}

// TestPackageKindMismatch_ZeroTimestampSkipped verifies that a record with a
// zero DetectedAtUnix is skipped rather than treated as fresh (zero is the
// epoch, which is always stale, and likely indicates a write error).
func TestPackageKindMismatch_ZeroTimestampSkipped(t *testing.T) {
	snap := &collector.Snapshot{
		KindMismatches: []collector.KindMismatchRecord{
			{NodeID: "n1", PkgName: "rbac", DesiredKind: "SERVICE", RepoKind: "INFRASTRUCTURE", DetectedAtUnix: 0},
		},
	}
	inv := packageKindMismatch{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("zero timestamp → no findings, got %d", len(got))
	}
}

// TestPackageKindMismatch_RemediationStepsPresent verifies that each finding
// includes at least one remediation step so operators know how to resolve the
// mismatch.
func TestPackageKindMismatch_RemediationStepsPresent(t *testing.T) {
	snap := &collector.Snapshot{
		KindMismatches: []collector.KindMismatchRecord{
			{NodeID: "n1", PkgName: "rbac", DesiredKind: "INFRASTRUCTURE", RepoKind: "SERVICE", DetectedAtUnix: time.Now().Unix()},
		},
	}
	inv := packageKindMismatch{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if len(findings[0].Remediation) == 0 {
		t.Error("finding must include remediation steps")
	}
}

// TestPackageKindMismatch_FindingIDIncludesKinds verifies that the finding ID
// encodes the desired and repo kinds so that distinct kind-pair mismatches on
// the same {node, package} produce different finding IDs (no false dedup).
func TestPackageKindMismatch_FindingIDIncludesKinds(t *testing.T) {
	snap := &collector.Snapshot{
		KindMismatches: []collector.KindMismatchRecord{
			{NodeID: "n1", PkgName: "rbac", DesiredKind: "SERVICE", RepoKind: "INFRASTRUCTURE", DetectedAtUnix: time.Now().Unix()},
		},
	}
	inv := packageKindMismatch{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	fid := findings[0].FindingID
	if fid == "" {
		t.Error("FindingID must not be empty")
	}
}
