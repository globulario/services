package main

import (
	"os"
	"testing"
)

// TestEmbeddedWorkflowMatchesCanonical guards against drift between the
// canonical YAML in workflow/definitions/ and the bundled copy that is
// compiled into the cluster-doctor binary via go:embed. Keep the files
// in sync; run `cp` from the canonical path if this test fails.
func TestEmbeddedWorkflowMatchesCanonical(t *testing.T) {
	canonical, err := os.ReadFile("../../workflow/definitions/remediate.doctor.finding.yaml")
	if err != nil {
		t.Fatalf("read canonical yaml: %v", err)
	}
	if string(canonical) != string(remediateDoctorFindingYAML) {
		t.Fatalf("embedded workflow_remediate_doctor_finding.yaml drifted from canonical definitions/remediate.doctor.finding.yaml — re-copy and rebuild")
	}
}
