// @awareness namespace=globular.platform
// @awareness component=platform_ai_executor.action_backend
// @awareness file_role=event_bus_action_dispatch_with_verification_loop
// @awareness implements=globular.platform:intent.ai.remediation_actions_dispatched_via_event_bus_not_direct_rpc
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	globular "github.com/globulario/services/golang/globular_service"
)

// ActionBackend executes a real remediation action and verifies the outcome.
type ActionBackend interface {
	// Supports returns true if this backend handles the given action type.
	Supports(actionType ai_executorpb.ActionType) bool

	// Execute performs the action. Returns result description and error.
	Execute(ctx context.Context, target string, diagnosis *ai_executorpb.Diagnosis) (result string, err error)

	// Verify checks if the action had the desired effect. Returns true if verified.
	Verify(ctx context.Context, target string) (ok bool, detail string, err error)
}

// actionDispatcher routes actions to the appropriate backend.
type actionDispatcher struct {
	backends []ActionBackend
}

func newActionDispatcher() *actionDispatcher {
	return &actionDispatcher{
		backends: []ActionBackend{
			&restartServiceBackend{},
			&drainEndpointBackend{},
			&blockIPBackend{},
			&circuitBreakerBackend{},
			&notifyAdminBackend{},
		},
	}
}

// dispatch finds the right backend and executes + verifies.
// Dispatch success does NOT mean repair is confirmed — verification is required.
//
func (ad *actionDispatcher) dispatch(ctx context.Context, action *ai_executorpb.RemediationAction, diagnosis *ai_executorpb.Diagnosis) (string, error) {
	for _, backend := range ad.backends {
		if backend.Supports(action.GetType()) {
			// Execute.
			result, err := backend.Execute(ctx, action.GetTarget(), diagnosis)
			if err != nil {
				return "", fmt.Errorf("execute %s: %w", action.GetType(), err)
			}

			// Verify after a brief delay (give the system time to react).
			time.Sleep(3 * time.Second)
			ok, detail, verifyErr := backend.Verify(ctx, action.GetTarget())
			if verifyErr != nil {
				logger.Warn("verification failed", "action", action.GetType(), "err", verifyErr)
				return result + " (verification inconclusive)", nil
			}
			if !ok {
				return "", fmt.Errorf("verification failed: %s", detail)
			}

			return result + " (verified: " + detail + ")", nil
		}
	}

	return "", fmt.Errorf("no backend supports action type %s", action.GetType())
}

// --- RestartServiceBackend: restarts a systemd unit via cluster controller ---

type restartServiceBackend struct{}

func (b *restartServiceBackend) Supports(t ai_executorpb.ActionType) bool {
	return t == ai_executorpb.ActionType_ACTION_RESTART_SERVICE
}

func (b *restartServiceBackend) Execute(ctx context.Context, target string, diag *ai_executorpb.Diagnosis) (string, error) {
	// Publish the restart request as an event.
	// The cluster controller subscribes to this and dispatches a
	// workflow-tracked restart through the node agent.
	payload := map[string]interface{}{
		"severity": "WARNING",
		"target":   target,
		"source":   "ai_executor",
	}
	if diag != nil {
		payload["incident_id"] = diag.GetIncidentId()
		payload["root_cause"] = diag.GetRootCause()
	}
	globular.PublishEvent("operation.restart_requested", payload)

	logger.Info("restart_service: request published", "target", target)
	return fmt.Sprintf("restart requested for %s", target), nil
}

func (b *restartServiceBackend) Verify(ctx context.Context, target string) (bool, string, error) {
	// Restart is dispatched via event bus to the cluster controller, which runs a
	// workflow-tracked restart through the node agent. We cannot confirm the outcome
	// here without polling the specific node's service state via a node-agent RPC —
	// a cluster-wide health check would pass even if the restarted service is still
	// down on its specific node (H2 fix: removing a false-positive that returned
	// verified whenever ANY node was healthy regardless of the target service).
	return false, fmt.Sprintf("inconclusive: restart of %s dispatched via event bus; confirm via node-agent or cluster doctor", target), nil
}

