package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/security"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTestServer returns a minimal NodeAgentServer for plan security tests.
func newTestServer(nodeID string) *NodeAgentServer {
	return &NodeAgentServer{
		nodeID:     nodeID,
		planStore:  &stubPlanStore{},
		state:      newNodeAgentState(),
		operations: map[string]*operation{},
	}
}

// TestApplyPlanV1_NodeIDMismatch verifies that a plan addressed to a different
// node is rejected with InvalidArgument.
func TestApplyPlanV1_NodeIDMismatch(t *testing.T) {
	srv := newTestServer("node-a")

	_, err := srv.ApplyPlanV1(context.Background(), &nodeagentpb.ApplyPlanV1Request{
		Plan: &planpb.NodePlan{
			NodeId: "node-b", // wrong node
			PlanId: "plan-1",
			Spec:   &planpb.PlanSpec{},
		},
	})
	if err == nil {
		t.Fatal("expected error for node_id mismatch, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

// TestApplyPlanV1_ClusterIDMismatch verifies that a plan targeting a different
// cluster is rejected when the local cluster ID is known.
func TestApplyPlanV1_ClusterIDMismatch(t *testing.T) {
	// Point the default validator at a fake domain so GetLocalClusterID returns
	// a known value without touching the real config layer.
	security.OverrideLocalClusterID(t, "cluster-local")

	srv := newTestServer("node-1")

	_, err := srv.ApplyPlanV1(context.Background(), &nodeagentpb.ApplyPlanV1Request{
		Plan: &planpb.NodePlan{
			NodeId:    "node-1",
			PlanId:    "plan-x",
			ClusterId: "cluster-foreign", // wrong cluster
			Spec:      &planpb.PlanSpec{},
		},
	})
	if err == nil {
		t.Fatal("expected error for cluster_id mismatch, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if st.Message() == "" {
		t.Fatal("expected non-empty error message")
	}
}

// TestApplyPlanV1_ClusterIDMatch verifies that a plan with the correct
// cluster_id is accepted.
func TestApplyPlanV1_ClusterIDMatch(t *testing.T) {
	security.OverrideLocalClusterID(t, "cluster-local")

	srv := newTestServer("node-1")

	resp, err := srv.ApplyPlanV1(context.Background(), &nodeagentpb.ApplyPlanV1Request{
		Plan: &planpb.NodePlan{
			NodeId:    "node-1",
			PlanId:    "plan-ok",
			ClusterId: "cluster-local", // matches
			Spec:      &planpb.PlanSpec{},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetPlanId() != "plan-ok" {
		t.Fatalf("unexpected plan_id: %s", resp.GetPlanId())
	}
}

// TestApplyPlanV1_ClusterIDEmpty verifies that a plan without cluster_id
// is still accepted (backwards-compatible: older controllers may not set it).
func TestApplyPlanV1_ClusterIDEmpty(t *testing.T) {
	security.OverrideLocalClusterID(t, "cluster-local")

	srv := newTestServer("node-1")

	_, err := srv.ApplyPlanV1(context.Background(), &nodeagentpb.ApplyPlanV1Request{
		Plan: &planpb.NodePlan{
			NodeId: "node-1",
			PlanId: "plan-legacy",
			// ClusterId intentionally empty
			Spec: &planpb.PlanSpec{},
		},
	})
	if err != nil {
		t.Fatalf("expected empty cluster_id to be accepted for backwards compat, got: %v", err)
	}
}
