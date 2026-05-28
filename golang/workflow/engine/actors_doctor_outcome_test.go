package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestVerifyConvergenceEmitsSucceededOutcomeWhenFindingResolved — wiring
// test for workflow.remediation_truth_consistency. After a successful
// execute step + verify-converged outcome, the verify handler writes a
// structured remediation_outcome with status SUCCEEDED and is_success
// true. Callers (CLI/MCP) read this verdict directly.
func TestVerifyConvergenceEmitsSucceededOutcomeWhenFindingResolved(t *testing.T) {
	cfg := DoctorRemediationConfig{
		VerifyConvergence: func(_ context.Context, _, _ string) (*Verification, error) {
			return &Verification{Converged: true}, nil
		},
	}
	req := ActionRequest{
		RunID:  "wf-1",
		StepID: "verify",
		Actor:  v1alpha1.ActorClusterDoctor,
		With:   map[string]any{"finding_id": "f-1"},
		Outputs: map[string]any{
			"execution_result": map[string]any{"executed": true, "reason": ""},
		},
	}
	res, err := doctorVerifyConvergence(cfg)(context.Background(), req)
	if err != nil {
		t.Fatalf("verify handler: %v", err)
	}
	if res == nil || !res.OK {
		t.Fatalf("verify result: %+v", res)
	}
	outcome, ok := req.Outputs["remediation_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("remediation_outcome not written, outputs=%v", req.Outputs)
	}
	if outcome["status"] != string(remediation.StatusSucceeded) {
		t.Fatalf("status: got %v, want SUCCEEDED", outcome["status"])
	}
	if outcome["is_success"] != true {
		t.Fatalf("is_success: got %v, want true", outcome["is_success"])
	}
	if outcome["workflow_run_id"] != "wf-1" {
		t.Fatalf("workflow_run_id not propagated, got %v", outcome["workflow_run_id"])
	}
}

// TestVerifyConvergenceFailsStepWhenFindingStillPresent — the workflow
// truth-consistency contract: dispatch success + verification reporting
// "still present" must fail the step so the workflow run status is not
// reported as success.
func TestVerifyConvergenceFailsStepWhenFindingStillPresent(t *testing.T) {
	cfg := DoctorRemediationConfig{
		VerifyConvergence: func(_ context.Context, _, _ string) (*Verification, error) {
			return &Verification{Converged: false, FindingStillPresent: true}, nil
		},
	}
	req := ActionRequest{
		RunID:  "wf-2",
		StepID: "verify",
		With:   map[string]any{"finding_id": "f-2"},
		Outputs: map[string]any{
			"execution_result": map[string]any{"executed": true, "reason": ""},
		},
	}
	res, err := doctorVerifyConvergence(cfg)(context.Background(), req)
	if err == nil {
		t.Fatal("verify handler must fail when finding still present")
	}
	if !strings.Contains(err.Error(), "still present") {
		t.Fatalf("error must mention 'still present', got: %v", err)
	}
	if res != nil {
		t.Fatalf("result must be nil on failure, got: %+v", res)
	}
	// Outcome must still be written so the workflow run carries the
	// verdict even though the step failed.
	outcome, ok := req.Outputs["remediation_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("remediation_outcome not written on failure, outputs=%v", req.Outputs)
	}
	if outcome["status"] != string(remediation.StatusDegraded) {
		t.Fatalf("status: got %v, want DEGRADED", outcome["status"])
	}
	if outcome["is_success"] != false {
		t.Fatalf("is_success: got %v, want false", outcome["is_success"])
	}
}

// TestVerifyConvergenceMarksFailedWhenExecuteNeverRan — defense in
// depth: if a workflow somehow reaches verify without execute having
// written an execution_result (e.g. skipped step, malformed YAML), the
// outcome must NOT default to PENDING — that would look like "in
// progress" forever. It must be FAILED.
func TestVerifyConvergenceMarksFailedWhenExecuteNeverRan(t *testing.T) {
	cfg := DoctorRemediationConfig{
		VerifyConvergence: func(_ context.Context, _, _ string) (*Verification, error) {
			return &Verification{Converged: true}, nil
		},
	}
	req := ActionRequest{
		RunID:   "wf-3",
		StepID:  "verify",
		With:    map[string]any{"finding_id": "f-3"},
		Outputs: map[string]any{}, // no execution_result
	}
	_, err := doctorVerifyConvergence(cfg)(context.Background(), req)
	if err != nil {
		t.Fatalf("verify handler: %v", err)
	}
	outcome := req.Outputs["remediation_outcome"].(map[string]any)
	if outcome["status"] != string(remediation.StatusFailed) {
		t.Fatalf("missing execute_result must yield FAILED status, got %v", outcome["status"])
	}
	if outcome["is_success"] != false {
		t.Fatal("missing execute_result must NOT be is_success=true")
	}
}
