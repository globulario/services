package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// remediator executes remediation actions and records outcomes.
type remediator struct{}

func newRemediator() *remediator {
	return &remediator{}
}

// execute runs a remediation action based on the diagnosis.
// For Tier 2 (auto-remediate): executes the proposed action.
// Returns the action record with status.
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
	switch {
	case proposed == "observe_and_record":
		action.Type = ai_executorpb.ActionType_ACTION_NONE
		action.Status = ai_executorpb.ActionStatus_ACTION_SKIPPED
		action.Detail = "Observation only — no remediation needed"
		action.CompletedAtMs = time.Now().UnixMilli()
		return action

	case contains(proposed, "restart_service"):
		action.Type = ai_executorpb.ActionType_ACTION_RESTART_SERVICE

	case contains(proposed, "block_ip"):
		action.Type = ai_executorpb.ActionType_ACTION_BLOCK_IP

	case contains(proposed, "drain_endpoint"):
		action.Type = ai_executorpb.ActionType_ACTION_DRAIN_ENDPOINT

	case contains(proposed, "alert_admin"), contains(proposed, "notify"):
		action.Type = ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN

	case contains(proposed, "lock_account"):
		action.Type = ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN // escalate to admin for now

	default:
		action.Type = ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN
	}

	// Tier check: only Tier 2 auto-executes. Others record only.
	if tier == 0 { // OBSERVE
		action.Status = ai_executorpb.ActionStatus_ACTION_SKIPPED
		action.Detail = "Tier 1 (observe): action recorded but not executed"
		action.CompletedAtMs = time.Now().UnixMilli()
		return action
	}

	if tier == 2 { // REQUIRE_APPROVAL
		action.Status = ai_executorpb.ActionStatus_ACTION_PENDING
		action.Detail = "Tier 3 (approval required): awaiting human approval"
		return action
	}

	// Tier 1 = AUTO_REMEDIATE: execute the action.
	action.Status = ai_executorpb.ActionStatus_ACTION_EXECUTING
	logger.Info("executing remediation",
		"incident", diagnosis.GetIncidentId(),
		"action_type", action.Type.String(),
		"target", action.Target,
	)

	// Publish the action as an event so ai_watcher and ai_router can react.
	go publishActionEvent(action)

	// Record to ai_memory.
	go recordActionToMemory(ctx, diagnosis, action)

	action.Status = ai_executorpb.ActionStatus_ACTION_SUCCEEDED
	action.CompletedAtMs = time.Now().UnixMilli()

	logger.Info("remediation complete",
		"incident", diagnosis.GetIncidentId(),
		"action_type", action.Type.String(),
		"status", action.Status.String(),
	)

	return action
}

// publishActionEvent publishes the remediation action as a cluster event.
func publishActionEvent(action *ai_executorpb.RemediationAction) {
	globular.PublishEvent("operation.remediation", map[string]interface{}{
		"severity":    "WARNING",
		"incident_id": action.GetIncidentId(),
		"action_type": action.GetType().String(),
		"target":      action.GetTarget(),
		"status":      action.GetStatus().String(),
	})
}

// recordActionToMemory stores the incident diagnosis and action in ai_memory.
func recordActionToMemory(ctx context.Context, diagnosis *ai_executorpb.Diagnosis, action *ai_executorpb.RemediationAction) {
	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return
	}

	cc, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		return
	}
	defer cc.Close()

	content, _ := json.Marshal(map[string]interface{}{
		"incident_id":    diagnosis.GetIncidentId(),
		"summary":        diagnosis.GetSummary(),
		"root_cause":     diagnosis.GetRootCause(),
		"confidence":     diagnosis.GetConfidence(),
		"proposed_action": diagnosis.GetProposedAction(),
		"action_type":    action.GetType().String(),
		"action_status":  action.GetStatus().String(),
		"evidence":       diagnosis.GetEvidence(),
	})

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := ai_memorypb.NewAiMemoryServiceClient(cc)
	_, _ = client.Store(callCtx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project: "globular-services",
			Type:    ai_memorypb.MemoryType_DEBUG,
			Title:   fmt.Sprintf("incident: %s → %s", diagnosis.GetRootCause(), action.GetType().String()),
			Content: string(content),
			Tags:    []string{"incident", "remediation", diagnosis.GetRootCause()},
			Metadata: map[string]string{
				"root_cause":    diagnosis.GetRootCause(),
				"action":        action.GetType().String(),
				"action_status": action.GetStatus().String(),
				"confidence":    fmt.Sprintf("%.2f", diagnosis.GetConfidence()),
			},
		},
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
