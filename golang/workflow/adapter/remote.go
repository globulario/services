package adapter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RemoteHandler dispatches engine actions to remote node-agents via a
// pluggable transport. It implements engine.ActionHandler, so it can be
// registered in the engine Router like any local handler.
//
// Usage:
//
//	rh := adapter.NewRemoteHandler(transport, "node-1")
//	router.Register(v1alpha1.ActorNodeAgent, "node.install_packages", rh.Handle)
//
// Or use RegisterRemoteNodeAgent to register all node-agent actions at once.
type RemoteHandler struct {
	Transport StepTransport
	NodeID    string // default target node; overridden by inputs["node_id"]
	OnProgress func(evt ProgressEvent) // optional progress callback
}

// NewRemoteHandler creates a handler for a specific node.
func NewRemoteHandler(transport StepTransport, nodeID string) *RemoteHandler {
	return &RemoteHandler{Transport: transport, NodeID: nodeID}
}

// Handle implements engine.ActionHandler. It translates an engine request
// into an adapter request, dispatches to the remote node-agent, and
// translates the result back.
func (rh *RemoteHandler) Handle(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
	nodeID := rh.NodeID
	if nid, ok := req.Inputs["node_id"].(string); ok && nid != "" {
		nodeID = nid
	}

	identity := ExecutionIdentity{
		RunID:         req.RunID,
		StepID:        req.StepID,
		WorkflowName:  "", // filled by caller if needed
		NodeID:        nodeID,
		CorrelationID: req.RunID + "/" + req.StepID,
	}

	stepReq := ExecuteStepRequest{
		Identity:     identity,
		Actor:        string(req.Actor),
		Action:       req.Action,
		Inputs:       mergeInputs(req.With, req.Inputs),
		DispatchTime: time.Now(),
	}

	// Apply context deadline if present.
	if deadline, ok := ctx.Deadline(); ok {
		stepReq.Deadline = deadline
	}

	log.Printf("adapter: dispatching %s::%s to node %s (run=%s step=%s)",
		req.Actor, req.Action, nodeID, req.RunID, req.StepID)

	result, err := rh.Transport.Dispatch(ctx, stepReq)
	if err != nil {
		return nil, fmt.Errorf("dispatch to node %s: %w", nodeID, err)
	}

	return translateResult(result), nil
}

// translateResult converts an adapter ResultEvent into an engine ActionResult.
func translateResult(evt *ResultEvent) *engine.ActionResult {
	if evt == nil {
		return &engine.ActionResult{OK: false, Message: "nil result from node-agent"}
	}

	ar := &engine.ActionResult{
		OK:      evt.Status == StatusSucceeded,
		Output:  evt.Outputs,
		Message: evt.Summary,
	}

	if evt.Error != nil {
		ar.Message = fmt.Sprintf("[%s] %s", evt.Error.ErrorClass, evt.Error.Message)
	}

	return ar
}

// mergeInputs combines step.With and workflow inputs into a single map
// for the remote request. With values take precedence.
func mergeInputs(with, inputs map[string]any) map[string]any {
	merged := make(map[string]any, len(inputs)+len(with))
	for k, v := range inputs {
		merged[k] = v
	}
	for k, v := range with {
		merged[k] = v
	}
	return merged
}

// RegisterRemoteNodeAgent registers a RemoteHandler for all standard
// node-agent actions in the engine router. This replaces the local
// in-process handlers with remote dispatch.
func RegisterRemoteNodeAgent(router *engine.Router, transport StepTransport, nodeID string) {
	rh := NewRemoteHandler(transport, nodeID)
	actions := []string{
		"node.install_packages",
		"node.verify_services_active",
		"node.sync_installed_state",
		"node.execute_plan",
	}
	for _, action := range actions {
		router.Register(v1alpha1.ActorNodeAgent, action, rh.Handle)
	}
}