// --- DrainEndpointBackend: tells ai_router to drain an endpoint ---

type drainEndpointBackend struct{}

func (b *drainEndpointBackend) Supports(t ai_executorpb.ActionType) bool {
	return t == ai_executorpb.ActionType_ACTION_DRAIN_ENDPOINT
}

func (b *drainEndpointBackend) Execute(ctx context.Context, target string, _ *ai_executorpb.Diagnosis) (string, error) {
	// Publish drain request for ai_router to consume.
	globular.PublishEvent("operation.drain_requested", map[string]interface{}{
		"severity": "WARNING",
		"target":   target,
		"source":   "ai_executor",
	})

	logger.Info("drain_endpoint: request published", "target", target)
	return fmt.Sprintf("drain requested for %s", target), nil
}

func (b *drainEndpointBackend) Verify(ctx context.Context, target string) (bool, string, error) {
	// Drain verification is async — the ai_router handles the grace period.
	// We cannot confirm the drain completed; report as inconclusive.
	return false, "inconclusive: drain is async, cannot confirm completion", nil
}

// --- BlockIPBackend: requests IP blocking via the event bus ---

type blockIPBackend struct{}

func (b *blockIPBackend) Supports(t ai_executorpb.ActionType) bool {
	return t == ai_executorpb.ActionType_ACTION_BLOCK_IP
}

func (b *blockIPBackend) Execute(ctx context.Context, target string, diag *ai_executorpb.Diagnosis) (string, error) {
	payload := map[string]interface{}{
		"severity": "ERROR",
		"target":   target,
		"source":   "ai_executor",
	}
	if diag != nil {
		payload["incident_id"] = diag.GetIncidentId()
		payload["root_cause"] = diag.GetRootCause()
	}
	globular.PublishEvent("operation.block_ip_requested", payload)
	logger.Info("block_ip: request published", "target", target)
	return fmt.Sprintf("block IP requested for %s", target), nil
}

func (b *blockIPBackend) Verify(_ context.Context, target string) (bool, string, error) {
	// IP blocking is handled by the network layer (iptables/firewall) via event
	// subscription; we cannot confirm it completed from here.
	return false, fmt.Sprintf("inconclusive: block_ip for %s is fire-and-forget via event bus", target), nil
}

// --- CircuitBreakerBackend: tightens circuit breakers via ai_router ---

type circuitBreakerBackend struct{}

func (b *circuitBreakerBackend) Supports(t ai_executorpb.ActionType) bool {
	return t == ai_executorpb.ActionType_ACTION_TIGHTEN_CIRCUIT_BREAKER
}

func (b *circuitBreakerBackend) Execute(ctx context.Context, target string, _ *ai_executorpb.Diagnosis) (string, error) {
	globular.PublishEvent("operation.circuit_breaker_tighten", map[string]interface{}{
		"severity": "WARNING",
		"target":   target,
		"source":   "ai_executor",
	})
	return fmt.Sprintf("circuit breaker tighten requested for %s", target), nil
}

func (b *circuitBreakerBackend) Verify(ctx context.Context, target string) (bool, string, error) {
	// Cannot confirm circuit breaker state changed; report as inconclusive.
	return false, "inconclusive: circuit breaker adjustment is fire-and-forget", nil
}

// --- NotifyAdminBackend: sends notification (always succeeds) ---

type notifyAdminBackend struct{}

func (b *notifyAdminBackend) Supports(t ai_executorpb.ActionType) bool {
	return t == ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN
}

func (b *notifyAdminBackend) Execute(ctx context.Context, target string, diagnosis *ai_executorpb.Diagnosis) (string, error) {
	globular.PublishEvent("alert.admin.notification", map[string]interface{}{
		"severity":        "ERROR",
		"target":          target,
		"root_cause":      diagnosis.GetRootCause(),
		"proposed_action": diagnosis.GetProposedAction(),
		"confidence":      diagnosis.GetConfidence(),
		"source":          "ai_executor",
	})
	return "admin notification sent", nil
}

func (b *notifyAdminBackend) Verify(ctx context.Context, target string) (bool, string, error) {
	return true, "notification delivered to event bus", nil
}

