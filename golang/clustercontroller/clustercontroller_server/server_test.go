package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	planpb "github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// newTestServer creates a server with a writable temp state path for tests.
func newTestServer(t *testing.T, state *controllerState) *server {
	t.Helper()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	srv := newServer(defaultClusterControllerConfig(), "", statePath, state, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234")
	return srv
}

func TestRestartUnitsForSpecChanges(t *testing.T) {
	httpSpec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "http",
	}
	httpsSpec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
	}

	httpUnits := restartUnitsForSpec(httpSpec)
	if len(httpUnits) == 0 {
		t.Fatalf("expected restart units for http spec")
	}
	if !containsUnit(httpUnits, "globular-dns.service") {
		t.Fatalf("http restart units missing globular-dns.service")
	}
	httpsUnits := restartUnitsForSpec(httpsSpec)
	if len(httpsUnits) <= len(httpUnits) {
		t.Fatalf("expected https spec to include additional units")
	}
	found := false
	for _, unit := range httpsUnits {
		if unit == "globular-storage.service" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("https restart units missing globular-storage.service")
	}
}

func containsUnit(units []string, target string) bool {
	for _, u := range units {
		if u == target {
			return true
		}
	}
	return false
}

func TestCompleteOperationMarksDone(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234")
	opID := "op-complete"
	nodeID := "node-1"
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "running", 10, false, ""))

	_, err := srv.CompleteOperation(context.Background(), &clustercontrollerpb.CompleteOperationRequest{
		OperationId: opID,
		NodeId:      nodeID,
		Success:     true,
		Message:     "done",
	})
	if err != nil {
		t.Fatalf("CompleteOperation error: %v", err)
	}
	op := srv.operations[opID]
	if op == nil || op.last == nil {
		t.Fatalf("operation state missing")
	}
	if !op.last.GetDone() {
		t.Fatalf("expected done=true, got %+v", op.last)
	}
	if op.last.GetPhase() != clustercontrollerpb.OperationPhase_OP_SUCCEEDED {
		t.Fatalf("expected succeeded phase, got %s", op.last.GetPhase())
	}
}

func TestCleanupTimedOutOperationsFails(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234")
	opID := "op-timeout"
	srv.operations[opID] = &operationState{
		created: time.Now().Add(-(operationTimeout + time.Minute)),
		nodeID:  "node-x",
	}
	srv.cleanupTimedOutOperations()
	op := srv.operations[opID]
	if op == nil || op.last == nil {
		t.Fatalf("expected operation event after timeout")
	}
	if !op.last.GetDone() {
		t.Fatalf("expected done true after timeout, got %+v", op.last)
	}
	if op.last.GetPhase() != clustercontrollerpb.OperationPhase_OP_FAILED {
		t.Fatalf("expected failed phase, got %s", op.last.GetPhase())
	}
	if !strings.Contains(op.last.GetMessage(), "timed out") {
		t.Fatalf("expected timeout message, got %q", op.last.GetMessage())
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	state := newControllerState()
	srv := newTestServer(t, state)

	_, err := srv.RemoveNode(context.Background(), &clustercontrollerpb.RemoveNodeRequest{
		NodeId: "nonexistent-node",
	})
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestRemoveNodeSuccess(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID: "node-1",
		Identity: storedIdentity{
			Hostname: "test-host",
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)
	srv.setLeader(true, "test", "127.0.0.1:1234")

	resp, err := srv.RemoveNode(context.Background(), &clustercontrollerpb.RemoveNodeRequest{
		NodeId: "node-1",
		Force:  true,
		Drain:  false,
	})
	if err != nil {
		t.Fatalf("RemoveNode error: %v", err)
	}
	if resp.GetOperationId() == "" {
		t.Fatal("expected operation_id in response")
	}
	if !strings.Contains(resp.GetMessage(), "removed") {
		t.Fatalf("expected 'removed' message, got: %s", resp.GetMessage())
	}

	// Verify node was actually removed
	srv.lock("test")
	if _, exists := srv.state.Nodes["node-1"]; exists {
		t.Fatal("expected node to be removed from state")
	}
	srv.unlock()
}

func TestGetClusterHealthEmpty(t *testing.T) {
	state := newControllerState()
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetTotalNodes() != 0 {
		t.Fatalf("expected 0 nodes, got %d", resp.GetTotalNodes())
	}
	if resp.GetStatus() != "unhealthy" {
		t.Fatalf("expected 'unhealthy' status for empty cluster, got %s", resp.GetStatus())
	}
}

func TestGetClusterHealthMixedNodes(t *testing.T) {
	state := newControllerState()
	now := time.Now()

	// Healthy node (seen recently)
	state.Nodes["healthy-node"] = &nodeState{
		NodeID:   "healthy-node",
		Identity: storedIdentity{Hostname: "healthy"},
		Status:   "healthy",
		LastSeen: now.Add(-30 * time.Second),
	}

	// Unhealthy node (seen long ago)
	state.Nodes["unhealthy-node"] = &nodeState{
		NodeID:    "unhealthy-node",
		Identity:  storedIdentity{Hostname: "unhealthy"},
		Status:    "unhealthy",
		LastSeen:  now.Add(-5 * time.Minute),
		LastError: "connection refused",
	}

	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetTotalNodes() != 2 {
		t.Fatalf("expected 2 nodes, got %d", resp.GetTotalNodes())
	}
	if resp.GetHealthyNodes() != 1 {
		t.Fatalf("expected 1 healthy node, got %d", resp.GetHealthyNodes())
	}
	if resp.GetUnhealthyNodes() != 1 {
		t.Fatalf("expected 1 unhealthy node, got %d", resp.GetUnhealthyNodes())
	}
	if resp.GetStatus() != "degraded" {
		t.Fatalf("expected 'degraded' status for mixed cluster, got %s", resp.GetStatus())
	}
}

func TestGetClusterHealthAllHealthy(t *testing.T) {
	state := newControllerState()
	now := time.Now()

	state.Nodes["node-1"] = &nodeState{
		NodeID:   "node-1",
		Identity: storedIdentity{Hostname: "node1"},
		Status:   "healthy",
		LastSeen: now.Add(-10 * time.Second),
	}
	state.Nodes["node-2"] = &nodeState{
		NodeID:   "node-2",
		Identity: storedIdentity{Hostname: "node2"},
		Status:   "healthy",
		LastSeen: now.Add(-20 * time.Second),
	}

	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetStatus() != "healthy" {
		t.Fatalf("expected 'healthy' status, got %s", resp.GetStatus())
	}
	if resp.GetHealthyNodes() != 2 {
		t.Fatalf("expected 2 healthy nodes, got %d", resp.GetHealthyNodes())
	}
}

func TestApplyNodePlanV1NilRequest(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	_, err := srv.ApplyNodePlanV1(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if !strings.Contains(err.Error(), "request is required") {
		t.Errorf("expected 'request is required' error, got: %v", err)
	}
}

func TestApplyNodePlanV1MissingNodeID(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "",
		Plan: &planpb.NodePlan{
			Spec: &planpb.PlanSpec{Steps: []*planpb.PlanStep{{Id: "test", Action: "test.action"}}},
		},
	}

	_, err := srv.ApplyNodePlanV1(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing node_id")
	}
	if !strings.Contains(err.Error(), "node_id is required") {
		t.Errorf("expected 'node_id is required' error, got: %v", err)
	}
}

func TestApplyNodePlanV1MissingPlan(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "node-1",
		Plan:   nil,
	}

	_, err := srv.ApplyNodePlanV1(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "plan is required") {
		t.Errorf("expected 'plan is required' error, got: %v", err)
	}
}

func TestApplyNodePlanV1PlanNodeMismatch(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "node-1",
		Plan: &planpb.NodePlan{
			NodeId: "node-2", // Mismatch
			Spec:   &planpb.PlanSpec{Steps: []*planpb.PlanStep{{Id: "test", Action: "test.action"}}},
		},
	}

	_, err := srv.ApplyNodePlanV1(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for node_id mismatch")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("expected mismatch error, got: %v", err)
	}
}

