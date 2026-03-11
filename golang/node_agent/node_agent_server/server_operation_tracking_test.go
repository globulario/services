package main

import (
	"context"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/plan/planpb"
)

// PR-D.3: Tests for operation tracking — multiple ops, specific ID lookup.

func TestGetPlanStatusV1_ReturnsStatusForMatchingOperationID(t *testing.T) {
	ps := &stubPlanStore{}
	srv := newTestServer("node-1")
	srv.planStore = ps

	// Simulate a stored plan status with a known plan_id.
	ps.status = &planpb.NodePlanStatus{
		PlanId: "op-abc",
		NodeId: "node-1",
		State:  planpb.PlanState_PLAN_RUNNING,
	}

	resp, err := srv.GetPlanStatusV1(context.Background(), &node_agentpb.GetPlanStatusV1Request{
		OperationId: "op-abc",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp.GetStatus().GetPlanId() != "op-abc" {
		t.Fatalf("expected plan_id=op-abc, got %q", resp.GetStatus().GetPlanId())
	}
	if resp.GetStatus().GetState() != planpb.PlanState_PLAN_RUNNING {
		t.Fatalf("expected PLAN_RUNNING, got %v", resp.GetStatus().GetState())
	}
}

func TestGetPlanStatusV1_RejectsNonMatchingOperationID(t *testing.T) {
	ps := &stubPlanStore{}
	srv := newTestServer("node-1")
	srv.planStore = ps

	// Store a plan status for op-abc.
	ps.status = &planpb.NodePlanStatus{
		PlanId: "op-abc",
		NodeId: "node-1",
		State:  planpb.PlanState_PLAN_SUCCEEDED,
	}

	// Request status for a different operation.
	_, err := srv.GetPlanStatusV1(context.Background(), &node_agentpb.GetPlanStatusV1Request{
		OperationId: "op-xyz",
	})
	if err == nil {
		t.Fatal("expected NotFound error for non-matching operation_id, got nil")
	}
	if !containsStr(err.Error(), "no active plan for operation_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPlanStatusV1_NoOperationID_ReturnsCurrentPlan(t *testing.T) {
	ps := &stubPlanStore{}
	srv := newTestServer("node-1")
	srv.planStore = ps

	ps.status = &planpb.NodePlanStatus{
		PlanId: "op-latest",
		NodeId: "node-1",
		State:  planpb.PlanState_PLAN_SUCCEEDED,
	}

	// Request without operation_id — should return whatever is current.
	resp, err := srv.GetPlanStatusV1(context.Background(), &node_agentpb.GetPlanStatusV1Request{})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp.GetStatus().GetPlanId() != "op-latest" {
		t.Fatalf("expected op-latest, got %q", resp.GetStatus().GetPlanId())
	}
}

func TestGetPlanStatusV1_NilStatus_WithOperationID_ReturnsNotFound(t *testing.T) {
	ps := &stubPlanStore{}
	srv := newTestServer("node-1")
	srv.planStore = ps
	// ps.status is nil (no plan has been applied).

	_, err := srv.GetPlanStatusV1(context.Background(), &node_agentpb.GetPlanStatusV1Request{
		OperationId: "op-nonexistent",
	})
	if err == nil {
		t.Fatal("expected error when no plan exists but operation_id is requested")
	}
}

func TestGetPlanStatusV1_DistinctOperations_DontCollide(t *testing.T) {
	ps := &stubPlanStore{}
	srv := newTestServer("node-1")
	srv.planStore = ps

	// Set current plan to operation A.
	ps.status = &planpb.NodePlanStatus{
		PlanId: "op-A",
		NodeId: "node-1",
		State:  planpb.PlanState_PLAN_RUNNING,
	}

	// Query for operation A — should succeed.
	resp, err := srv.GetPlanStatusV1(context.Background(), &node_agentpb.GetPlanStatusV1Request{
		OperationId: "op-A",
	})
	if err != nil {
		t.Fatalf("op-A should match: %v", err)
	}
	if resp.GetStatus().GetState() != planpb.PlanState_PLAN_RUNNING {
		t.Fatalf("op-A state should be RUNNING, got %v", resp.GetStatus().GetState())
	}

	// Query for operation B — should fail since current plan is A.
	_, err = srv.GetPlanStatusV1(context.Background(), &node_agentpb.GetPlanStatusV1Request{
		OperationId: "op-B",
	})
	if err == nil {
		t.Fatal("op-B should not match when current plan is op-A")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && findSub(s, sub)
}

func findSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
