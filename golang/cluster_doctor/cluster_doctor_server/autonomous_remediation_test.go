package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
)

// Proof suite for the autonomous remediation path.
//
// Everything here is deterministic and infra-free: no cluster, no workflow
// server, no behavioral memory. The pieces under test were shaped to make that
// possible — classifyRemediationRun is a pure function over a response, the
// binding table exposes outstanding(), and the resolver takes an explicit mode.

// ── helpers ────────────────────────────────────────────────────────────────

func outputsJSON(t *testing.T, m map[string]any) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal outputs: %v", err)
	}
	return string(b)
}

func autonomousFinding() rules.Finding {
	return rules.Finding{
		FindingID:       "f-auto-1",
		InvariantID:     "node.systemd.units_running",
		EntityRef:       "node-1/globular-log.service",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Remediation: []*cluster_doctorpb.RemediationStep{{
			Order: 1,
			Action: &cluster_doctorpb.RemediationAction{
				ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
				Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
				Params:     map[string]string{"node_id": "node-1", "unit": "globular-log.service"},
			},
		}},
	}
}

// fakeWorkflowClient records every ExecuteWorkflow call and replays a scripted
// response, so a test can assert what the doctor SENT as well as how it
// classified what came back.
type fakeWorkflowClient struct {
	workflowpb.WorkflowServiceClient
	requests []*workflowpb.ExecuteWorkflowRequest
	respond  func(req *workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error)
}

func (f *fakeWorkflowClient) ExecuteWorkflow(_ context.Context, in *workflowpb.ExecuteWorkflowRequest, _ ...grpc.CallOption) (*workflowpb.ExecuteWorkflowResponse, error) {
	f.requests = append(f.requests, in)
	if f.respond != nil {
		return f.respond(in)
	}
	return &workflowpb.ExecuteWorkflowResponse{RunId: in.GetRunId(), Status: "SUCCEEDED"}, nil
}

func autonomousServer(fake *fakeWorkflowClient) *ClusterDoctorServer {
	srv := &ClusterDoctorServer{
		workflowClient: fake,
		clusterID:      "c1",
		cfg:            &clusterdoctorConfig{Port: 10300},
		// Injected so the test never consults real service discovery, and so
		// the advertised callback is a ROUTABLE address rather than loopback.
		actorEndpointResolver: func() string { return "10.0.0.63:10300" },
	}
	srv.isAuthoritative.Store(true)
	return srv
}

// ── classification matrix ──────────────────────────────────────────────────

