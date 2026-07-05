package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestRunWorkflow_NodeIdentityFence_RejectsWrongTarget verifies that a
// node-targeted workflow whose target_node_id does not match this agent's own
// identity is rejected before any action runs — "install on node X" must never
// execute on node Y even if a mis-resolved endpoint routed it here.
// (four_layer.workflow_actor_attribution_required)
func TestRunWorkflow_NodeIdentityFence_RejectsWrongTarget(t *testing.T) {
	srv := &NodeAgentServer{nodeID: "node-A"}

	resp, err := srv.RunWorkflow(context.Background(), &node_agentpb.RunWorkflowRequest{
		WorkflowName: "install-package",
		Inputs:       map[string]string{"target_node_id": "node-B"},
	})
	if err != nil {
		t.Fatalf("expected a structured FAILED response, got Go error: %v", err)
	}
	if resp.GetStatus() != "FAILED" {
		t.Fatalf("expected status FAILED, got %q", resp.GetStatus())
	}
	if !strings.Contains(resp.GetError(), "node-identity fence") {
		t.Fatalf("expected node-identity fence error, got %q", resp.GetError())
	}
	if !strings.Contains(resp.GetError(), "node-B") || !strings.Contains(resp.GetError(), "node-A") {
		t.Fatalf("fence error should name both intended and actual node, got %q", resp.GetError())
	}
}
