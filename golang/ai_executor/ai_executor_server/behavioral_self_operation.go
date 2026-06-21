package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
)

const (
	sourceKindAgentToolAttempt      = "agent_tool_attempt"
	sourceKindAgentBlockedAction    = "agent_blocked_action"
	sourceKindAgentClassifierDenial = "agent_classifier_denial"
	sourceKindAgentSafetyHookEvent  = "agent_safety_hook_event"
)

type behavioralSelfOperation struct {
	sourceKind    string
	signalKind    bpb.SignalKind
	outcomeStatus string
	theme         string
	severity      string
	incidentID    string
	actionType    string
	target        string
	note          string
	reason        string
	metadata      map[string]string
}

var emitBehavioralSelfOperation = func(ctx context.Context, event behavioralSelfOperation) {
	recordBehavioralSelfOperation(ctx, event)
}

func recordBehavioralSelfOperation(ctx context.Context, event behavioralSelfOperation) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("behavioral self-operation feed panicked (suppressed)", "recover", r)
		}
	}()

	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return
	}
	baseOpts, err := globular.InternalDialOptions()
	if err != nil {
		return
	}
	// Lazy dial (no WithBlock); WithTimeout inert, per-RPC ctx carries the deadline.
	// Not migrated to grpc.NewClient in a lint cleanup — it changes default target
	// resolution (dns vs passthrough). Future: NewClient + drop WithTimeout.
	opts := append(baseOpts, grpc.WithTimeout(2*time.Second)) //nolint:staticcheck // see note
	cc, err := grpc.Dial(addr, opts...)                       //nolint:staticcheck // see note
	if err != nil {
		return
	}
	defer func() { _ = cc.Close() }()

	client := bpb.NewBehavioralMemoryServiceClient(cc)
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	now := time.Now().Unix()
	clusterID, _ := security.GetLocalClusterID()
	md := cloneMetadata(event.metadata)
	md["agent_id"] = behavioralAgentID
	if event.actionType != "" {
		md["action_type"] = event.actionType
	}
	if event.reason != "" {
		md["reason"] = event.reason
	}

	sigRsp, err := client.RecordSignal(callCtx, &bpb.RecordSignalRequest{
		Signal: &bpb.Signal{
			Project:        behavioralProject,
			Domain:         behavioralDomain,
			Kind:           event.signalKind,
			SourceKind:     event.sourceKind,
			SourceRef:      fmt.Sprintf("ai_executor:self_operation:%s:%s:%d", event.sourceKind, event.incidentID, now),
			EntityRef:      firstNonEmpty(event.target, event.actionType, event.incidentID),
			Scope:          clusterID,
			ClusterId:      clusterID,
			Severity:       firstNonEmpty(event.severity, "info"),
			AuthorityLevel: bpb.ObservationAuthorityLevel_OBSERVATION_AUTHORITY_LEVEL_INTERPRETATION,
			ObservedAt:     now,
			Payload:        firstNonEmpty(event.note, event.reason),
			AgentId:        behavioralAgentID,
			Status:         bpb.GovernanceStatus_RAW_SIGNAL,
			CreatedAt:      now,
			Metadata:       md,
		},
	})
	if err != nil {
		return
	}
	if event.outcomeStatus == "" {
		return
	}
	md["signal_id"] = sigRsp.GetSignalId()
	_, _ = client.RecordOutcome(callCtx, &bpb.RecordOutcomeRequest{
		Outcome: &bpb.Outcome{
			Project:    behavioralProject,
			Domain:     behavioralDomain,
			Status:     event.outcomeStatus,
			IncidentId: event.incidentID,
			Theme:      event.theme,
			AgentId:    behavioralAgentID,
			Note:       firstNonEmpty(event.note, event.reason),
			Metadata:   md,
		},
	})
}

