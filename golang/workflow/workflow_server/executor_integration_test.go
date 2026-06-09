package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─── Track A.2: Full ExecuteWorkflow round-trip ──────────────────────────────

// TestExecuteWorkflowFullRoundTrip verifies the complete centralized execution
// path: WorkflowService loads a definition, runs the engine, dispatches steps
// to a mock actor via gRPC callback, auto-records steps, and returns a
// coherent response.
//
// This is the Track A.2 integration test from test/strategy.md.
func TestExecuteWorkflowFullRoundTrip(t *testing.T) {
	// Start mock actor server that handles a simple 2-step workflow.
	mock := &roundTripMockActor{}
	actorLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	actorGS := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(actorGS, mock)
	go actorGS.Serve(actorLis)
	defer actorGS.Stop()

	actorAddr := actorLis.Addr().String()

	// Build a minimal workflow definition in-memory (no MinIO needed).
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata: v1alpha1.WorkflowMetadata{
			Name: "test.round_trip",
		},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "step_one",
					Actor:  "test-actor",
					Action: "test.action_one",
					Export: &v1alpha1.ScalarString{Raw: "result_one"},
				},
				{
					ID:        "step_two",
					Actor:     "test-actor",
					Action:    "test.action_two",
					DependsOn: []string{"step_one"},
					Export:    &v1alpha1.ScalarString{Raw: "result_two"},
				},
			},
		},
	}

	// Build a router with fallback to the mock actor.
	router := engine.NewRouter()
	dispatcher := newActorDispatcher(map[string]string{
		"test-actor": actorAddr,
	})
	defer dispatcher.close()

	// Override the dispatcher's client to use insecure (no TLS in tests).
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, actorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial mock actor: %v", err)
	}
	dispatcher.conns["test-actor"] = conn
	dispatcher.clients["test-actor"] = workflowpb.NewWorkflowActorServiceClient(conn)

	router.RegisterFallback(v1alpha1.ActorType("test-actor"), dispatcher.makeHandler("test-actor"))

	// Create a minimal engine and execute directly (no ScyllaDB needed).
	var recordedSteps []string
	eng := &engine.Engine{
		Router: router,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			recordedSteps = append(recordedSteps, fmt.Sprintf("%s:%s", step.ID, step.Status))
		},
	}

	inputs := map[string]any{
		"test_input": "hello",
	}
	run, execErr := eng.Execute(ctx, def, inputs)

	// ── Verify execution succeeded ──────────────────────────────────────
	if execErr != nil {
		t.Fatalf("execute failed: %v", execErr)
	}
	if run.Status != engine.RunSucceeded {
		t.Fatalf("run status = %s, want SUCCEEDED", run.Status)
	}

	// ── Verify both steps executed in order ──────────────────────────────
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 actor calls, got %d", len(mock.calls))
	}
	if mock.calls[0].Action != "test.action_one" {
		t.Errorf("call[0] action = %s, want test.action_one", mock.calls[0].Action)
	}
	if mock.calls[1].Action != "test.action_two" {
		t.Errorf("call[1] action = %s, want test.action_two", mock.calls[1].Action)
	}

	// ── Verify step outputs propagated ──────────────────────────────────
	if run.Outputs["result_one"] == nil {
		t.Error("result_one not in run outputs")
	}
	if run.Outputs["result_two"] == nil {
		t.Error("result_two not in run outputs")
	}

	// ── Verify OnStepDone callbacks fired ────────────────────────────────
	if len(recordedSteps) != 2 {
		t.Fatalf("expected 2 recorded steps, got %d", len(recordedSteps))
	}
	if recordedSteps[0] != "step_one:SUCCEEDED" {
		t.Errorf("recorded[0] = %s, want step_one:SUCCEEDED", recordedSteps[0])
	}
	if recordedSteps[1] != "step_two:SUCCEEDED" {
		t.Errorf("recorded[1] = %s, want step_two:SUCCEEDED", recordedSteps[1])
	}
}

// TestExecuteWorkflowActorRejectsUnknownAction verifies that when an actor
// receives an unknown action, the workflow fails cleanly with an explicit
// error — no silent no-ops.
func TestExecuteWorkflowActorRejectsUnknownAction(t *testing.T) {
	mock := &roundTripMockActor{}
	actorLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	actorGS := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(actorGS, mock)
	go actorGS.Serve(actorLis)
	defer actorGS.Stop()

	actorAddr := actorLis.Addr().String()

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.unknown_action"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "bad_step",
					Actor:  "test-actor",
					Action: "completely.unknown.action",
				},
			},
		},
	}

	router := engine.NewRouter()
	dispatcher := newActorDispatcher(map[string]string{"test-actor": actorAddr})
	defer dispatcher.close()

	ctx := context.Background()
	conn, _ := grpc.DialContext(ctx, actorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	dispatcher.conns["test-actor"] = conn
	dispatcher.clients["test-actor"] = workflowpb.NewWorkflowActorServiceClient(conn)
	router.RegisterFallback(v1alpha1.ActorType("test-actor"), dispatcher.makeHandler("test-actor"))

	eng := &engine.Engine{Router: router}
	run, err := eng.Execute(ctx, def, nil)

	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
	if run == nil || run.Status != engine.RunFailed {
		t.Errorf("run status should be FAILED")
	}
}

