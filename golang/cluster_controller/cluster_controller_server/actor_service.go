// actor_service.go implements WorkflowActorService for the cluster-controller.
//
// The workflow service calls back into this service when executing steps
// assigned to actors owned by the controller: cluster-controller, node-agent,
// installer, and repository. Each action is resolved against a per-run
// Router (keyed by run_id) that carries workflow-specific config closures.
//
// Unknown actions are rejected with an error — never silently accepted.
// See docs/centralized-workflow-execution.md §4.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// ControllerActorServer implements WorkflowActorServiceServer. It dispatches
// incoming ExecuteAction calls to per-run Routers that are registered by the
// controller's workflow runner methods before calling ExecuteWorkflow.
//
// The per-run Router pattern allows each workflow execution to have its own
// config closures (e.g., release-specific state) while sharing the actor
// service endpoint.
type ControllerActorServer struct {
	workflowpb.UnimplementedWorkflowActorServiceServer

	mu      sync.RWMutex
	routers map[string]*engine.Router // run_id or correlation_id → Router
}

// NewControllerActorServer creates an actor server with an empty router registry.
func NewControllerActorServer() *ControllerActorServer {
	return &ControllerActorServer{
		routers: make(map[string]*engine.Router),
	}
}

// RegisterRouter associates a Router with a run/correlation ID. The workflow
// runner calls this before invoking ExecuteWorkflow so that callbacks from
// the workflow service can find the right action handlers.
func (s *ControllerActorServer) RegisterRouter(id string, router *engine.Router) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routers[id] = router
}

// UnregisterRouter removes the Router for a run/correlation ID. Called after
// the workflow completes to prevent memory leaks.
func (s *ControllerActorServer) UnregisterRouter(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.routers, id)
}

func (s *ControllerActorServer) resolveRouter(runID string) *engine.Router {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.routers[runID]; ok {
		return r
	}
	return nil
}

func (s *ControllerActorServer) ExecuteAction(ctx context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
	if req.Action == "" {
		return nil, fmt.Errorf("action is required")
	}
	if req.Actor == "" {
		return nil, fmt.Errorf("actor is required")
	}

	// Look up the per-run Router registered by the workflow runner.
	router := s.resolveRouter(req.RunId)
	if router == nil {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("controller: no router registered for run_id=%q", req.RunId),
		}, nil
	}

	handler, ok := router.Resolve(v1alpha1.ActorType(req.Actor), req.Action)
	if !ok {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("controller: unknown actor=%q action=%q", req.Actor, req.Action),
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
		Actor:   v1alpha1.ActorType(req.Actor),
		Action:  req.Action,
		With:    with,
		Inputs:  inputs,
		Outputs: outputs,
	}

	result, err := handler(ctx, actionReq)
	if err != nil {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("controller actor=%s action=%s failed: %v", req.Actor, req.Action, err),
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