func newSelfOperationAttempt(job *ai_executorpb.Job) behavioralSelfOperation {
	if job == nil {
		return behavioralSelfOperation{}
	}
	diag := job.GetDiagnosis()
	action := canonicalSelfOperationAction(firstNonEmpty(job.GetActionTarget(), diag.GetProposedAction()))
	return behavioralSelfOperation{
		sourceKind: sourceKindAgentToolAttempt,
		signalKind: bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT,
		theme:      "agent.self_operation.attempt." + action,
		severity:   "info",
		incidentID: job.GetIncidentId(),
		actionType: action,
		note:       fmt.Sprintf("agent attempted %s for incident %s", action, job.GetIncidentId()),
		metadata: map[string]string{
			"job_state":     job.GetState().String(),
			"tier":          fmt.Sprintf("%d", job.GetTier()),
			"root_cause":    diag.GetRootCause(),
			"summary":       diag.GetSummary(),
			"action_reason": diag.GetActionReason(),
		},
	}
}

func newBlockedSelfOperation(job *ai_executorpb.Job) behavioralSelfOperation {
	if job == nil {
		return behavioralSelfOperation{}
	}
	diag := job.GetDiagnosis()
	action := canonicalSelfOperationAction(firstNonEmpty(job.GetActionTarget(), diag.GetProposedAction()))
	note := fmt.Sprintf("agent action %s was blocked for incident %s", action, job.GetIncidentId())
	if job.GetDeniedReason() != "" {
		note = note + ": " + job.GetDeniedReason()
	}
	return behavioralSelfOperation{
		sourceKind:    sourceKindAgentBlockedAction,
		signalKind:    bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT,
		outcomeStatus: "blocked",
		theme:         "agent.self_operation.blocked." + action,
		severity:      "warning",
		incidentID:    job.GetIncidentId(),
		actionType:    action,
		note:          note,
		reason:        job.GetDeniedReason(),
		metadata: map[string]string{
			"denied_by":     job.GetDeniedBy(),
			"denied_reason": job.GetDeniedReason(),
			"job_state":     job.GetState().String(),
			"tier":          fmt.Sprintf("%d", job.GetTier()),
			"root_cause":    diag.GetRootCause(),
		},
	}
}

func newClassifierDenialSelfOperation(req *ai_executorpb.PeerProposalRequest, vote ai_executorpb.PeerVote, reason string) behavioralSelfOperation {
	action := behavioralActionVerb(req.GetProposedAction())
	return behavioralSelfOperation{
		sourceKind:    sourceKindAgentClassifierDenial,
		signalKind:    bpb.SignalKind_SIGNAL_AGENT_INTERPRETATION,
		outcomeStatus: "blocked",
		theme:         "agent.self_operation.classifier_denial." + action,
		severity:      "warning",
		incidentID:    req.GetDiagnosis().GetIncidentId(),
		actionType:    action,
		target:        req.GetTarget(),
		note:          fmt.Sprintf("peer proposal %s was %s by classifier", action, strings.ToLower(vote.String())),
		reason:        reason,
		metadata: map[string]string{
			"vote":       vote.String(),
			"target":     req.GetTarget(),
			"tier":       fmt.Sprintf("%d", req.GetTier()),
			"root_cause": req.GetDiagnosis().GetRootCause(),
		},
	}
}

func newSafetyHookSelfOperation(req *ai_executorpb.PeerProposalRequest, reason string) behavioralSelfOperation {
	action := behavioralActionVerb(req.GetProposedAction())
	return behavioralSelfOperation{
		sourceKind:    sourceKindAgentSafetyHookEvent,
		signalKind:    bpb.SignalKind_SIGNAL_AGENT_INTERPRETATION,
		outcomeStatus: "blocked",
		theme:         "agent.self_operation.safety_hook." + action,
		severity:      "warning",
		incidentID:    req.GetDiagnosis().GetIncidentId(),
		actionType:    action,
		target:        req.GetTarget(),
		note:          fmt.Sprintf("safety hook escalated %s for human approval", action),
		reason:        reason,
		metadata: map[string]string{
			"target":     req.GetTarget(),
			"tier":       fmt.Sprintf("%d", req.GetTier()),
			"root_cause": req.GetDiagnosis().GetRootCause(),
		},
	}
}

func canonicalSelfOperationAction(action string) string {
	action = strings.TrimSpace(strings.ToLower(action))
	if action == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_", "/", "_")
	return replacer.Replace(action)
}

func cloneMetadata(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+4)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
