// actor_service.go implements WorkflowActorService for the cluster-doctor.
//
// The workflow service calls back into this service when executing steps
// assigned to actor "cluster-doctor". Each action is resolved against the
// doctor's local Router (wired to finding cache, ExecuteRemediation, etc).
//
// Unknown actions are rejected with an error — never silently accepted.
// See docs/centralized-workflow-execution.md §4.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// DoctorActorServer implements WorkflowActorServiceServer. It dispatches
// incoming ExecuteAction calls to the doctor's local action handlers.
type DoctorActorServer struct {
	workflowpb.UnimplementedWorkflowActorServiceServer
	router *engine.Router
}

// NewDoctorActorServer creates an actor server backed by the given Router.
// The Router should have RegisterDoctorRemediationActions called on it
// with a live DoctorRemediationConfig wired to the doctor's state.
func NewDoctorActorServer(router *engine.Router) *DoctorActorServer {
	return &DoctorActorServer{router: router}
}

func (s *DoctorActorServer) ExecuteAction(ctx context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
	if req.Action == "" {
		return nil, fmt.Errorf("action is required")
	}

	// Resolve against explicit registrations only — no fallbacks.
	handler, ok := s.router.Resolve(v1alpha1.ActorClusterDoctor, req.Action)
	if !ok {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("cluster-doctor: unknown action %q", req.Action),
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
		Actor:   v1alpha1.ActorClusterDoctor,
		Action:  req.Action,
		With:    with,
		Inputs:  inputs,
		Outputs: outputs,
	}

	result, err := handler(ctx, actionReq)
	if err != nil {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("cluster-doctor action %s failed: %v", req.Action, err),
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
