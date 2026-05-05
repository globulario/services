package rules

import (
	"errors"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestWorkflowServiceReachable_NoErrors verifies that when the snapshot has
// no DataErrors the invariant produces no findings.
func TestWorkflowServiceReachable_NoErrors(t *testing.T) {
	snap := &collector.Snapshot{}
	inv := workflowServiceReachable{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("no DataErrors → no findings, got %d", len(got))
	}
}

// TestWorkflowServiceReachable_NonWorkflowError verifies that errors from
// other services (cluster_controller, node_agent, etcd) do not trigger the
// workflow.service_unavailable finding.
func TestWorkflowServiceReachable_NonWorkflowError(t *testing.T) {
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{Service: "cluster_controller", RPC: "ListNodes", Err: errors.New("connection refused")},
			{Service: "etcd", RPC: "Get", Err: fmt.Errorf("Unavailable: etcd down")},
		},
	}
	inv := workflowServiceReachable{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("non-workflow errors → no findings, got %d", len(got))
	}
}

// TestWorkflowServiceReachable_WorkflowConnectionRefused verifies that a
// "connection refused" error on a workflow RPC fires the
// workflow.service_unavailable finding at SEVERITY_ERROR.
func TestWorkflowServiceReachable_WorkflowConnectionRefused(t *testing.T) {
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{Service: "workflow", RPC: "ListStepOutcomes", Err: fmt.Errorf("rpc error: connection refused")},
		},
	}
	inv := workflowServiceReachable{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "workflow.service_unavailable" {
		t.Errorf("InvariantID = %q, want workflow.service_unavailable", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("Severity = %v, want SEVERITY_ERROR", f.Severity)
	}
}

// TestWorkflowServiceReachable_WorkflowUnavailable verifies that a gRPC
// Unavailable message from the workflow service fires the finding.
func TestWorkflowServiceReachable_WorkflowUnavailable(t *testing.T) {
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{Service: "workflow", RPC: "ListRuns(BLOCKED)", Err: fmt.Errorf("rpc error: code = Unavailable desc = transport closing")},
		},
	}
	inv := workflowServiceReachable{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].InvariantID != "workflow.service_unavailable" {
		t.Errorf("InvariantID = %q, want workflow.service_unavailable", findings[0].InvariantID)
	}
}

// TestWorkflowServiceReachable_NonUnavailableWorkflowError verifies that
// workflow RPC errors that are NOT Unavailable (e.g. PermissionDenied,
// NotFound) do not fire the workflow.service_unavailable finding — those
// indicate the service IS running but rejected the request.
func TestWorkflowServiceReachable_NonUnavailableWorkflowError(t *testing.T) {
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{Service: "workflow", RPC: "ListStepOutcomes", Err: fmt.Errorf("rpc error: code = PermissionDenied desc = access denied")},
		},
	}
	inv := workflowServiceReachable{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("PermissionDenied → not workflow.service_unavailable, got %d findings", len(got))
	}
}

// TestWorkflowServiceReachable_OnlyFirstUnavailable verifies that the
// invariant returns exactly one finding even when multiple workflow RPCs
// all fail with Unavailable (avoids duplicate findings per RPC).
func TestWorkflowServiceReachable_OnlyFirstUnavailable(t *testing.T) {
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{Service: "workflow", RPC: "ListStepOutcomes", Err: fmt.Errorf("connection refused")},
			{Service: "workflow", RPC: "ListWorkflowSummaries", Err: fmt.Errorf("Unavailable: transport closing")},
			{Service: "workflow", RPC: "ListDriftUnresolved", Err: fmt.Errorf("Unavailable: transport closing")},
		},
	}
	inv := workflowServiceReachable{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Errorf("expected exactly 1 finding for repeated Unavailable errors, got %d", len(findings))
	}
}

// TestWorkflowServiceReachable_RemedationStepsPresent verifies that the
// finding includes remediation steps so operators know what to do.
func TestWorkflowServiceReachable_RemedationStepsPresent(t *testing.T) {
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{Service: "workflow", RPC: "ListStepOutcomes", Err: fmt.Errorf("connection refused")},
		},
	}
	inv := workflowServiceReachable{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if len(findings[0].Remediation) == 0 {
		t.Error("finding must include at least one remediation step")
	}
}

// TestIsWorkflowUnavailableErr covers the detection function directly.
func TestIsWorkflowUnavailableErr(t *testing.T) {
	cases := []struct {
		err       error
		want      bool
		desc      string
	}{
		{nil, false, "nil"},
		{fmt.Errorf("connection refused"), true, "connection refused"},
		{fmt.Errorf("rpc error: code = Unavailable desc = transport closing"), true, "Unavailable in message"},
		{fmt.Errorf("no route to host"), true, "no route to host"},
		{fmt.Errorf("transport: Error while dialing dial tcp"), true, "dial error"},
		{fmt.Errorf("rpc error: code = PermissionDenied desc = access denied"), false, "PermissionDenied"},
		{fmt.Errorf("rpc error: code = NotFound"), false, "NotFound"},
		{fmt.Errorf("context deadline exceeded"), false, "deadline exceeded (not an Unavailable)"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := isWorkflowUnavailableErr(tc.err)
			if got != tc.want {
				t.Errorf("isWorkflowUnavailableErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
