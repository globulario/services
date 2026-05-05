package main

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestRunInstallPackage_NativeDepMissing_BlocksBeforeInstall(t *testing.T) {
	origDeps := packageNativeDeps
	origWriter := writeConvergenceResult
	t.Cleanup(func() {
		packageNativeDeps = origDeps
		writeConvergenceResult = origWriter
	})

	const pkg = "pr8-native-dep-block-test"
	packageNativeDeps = map[string][]string{
		pkg: {"libdefinitelymissing-pr8.so.999"},
	}

	results := make(chan *installed_state.ConvergenceResultV1, 1)
	writeConvergenceResult = func(ctx context.Context, r *installed_state.ConvergenceResultV1) error {
		results <- r
		return nil
	}

	srv := &NodeAgentServer{
		nodeID: "node-pr8",
	}
	req := &node_agentpb.RunWorkflowRequest{
		WorkflowName: "install-package",
		Inputs: map[string]string{
			"package_name": pkg,
			"kind":         "SERVICE",
			"version":      "1.2.3",
			"build_id":     "b-123",
			"workflow_id":  "wf-pr8",
		},
	}

	resp, err := srv.runInstallPackage(context.Background(), req)
	if err != nil {
		t.Fatalf("runInstallPackage returned error: %v", err)
	}
	if resp.GetStatus() != "SUCCEEDED" {
		t.Fatalf("status=%s, want SUCCEEDED (blocked preflight path)", resp.GetStatus())
	}
	if resp.GetStepsSucceeded() != 1 {
		t.Fatalf("steps_succeeded=%d, want 1", resp.GetStepsSucceeded())
	}

	select {
	case r := <-results:
		if r.Outcome != installed_state.OutcomeBlockedMissingNativeDep {
			t.Fatalf("outcome=%s, want %s", r.Outcome, installed_state.OutcomeBlockedMissingNativeDep)
		}
		if r.ReasonCode != "missing_native_dep" {
			t.Fatalf("reason_code=%q, want missing_native_dep", r.ReasonCode)
		}
		if r.Evidence["missing_lib"] != "libdefinitelymissing-pr8.so.999" {
			t.Fatalf("missing_lib=%q, want libdefinitelymissing-pr8.so.999", r.Evidence["missing_lib"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for convergence result write")
	}
}

