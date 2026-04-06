package engine

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestPreCompletedStepsSkipped verifies that the engine skips steps
// marked as pre-completed (from a prior execution) and only executes
// remaining pending steps.
func TestPreCompletedStepsSkipped(t *testing.T) {
	executedSteps := make(map[string]bool)

	router := NewRouter()
	router.Register(v1alpha1.ActorType("test"), "test.action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executedSteps[req.StepID] = true
		return &ActionResult{OK: true, Output: map[string]any{"done": true}}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "step_a", Actor: "test", Action: "test.action"},
				{ID: "step_b", Actor: "test", Action: "test.action", DependsOn: []string{"step_a"}},
				{ID: "step_c", Actor: "test", Action: "test.action", DependsOn: []string{"step_b"}},
			},
		},
	}

	eng := &Engine{
		Router: router,
		// step_a already completed in prior execution.
		PreCompleted: map[string]StepStatus{
			"step_a": StepSucceeded,
		},
	}

	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Fatalf("run status = %s, want SUCCEEDED", run.Status)
	}

	// step_a should NOT have been executed (pre-completed).
	if executedSteps["step_a"] {
		t.Error("step_a was re-executed but should have been skipped (pre-completed)")
	}
	// step_b and step_c should have been executed.
	if !executedSteps["step_b"] {
		t.Error("step_b was not executed")
	}
	if !executedSteps["step_c"] {
		t.Error("step_c was not executed")
	}
}

// TestPreCompletedFailedStepBlocksDownstream verifies that a step
// pre-completed as FAILED correctly blocks dependent steps.
func TestPreCompletedFailedStepBlocksDownstream(t *testing.T) {
	executedSteps := make(map[string]bool)

	router := NewRouter()
	router.Register(v1alpha1.ActorType("test"), "test.action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executedSteps[req.StepID] = true
		return &ActionResult{OK: true}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.fail"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "step_a", Actor: "test", Action: "test.action"},
				{ID: "step_b", Actor: "test", Action: "test.action", DependsOn: []string{"step_a"}},
			},
		},
	}

	eng := &Engine{
		Router: router,
		// step_a failed in prior execution — step_b should not run.
		PreCompleted: map[string]StepStatus{
			"step_a": StepFailed,
		},
	}

	run, err := eng.Execute(context.Background(), def, nil)

	// Run should fail because step_a is FAILED.
	if err == nil {
		t.Fatal("expected error when pre-completed step is FAILED")
	}
	if run.Status != RunFailed {
		t.Errorf("run status = %s, want FAILED", run.Status)
	}
	if executedSteps["step_b"] {
		t.Error("step_b should not execute when step_a is FAILED")
	}
}

// TestPreCompletedAllStepsDone verifies that if all steps are pre-completed
// as SUCCEEDED, the run completes immediately without executing anything.
func TestPreCompletedAllStepsDone(t *testing.T) {
	executedSteps := make(map[string]bool)

	router := NewRouter()
	router.Register(v1alpha1.ActorType("test"), "test.action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executedSteps[req.StepID] = true
		return &ActionResult{OK: true}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.noop"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "step_a", Actor: "test", Action: "test.action"},
				{ID: "step_b", Actor: "test", Action: "test.action", DependsOn: []string{"step_a"}},
			},
		},
	}

	eng := &Engine{
		Router: router,
		PreCompleted: map[string]StepStatus{
			"step_a": StepSucceeded,
			"step_b": StepSucceeded,
		},
	}

	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Errorf("run status = %s, want SUCCEEDED", run.Status)
	}
	if len(executedSteps) > 0 {
		t.Errorf("no steps should execute when all pre-completed, but %d executed", len(executedSteps))
	}
}

// TestPreCompletedSkippedStepSatisfiesDeps verifies that a SKIPPED
// pre-completed step counts as a satisfied dependency.
func TestPreCompletedSkippedStepSatisfiesDeps(t *testing.T) {
	executedSteps := make(map[string]bool)

	router := NewRouter()
	router.Register(v1alpha1.ActorType("test"), "test.action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		executedSteps[req.StepID] = true
		return &ActionResult{OK: true}, nil
	})

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.resume.skip"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "step_a", Actor: "test", Action: "test.action"},
				{ID: "step_b", Actor: "test", Action: "test.action", DependsOn: []string{"step_a"}},
			},
		},
	}

	eng := &Engine{
		Router: router,
		PreCompleted: map[string]StepStatus{
			"step_a": StepSkipped, // skipped satisfies deps
		},
	}

	run, err := eng.Execute(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !executedSteps["step_b"] {
		t.Error("step_b should execute when step_a is SKIPPED")
	}
	if run.Status != RunSucceeded {
		t.Errorf("run status = %s, want SUCCEEDED", run.Status)
	}
}
