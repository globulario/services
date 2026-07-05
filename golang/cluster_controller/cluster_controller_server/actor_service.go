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
	// defaultRouter is a last-resort fallback for context-free actions when no
	// per-run router is registered and none can be rebuilt. It must contain only
	// safe handlers; it must NOT be used for a generation-guarded release write
	// (that path rebuilds or refuses — see ExecuteAction).
	defaultRouter *engine.Router
	// rebuild reconstructs a per-run router from a callback's self-describing
	// inputs when the in-memory registry lost it (controller restart/failover).
	// Injected so the actor server stays decoupled from *server. Returns
	// (router, isReleaseCallback) — see rebuildReleaseRouterFromInputs.
	rebuild func(runID, inputsJSON string) (*engine.Router, bool)
}

// SetRouterRebuilder installs the per-run router rebuild function. Wired at
// startup to srv.rebuildReleaseRouterFromInputs.
func (s *ControllerActorServer) SetRouterRebuilder(fn func(runID, inputsJSON string) (*engine.Router, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuild = fn
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

// resolvePerRunRouter returns the per-run router registered for this run (exact
// or, for foreach sub-runs "parent[i]", the parent). It does NOT fall back to the
// default router — callers decide fallback vs. rebuild vs. refuse.
func (s *ControllerActorServer) resolvePerRunRouter(runID string) *engine.Router {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.routers[runID]; ok {
		return r
	}
	if idx := strings.LastIndex(runID, "["); idx > 0 {
		parent := runID[:idx]
		if r, ok := s.routers[parent]; ok {
			return r
		}
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

	// Look up the per-run Router registered by the workflow runner. On a miss
	// (this controller restarted or failed over after dispatch, so the in-memory
	// registry is gone), rebuild it from the callback's self-describing inputs so
	// the per-run GENERATION GUARD is preserved. A release write whose generation
	// cannot be recovered is REFUSED rather than written through a guard-disabled
	// router — the run then fails and the reconcile backstop re-drives the release
	// cleanly (fire-and-reconcile, never fire-and-pray).
	router := s.resolvePerRunRouter(req.RunId)
	if router == nil {
		s.mu.RLock()
		rebuild := s.rebuild
		def := s.defaultRouter
		s.mu.RUnlock()
		var isRelease bool
		if rebuild != nil {
			router, isRelease = rebuild(req.RunId, req.InputsJson)
		}
		switch {
		case router != nil:
			s.RegisterRouter(req.RunId, router) // cache for the rest of this run's callbacks
		case isRelease:
			return &workflowpb.ExecuteActionResponse{
				Ok:      false,
				Message: fmt.Sprintf("controller: no router for run_id=%q and dispatch generation unrecoverable — refusing guard-less release write (will be re-driven)", req.RunId),
			}, nil
		case def != nil:
			router = def
		default:
			return &workflowpb.ExecuteActionResponse{
				Ok:      false,
				Message: fmt.Sprintf("controller: no router registered for run_id=%q", req.RunId),
			}, nil
		}
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
