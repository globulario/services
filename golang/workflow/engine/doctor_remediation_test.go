package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// fakeDoctor records calls and returns scripted responses. Used to exercise
// the remediate.doctor.finding workflow without a real cluster-doctor.
type fakeDoctor struct {
	resolved   *ResolvedFinding
	execResult *ExecutionResult
	verify     *Verification
	resolveErr error
	execErr    error
	verifyErr  error

	resolveCalls int
	execCalls    int
	verifyCalls  int
	failedCalls  int
}

func (f *fakeDoctor) config() DoctorRemediationConfig {
	return DoctorRemediationConfig{
		ResolveFinding: func(ctx context.Context, findingID string, stepIndex uint32) (*ResolvedFinding, error) {
			f.resolveCalls++
			if f.resolveErr != nil {
				return nil, f.resolveErr
			}
			rf := *f.resolved
			rf.FindingID = findingID
			rf.StepIndex = stepIndex
			return &rf, nil
		},
		ExecuteRemediation: func(ctx context.Context, findingID string, stepIndex uint32, approvalToken string, dryRun bool) (*ExecutionResult, error) {
			f.execCalls++
			if f.execErr != nil {
				return nil, f.execErr
			}
			return f.execResult, nil
		},
		VerifyConvergence: func(ctx context.Context, findingID, nodeID string) (*Verification, error) {
			f.verifyCalls++
			if f.verifyErr != nil {
				return nil, f.verifyErr
			}
			return f.verify, nil
		},
		MarkFailed: func(ctx context.Context, findingID string) error {
			f.failedCalls++
			return nil
		},
	}
}

func loadDoctorFindingDef(t *testing.T) *v1alpha1.WorkflowDefinition {
	t.Helper()
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile("../definitions/remediate.doctor.finding.yaml")
	if err != nil {
		t.Fatalf("load remediate.doctor.finding.yaml: %v", err)
	}
	return def
}

func runDoctorWorkflow(t *testing.T, cfg DoctorRemediationConfig, inputs map[string]any) *Run {
	t.Helper()
	router := NewRouter()
	RegisterDoctorRemediationActions(router, cfg)
	eng := &Engine{Router: router}
	def := loadDoctorFindingDef(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run, _ := eng.Execute(ctx, def, inputs)
	// Engine returns non-nil err whenever a step fails; tests read run.Status
	// directly, so we do not t.Fatalf here. A nil run is still a bug.
	if run == nil {
		t.Fatalf("execute returned nil run")
	}
	return run
}

// Happy path: LOW risk restart, auto-executable, no approval needed, verifies clean.
func TestDoctorFindingWorkflow_HappyPath(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID:      "node-1",
			ActionType:  "SYSTEMCTL_RESTART",
			Risk:        "RISK_LOW",
			Idempotent:  true,
			Description: "restart globular-file.service on node-1",
			HasAction:   true,
		},
		execResult: &ExecutionResult{
			AuditID:  "rem-123",
			Status:   "executed",
			Executed: true,
			Output:   "restart globular-file.service on node-1: state=active",
		},
		verify: &Verification{Converged: true},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{
		"finding_id": "abc123",
		"step_index": 0,
		"dry_run":    false,
	})
	if run.Status != RunSucceeded {
		t.Fatalf("expected SUCCEEDED, got %s", run.Status)
	}
	if fake.resolveCalls != 1 || fake.execCalls != 1 || fake.verifyCalls != 1 {
		t.Fatalf("call counts: resolve=%d exec=%d verify=%d, want 1/1/1",
			fake.resolveCalls, fake.execCalls, fake.verifyCalls)
	}
	// Verify outputs propagated through the pipeline.
	if _, ok := run.Outputs["resolved_finding"].(map[string]any); !ok {
		t.Errorf("resolved_finding missing from outputs")
	}
	if _, ok := run.Outputs["risk_assessment"].(map[string]any); !ok {
		t.Errorf("risk_assessment missing from outputs")
	}
	if _, ok := run.Outputs["execution_result"].(map[string]any); !ok {
		t.Errorf("execution_result missing from outputs")
	}
	if _, ok := run.Outputs["verification"].(map[string]any); !ok {
		t.Errorf("verification missing from outputs")
	}
}

// MEDIUM risk with no approval token → require_approval step runs & fails workflow.
func TestDoctorFindingWorkflow_MissingApproval(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID: "node-1", ActionType: "PACKAGE_REINSTALL",
			Risk: "RISK_MEDIUM", HasAction: true,
		},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{
		"finding_id":     "abc123",
		"approval_token": "",
	})
	if run.Status == RunSucceeded {
		t.Fatalf("expected workflow to fail, got SUCCEEDED")
	}
	if fake.execCalls != 0 {
		t.Errorf("ExecuteRemediation should not have been called without approval, got %d calls", fake.execCalls)
	}
	if fake.verifyCalls != 0 {
		t.Errorf("VerifyConvergence should not have been called after approval gate, got %d", fake.verifyCalls)
	}
}

// HIGH risk WITH approval token → all five stages run.
func TestDoctorFindingWorkflow_HighRiskApproved(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID: "node-1", ActionType: "SYSTEMCTL_RESTART",
			Risk: "RISK_HIGH", HasAction: true,
		},
		execResult: &ExecutionResult{Status: "executed", Executed: true, AuditID: "rem-42"},
		verify:     &Verification{Converged: true},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{
		"finding_id":     "abc123",
		"approval_token": "signed-token-xyz",
	})
	if run.Status != RunSucceeded {
		t.Fatalf("expected SUCCEEDED with approval, got %s", run.Status)
	}
	if fake.execCalls != 1 {
		t.Errorf("ExecuteRemediation call count = %d, want 1", fake.execCalls)
	}
}

