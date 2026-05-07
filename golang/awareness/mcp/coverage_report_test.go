package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCoverageReport_ComponentWithoutFailureModes verifies that a component
// present in defaultKnownComponents but absent from failure_modes.yaml is
// classified as missing_failure_modes.
func TestCoverageReport_ComponentWithoutFailureModes(t *testing.T) {
	docsDir := t.TempDir()
	// Write a failure_modes.yaml that has one component but NOT "etcd".
	fm := `failure_modes:
  - id: minio.disk_full
    description: MinIO disk full
`
	if err := os.WriteFile(filepath.Join(docsDir, "failure_modes.yaml"), []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}

	fmByComp := loadFailureModesByComponent(docsDir)
	set := buildComponentSet(nil) // all defaults

	// "etcd" is in defaults but not in failure_modes.yaml.
	if _, ok := fmByComp["etcd"]; ok {
		t.Error("expected etcd to have no failure modes in this YAML")
	}
	if !set["etcd"] {
		t.Error("etcd should be in the default component set")
	}

	status := computeComponentCoverageStatus(fmByComp["etcd"], nil)
	if status != "missing_failure_modes" {
		t.Errorf("expected missing_failure_modes, got %q", status)
	}
}

// TestCoverageReport_EtcdCovered verifies that a component with failure modes
// AND test evidence is classified as "covered".
func TestCoverageReport_EtcdCovered(t *testing.T) {
	fms := []string{"etcd.nospace_alarm", "etcd.leader_lost"}
	tests := []string{"TestEtcd_Nospace", "TestEtcd_LeaderLost"}

	status := computeComponentCoverageStatus(fms, tests)
	if status != "covered" {
		t.Errorf("expected covered, got %q", status)
	}
}

// TestCoverageReport_PendingProposalAge verifies that countStaleProposals
// correctly counts proposals older than the SLA threshold.
func TestCoverageReport_PendingProposalAge(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a DRAFT proposal with a very old created_at.
	oldProposal := `proposal:
  id: test-old-proposal
  status: DRAFT
  created_at: "2000-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(proposalsDir, "old.yaml"), []byte(oldProposal), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := countStaleProposals(docsDir, 24.0)
	if stale != 1 {
		t.Errorf("expected 1 stale proposal, got %d", stale)
	}
}

// TestCoverageReport_UnverifiedImplementedGap verifies that buildTopGaps
// includes a P0 gap when unverified implemented gaps are present.
func TestCoverageReport_UnverifiedImplementedGap(t *testing.T) {
	components := []componentCoverageEntry{
		{Component: "etcd", FailureModes: []string{"etcd.nospace"}, Tests: []string{"TestEtcd_Nospace"}, CoverageStatus: "covered"},
	}
	gaps := buildTopGaps(components, t.TempDir(), 24.0, true, 2)

	found := false
	for _, g := range gaps {
		if g.GapID == "coverage.implemented_gaps.unverified" {
			found = true
			if g.Priority != "P0" {
				t.Errorf("expected P0, got %q", g.Priority)
			}
		}
	}
	if !found {
		t.Error("expected coverage.implemented_gaps.unverified gap to be present")
	}
}
