package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─── Mock actor service ──────────────────────────────────────────────────────

// mockActorServer implements WorkflowActorServiceServer for testing.
type mockActorServer struct {
	workflowpb.UnimplementedWorkflowActorServiceServer
	calls []mockActorCall
}

type mockActorCall struct {
	Action string
	With   map[string]any
}

func (m *mockActorServer) ExecuteAction(_ context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
	var with map[string]any
	if req.WithJson != "" {
		json.Unmarshal([]byte(req.WithJson), &with)
	}
	m.calls = append(m.calls, mockActorCall{Action: req.Action, With: with})

	// Route by action to simulate real actor behavior.
	switch req.Action {
	case "doctor.resolve_finding":
		output := map[string]any{
			"finding_id":  "test-finding-001",
			"step_index":  0,
			"node_id":     "node-1",
			"action_type": "SYSTEMCTL_RESTART",
			"risk":        "RISK_LOW",
			"idempotent":  true,
			"description": "restart globular-dns.service",
			"has_action":  true,
		}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "doctor.assess_risk":
		output := map[string]any{
			"auto_executable":   true,
			"requires_approval": false,
			"reason":            "",
		}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "doctor.require_approval":
		output := map[string]any{"gated": false}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "doctor.execute_remediation":
		output := map[string]any{
			"audit_id": "audit-001",
			"status":   "executed",
			"executed": true,
			"output":   "restarted",
			"reason":   "",
		}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "doctor.verify_convergence":
		output := map[string]any{
			"converged":             true,
			"finding_still_present": false,
			"remaining_related":     0,
		}
		b, _ := json.Marshal(output)
		return &workflowpb.ExecuteActionResponse{Ok: true, OutputJson: string(b)}, nil

	case "doctor.mark_failed":
		return &workflowpb.ExecuteActionResponse{Ok: true}, nil

	default:
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("unknown action: %s", req.Action),
		}, nil
	}
}

// ─── Dispatcher tests ────────────────────────────────────────────────────────

// TestActorDispatcherRemoteCallHappyPath starts a mock actor gRPC server
// and verifies the dispatcher can call it, serialize/deserialize correctly,
// and get results back.
func TestActorDispatcherRemoteCallHappyPath(t *testing.T) {
	// Start mock actor server.
	mock := &mockActorServer{}
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(gs, mock)
	go gs.Serve(lis)
	defer gs.Stop()

	addr := lis.Addr().String()

	// Create dispatcher with the mock endpoint.
	d := &actorDispatcher{
		endpoints: map[string]string{"test-actor": addr},
		conns:     make(map[string]*grpc.ClientConn),
		clients:   make(map[string]workflowpb.WorkflowActorServiceClient),
	}
	defer d.close()

	// Override getClient to use insecure transport (no TLS in unit tests).
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial mock: %v", err)
	}
	d.conns["test-actor"] = conn
	d.clients["test-actor"] = workflowpb.NewWorkflowActorServiceClient(conn)

	// Call the dispatcher handler.
	handler := d.makeHandler("test-actor")

	// Import engine types for the test.
	result, err := handler(ctx, makeTestActionRequest(
		"doctor.resolve_finding",
		map[string]any{"finding_id": "test-finding-001", "step_index": 0},
	))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK=true, got false: %s", result.Message)
	}
	if result.Output["finding_id"] != "test-finding-001" {
		t.Errorf("expected finding_id=test-finding-001, got %v", result.Output["finding_id"])
	}
	if result.Output["action_type"] != "SYSTEMCTL_RESTART" {
		t.Errorf("expected action_type=SYSTEMCTL_RESTART, got %v", result.Output["action_type"])
	}

	// Verify the mock received the call.
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}
	if mock.calls[0].Action != "doctor.resolve_finding" {
		t.Errorf("expected action doctor.resolve_finding, got %s", mock.calls[0].Action)
	}
}

// TestActorDispatcherRejectsUnknownAction verifies that the dispatcher
// propagates rejection from the actor when an unknown action is sent.
func TestActorDispatcherRejectsUnknownAction(t *testing.T) {
	mock := &mockActorServer{}
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	workflowpb.RegisterWorkflowActorServiceServer(gs, mock)
	go gs.Serve(lis)
	defer gs.Stop()

	addr := lis.Addr().String()

	d := &actorDispatcher{
		endpoints: map[string]string{"test-actor": addr},
		conns:     make(map[string]*grpc.ClientConn),
		clients:   make(map[string]workflowpb.WorkflowActorServiceClient),
	}
	defer d.close()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial mock: %v", err)
	}
	d.conns["test-actor"] = conn
	d.clients["test-actor"] = workflowpb.NewWorkflowActorServiceClient(conn)

	handler := d.makeHandler("test-actor")

	_, err = handler(ctx, makeTestActionRequest("totally.bogus.action", nil))
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
}

// TestActorDispatcherNoEndpoint verifies the dispatcher fails cleanly
// when no endpoint is configured for an actor.
func TestActorDispatcherNoEndpoint(t *testing.T) {
	d := &actorDispatcher{
		endpoints: map[string]string{},
		conns:     make(map[string]*grpc.ClientConn),
		clients:   make(map[string]workflowpb.WorkflowActorServiceClient),
	}

	handler := d.makeHandler("missing-actor")
	_, err := handler(context.Background(), makeTestActionRequest("some.action", nil))
	if err == nil {
		t.Fatal("expected error for missing endpoint, got nil")
	}
}

// ─── Test helpers ────────────────────────────────────────────────────────────

// makeTestActionRequest is a helper — import engine.ActionRequest would create
// a circular dependency in package main tests, so we build them inline.
// Since executor.go and the test are in the same package, we use engine types directly.
func makeTestActionRequest(action string, with map[string]any) engine.ActionRequest {
	return engine.ActionRequest{
		RunID:   "test-run-001",
		StepID:  "test-step-001",
		Action:  action,
		With:    with,
		Inputs:  map[string]any{"finding_id": "test-finding-001"},
		Outputs: map[string]any{},
	}
}
