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
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// workflowGateLastLogMs is the Unix millisecond timestamp of the last
// "workflow backend unhealthy" log line. Used to rate-limit log spam when
// the circuit breaker is open and every reconcile tick is gated.
var workflowGateLastLogMs atomic.Int64

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

	// Health gate: block dispatch if workflow backend is under pressure.
	if srv.workflowGate != nil {
		if err := srv.workflowGate.Check(); err != nil {
			// Rate-limit: log at most once per minute to avoid log spam during
			// a circuit-breaker-open period where every reconcile tick is gated.
			nowMs := time.Now().UnixMilli()
			if nowMs-workflowGateLastLogMs.Load() > 60_000 {
				workflowGateLastLogMs.Store(nowMs)
				log.Printf("workflow backend unhealthy, reconcile deferred: %v", err)
			}
			srv.emitClusterEvent("workflow.backend_pressure", map[string]interface{}{
				"severity": "WARNING",
				"message":  err.Error(),
			})
			return nil, err
		}
	}

	// Posture gate (Gate 1): suppress ROLLOUT-class dispatch in RECOVERY_ONLY.
	// postureGateCheck returns a transient error ("posture gate: …") so the
	// release pipeline classifies it as retryable and keeps the release RESOLVED.
	// No goroutine is blocked — the caller returns immediately.
	if err := postureGateCheck(ClusterPosture(srv.posture.Load()), workflowName); err != nil {
		class := mapWorkflowToClass(workflowName)
		postureGateSuppressedTotal.WithLabelValues(PostureRecoveryOnly.String(), string(class)).Inc()
		log.Printf("posture gate: suppressed workflow=%s class=%s posture=RECOVERY_ONLY workflow_cb_open=%v reconcile_cb_open=%v",
			workflowName, class, srv.workflowGate.IsOpen(), srv.reconcileBreaker.IsOpen())
		srv.emitClusterEvent("posture.gate_suppressed", map[string]interface{}{
			"severity": "WARNING",
			"workflow": workflowName,
			"class":    string(class),
			"posture":  PostureRecoveryOnly.String(),
		})
		return nil, err
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
	controllerEndpoint := srv.resolveControllerEndpoint()
	if controllerEndpoint == "" {
		return nil, fmt.Errorf("cannot resolve controller callback endpoint — service registry may not have this node's controller address")
	}

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
			// workflow-service actor (start_child / wait_child_terminal) must
			// route back to this controller so the registered StartChild and
			// WaitChildTerminal callbacks are invoked. Without this, the workflow
			// service handles these actions locally with a no-op config, causing
			// child workflows (release.apply.package) to silently return mock-run
			// instead of actually dispatching to node-agents.
			"workflow-service": controllerEndpoint,
		},
		CorrelationId: correlationID,
	})
	if err != nil {
		log.Printf("workflow %s (correlation=%s): RPC failed: %v", workflowName, correlationID, err)
		// Record transport failure for circuit breaker (not business failures).
		if srv.workflowGate != nil {
			srv.workflowGate.RecordFailure()
		}
		return nil, fmt.Errorf("ExecuteWorkflow %s: %w", workflowName, err)
	}

	// RPC succeeded — close circuit breaker if it was open.
	if srv.workflowGate != nil {
		srv.workflowGate.RecordSuccess()
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

	controllerEndpoint := srv.resolveControllerEndpoint()
	if controllerEndpoint == "" {
		return nil, fmt.Errorf("cannot resolve controller callback endpoint")
	}

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
			"workflow-service":   controllerEndpoint,
		},
		CorrelationId: correlationID,
	})
	if err != nil {
		log.Printf("workflow %s (correlation=%s): RPC failed: %v", workflowName, correlationID, err)
		return nil, fmt.Errorf("ExecuteWorkflow %s: %w", workflowName, err)
	}

	return resp, nil
}

// resolveControllerEndpoint returns the callback address for this controller
// instance. It tries multiple sources to be resilient to service registry
// timing issues during leadership changes.
//
// Resolution order:
//  1. etcd service registry (canonical, written by setLeader)
//  2. server config Port + local routable IP (always available)
func (srv *server) resolveControllerEndpoint() string {
	// 1. Service registry — canonical source.
	if addr := config.ResolveLocalServiceAddr("cluster_controller.ClusterControllerService"); addr != "" {
		return addr
	}
	// 2. Fallback: build from known port + local IP.
	localIP := config.GetRoutableIPv4()
	if localIP != "" {
		port := 12000
		if srv.cfg != nil && srv.cfg.Port > 0 {
			port = srv.cfg.Port
		}
		addr := fmt.Sprintf("%s:%d", localIP, port)
		log.Printf("workflow: controller endpoint fallback to %s (registry empty)", addr)
		return addr
	}
	return ""
}
