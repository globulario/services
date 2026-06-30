// @awareness namespace=globular.platform
// @awareness component=platform_ai_executor.behavioral_feed
// @awareness file_role=afferent_nerve_feeds_executor_experience_into_behavioral_memory
// @awareness implements=globular.platform:intent.ai.watcher_to_executor_causal_chain
// @awareness implements=globular.platform:intent.ai.supplementary_not_required
// @awareness implements=globular.platform:intent.knowledge.promote_incidents_into_graph
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	bpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// Behavioral-memory lives in the same binary as ai_memory and is reached at the
// same service address. Experience is recorded under the cluster-operator domain.
const (
	behavioralProject = "globular-services"
	behavioralDomain  = "cluster_operator"
	behavioralAgentID = "ai_executor"
)

var emitBehavioralExperience = func(ctx context.Context, diagnosis *ai_executorpb.Diagnosis, action *ai_executorpb.RemediationAction) {
	recordBehavioralExperience(ctx, diagnosis, action)
}

func newBehavioralTraceAction(diagnosis *ai_executorpb.Diagnosis, status ai_executorpb.ActionStatus) *ai_executorpb.RemediationAction {
	if diagnosis == nil {
		return &ai_executorpb.RemediationAction{Status: status}
	}
	proposed := diagnosis.GetProposedAction()
	return &ai_executorpb.RemediationAction{
		IncidentId: diagnosis.GetIncidentId(),
		Type:       classifyAction(proposed),
		Status:     status,
		Target:     proposed,
		Detail:     diagnosis.GetActionReason(),
	}
}

