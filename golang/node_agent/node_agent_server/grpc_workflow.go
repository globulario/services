package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
)

// RunWorkflow implements the gRPC endpoint for workflow execution.
// The controller (or CLI) calls this to trigger a workflow on the node.
func (srv *NodeAgentServer) RunWorkflow(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	name := req.GetWorkflowName()
	if name == "" {
		name = "node.join"
	}

	// Resolve definition path.
	defPath := req.GetDefinitionPath()
	if defPath == "" {
		defPath = resolveWorkflowPath(name)
	}
	if defPath == "" {
		return nil, fmt.Errorf("workflow definition %q not found", name)
	}

	// Build inputs from request + local state.
	inputs := make(map[string]any)
	for k, v := range req.GetInputs() {
		inputs[k] = v
	}
	// Fill in defaults from local state.
	if _, ok := inputs["cluster_id"]; !ok {
		inputs["cluster_id"] = "globular.internal"
	}
	if _, ok := inputs["node_id"]; !ok {
		inputs["node_id"] = srv.nodeID
	}
	if _, ok := inputs["node_hostname"]; !ok && srv.state != nil {
		inputs["node_hostname"] = srv.state.NodeName
	}
	if _, ok := inputs["node_ip"]; !ok && srv.state != nil {
		inputs["node_ip"] = srv.state.AdvertiseIP
	}

	log.Printf("grpc-workflow: starting %s (def=%s)", name, defPath)
	start := time.Now()

	run, err := srv.RunWorkflowDefinition(ctx, defPath, inputs)
	elapsed := time.Since(start)

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
	}

	if run != nil {
		resp.RunId = run.ID
		resp.Status = string(run.Status)
		for _, st := range run.Steps {
			resp.StepsTotal++
			switch st.Status {
			case engine.StepSucceeded:
				resp.StepsSucceeded++
			case engine.StepFailed:
				resp.StepsFailed++
			}
		}
	}

	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
	}

	return resp, nil
}

// resolveWorkflowPath finds a workflow YAML by name.
func resolveWorkflowPath(name string) string {
	candidates := []string{
		fmt.Sprintf("/var/lib/globular/workflows/%s.yaml", name),
		fmt.Sprintf("/tmp/%s.yaml", name),
		fmt.Sprintf("/usr/lib/globular/workflows/%s.yaml", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
