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

	// A handler may return BOTH a structured output delta and an error: a
	// semantic failure must still deliver its receipt, or the caller cannot
	// tell a governed refusal from a broken executor. Same contract as the
	// cluster-doctor actor service.
	result, err := handler(ctx, actionReq)
	failMsg := ""
	if err != nil {
		failMsg = fmt.Sprintf("compute action %s failed: %v", req.Action, err)
	}

	resp := &workflowpb.ExecuteActionResponse{
		Ok: err == nil && result != nil && result.OK,
	}
	switch {
	case failMsg != "":
		resp.Message = failMsg
	case result != nil:
		resp.Message = result.Message
	}
	if result != nil && result.Output != nil {
		if b, err := json.Marshal(result.Output); err == nil {
			resp.OutputJson = string(b)
		}
	}
	return resp, nil
}
