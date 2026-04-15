package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
)

// remediator executes remediation actions via real backends and verifies outcomes.
type remediator struct {
	dispatcher *actionDispatcher
}

func newRemediator() *remediator {
	return &remediator{
		dispatcher: newActionDispatcher(),
	}
}

// execute runs a remediation action based on the diagnosis and tier.
func (r *remediator) execute(ctx context.Context, diagnosis *ai_executorpb.Diagnosis, tier int32) *ai_executorpb.RemediationAction {
	action := &ai_executorpb.RemediationAction{
		Id:          Utility.RandomUUID(),
		IncidentId:  diagnosis.GetIncidentId(),
		Target:      diagnosis.GetProposedAction(),
		Detail:      diagnosis.GetActionReason(),
		StartedAtMs: time.Now().UnixMilli(),
	}

	// Determine action type from proposed action string.
	proposed := diagnosis.GetProposedAction()
	action.Type = classifyAction(proposed)

	// Tier 0 (observe): record only, don't execute.
	if tier == 0 {
		action.Status = ai_executorpb.ActionStatus_ACTION_SKIPPED
		action.Detail = "Tier 1 (observe): diagnosed and recorded, no execution"
		action.CompletedAtMs = time.Now().UnixMilli()
		go r.recordOutcome(ctx, diagnosis, action)
		return action
	}

	// Tier 2 (approval required): don't execute yet, wait for approval.
	if tier == 2 {
		action.Status = ai_executorpb.ActionStatus_ACTION_PENDING
		action.Detail = "Tier 3 (approval required): awaiting human approval"
		return action
	}

	// Tier 1 (auto-remediate): execute via real backend.
	action.Status = ai_executorpb.ActionStatus_ACTION_EXECUTING

	logger.Info("executing remediation",
		"incident", diagnosis.GetIncidentId(),
		"action_type", action.Type.String(),
		"target", action.Target,
	)

	// Dispatch to real backend with verification.
	result, err := r.dispatcher.dispatch(ctx, action, diagnosis)
	if err != nil {
		action.Status = ai_executorpb.ActionStatus_ACTION_FAILED
		action.Error = err.Error()
		action.CompletedAtMs = time.Now().UnixMilli()
		logger.Error("remediation failed",
			"incident", diagnosis.GetIncidentId(),
			"action_type", action.Type.String(),
			"err", err,
		)
	} else {
		action.Status = ai_executorpb.ActionStatus_ACTION_SUCCEEDED
		action.Detail = result
		action.CompletedAtMs = time.Now().UnixMilli()
		logger.Info("remediation succeeded",
			"incident", diagnosis.GetIncidentId(),
			"action_type", action.Type.String(),
			"result", result,
		)
	}

	// Publish outcome event.
	go func() {
		msg := fmt.Sprintf("%s %s → %s", action.GetType().String(), action.GetTarget(), action.GetStatus().String())
		globular.PublishEvent("operation.remediation.completed", map[string]interface{}{
			"severity":    statusSeverity(action.Status),
			"message":     msg,
			"incident_id": action.GetIncidentId(),
			"action_type": action.GetType().String(),
			"status":      action.GetStatus().String(),
			"target":      action.GetTarget(),
			"result":      action.GetDetail(),
			"error":       action.GetError(),
			"service":     "ai_executor",
		})
	}()

	// Record to ai_memory.
	go r.recordOutcome(ctx, diagnosis, action)

	return action
}

// classifyAction maps a proposed action string to an ActionType.
func classifyAction(proposed string) ai_executorpb.ActionType {
	switch {
	case proposed == "observe_and_record":
		return ai_executorpb.ActionType_ACTION_NONE
	case strings.Contains(proposed, "restart_service"):
		return ai_executorpb.ActionType_ACTION_RESTART_SERVICE
	case strings.Contains(proposed, "block_ip"):
		return ai_executorpb.ActionType_ACTION_BLOCK_IP
	case strings.Contains(proposed, "drain_endpoint"):
		return ai_executorpb.ActionType_ACTION_DRAIN_ENDPOINT
	case strings.Contains(proposed, "clear"):
		return ai_executorpb.ActionType_ACTION_CLEAR_STORAGE
	case strings.Contains(proposed, "renew_cert"):
		return ai_executorpb.ActionType_ACTION_RENEW_CERT
	case strings.Contains(proposed, "notify"), strings.Contains(proposed, "alert_admin"):
		return ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN
	default:
		return ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN
	}
}

func statusSeverity(s ai_executorpb.ActionStatus) string {
	switch s {
	case ai_executorpb.ActionStatus_ACTION_SUCCEEDED:
		return "INFO"
	case ai_executorpb.ActionStatus_ACTION_FAILED:
		return "ERROR"
	default:
		return "WARNING"
	}
}

// recordOutcome stores the incident diagnosis and action in ai_memory.
func (r *remediator) recordOutcome(ctx context.Context, diagnosis *ai_executorpb.Diagnosis, action *ai_executorpb.RemediationAction) {
	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return
	}

	baseOpts, err := globular.InternalDialOptions()
	if err != nil {
		logger.Error("internal TLS unavailable for memory store", "err", err)
		return
	}
	opts := append(baseOpts, grpc.WithTimeout(2*time.Second))
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return
	}
	defer cc.Close()

	content, _ := json.Marshal(map[string]interface{}{
		"incident_id":     diagnosis.GetIncidentId(),
		"summary":         diagnosis.GetSummary(),
		"root_cause":      diagnosis.GetRootCause(),
		"confidence":      diagnosis.GetConfidence(),
		"proposed_action": diagnosis.GetProposedAction(),
		"action_type":     action.GetType().String(),
		"action_status":   action.GetStatus().String(),
		"action_result":   action.GetDetail(),
		"action_error":    action.GetError(),
		"evidence":        diagnosis.GetEvidence(),
		"verified":        action.GetStatus() == ai_executorpb.ActionStatus_ACTION_SUCCEEDED,
	})

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := ai_memorypb.NewAiMemoryServiceClient(cc)
	_, _ = client.Store(callCtx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project: "globular-services",
			Type:    ai_memorypb.MemoryType_DEBUG,
			Title:   fmt.Sprintf("incident: %s → %s (%s)", diagnosis.GetRootCause(), action.GetType(), action.GetStatus()),
			Content: string(content),
			Tags:    []string{"incident", "remediation", diagnosis.GetRootCause()},
			Metadata: map[string]string{
				"root_cause":    diagnosis.GetRootCause(),
				"action":        action.GetType().String(),
				"action_status": action.GetStatus().String(),
				"confidence":    fmt.Sprintf("%.2f", diagnosis.GetConfidence()),
				"verified":      fmt.Sprintf("%v", action.GetStatus() == ai_executorpb.ActionStatus_ACTION_SUCCEEDED),
			},
		},
	})
}
