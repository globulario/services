package main

import (
	"testing"
)

// TestRemediationWorkflowNameMatchesCanonical verifies the constant matches
// the canonical definition name in workflow/definitions/.
func TestRemediationWorkflowNameMatchesCanonical(t *testing.T) {
	if remediationWorkflowName != "remediate.doctor.finding" {
		t.Fatalf("remediationWorkflowName = %q, want %q", remediationWorkflowName, "remediate.doctor.finding")
	}
}

// TestRunRemediationWorkflowRequiresWorkflowClient verifies that the
// method returns a clear error when the workflow service is not configured.
func TestRunRemediationWorkflowRequiresWorkflowClient(t *testing.T) {
	s := &ClusterDoctorServer{} // no workflowClient
	_, err := s.RunRemediationWorkflow(nil, "finding-001", 0, "", false)
	if err == nil {
		t.Fatal("expected error when workflowClient is nil")
	}
}

// TestRunRemediationWorkflowRequiresFindingID verifies input validation.
func TestRunRemediationWorkflowRequiresFindingID(t *testing.T) {
	s := &ClusterDoctorServer{}
	_, err := s.RunRemediationWorkflow(nil, "", 0, "", false)
	if err == nil {
		t.Fatal("expected error when finding_id is empty")
	}
}