// TestClassifyRemediationRun covers every disposition the healer can observe,
// including the two that P1-A proved could be written but never reached.
func TestClassifyRemediationRun(t *testing.T) {
	const runID = "run-classify"

	for _, tc := range []struct {
		name            string
		outputs         map[string]any
		status          string
		dryRun          bool
		wantDisposition rules.DispatchDisposition
		wantCheckID     string
		wantGovStatus   string
		why             string
	}{
		{
			name: "converged",
			outputs: map[string]any{
				"execution_result": map[string]any{"executed": true, "audit_id": "rem-1",
					"action_check_id": "chk-1", "dispatched_at": "2026-08-04T12:00:00Z"},
				"verification": map[string]any{"converged": true},
			},
			status:          "SUCCEEDED",
			wantDisposition: rules.DispatchConverged,
			wantCheckID:     "chk-1",
			why:             "ran and the finding cleared — the only auto-fix",
		},
		{
			name: "executed but not converged",
			outputs: map[string]any{
				"execution_result": map[string]any{"executed": true, "audit_id": "rem-2", "action_check_id": "chk-2"},
				"verification":     map[string]any{"converged": false},
			},
			status:          "SUCCEEDED",
			wantDisposition: rules.DispatchExecutedNotConverged,
			wantCheckID:     "chk-2",
			why:             "a restart that left the invariant violated is not a fix",
		},
		{
			name: "executed but unverified",
			outputs: map[string]any{
				"execution_result": map[string]any{"executed": true, "audit_id": "rem-3", "action_check_id": "chk-3"},
			},
			status:          "FAILED",
			wantDisposition: rules.DispatchExecutedUnverified,
			wantCheckID:     "chk-3",
			why:             "a real mutation with unknown convergence must not be guessed either way",
		},
		{
			name: "converged without a dispatch instant cannot be an auto-fix",
			outputs: map[string]any{
				"execution_result": map[string]any{"executed": true, "audit_id": "rem-5"},
				"verification":     map[string]any{"converged": true},
			},
			status:          "SUCCEEDED",
			wantDisposition: rules.DispatchExecutedUnverified,
			why: "the finding cleared, but with no dispatch instant the reading cannot be placed " +
				"after the action — counting it as AutoFixed would attribute a success we cannot prove",
		},
		{
			name: "governed refusal",
			outputs: map[string]any{
				"dispatch_result": map[string]any{
					"disposition":     engine.DispositionRefused,
					"action_check_id": "chk-refused",
					"status":          "needs_evidence",
					"executed":        false,
				},
			},
			status:          "FAILED",
			wantDisposition: rules.DispatchRefused,
			wantCheckID:     "chk-refused",
			wantGovStatus:   "needs_evidence",
			why:             "P1-A: refusal must be classifiable from structured output that survives the failed step",
		},
		{
			name: "governance unavailable",
			outputs: map[string]any{
				"dispatch_result": map[string]any{
					"disposition":            engine.DispositionExecutionFailed,
					"governance_unavailable": true,
					"executed":               false,
				},
			},
			status:          "FAILED",
			wantDisposition: rules.DispatchExecutionFailed,
			why:             "an unreachable governor is infrastructure failure, not a decision",
		},
		{
			name:            "execution failed with no outputs",
			outputs:         map[string]any{},
			status:          "FAILED",
			wantDisposition: rules.DispatchExecutionFailed,
			why:             "a run with no account of itself is a failure, never a success",
		},
		{
			name:            "dry run",
			outputs:         map[string]any{"execution_result": map[string]any{"executed": false}},
			status:          "SUCCEEDED",
			dryRun:          true,
			wantDisposition: rules.DispatchProposed,
			why:             "a rehearsal is not an outcome",
		},
		{
			name: "ungoverned attempt carries an empty check id honestly",
			outputs: map[string]any{
				"execution_result": map[string]any{"executed": true, "audit_id": "rem-4",
					"dispatched_at": "2026-08-04T12:00:00Z"},
				"verification": map[string]any{"converged": true},
			},
			status:          "SUCCEEDED",
			wantDisposition: rules.DispatchConverged,
			wantCheckID:     "",
			why:             "no principle governed this action; inventing an id would forge a decision",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &workflowpb.ExecuteWorkflowResponse{
				RunId: runID, Status: tc.status, OutputsJson: outputsJSON(t, tc.outputs),
			}
			got := classifyRemediationRun(runID, resp, tc.dryRun)
			if got.Disposition != tc.wantDisposition {
				t.Errorf("disposition = %q, want %q — %s", got.Disposition, tc.wantDisposition, tc.why)
			}
			if got.ActionCheckID != tc.wantCheckID {
				t.Errorf("action_check_id = %q, want %q", got.ActionCheckID, tc.wantCheckID)
			}
			if tc.wantGovStatus != "" && got.GovernanceStatus != tc.wantGovStatus {
				t.Errorf("governance_status = %q, want %q", got.GovernanceStatus, tc.wantGovStatus)
			}
			if got.WorkflowRunID != runID {
				t.Errorf("workflow_run_id = %q, want %q", got.WorkflowRunID, runID)
			}
		})
	}
}

