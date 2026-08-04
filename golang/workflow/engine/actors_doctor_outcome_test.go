package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
		VerifyConvergence: func(_ context.Context, _, _ string, _ time.Time) (*Verification, error) {
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
	outcome, ok := res.Output["remediation_outcome"].(map[string]any)
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
		VerifyConvergence: func(_ context.Context, _, _ string, _ time.Time) (*Verification, error) {
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
	// A failed step MUST now carry its receipt. Returning nil was the old
	// contract, and it is precisely why a governed refusal became
	// indistinguishable from an executor malfunction once the step crossed the
	// actor RPC: the transport had nothing to serialize. The step still fails —
	// OK is false and err is non-nil — but the verdict travels with it.
	if res == nil {
		t.Fatal("result must carry the failure receipt, not be nil")
	}
	if res.OK {
		t.Error("a non-converged verification must not report OK")
	}
	if _, ok := res.Output["remediation_outcome"]; !ok {
		t.Errorf("failure receipt must include remediation_outcome; got %v", res.Output)
	}
	if _, ok := res.Output["verification"]; !ok {
		t.Errorf("failure receipt must include verification; got %v", res.Output)
	}
	// Outcome must still be written so the workflow run carries the
	// verdict even though the step failed.
	outcome, ok := res.Output["remediation_outcome"].(map[string]any)
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
		VerifyConvergence: func(_ context.Context, _, _ string, _ time.Time) (*Verification, error) {
			return &Verification{Converged: true}, nil
		},
	}
	req := ActionRequest{
		RunID:   "wf-3",
		StepID:  "verify",
		With:    map[string]any{"finding_id": "f-3"},
		Outputs: map[string]any{}, // no execution_result
	}
	res, err := doctorVerifyConvergence(cfg)(context.Background(), req)
	if err != nil {
		t.Fatalf("verify handler: %v", err)
	}
	outcome := res.Output["remediation_outcome"].(map[string]any)
	if outcome["status"] != string(remediation.StatusFailed) {
		t.Fatalf("missing execute_result must yield FAILED status, got %v", outcome["status"])
	}
	if outcome["is_success"] != false {
		t.Fatal("missing execute_result must NOT be is_success=true")
	}
}

// TestEngine_TerminalDeltaSemantics pins when a returned output delta is merged
// into run outputs, across every terminal shape.
//
// The engine is the single writer of run outputs, so these rules ARE the
// contract handlers rely on. Two of them were wrong: the terminal-error path
// merged nothing (so a locally-executed governed refusal lost its receipt while
// the remote one kept it), and the retry-backoff cancellation exit discarded
// the attempt that cancellation had just made terminal.
func TestEngine_TerminalDeltaSemantics(t *testing.T) {
	receipt := func(tag string) map[string]any {
		return map[string]any{"dispatch_result": map[string]any{"disposition": tag}}
	}

	t.Run("terminal result plus error keeps its receipt", func(t *testing.T) {
		router := NewRouter()
		router.RegisterFallback(v1alpha1.ActorClusterDoctor,
			func(context.Context, ActionRequest) (*ActionResult, error) {
				// Deliberately does NOT mutate req.Outputs — the whole point is
				// that a handler communicates only through its return value.
				return &ActionResult{OK: false, Output: receipt("REFUSED"), Message: "refused"},
					fmt.Errorf("blocked by governance")
			})
		run := runOneStep(t, router, 1)
		if run.Status == RunSucceeded {
			t.Fatal("the step must fail")
		}
		if _, ok := run.Outputs["dispatch_result"]; !ok {
			t.Errorf("the terminal attempt's receipt must survive; outputs=%v", run.Outputs)
		}
	})

	t.Run("superseded retry receipt does not leak", func(t *testing.T) {
		attempt := 0
		router := NewRouter()
		router.RegisterFallback(v1alpha1.ActorClusterDoctor,
			func(context.Context, ActionRequest) (*ActionResult, error) {
				attempt++
				if attempt == 1 {
					return &ActionResult{OK: false, Output: receipt("ATTEMPT_A")}, nil
				}
				return &ActionResult{OK: true, Output: map[string]any{"final": "B"}}, nil
			})
		run := runOneStep(t, router, 2)
		if run.Outputs["final"] != "B" {
			t.Errorf("the successful attempt's delta must be merged; outputs=%v", run.Outputs)
		}
		if _, leaked := run.Outputs["dispatch_result"]; leaked {
			t.Error("a superseded retry's receipt must NOT appear in final outputs — it describes " +
				"an attempt the engine did not accept as terminal")
		}
	})
}

// runOneStep executes a one-step definition with the given retry budget.
func runOneStep(t *testing.T, router *Router, attempts int) *Run {
	t.Helper()
	// Built through the loader so the definition satisfies the SAME validation
	// production definitions do; a hand-built struct skips it and the engine
	// refuses to compile.
	yamlDef := "apiVersion: workflow.globular.io/v1alpha1\n" +
		"kind: WorkflowDefinition\n" +
		"metadata:\n  name: terminal.delta.test\n" +
		"spec:\n  strategy:\n    mode: single\n  steps:\n" +
		"    - id: only\n      actor: cluster-doctor\n      action: doctor.anything\n"
	if attempts > 1 {
		yamlDef += "      retry:\n        maxAttempts: " + fmt.Sprint(attempts) + "\n"
	}
	def, loadErr := v1alpha1.NewLoader().LoadBytes([]byte(yamlDef))
	if loadErr != nil {
		t.Fatalf("load test definition: %v", loadErr)
	}
	_ = def
	eng := &Engine{Router: router, RunID: "run-terminal-delta"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run, execErr := eng.Execute(ctx, def, map[string]any{})
	if run == nil {
		t.Fatalf("nil run (err=%v)", execErr)
	}
	return run
}
