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
	// Serve returns ErrServerStopped on the deferred Stop below; not actionable.
	go func() { _ = actorGS.Serve(actorLis) }()
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
	//nolint:staticcheck // DialContext+WithBlock is retained deliberately: this test
	// must not proceed until the in-process mock actor is accepting connections, and
	// grpc.NewClient has no blocking equivalent. Replacing it would make the test
	// race the server goroutine.
	conn, err := grpc.DialContext(ctx, actorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck // see above: blocking dial is required here
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
	// Serve returns ErrServerStopped from the deferred Stop below; not actionable.
	go func() { _ = actorGS.Serve(actorLis) }()
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
	go func() { _ = actorGS.Serve(actorLis) }()
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

// TestWorkflowServiceActorRoutesToFallback_NotLocalNoop is the regression test
// for the bug where cluster.reconcile's per-item remediation dispatch
// (actor: workflow-service, action: workflow.start_child /
// workflow.wait_child_terminal) always silently "succeeded" without ever
// running the child workflow.
//
// Root cause: the router this test builds mirrors ExecuteWorkflow/ResumeRun's
// production router construction. It used to also call
// engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{})
// — a local, exact-match (actor, action) registration. Router.Resolve checks
// exact handlers before falling back to RegisterFallback, so that no-op
// registration always won, even though a real "workflow-service" endpoint was
// supplied via ActorEndpoints (as cluster-controller always does). The no-op
// handlers (workflowStartChild/workflowWaitChildTerminal with a nil config)
// return a hardcoded {"run_id":"mock-run"} / {"status":"SUCCEEDED"} without
// ever making a network call — so mark_item_terminal always observed a fake
// SUCCEEDED and cluster.reconcile's missing_package/version_drift remediation
// never actually dispatched release.apply.package to node-agent.
//
// This test builds the router the way production now does — fallback only,
// no local workflow-service registration — and asserts that
// workflow.start_child / workflow.wait_child_terminal are dispatched over the
// wire to the registered actor endpoint, not answered locally.
func TestWorkflowServiceActorRoutesToFallback_NotLocalNoop(t *testing.T) {
	mock := &roundTripMockActor{}
	actorLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	actorGS := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(actorGS, mock)
	go func() { _ = actorGS.Serve(actorLis) }()
	defer actorGS.Stop()

	actorAddr := actorLis.Addr().String()

	def := &v1alpha1.WorkflowDefinition{
		APIVersion: "workflow.globular.io/v1alpha1",
		Kind:       "WorkflowDefinition",
		Metadata:   v1alpha1.WorkflowMetadata{Name: "test.workflow_service_dispatch"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Steps: []v1alpha1.WorkflowStepSpec{
				{
					ID:     "start_child",
					Actor:  "workflow-service",
					Action: "workflow.start_child",
					Export: &v1alpha1.ScalarString{Raw: "child_run"},
				},
				{
					ID:        "wait_child",
					Actor:     "workflow-service",
					Action:    "workflow.wait_child_terminal",
					DependsOn: []string{"start_child"},
					Export:    &v1alpha1.ScalarString{Raw: "child_result"},
				},
			},
		},
	}

	// Mirrors production: only RegisterFallback for each ActorEndpoints entry.
	// Deliberately does NOT call RegisterWorkflowServiceActions — that is the
	// bug this test guards against reintroducing.
	router := engine.NewRouter()
	dispatcher := newActorDispatcher(map[string]string{"workflow-service": actorAddr})
	defer dispatcher.close()

	ctx := context.Background()
	//nolint:staticcheck // DialContext+WithBlock is deliberate: the test must not
	// proceed until the in-process mock actor accepts connections, and grpc.NewClient
	// has no blocking equivalent. Replacing it would race the server goroutine.
	conn, err := grpc.DialContext(ctx, actorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial mock actor: %v", err)
	}
	dispatcher.conns["workflow-service"] = conn
	dispatcher.clients["workflow-service"] = workflowpb.NewWorkflowActorServiceClient(conn)
	router.RegisterFallback(v1alpha1.ActorType("workflow-service"), dispatcher.makeHandler("workflow-service"))

	eng := &engine.Engine{Router: router}
	run, execErr := eng.Execute(ctx, def, nil)
	if execErr != nil {
		t.Fatalf("execute failed: %v", execErr)
	}
	if run.Status != engine.RunSucceeded {
		t.Fatalf("run status = %s, want SUCCEEDED", run.Status)
	}

	// The decisive assertion: both actions were dispatched over the wire to
	// the mock actor. A shadowing local no-op would leave mock.calls empty.
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 real actor dispatches, got %d — workflow-service actions were answered locally (no-op shadowing the fallback)", len(mock.calls))
	}
	if mock.calls[0].Action != "workflow.start_child" {
		t.Errorf("call[0] action = %s, want workflow.start_child", mock.calls[0].Action)
	}
	if mock.calls[1].Action != "workflow.wait_child_terminal" {
		t.Errorf("call[1] action = %s, want workflow.wait_child_terminal", mock.calls[1].Action)
	}

	// The no-op would have produced run_id="mock-run". A real dispatch
	// produces the mock actor's distinctive run_id, proving the network path
	// was exercised rather than the local hardcoded stub.
	childRun, _ := run.Outputs["child_run"].(map[string]any)
	if childRun == nil || childRun["run_id"] != "real-child-run-id" {
		t.Errorf("child_run = %v, want run_id=real-child-run-id (got the no-op's mock-run instead?)", childRun)
	}
	childResult, _ := run.Outputs["child_result"].(map[string]any)
	if childResult == nil || childResult["status"] != "REAL_DISPATCH_SUCCEEDED" {
		t.Errorf("child_result = %v, want status=REAL_DISPATCH_SUCCEEDED (got the no-op's generic SUCCEEDED instead?)", childResult)
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
	go func() { _ = actorGS.Serve(actorLis) }()
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

	case "workflow.start_child":
		// Distinctive run_id — proves a REAL dispatch, not the local no-op's
		// hardcoded "mock-run".
		output := map[string]any{"run_id": "real-child-run-id", "workflow_name": req.Action}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "workflow.wait_child_terminal":
		// Distinctive status — proves a REAL dispatch, not the local no-op's
		// hardcoded generic "SUCCEEDED".
		output := map[string]any{"status": "REAL_DISPATCH_SUCCEEDED", "run_id": "real-child-run-id"}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	default:
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("unknown action: %s", req.Action),
		}, nil
	}
}
