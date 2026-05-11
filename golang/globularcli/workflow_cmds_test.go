package main

import "testing"

func TestWorkflowEndpoint_UsesExplicitFlag(t *testing.T) {
	oldWorkflowAddr := workflowAddr
	oldController := rootCfg.controllerAddr
	t.Cleanup(func() {
		workflowAddr = oldWorkflowAddr
		rootCfg.controllerAddr = oldController
	})

	workflowAddr = "10.0.0.63:10004"
	rootCfg.controllerAddr = "globular.internal"

	got := workflowEndpoint()
	if got != "10.0.0.63:10004" {
		t.Fatalf("workflowEndpoint() = %q, want explicit workflow addr", got)
	}
}

