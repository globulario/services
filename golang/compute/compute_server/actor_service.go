// actor_service.go implements WorkflowActorService for the compute service.
//
// The workflow service calls back into this service when executing steps
// assigned to actor "compute". Each action is resolved against the compute
// service's local Router (wired to job submission, unit execution, and
// aggregation handlers in workflow_actions.go).
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// ComputeActorServer implements WorkflowActorServiceServer. It dispatches
// incoming ExecuteAction calls to the compute service's action handlers.
type ComputeActorServer struct {
	workflowpb.UnimplementedWorkflowActorServiceServer
	router *engine.Router
}

// NewComputeActorServer creates an actor server backed by the given Router.
func NewComputeActorServer(router *engine.Router) *ComputeActorServer {
	return &ComputeActorServer{router: router}
}

func (s *ComputeActorServer) ExecuteAction(ctx context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
	if req.Action == "" {
		return nil, fmt.Errorf("action is required")
	}

	handler, ok := s.router.Resolve(v1alpha1.ActorCompute, req.Action)
	if !ok {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("compute: unknown action %q", req.Action),
		}, nil
	}

	// Deserialize inputs from JSON.
	with := make(map[string]any)
	if req.WithJson != "" {
		json.Unmarshal([]byte(req.WithJson), &with)
	}
	inputs := make(map[string]any)
	if req.InputsJson != "" {
		json.Unmarshal([]byte(req.InputsJson), &inputs)
	}
	outputs := make(map[string]any)
	if req.OutputsJson != "" {
		json.Unmarshal([]byte(req.OutputsJson), &outputs)
	}

	actionReq := engine.ActionRequest{
		RunID:   req.RunId,
		StepID:  req.StepId,
		Actor:   v1alpha1.ActorCompute,
		Action:  req.Action,
		With:    with,
		Inputs:  inputs,
		Outputs: outputs,
	}

	result, err := handler(ctx, actionReq)
	if err != nil {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("compute action %s failed: %v", req.Action, err),
		}, nil
	}

	resp := &workflowpb.ExecuteActionResponse{
		Ok:      result.OK,
		Message: result.Message,
	}
	if result.Output != nil {
		if b, err := json.Marshal(result.Output); err == nil {
			resp.OutputJson = string(b)
		}
	}
	return resp, nil
}
