// workflow_execute.go provides the centralized workflow execution helper
// used by all controller workflow runners. It handles:
//   - Router registration with the actor service
//   - Building the ExecuteWorkflow request
//   - Calling the workflow service
//   - Cleanup after execution
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// executeWorkflowCentralized delegates workflow execution to the centralized
// WorkflowService. It registers the provided Router with the actor service
// so that callbacks can find the right action handlers, then calls
// ExecuteWorkflow and waits for completion.
//
// The correlationID is used both as the Router lookup key and the workflow
// service's correlation_id for run deduplication.
func (srv *server) executeWorkflowCentralized(
	ctx context.Context,
	workflowName string,
	correlationID string,
	inputs map[string]any,
	router *engine.Router,
) (*workflowpb.ExecuteWorkflowResponse, error) {
	if srv.workflowClient == nil {
		return nil, fmt.Errorf("workflow service not configured (workflowClient is nil — check CLUSTER_WORKFLOW_SERVICE_ADDR or etcd service registry)")
	}

	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("marshal inputs: %w", err)
	}

	// Register the per-run Router so the actor service can dispatch
	// callbacks to the right handlers.
	srv.actorServer.RegisterRouter(correlationID, router)
	defer srv.actorServer.UnregisterRouter(correlationID)

	// Callback endpoint: the workflow service calls back to THIS controller
	// for actor dispatch. Use our real address from the service registry,
	// not localhost — the workflow service may be on another node.
	controllerEndpoint := config.ResolveLocalServiceAddr("cluster_controller.ClusterControllerService")

	log.Printf("workflow %s: dispatching to workflow service (callback=%s, correlation=%s)",
		workflowName, controllerEndpoint, correlationID)

	resp, err := srv.workflowClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    srv.cfg.ClusterDomain,
		WorkflowName: workflowName,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-controller": controllerEndpoint,
			"node-agent":         controllerEndpoint, // controller proxies to real node-agents
			"installer":          controllerEndpoint,
			"repository":         controllerEndpoint,
		},
		CorrelationId: correlationID,
	})
	if err != nil {
		log.Printf("workflow %s (correlation=%s): RPC failed: %v", workflowName, correlationID, err)
		return nil, fmt.Errorf("ExecuteWorkflow %s: %w", workflowName, err)
	}

	if resp.Status == "FAILED" {
		log.Printf("workflow %s (correlation=%s): FAILED — %s", workflowName, correlationID, resp.Error)
	} else {
		log.Printf("workflow %s (correlation=%s): %s", workflowName, correlationID, resp.Status)
	}

	return resp, nil
}

// executeWorkflowCentralizedWithRegistration is a convenience wrapper that
// registers a per-run Router with the provided correlation ID BEFORE the
// workflow service assigns a run_id. When the workflow service creates the
// run, it uses the correlation_id as the run_id prefix, so the actor
// service can find the Router.
//
// After the workflow completes, the Router is unregistered.
// The caller must register the router using the run_id returned in the
// response.
func (srv *server) executeWorkflowWithRunIDRouter(
	ctx context.Context,
	workflowName string,
	correlationID string,
	inputs map[string]any,
	router *engine.Router,
) (*workflowpb.ExecuteWorkflowResponse, error) {
	if srv.workflowClient == nil {
		return nil, fmt.Errorf("workflow service not configured (workflowClient is nil)")
	}

	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("marshal inputs: %w", err)
	}

	controllerEndpoint := config.ResolveLocalServiceAddr("cluster_controller.ClusterControllerService")

	srv.actorServer.RegisterRouter(correlationID, router)
	defer srv.actorServer.UnregisterRouter(correlationID)

	resp, err := srv.workflowClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    srv.cfg.ClusterDomain,
		WorkflowName: workflowName,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-controller": controllerEndpoint,
			"node-agent":         controllerEndpoint,
			"installer":          controllerEndpoint,
			"repository":         controllerEndpoint,
		},
		CorrelationId: correlationID,
	})
	if err != nil {
		log.Printf("workflow %s (correlation=%s): RPC failed: %v", workflowName, correlationID, err)
		return nil, fmt.Errorf("ExecuteWorkflow %s: %w", workflowName, err)
	}

	return resp, nil
}
