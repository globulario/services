package engine

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestDoctorActorHandlersPropagateCorrelationAndRunID — wiring test for
// audit.retention_and_correlation_policy: the workflow engine MUST stamp
// every doctor-facing callback with the workflow run id and a correlation
// id derived from (run, step). The doctor reads these via
// remediation.CorrelationFromContext / WorkflowRunFromContext to populate
// CorrelationID and WorkflowRunID on every RemediationAudit.
func TestDoctorActorHandlersPropagateCorrelationAndRunID(t *testing.T) {
	var seenCorrelation, seenRun string
	cfg := DoctorRemediationConfig{
		ResolveFinding: func(ctx context.Context, findingID string, stepIndex uint32) (*ResolvedFinding, error) {
			seenCorrelation = remediation.CorrelationFromContext(ctx)
			seenRun = remediation.WorkflowRunFromContext(ctx)
			return &ResolvedFinding{
				FindingID:  findingID,
				StepIndex:  stepIndex,
				ActionType: "SYSTEMCTL_RESTART",
				Risk:       "RISK_LOW",
				HasAction:  true,
			}, nil
		},
	}
	h := doctorResolveFinding(cfg)

	req := ActionRequest{
		RunID:   "wf-run-789",
		StepID:  "resolve",
		Actor:   v1alpha1.ActorClusterDoctor,
		Action:  "doctor.resolve_finding",
		With:    map[string]any{"finding_id": "f-corr"},
		Outputs: map[string]any{},
	}
	if _, err := h(context.Background(), req); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if seenRun != "wf-run-789" {
		t.Fatalf("workflow run id not propagated: got %q, want wf-run-789", seenRun)
	}
	if seenCorrelation != "wf-run-789/resolve" {
		t.Fatalf("correlation id: got %q, want wf-run-789/resolve", seenCorrelation)
	}

	// Same wiring on the execute and verify handlers.
	var execRun, verifyRun string
	cfgExec := DoctorRemediationConfig{
		ExecuteRemediation: func(ctx context.Context, _ string, _ uint32, _ string, _ bool) (*ExecutionResult, error) {
			execRun = remediation.WorkflowRunFromContext(ctx)
			return &ExecutionResult{Status: "executed", Executed: true}, nil
		},
		VerifyConvergence: func(ctx context.Context, _, _ string) (*Verification, error) {
			verifyRun = remediation.WorkflowRunFromContext(ctx)
			return &Verification{Converged: true}, nil
		},
	}
	execReq := ActionRequest{
		RunID:   "wf-run-789",
		StepID:  "execute",
		With:    map[string]any{"finding_id": "f-corr", "step_index": uint32(0), "dry_run": true},
		Outputs: map[string]any{},
	}
	if _, err := doctorExecuteRemediation(cfgExec)(context.Background(), execReq); err != nil {
		t.Fatalf("execute handler: %v", err)
	}
	if execRun != "wf-run-789" {
		t.Fatalf("execute did not propagate run id: got %q", execRun)
	}

	verifyReq := ActionRequest{
		RunID:   "wf-run-789",
		StepID:  "verify",
		With:    map[string]any{"finding_id": "f-corr"},
		Outputs: map[string]any{},
	}
	if _, err := doctorVerifyConvergence(cfgExec)(context.Background(), verifyReq); err != nil {
		t.Fatalf("verify handler: %v", err)
	}
	if verifyRun != "wf-run-789" {
		t.Fatalf("verify did not propagate run id: got %q", verifyRun)
	}
}