// recordBehavioralExperience is the afferent nerve from the executor's runtime
// experience into the governed behavioral-memory ladder:
//
//	Signal (the observed incident) → advisory CheckAction (what the gate would say)
//	→ Outcome (what actually happened) — linked via action_check_id.
//
// It is STRICTLY non-fatal and best-effort. Behavioral-memory is an AI-supplementary
// surface (intent.ai.supplementary_not_required): if it is unreachable, slow, or
// buggy, remediation must proceed unchanged. Hence: a recover() guard (an unrecovered
// panic in this goroutine would otherwise take down the executor), a short dial
// timeout, and silent degradation on every error — exactly how recordOutcome already
// treats flat ai_memory.
//
// The CheckAction here is ADVISORY today: no cluster-operator principles are promoted
// yet, so it returns "allowed" and never blocks. Its value now is the audit row the
// outcome links to (bm:resultedFrom) and seeding the evidence base; when principles
// are promoted, this same call becomes a real pre-execution gate (a follow-up moves
// it ahead of dispatch). This file performs NO os/exec and NO etcd writes — it only
// makes gRPC calls to BehavioralMemoryService.
func recordBehavioralExperience(ctx context.Context, diagnosis *ai_executorpb.Diagnosis, action *ai_executorpb.RemediationAction) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("behavioral feed panicked (suppressed — executor unaffected)", "recover", r)
		}
	}()

	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		logger.Warn("behavioral feed skipped: ai_memory address unresolved")
		return // behavioral-memory not resolvable → degrade silently
	}
	baseOpts, err := globular.InternalDialOptions()
	if err != nil {
		logger.Warn("behavioral feed skipped: internal dial options unavailable", "err", err)
		return
	}
	// grpc.Dial/WithTimeout are deprecated. NOT migrated to grpc.NewClient here:
	//  - semantics: this dial has no WithBlock, so it is already lazy; WithTimeout
	//    is inert without WithBlock, and the real deadline is the per-RPC
	//    context.WithTimeout below. So the OLD code does NOT depend on
	//    eager/blocking dial behavior.
	//  - why NewClient would still change behavior: it switches default target
	//    resolution (dns vs Dial's passthrough) — a runtime change we will not make
	//    unvalidated in this high-risk observer path during a lint-only cleanup.
	//  - future migration: grpc.NewClient(addr, baseOpts...), drop WithTimeout,
	//    validated against the mesh resolver.
	opts := append(baseOpts, grpc.WithTimeout(2*time.Second)) //nolint:staticcheck // see note above
	cc, err := grpc.Dial(addr, opts...)                       //nolint:staticcheck // see note above
	if err != nil {
		logger.Warn("behavioral feed skipped: dial failed", "addr", addr, "err", err)
		return
	}
	defer func() { _ = cc.Close() }()

	client := bpb.NewBehavioralMemoryServiceClient(cc)
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	incidentID := diagnosis.GetIncidentId()
	rootCause := diagnosis.GetRootCause()
	target := action.GetTarget()

	// 1. Signal — the observed runtime fact the executor diagnosed.
	if _, err := client.RecordSignal(callCtx, &bpb.RecordSignalRequest{
		Signal: &bpb.Signal{
			Project:    behavioralProject,
			Domain:     behavioralDomain,
			Kind:       bpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT,
			SourceKind: "agent",
			SourceRef:  "ai_executor:incident:" + incidentID,
			EntityRef:  target,
			Payload:    diagnosis.GetSummary(),
			Confidence: diagnosis.GetConfidence(),
			AgentId:    behavioralAgentID,
			ObservedAt: time.Now().Unix(),
			Metadata: map[string]string{
				"incident_id": incidentID,
				"root_cause":  rootCause,
			},
		},
	}); err != nil {
		logger.Warn("behavioral feed signal record failed", "incident", incidentID, "err", err)
		return // behavioral-memory unreachable → stop here, silently
	}

	// 2. Advisory CheckAction — records what the gate would say + the audit row the
	//    outcome links to. Non-blocking today.
	actionVerb := behavioralActionVerb(action.GetType())
	actionCheckID := ""
	if chk, err := client.CheckAction(callCtx, &bpb.CheckActionRequest{
		Project:    behavioralProject,
		Domain:     behavioralDomain,
		ActionType: actionVerb,
		Target:     target,
		AgentId:    behavioralAgentID,
	}); err == nil && chk.GetResult() != nil {
		actionCheckID = chk.GetResult().GetId()
		if chk.GetResult().GetStatus() != "allowed" {
			logger.Warn("behavioral gate (advisory) would not allow remediation",
				"incident", incidentID, "action", actionVerb,
				"verdict", chk.GetResult().GetStatus(),
				"missing_evidence", chk.GetResult().GetMissingEvidence())
		}
	} else if err != nil {
		logger.Warn("behavioral feed action check failed", "incident", incidentID, "action", actionVerb, "err", err)
	}

	// 3. Outcome — what actually happened (the learning feedback). Only for taken
	//    actions with a terminal result; observe-only / pending record no outcome.
	status := behavioralOutcomeStatus(action.GetStatus())
	if status == "" {
		return
	}
	if _, err := client.RecordOutcome(callCtx, &bpb.RecordOutcomeRequest{
		Outcome: &bpb.Outcome{
			Project:       behavioralProject,
			Domain:        behavioralDomain,
			ActionCheckId: actionCheckID,
			Status:        status,
			IncidentId:    incidentID,
			Theme:         rootCause,
			AgentId:       behavioralAgentID,
			Note:          fmt.Sprintf("%s %s → %s", action.GetType().String(), target, action.GetStatus().String()),
		},
	}); err != nil {
		logger.Warn("behavioral feed outcome record failed", "incident", incidentID, "err", err)
	}
}

// behavioralActionVerb maps the executor's ActionType to a behavioral action_type
// token (the vocabulary forbidden-moves / principles match on as the pack grows).
func behavioralActionVerb(t ai_executorpb.ActionType) string {
	switch t {
	case ai_executorpb.ActionType_ACTION_RESTART_SERVICE:
		return "restart_service"
	case ai_executorpb.ActionType_ACTION_CLEAR_STORAGE:
		return "clear_storage"
	case ai_executorpb.ActionType_ACTION_RENEW_CERT:
		return "renew_cert"
	case ai_executorpb.ActionType_ACTION_BLOCK_IP:
		return "block_ip"
	case ai_executorpb.ActionType_ACTION_DRAIN_ENDPOINT:
		return "drain_endpoint"
	case ai_executorpb.ActionType_ACTION_NOTIFY_ADMIN:
		return "notify_admin"
	default:
		return "observe"
	}
}

// behavioralOutcomeStatus maps a terminal ActionStatus to a behavioral outcome
// status (success|failure|blocked|reverted), or "" when there is no outcome to
// record yet (pending/executing/observe-only).
func behavioralOutcomeStatus(s ai_executorpb.ActionStatus) string {
	switch s {
	case ai_executorpb.ActionStatus_ACTION_SUCCEEDED:
		return "success"
	case ai_executorpb.ActionStatus_ACTION_FAILED:
		return "failure"
	default:
		return ""
	}
}
