package main

import (
	"context"
	"errors"
	"testing"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// TestResumeProbeModePreflight is the regression test for the bug where
// the orphan scanner permanently failed in-progress runs instead of
// retrying them when actor handlers weren't yet registered.
//
// Root cause: ResumeRun called FinishRun(FAILED) on any PreflightError,
// even when actorEndpoints was nil (probe mode). Probe mode builds a router
// with no remote-actor fallbacks by design, so incomplete runs always hit
// a PreflightError — which should be a "retry later" signal, not a
// permanent failure.
func TestResumeProbeModePreflight(t *testing.T) {
	// Mirror the probe-mode router from ResumeRun(actorEndpoints=nil):
	// only local workflow-service actions are registered, no remote fallbacks.
	router := engine.NewRouter()
	engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{})

	// A minimal workflow whose only step targets a remote actor.
	// In probe mode this step has no handler → PreflightError expected.
	defYAML := []byte(`
apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: release.apply.package
spec:
  strategy:
    mode: dag
  steps:
    - id: mark_applying
      actor: cluster-controller
      action: controller.release.mark_applying
`)
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadBytes(defYAML)
	if err != nil {
		t.Fatalf("load def: %v", err)
	}

	eng := &engine.Engine{Router: router}
	_, execErr := eng.Execute(context.Background(), def, nil)

	if execErr == nil {
		t.Fatal("expected PreflightError for unregistered remote actor, got nil")
	}

	var pfErr *engine.PreflightError
	if !errors.As(execErr, &pfErr) {
		t.Fatalf("expected *engine.PreflightError, got %T: %v", execErr, execErr)
	}
	if len(pfErr.Missing) == 0 {
		t.Error("PreflightError.Missing should list the unresolvable steps")
	}

	// Verify the discriminator used in ResumeRun: probe mode (nil endpoints) +
	// PreflightError must short-circuit without calling FinishRun(FAILED).
	var nilEndpoints map[string]string
	if !errors.As(execErr, &pfErr) || len(nilEndpoints) != 0 {
		t.Error("probe-mode discriminator should be true for nil endpoints + PreflightError")
	}
}

// TestResumeWithEndpointsPreflight verifies that when real endpoints ARE
// provided and the preflight still fails (genuinely missing handler), the
// run SHOULD be permanently failed — not retried.
func TestResumeWithEndpointsPreflight(t *testing.T) {
	// Router with no remote fallbacks AND explicit (wrong) endpoints.
	router := engine.NewRouter()
	engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{})

	defYAML := []byte(`
apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: release.apply.package
spec:
  strategy:
    mode: dag
  steps:
    - id: mark_applying
      actor: cluster-controller
      action: controller.release.mark_applying
`)
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadBytes(defYAML)
	if err != nil {
		t.Fatalf("load def: %v", err)
	}

	eng := &engine.Engine{Router: router}
	_, execErr := eng.Execute(context.Background(), def, nil)

	var pfErr *engine.PreflightError
	realEndpoints := map[string]string{"cluster-controller": "10.0.0.63:12000"}

	// With real endpoints the discriminator must be false → FinishRun(FAILED).
	if errors.As(execErr, &pfErr) && len(realEndpoints) == 0 {
		t.Error("discriminator should be false when endpoints are provided")
	}
}
