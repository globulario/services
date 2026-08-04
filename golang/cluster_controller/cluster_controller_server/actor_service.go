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
	"strings"
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
	// defaultRouter is used when no per-run router is registered (e.g. if the
	// controller restarted mid-run). It should only contain safe no-op handlers.
	defaultRouter *engine.Router
}

// NewControllerActorServer creates an actor server with an empty router registry.
func NewControllerActorServer() *ControllerActorServer {
	return &ControllerActorServer{
		routers: make(map[string]*engine.Router),
	}
}

// SetDefaultRouter installs a fallback Router used when a run-specific router
// cannot be found (e.g., after a controller restart). Keep this limited to
// safe/idempotent handlers.
func (s *ControllerActorServer) SetDefaultRouter(router *engine.Router) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultRouter = router
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
	// Foreach sub-steps use suffixed run IDs like "parent-id[0]", "parent-id[1]".
	// Fall back to the parent ID by stripping the bracket suffix.
	if idx := strings.LastIndex(runID, "["); idx > 0 {
		parent := runID[:idx]
		if r, ok := s.routers[parent]; ok {
			return r
		}
	}
	if s.defaultRouter != nil {
		return s.defaultRouter
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
		// Fail closed. Continuing with an empty map would run the handler
		// against a fabricated view of the run, and the substitution would be
		// invisible in the result.
		if err := json.Unmarshal([]byte(req.WithJson), &with); err != nil {
			return &workflowpb.ExecuteActionResponse{
				Ok:      false,
				Message: fmt.Sprintf("controller: malformed WithJson: %v", err),
			}, nil
		}
	}
	inputs := make(map[string]any)
	if req.InputsJson != "" {
		// Fail closed. Continuing with an empty map would run the handler
		// against a fabricated view of the run, and the substitution would be
		// invisible in the result.
		if err := json.Unmarshal([]byte(req.InputsJson), &inputs); err != nil {
			return &workflowpb.ExecuteActionResponse{
				Ok:      false,
				Message: fmt.Sprintf("controller: malformed InputsJson: %v", err),
			}, nil
		}
	}
	outputs := make(map[string]any)
	if req.OutputsJson != "" {
		// Fail closed. Continuing with an empty map would run the handler
		// against a fabricated view of the run, and the substitution would be
		// invisible in the result.
		if err := json.Unmarshal([]byte(req.OutputsJson), &outputs); err != nil {
			return &workflowpb.ExecuteActionResponse{
				Ok:      false,
				Message: fmt.Sprintf("controller: malformed OutputsJson: %v", err),
			}, nil
		}
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

	// A handler may return BOTH a structured output delta and an error: a
	// semantic failure must still deliver its receipt, or the caller cannot
	// tell a governed refusal from a broken executor. Same contract as the
	// cluster-doctor actor service.
	result, err := handler(ctx, actionReq)
	failMsg := ""
	if err != nil {
		failMsg = fmt.Sprintf("controller actor=%s action=%s failed: %v", req.Actor, req.Action, err)
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
		b, mErr := json.Marshal(result.Output)
		if mErr != nil {
			// Never drop a receipt silently. A caller that receives a failure
			// with no receipt cannot tell a governed refusal from a broken
			// executor — the exact ambiguity this contract exists to remove.
			return &workflowpb.ExecuteActionResponse{
				Ok:      false,
				Message: fmt.Sprintf("controller: output delta not serializable: %v", mErr),
			}, nil
		}
		resp.OutputJson = string(b)
	}
	return resp, nil
}
