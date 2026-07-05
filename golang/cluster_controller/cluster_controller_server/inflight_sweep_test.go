package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

// TestIsTerminalRunStatus pins which run states the sweep treats as done.
// DEFERRED must NOT be terminal (a deferred run re-runs later).
func TestIsTerminalRunStatus(t *testing.T) {
	terminal := map[workflowpb.RunStatus]bool{
		workflowpb.RunStatus_RUN_STATUS_SUCCEEDED:   true,
		workflowpb.RunStatus_RUN_STATUS_FAILED:      true,
		workflowpb.RunStatus_RUN_STATUS_CANCELED:    true,
		workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK: true,
		workflowpb.RunStatus_RUN_STATUS_SUPERSEDED:  true,
		workflowpb.RunStatus_RUN_STATUS_EXECUTING:   false,
		workflowpb.RunStatus_RUN_STATUS_DEFERRED:    false,
		workflowpb.RunStatus_RUN_STATUS_PENDING:     false,
		workflowpb.RunStatus_RUN_STATUS_UNKNOWN:     false,
	}
	for st, want := range terminal {
		if got := isTerminalRunStatus(st); got != want {
			t.Errorf("isTerminalRunStatus(%s) = %v, want %v", st, got, want)
		}
	}
}

// TestInflightSweepDecide pins the completion policy for a detached release run.
func TestInflightSweepDecide(t *testing.T) {
	const hard = 30 * time.Minute
	young := 1 * time.Minute
	old := 31 * time.Minute

	cases := []struct {
		name       string
		runVisible bool
		status     workflowpb.RunStatus
		age        time.Duration
		want       sweepVerdict
	}{
		{"visible SUCCEEDED → finalize success", true, workflowpb.RunStatus_RUN_STATUS_SUCCEEDED, young, sweepFinalizeSuccess},
		{"visible FAILED → finalize failed", true, workflowpb.RunStatus_RUN_STATUS_FAILED, young, sweepFinalizeFailed},
		{"visible SUPERSEDED → finalize failed", true, workflowpb.RunStatus_RUN_STATUS_SUPERSEDED, young, sweepFinalizeFailed},
		{"visible EXECUTING within timeout → leave", true, workflowpb.RunStatus_RUN_STATUS_EXECUTING, young, sweepLeave},
		{"visible DEFERRED within timeout → leave", true, workflowpb.RunStatus_RUN_STATUS_DEFERRED, young, sweepLeave},
		{"not visible within timeout → leave", false, workflowpb.RunStatus_RUN_STATUS_UNKNOWN, young, sweepLeave},
		{"not visible past timeout → force-fail", false, workflowpb.RunStatus_RUN_STATUS_UNKNOWN, old, sweepForceFailTimeout},
		{"visible EXECUTING past timeout → force-fail", true, workflowpb.RunStatus_RUN_STATUS_EXECUTING, old, sweepForceFailTimeout},
		{"visible SUCCEEDED even past timeout → finalize success", true, workflowpb.RunStatus_RUN_STATUS_SUCCEEDED, old, sweepFinalizeSuccess},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inflightSweepDecide(tc.runVisible, tc.status, tc.age, hard); got != tc.want {
				t.Fatalf("inflightSweepDecide = %d, want %d", got, tc.want)
			}
		})
	}
}