// TestClassifyRemediationRun_ConflictingActionCheckIDsFailClassification proves
// two decision ids in one run are refused rather than silently reconciled.
//
// Two non-empty different ids mean the run contains two governance decisions
// for one action — the duplication this architecture exists to prevent.
// Choosing either would attribute the attempt to a decision that may not have
// authorized it.
func TestClassifyRemediationRun_ConflictingActionCheckIDsFailClassification(t *testing.T) {
	resp := &workflowpb.ExecuteWorkflowResponse{
		RunId: "run-conflict", Status: "SUCCEEDED",
		OutputsJson: outputsJSON(t, map[string]any{
			"dispatch_result":  map[string]any{"action_check_id": "chk-A"},
			"execution_result": map[string]any{"executed": true, "action_check_id": "chk-B"},
			"verification":     map[string]any{"converged": true},
		}),
	}
	got := classifyRemediationRun("run-conflict", resp, false)
	if got.Disposition != rules.DispatchExecutionFailed {
		t.Errorf("disposition = %q, want %q — conflicting decision ids must fail classification, "+
			"not select one", got.Disposition, rules.DispatchExecutionFailed)
	}
	if got.Err == nil || !strings.Contains(got.Err.Error(), "conflicting action_check_ids") {
		t.Errorf("error must name the conflict; got %v", got.Err)
	}
}

// ── confirmed run identity ─────────────────────────────────────────────────