func TestApplyNodePlanV1EmptySteps(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "node-1",
		Plan: &planpb.NodePlan{
			Spec: &planpb.PlanSpec{Steps: []*planpb.PlanStep{}},
		},
	}

	_, err := srv.ApplyNodePlanV1(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
	if !strings.Contains(err.Error(), "at least one step") {
		t.Errorf("expected 'at least one step' error, got: %v", err)
	}
}

func TestApplyNodePlanV1NodeNotFound(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "nonexistent-node",
		Plan: &planpb.NodePlan{
			Spec: &planpb.PlanSpec{Steps: []*planpb.PlanStep{{Id: "test", Action: "test.action"}}},
		},
	}

	_, err := srv.ApplyNodePlanV1(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for node not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestApplyNodePlanV1NoAgentEndpoint(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID:        "node-1",
		AgentEndpoint: "", // No endpoint
	}
	srv := newTestServer(t, state)

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "node-1",
		Plan: &planpb.NodePlan{
			Spec: &planpb.PlanSpec{Steps: []*planpb.PlanStep{{Id: "test", Action: "test.action"}}},
		},
	}

	_, err := srv.ApplyNodePlanV1(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing agent endpoint")
	}
	if !strings.Contains(err.Error(), "no agent endpoint") {
		t.Errorf("expected 'no agent endpoint' error, got: %v", err)
	}
}

func TestApplyNodePlanV1SetsNodeIDInPlan(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID:        "node-1",
		AgentEndpoint: "127.0.0.1:50051", // Mock endpoint (will fail to connect, but validates request processing)
	}
	srv := newTestServer(t, state)

	req := &clustercontrollerpb.ApplyNodePlanV1Request{
		NodeId: "node-1",
		Plan: &planpb.NodePlan{
			NodeId: "", // Empty, should be set to request node_id
			Spec: &planpb.PlanSpec{Steps: []*planpb.PlanStep{{
				Id:     "test-step",
				Action: "test.action",
				Args:   &structpb.Struct{Fields: map[string]*structpb.Value{}},
			}}},
		},
	}

	// This will fail at dispatch (no real agent), but should pass validation
	_, err := srv.ApplyNodePlanV1(context.Background(), req)

	// Expect a dispatch error (agent connection failure), not a validation error
	if err != nil && !strings.Contains(err.Error(), "dispatch") && !strings.Contains(err.Error(), "agent") && !strings.Contains(err.Error(), "connect") {
		t.Errorf("expected dispatch/connection error, got: %v", err)
	}

	// Verify plan.NodeId was set in the request
	if req.Plan.NodeId != "node-1" {
		t.Errorf("expected plan.NodeId to be set to 'node-1', got %q", req.Plan.NodeId)
	}
}
