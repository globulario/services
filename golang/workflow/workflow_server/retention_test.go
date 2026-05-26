package main

import (
	"testing"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

func TestIsTerminalStatusInt(t *testing.T) {
	terminal := []workflowpb.RunStatus{
		workflowpb.RunStatus_RUN_STATUS_SUCCEEDED,
		workflowpb.RunStatus_RUN_STATUS_FAILED,
		workflowpb.RunStatus_RUN_STATUS_CANCELED,
		workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK,
		workflowpb.RunStatus_RUN_STATUS_SUPERSEDED,
	}
	for _, s := range terminal {
		if !isTerminalStatusInt(int(s)) {
			t.Errorf("expected status %d (%s) to be terminal", s, s)
		}
	}

	active := []workflowpb.RunStatus{
		workflowpb.RunStatus_RUN_STATUS_UNKNOWN,
		workflowpb.RunStatus_RUN_STATUS_PENDING,
		workflowpb.RunStatus_RUN_STATUS_EXECUTING,
		workflowpb.RunStatus_RUN_STATUS_BLOCKED,
		workflowpb.RunStatus_RUN_STATUS_RETRYING,
		workflowpb.RunStatus_RUN_STATUS_DEFERRED,
	}
	for _, s := range active {
		if isTerminalStatusInt(int(s)) {
			t.Errorf("expected status %d (%s) to NOT be terminal", s, s)
		}
	}
}