// TestExecuteWorkflowCallbackInputsPropagated verifies that workflow inputs
// and accumulated step outputs are correctly serialized and sent to the
// actor callback.
func TestExecuteWorkflowCallbackInputsPropagated(t *testing.T) {
	mock := &roundTripMockActor{}
	actorLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	actorGS := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(actorGS, mock)
	go actorGS.Serve(actorLis)
	defer actorGS.Stop()

	actorAddr := actorLis.Addr().String()

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.inputs"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "check_inputs",
					Actor:  "test-actor",
					Action: "test.check_inputs",
				},
			},
		},
	}

	router := engine.NewRouter()
	dispatcher := newActorDispatcher(map[string]string{"test-actor": actorAddr})
	defer dispatcher.close()

	ctx := context.Background()
	conn, _ := grpc.DialContext(ctx, actorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	dispatcher.conns["test-actor"] = conn
	dispatcher.clients["test-actor"] = workflowpb.NewWorkflowActorServiceClient(conn)
	router.RegisterFallback(v1alpha1.ActorType("test-actor"), dispatcher.makeHandler("test-actor"))

	eng := &engine.Engine{Router: router}
	inputs := map[string]any{
		"cluster_id": "test-cluster",
		"node_id":    "node-42",
	}
	_, err = eng.Execute(ctx, def, inputs)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	// Verify the actor received the inputs.
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}
	if mock.calls[0].Inputs["cluster_id"] != "test-cluster" {
		t.Errorf("cluster_id = %v, want test-cluster", mock.calls[0].Inputs["cluster_id"])
	}
	if mock.calls[0].Inputs["node_id"] != "node-42" {
		t.Errorf("node_id = %v, want node-42", mock.calls[0].Inputs["node_id"])
	}
}

// ─── Mock actor ──────────────────────────────────────────────────────────────

// TestExecuteWorkflow_StartRunCommitFailure_NoSideEffects is the commit-first
// ratchet (T3) for meta.state_mutations_must_be_durably_committed_before_side_effects.
// If the durable run record (StartRun) cannot commit, ExecuteWorkflow MUST refuse
// to dispatch any actor side effect and MUST return an error so the caller retries
// — never execute uncommitted (the "lost intent / untraceable change" bug class).
// Regression for the pre-2026-06-09 executor, which logged the failure and
// proceeded anyway ("Non-fatal: execution proceeds even if recording fails").
func TestExecuteWorkflow_StartRunCommitFailure_NoSideEffects(t *testing.T) {
	// Minimal one-step workflow, served through the EtcdFetcher seam (no etcd).
	const defYAML = `apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: test.commit_first
spec:
  strategy:
    mode: dag
  steps:
    - id: only_step
      actor: cluster-controller
      action: test.side_effect
`
	prevFetcher := v1alpha1.EtcdFetcher
	v1alpha1.EtcdFetcher = func(string) ([]byte, error) { return []byte(defYAML), nil }
	defer func() { v1alpha1.EtcdFetcher = prevFetcher }()

	// A real mock actor that counts dispatches — it must stay at zero.
	mock := &roundTripMockActor{}
	actorLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	actorGS := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(actorGS, mock)
	go actorGS.Serve(actorLis)
	defer actorGS.Stop()

	// Force the durable run-start commit to fail (simulate a Scylla write error).
	prevStart := startRunFn
	startRunFn = func(_ *server, _ context.Context, _ *workflowpb.StartRunRequest) (*workflowpb.WorkflowRun, error) {
		return nil, fmt.Errorf("simulated scylla write failure")
	}
	defer func() { startRunFn = prevStart }()

	srv := &server{} // nil depHealth/leaseManager/deferStore → pre-StartRun gates pass

	resp, err := srv.ExecuteWorkflow(context.Background(), &workflowpb.ExecuteWorkflowRequest{
		WorkflowName:   "test.commit_first",
		ClusterId:      "test-cluster",
		ActorEndpoints: map[string]string{"cluster-controller": actorLis.Addr().String()},
		// CorrelationId intentionally empty so the defer-state checks are skipped.
	})

	// 1. Must fail loudly so the caller retries the whole dispatch.
	if err == nil {
		t.Fatalf("ExecuteWorkflow must return an error when StartRun commit fails, got resp=%v", resp)
	}
	if !strings.Contains(err.Error(), "run-start commit failed") {
		t.Errorf("error must explain the commit refusal, got: %v", err)
	}
	// 2. The decisive assertion: NO actor side effect was dispatched.
	mock.mu.Lock()
	n := len(mock.calls)
	mock.mu.Unlock()
	if n != 0 {
		t.Errorf("StartRun commit failed but %d actor side effect(s) were dispatched — uncommitted execution", n)
	}
}

type roundTripMockActor struct {
	workflowpb.UnimplementedWorkflowActorServiceServer
	mu    sync.Mutex
	calls []roundTripCall
}

type roundTripCall struct {
	Action string
	Inputs map[string]any
}

func (m *roundTripMockActor) ExecuteAction(_ context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
	var inputs map[string]any
	if req.InputsJson != "" {
		json.Unmarshal([]byte(req.InputsJson), &inputs)
	}

	m.mu.Lock()
	m.calls = append(m.calls, roundTripCall{Action: req.Action, Inputs: inputs})
	m.mu.Unlock()

	switch req.Action {
	case "test.action_one":
		output := map[string]any{"step": "one", "value": 42}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "test.action_two":
		output := map[string]any{"step": "two", "final": true}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "test.check_inputs":
		// Echo back — proves inputs were received.
		return &workflowpb.ExecuteActionResponse{Ok: true}, nil

	default:
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("unknown action: %s", req.Action),
		}, nil
	}
}