// TestRunAutonomousRemediation_RunIdentity proves P1-C and the run/correlation
// split: a WorkflowRunID is only ever a CONFIRMED run.
func TestRunAutonomousRemediation_RunIdentity(t *testing.T) {
	f := autonomousFinding()

	t.Run("confirmed run id is used", func(t *testing.T) {
		fake := &fakeWorkflowClient{}
		srv := autonomousServer(fake)
		res, err := srv.runAutonomousRemediation(context.Background(), f, 0, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fake.requests) != 1 {
			t.Fatalf("workflow started %d times, want exactly 1", len(fake.requests))
		}
		if res.WorkflowRunID != fake.requests[0].GetRunId() {
			t.Errorf("run id = %q, want the confirmed returned id %q",
				res.WorkflowRunID, fake.requests[0].GetRunId())
		}
	})

	t.Run("rpc failure claims no run id", func(t *testing.T) {
		fake := &fakeWorkflowClient{respond: func(*workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return nil, fmt.Errorf("transport failure")
		}}
		srv := autonomousServer(fake)
		res, err := srv.runAutonomousRemediation(context.Background(), f, 0, false)
		if err == nil {
			t.Fatal("a transport failure must be an error")
		}
		if res.WorkflowRunID != "" {
			t.Errorf("run id = %q, want empty — after a transport failure the commit is "+
				"UNCONFIRMED and the proposed id is a request, not a receipt", res.WorkflowRunID)
		}
	})

	t.Run("empty returned run id fails", func(t *testing.T) {
		fake := &fakeWorkflowClient{respond: func(*workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return &workflowpb.ExecuteWorkflowResponse{RunId: "", Status: "SUCCEEDED"}, nil
		}}
		srv := autonomousServer(fake)
		if _, err := srv.runAutonomousRemediation(context.Background(), f, 0, false); err == nil {
			t.Error("a response naming no run proves nothing was committed")
		}
	})

	t.Run("mismatched returned run id fails", func(t *testing.T) {
		fake := &fakeWorkflowClient{respond: func(*workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return &workflowpb.ExecuteWorkflowResponse{RunId: "some-other-run", Status: "SUCCEEDED"}, nil
		}}
		srv := autonomousServer(fake)
		if _, err := srv.runAutonomousRemediation(context.Background(), f, 0, false); err == nil {
			t.Error("a different run id means the executed run is not the run our finding was bound to")
		}
	})

	t.Run("repeated attempts get distinct run ids and one stable correlation", func(t *testing.T) {
		fake := &fakeWorkflowClient{}
		srv := autonomousServer(fake)
		for i := 0; i < 3; i++ {
			if _, err := srv.runAutonomousRemediation(context.Background(), f, 0, false); err != nil {
				t.Fatalf("attempt %d: %v", i, err)
			}
		}
		seen := map[string]bool{}
		correlations := map[string]bool{}
		for _, r := range fake.requests {
			if seen[r.GetRunId()] {
				t.Errorf("run id %q reused — repeated repairs must be distinct runs, or the second "+
					"is refused by the run lease as already owned", r.GetRunId())
			}
			seen[r.GetRunId()] = true
			correlations[r.GetCorrelationId()] = true
		}
		if len(correlations) != 1 {
			t.Errorf("correlation ids = %v, want exactly 1 stable value joining the attempts", correlations)
		}
	})

	t.Run("autonomous runs declare the binding-required contract", func(t *testing.T) {
		fake := &fakeWorkflowClient{}
		srv := autonomousServer(fake)
		if _, err := srv.runAutonomousRemediation(context.Background(), f, 0, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var inputs map[string]any
		if err := json.Unmarshal([]byte(fake.requests[0].GetInputsJson()), &inputs); err != nil {
			t.Fatalf("inputs: %v", err)
		}
		if inputs["finding_binding_mode"] != engine.FindingBindingAutonomousRequired {
			t.Errorf("finding_binding_mode = %v, want %q — the resolution contract must be explicit, "+
				"never inferred from whether a binding row happens to exist",
				inputs["finding_binding_mode"], engine.FindingBindingAutonomousRequired)
		}
	})
}

// TestRunAutonomousRemediation_FailsClosedWithoutWorkflowService proves there
// is no direct-execution fallback.
func TestRunAutonomousRemediation_FailsClosedWithoutWorkflowService(t *testing.T) {
	srv := &ClusterDoctorServer{clusterID: "c1", cfg: &clusterdoctorConfig{Port: 10300},
		actorEndpointResolver: func() string { return "10.0.0.63:10300" }} // no workflow client
	srv.isAuthoritative.Store(true)
	res, err := srv.runAutonomousRemediation(context.Background(), autonomousFinding(), 0, false)
	if err == nil {
		t.Fatal("an unconfigured Workflow Service must fail closed")
	}
	if res.Executed {
		t.Error("nothing may execute outside a governed run")
	}
	if res.WorkflowRunID != "" {
		t.Error("no run was started, so no run id may be claimed")
	}
}

// TestRunAutonomousRemediation_BindingReleasedOnEveryTerminalPath proves a
// binding never outlives its run. One that did would leak and, worse, could
// offer a stale finding to a later attempt.
func TestRunAutonomousRemediation_BindingReleasedOnEveryTerminalPath(t *testing.T) {
	f := autonomousFinding()

	for _, tc := range []struct {
		name    string
		respond func(*workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error)
	}{
		{"success", nil},
		{"rpc failure", func(*workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return nil, fmt.Errorf("boom")
		}},
		{"empty run id", func(*workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return &workflowpb.ExecuteWorkflowResponse{Status: "FAILED"}, nil
		}},
		{"refusal", func(in *workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return &workflowpb.ExecuteWorkflowResponse{RunId: in.GetRunId(), Status: "FAILED",
				OutputsJson: `{"dispatch_result":{"disposition":"REFUSED"}}`}, nil
		}},
		{"verifier error", func(in *workflowpb.ExecuteWorkflowRequest) (*workflowpb.ExecuteWorkflowResponse, error) {
			return &workflowpb.ExecuteWorkflowResponse{RunId: in.GetRunId(), Status: "FAILED",
				OutputsJson: `{"execution_result":{"executed":true}}`}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := autonomousServer(&fakeWorkflowClient{respond: tc.respond})
			_, _ = srv.runAutonomousRemediation(context.Background(), f, 0, false)
			if n := srv.runFindings.outstanding(); n != 0 {
				t.Errorf("%d binding(s) outstanding after a terminal run, want 0 — a binding that "+
					"outlives its run leaks and can offer a stale finding to a later attempt", n)
			}
		})
	}

	t.Run("cancelled context", func(t *testing.T) {
		srv := autonomousServer(&fakeWorkflowClient{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _ = srv.runAutonomousRemediation(ctx, f, 0, false)
		if n := srv.runFindings.outstanding(); n != 0 {
			t.Errorf("%d binding(s) outstanding after cancellation, want 0", n)
		}
	})
}
