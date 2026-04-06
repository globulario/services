package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestResumeRetryPolicy verifies that resume_policy=retry re-executes
// the step unconditionally (same as legacy behavior).
func TestResumeRetryPolicy(t *testing.T) {
	executed := false
	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executed = true
		return &ActionResult{OK: true, Output: map[string]any{"done": true}}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.retry"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID: "step_a", Actor: v1alpha1.ActorNodeAgent, Action: "node.action",
					Execution: &v1alpha1.StepExecution{
						Idempotency:  v1alpha1.IdempotencySafeRetry,
						ResumePolicy: v1alpha1.ResumePolicyRetry,
					},
				},
			},
		},
	}

	eng := &Engine{Router: router, IsResume: true}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !executed {
		t.Error("step should have been re-executed with retry policy")
	}
	if run.Status != RunSucceeded {
		t.Errorf("run status = %s, want SUCCEEDED", run.Status)
	}
}

// TestResumeVerifyEffectSkips verifies that when verification proves the
// effect exists, the step is skipped without re-execution.
func TestResumeVerifyEffectSkips(t *testing.T) {
	stepExecuted := false
	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.install", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		stepExecuted = true
		return &ActionResult{OK: true}, nil
	})
	// Verification handler says effect already present.
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_installed", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true, Output: map[string]any{"installed": true}}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.verify_skip"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID: "install", Actor: v1alpha1.ActorNodeAgent, Action: "node.install",
					Execution: &v1alpha1.StepExecution{
						Idempotency:  v1alpha1.IdempotencyVerifyThenContinue,
						ResumePolicy: v1alpha1.ResumePolicyVerifyEffect,
					},
					Verification: &v1alpha1.StepVerification{
						Actor:  v1alpha1.ActorNodeAgent,
						Action: "node.verify_installed",
						Success: v1alpha1.VerifySuccess{
							Expr: "installed == true",
						},
					},
				},
			},
		},
	}

	eng := &Engine{Router: router, IsResume: true}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if stepExecuted {
		t.Error("install step should NOT have executed — verification proved effect exists")
	}
	if run.Status != RunSucceeded {
		t.Errorf("run status = %s, want SUCCEEDED", run.Status)
	}
}

// TestResumeVerifyEffectReexecutes verifies that when verification says
// the effect is absent, the step is re-executed normally.
func TestResumeVerifyEffectReexecutes(t *testing.T) {
	stepExecuted := false
	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.install", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		stepExecuted = true
		return &ActionResult{OK: true}, nil
	})
	// Verification handler says effect is absent.
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_installed", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true, Output: map[string]any{"installed": false}}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.verify_exec"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID: "install", Actor: v1alpha1.ActorNodeAgent, Action: "node.install",
					Execution: &v1alpha1.StepExecution{
						Idempotency:  v1alpha1.IdempotencyVerifyThenContinue,
						ResumePolicy: v1alpha1.ResumePolicyVerifyEffect,
					},
					Verification: &v1alpha1.StepVerification{
						Actor:  v1alpha1.ActorNodeAgent,
						Action: "node.verify_installed",
						Success: v1alpha1.VerifySuccess{
							Expr: "installed == true",
						},
					},
				},
			},
		},
	}

	eng := &Engine{Router: router, IsResume: true}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !stepExecuted {
		t.Error("install step SHOULD have executed — verification proved effect absent")
	}
	if run.Status != RunSucceeded {
		t.Errorf("run status = %s, want SUCCEEDED", run.Status)
	}
}

// TestResumeVerifyInconclusiveManualApproval verifies that inconclusive
// verification + manual_approval idempotency blocks the step.
func TestResumeVerifyInconclusiveManualApproval(t *testing.T) {
	stepExecuted := false
	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.dangerous", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		stepExecuted = true
		return &ActionResult{OK: true}, nil
	})
	// Verification handler returns error → inconclusive.
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_dangerous", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return nil, fmt.Errorf("cannot reach node")
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.inconclusive"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID: "dangerous", Actor: v1alpha1.ActorNodeAgent, Action: "node.dangerous",
					Execution: &v1alpha1.StepExecution{
						Idempotency:  v1alpha1.IdempotencyManualApproval,
						ResumePolicy: v1alpha1.ResumePolicyVerifyEffect,
					},
					Verification: &v1alpha1.StepVerification{
						Actor:  v1alpha1.ActorNodeAgent,
						Action: "node.verify_dangerous",
						Success: v1alpha1.VerifySuccess{
							Expr: "safe == true",
						},
					},
				},
			},
		},
	}

	eng := &Engine{Router: router, IsResume: true}
	run, err := eng.Execute(context.Background(), def, nil)

	if stepExecuted {
		t.Error("dangerous step should NOT have executed — inconclusive + manual_approval → blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked step")
	}
	if run.Status != RunFailed {
		t.Errorf("run status = %s, want FAILED", run.Status)
	}
}

// TestResumeFallbackForLegacySteps verifies that steps without execution
// metadata are re-executed normally (backward compatible).
func TestResumeFallbackForLegacySteps(t *testing.T) {
	executed := false
	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.legacy", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executed = true
		return &ActionResult{OK: true}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.legacy"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID: "legacy", Actor: v1alpha1.ActorNodeAgent, Action: "node.legacy",
					// No Execution metadata — legacy step.
				},
			},
		},
	}

	eng := &Engine{Router: router, IsResume: true}
	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !executed {
		t.Error("legacy step should re-execute normally in resume mode")
	}
	if run.Status != RunSucceeded {
		t.Errorf("run status = %s, want SUCCEEDED", run.Status)
	}
}

// TestResumePolicyFail verifies that resume_policy=fail fails the step
// conservatively.
func TestResumePolicyFail(t *testing.T) {
	executed := false
	router := NewRouter()
	router.Register(v1alpha1.ActorNodeAgent, "node.risky", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executed = true
		return &ActionResult{OK: true}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.fail"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID: "risky", Actor: v1alpha1.ActorNodeAgent, Action: "node.risky",
					Execution: &v1alpha1.StepExecution{
						ResumePolicy: v1alpha1.ResumePolicyFail,
					},
				},
			},
		},
	}

	eng := &Engine{Router: router, IsResume: true}
	run, _ := eng.Execute(context.Background(), def, nil)
	if executed {
		t.Error("step should NOT execute with resume_policy=fail")
	}
	if run.Status != RunFailed {
		t.Errorf("run status = %s, want FAILED", run.Status)
	}
}