// Dry-run skips verify_convergence by YAML when clause.
func TestDoctorFindingWorkflow_DryRunSkipsVerify(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID: "node-1", ActionType: "SYSTEMCTL_RESTART",
			Risk: "RISK_LOW", HasAction: true,
		},
		execResult: &ExecutionResult{Status: "dry_run_ok", Executed: false, AuditID: "rem-dry"},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{
		"finding_id": "abc123",
		"dry_run":    true,
	})
	if run.Status != RunSucceeded {
		t.Fatalf("expected SUCCEEDED on dry-run, got %s", run.Status)
	}
	if fake.verifyCalls != 0 {
		t.Errorf("VerifyConvergence should be skipped on dry_run, got %d calls", fake.verifyCalls)
	}
}

// Finding with no structured action → resolve step fails cleanly.
func TestDoctorFindingWorkflow_NoStructuredAction(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{HasAction: false, NodeID: "node-1"},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{"finding_id": "abc"})
	if run.Status == RunSucceeded {
		t.Fatalf("expected failure for action-less finding, got SUCCEEDED")
	}
	if fake.execCalls != 0 {
		t.Errorf("ExecuteRemediation called despite no action")
	}
}

// ExecuteRemediation rejected → verify_convergence should not run, onFailure fires.
func TestDoctorFindingWorkflow_ExecuteRejected(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID: "node-1", ActionType: "SYSTEMCTL_RESTART",
			Risk: "RISK_LOW", HasAction: true,
		},
		execResult: &ExecutionResult{
			Status:   "rejected",
			Executed: false,
			Reason:   "systemctl refuses unit: not Globular-managed",
		},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{"finding_id": "abc"})
	if run.Status == RunSucceeded {
		t.Fatalf("expected failure on execute rejection, got SUCCEEDED")
	}
	if fake.verifyCalls != 0 {
		t.Errorf("VerifyConvergence must not run after rejection, got %d", fake.verifyCalls)
	}
	if fake.failedCalls != 1 {
		t.Errorf("MarkFailed should fire via onFailure, got %d calls", fake.failedCalls)
	}
}

// Verify returns not-converged → workflow fails, onFailure fires.
func TestDoctorFindingWorkflow_NotConverged(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID: "node-1", ActionType: "SYSTEMCTL_RESTART",
			Risk: "RISK_LOW", HasAction: true,
		},
		execResult: &ExecutionResult{Status: "executed", Executed: true, AuditID: "rem-1"},
		verify:     &Verification{Converged: false, FindingStillPresent: true},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{"finding_id": "abc"})
	if run.Status == RunSucceeded {
		t.Fatalf("expected failure on non-convergence, got SUCCEEDED")
	}
	if fake.failedCalls != 1 {
		t.Errorf("MarkFailed should fire via onFailure, got %d", fake.failedCalls)
	}
}

// Pipeline step order is resolve → assess → execute → verify. require_approval
// only runs when guard trips. Verify the step ID set matches the spec.
func TestDoctorFindingWorkflow_PipelineSteps(t *testing.T) {
	fake := &fakeDoctor{
		resolved: &ResolvedFinding{
			NodeID: "node-1", ActionType: "SYSTEMCTL_RESTART",
			Risk: "RISK_LOW", HasAction: true,
		},
		execResult: &ExecutionResult{Status: "executed", Executed: true, AuditID: "rem-1"},
		verify:     &Verification{Converged: true},
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{"finding_id": "abc"})
	if run.Status != RunSucceeded {
		t.Fatalf("expected SUCCEEDED, got %s", run.Status)
	}
	wantSteps := []string{"resolve_finding", "assess_risk", "execute_remediation", "verify_convergence"}
	for _, s := range wantSteps {
		if _, ok := run.Steps[s]; !ok {
			t.Errorf("step %q missing from run, have: %v", s, stepKeys(run.Steps))
		}
	}
	// Pipeline must never name or invoke "plan" anything.
	for id := range run.Steps {
		if strings.Contains(id, "plan") {
			t.Errorf("step %q references plan vocabulary", id)
		}
	}
}

func stepKeys(m map[string]*StepState) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Sanity: resolver failure propagates as workflow failure.
// (Schema-level required-field enforcement is out of scope here; the
// engine substitutes $.finding_id literally if unresolved, and the
// injected resolver is responsible for erroring on unknown IDs.)
func TestDoctorFindingWorkflow_ResolverError(t *testing.T) {
	fake := &fakeDoctor{
		resolveErr: fmt.Errorf("finding xyz not found in last snapshot"),
	}
	run := runDoctorWorkflow(t, fake.config(), map[string]any{"finding_id": "xyz"})
	if run.Status == RunSucceeded {
		t.Fatalf("expected failure when resolver errors")
	}
	if fake.execCalls != 0 {
		t.Errorf("ExecuteRemediation must not be called after resolve failure")
	}
}

// Extra guard: ensure no plan vocabulary leaks into the workflow spec.
func TestDoctorFindingDefinitionNoPlanWords(t *testing.T) {
	def := loadDoctorFindingDef(t)
	badWords := []string{"plan", "compile_plan", "dispatch_plan"}
	for _, s := range def.Spec.Steps {
		idLower := strings.ToLower(s.ID)
		for _, bad := range badWords {
			if strings.Contains(idLower, bad) {
				t.Errorf("step id %q contains plan vocabulary", s.ID)
			}
		}
		if strings.Contains(strings.ToLower(s.Action), "plan") {
			t.Errorf("step action %q contains plan vocabulary", s.Action)
		}
	}
	_ = fmt.Sprint // silence unused in some build configs
}
