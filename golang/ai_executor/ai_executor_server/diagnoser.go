package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// diagnoser gathers evidence from cluster services, then uses Claude
// for reasoning when available, with deterministic fallback.
type diagnoser struct {
	controllerAddr string
	memoryAddr     string
	claude         *claudeClient
}

func newDiagnoser() *diagnoser {
	return &diagnoser{
		claude: newClaudeClient(),
	}
}

// diagnose gathers context and builds a diagnosis for an incident.
func (d *diagnoser) diagnose(ctx context.Context, req *ai_executorpb.ProcessIncidentRequest) (*ai_executorpb.Diagnosis, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var evidence []string
	var rootCause string
	var confidence float32

	eventName := req.GetTriggerEventName()
	ruleID := req.GetRuleId()

	// Parse trigger event data for context.
	var eventPayload map[string]interface{}
	if len(req.GetTriggerEventData()) > 0 {
		_ = json.Unmarshal(req.GetTriggerEventData(), &eventPayload)
	}

	// 1. Gather cluster health.
	health, err := d.getClusterHealth(callCtx)
	if err == nil && health != nil {
		healthEvidence := fmt.Sprintf("cluster health: %d healthy, %d unhealthy, %d unknown nodes",
			health.HealthyNodes, health.UnhealthyNodes, health.UnknownNodes)
		evidence = append(evidence, healthEvidence)

		if health.UnhealthyNodes > 0 {
			for _, nh := range health.NodeHealth {
				if nh.Status == "unhealthy" {
					evidence = append(evidence, fmt.Sprintf("node %s unhealthy: %s", nh.NodeId, nh.LastError))
				}
			}
		}
	} else {
		evidence = append(evidence, "cluster health: unavailable")
	}

	// 2. Query ai_memory for similar past incidents.
	pastIncidents := d.queryPastIncidents(callCtx, ruleID, eventName)
	if len(pastIncidents) > 0 {
		evidence = append(evidence, fmt.Sprintf("found %d similar past incidents", len(pastIncidents)))
		// If we've seen this before with a known root cause, boost confidence.
		for _, past := range pastIncidents {
			if past.rootCause != "" {
				rootCause = past.rootCause
				confidence = 0.7
				evidence = append(evidence, fmt.Sprintf("past root cause: %s", past.rootCause))
				break
			}
		}
	}

	// 3. Try Claude for intelligent analysis (falls back to deterministic).
	var proposedAction, actionReason string

	if d.claude.isAvailable() && rootCause == "" {
		healthStr := ""
		if health != nil {
			healthStr = fmt.Sprintf("%d healthy, %d unhealthy, %d unknown nodes",
				health.HealthyNodes, health.UnhealthyNodes, health.UnknownNodes)
		}

		analysis, err := d.claude.analyzeIncident(callCtx, req, evidence, healthStr)
		if err == nil && analysis != nil {
			rootCause = analysis.RootCause
			confidence = float32(analysis.Confidence)
			proposedAction = analysis.ProposedAction
			actionReason = analysis.Rationale
			evidence = append(evidence, "claude_analysis: "+analysis.Summary)
			if analysis.Detail != "" {
				evidence = append(evidence, "claude_detail: "+analysis.Detail)
			}
			if analysis.RiskLevel == "high" && proposedAction != "observe_and_record" && proposedAction != "notify_admin" {
				// Safety: Claude said high risk — escalate to notify instead of auto-fix.
				actionReason = actionReason + " [SAFETY: high risk, escalated to notify]"
				proposedAction = "notify_admin"
			}
			logger.Info("claude_diagnosis",
				"incident", req.GetIncidentId(),
				"root_cause", rootCause,
				"confidence", confidence,
				"proposed_action", proposedAction,
				"risk_level", analysis.RiskLevel,
			)
		} else {
			if err != nil {
				logger.Warn("claude analysis failed, using deterministic fallback", "err", err)
				evidence = append(evidence, "claude: unavailable, using deterministic analysis")
			}
		}
	}

	// 4. Deterministic fallback if Claude didn't provide an answer.
	if rootCause == "" {
		rootCause, confidence = d.analyzeEventPattern(eventName, eventPayload, req.GetEventBatch())
	}
	if proposedAction == "" {
		proposedAction, actionReason = d.proposeAction(ruleID, rootCause, eventPayload)
	}

	summary := fmt.Sprintf("%s triggered by %s (%d events in batch)",
		ruleID, eventName, len(req.GetEventBatch()))

	detail := fmt.Sprintf("Rule: %s\nTrigger: %s\nBatch size: %d\nRoot cause: %s\nConfidence: %.0f%%\nEvidence:\n  - %s",
		ruleID, eventName, len(req.GetEventBatch()), rootCause, confidence*100,
		strings.Join(evidence, "\n  - "))

	return &ai_executorpb.Diagnosis{
		IncidentId:     req.GetIncidentId(),
		Summary:        summary,
		Detail:         detail,
		Evidence:       evidence,
		RootCause:      rootCause,
		Confidence:     confidence,
		ProposedAction: proposedAction,
		ActionReason:   actionReason,
		DiagnosedAtMs:  time.Now().UnixMilli(),
	}, nil
}

// analyzeEventPattern determines the likely root cause from the event type.
func (d *diagnoser) analyzeEventPattern(eventName string, payload map[string]interface{}, batch []string) (rootCause string, confidence float32) {
	switch {
	case strings.HasPrefix(eventName, "alert.dos"):
		return "denial_of_service_attack", 0.8

	case strings.HasPrefix(eventName, "alert.slowloris"):
		return "slowloris_connection_exhaustion", 0.7

	case strings.HasPrefix(eventName, "alert.error.spike"):
		return "service_overload_or_cascade_failure", 0.5

	case strings.HasPrefix(eventName, "alert.auth.failed"):
		account, _ := payload["account"].(string)
		if account == "sa" {
			return "brute_force_attack_on_superadmin", 0.8
		}
		return "credential_stuffing_or_misconfiguration", 0.5

	case strings.HasPrefix(eventName, "alert.auth.denied"):
		return "rbac_misconfiguration_or_unauthorized_access", 0.4

	case eventName == "service.exited":
		unit, _ := payload["unit"].(string)
		return fmt.Sprintf("service_crash: %s", unit), 0.6

	case eventName == "cluster.health.degraded":
		reason, _ := payload["reason"].(string)
		return fmt.Sprintf("node_health_degraded: %s", reason), 0.6

	case eventName == "operation.stalled":
		return "plan_execution_stuck", 0.5

	default:
		return "unknown_anomaly", 0.2
	}
}

// proposeAction suggests a remediation based on rule and root cause.
func (d *diagnoser) proposeAction(ruleID, rootCause string, payload map[string]interface{}) (action, reason string) {
	switch ruleID {
	case "service-crash":
		unit, _ := payload["unit"].(string)
		return fmt.Sprintf("restart_service:%s", unit),
			"Service exited unexpectedly — restart to restore availability"

	case "dos-detected":
		addr, _ := payload["remote_addr"].(string)
		return fmt.Sprintf("drain_endpoint:affected + block_ip:%s", addr),
			"Active DoS attack — drain affected endpoint and block source"

	case "slowloris-detected":
		addr, _ := payload["remote_addr"].(string)
		return fmt.Sprintf("reduce_max_connections + block_ip:%s", addr),
			"Slowloris attack exhausting connections — reduce limits and block source"

	case "brute-force-detect":
		return "lock_account:temporary + alert_admin",
			"Repeated login failures suggest brute force — temporary lock"

	case "error-rate-spike":
		return "tighten_circuit_breakers + investigate_logs",
			"High error rate — tighten circuit breakers to contain cascade"

	case "health-check-fail":
		return "investigate_node + attempt_recovery",
			"Node unhealthy — investigate root cause and attempt recovery"

	case "convergence-stalled":
		return "redispatch_plan + investigate_node",
			"Plan stuck — redispatch and investigate blocking condition"

	default:
		return "observe_and_record",
			"Unknown pattern — record findings for future learning"
	}
}

// pastIncidentSummary holds info from a past similar incident.
type pastIncidentSummary struct {
	rootCause string
	action    string
}

// queryPastIncidents checks ai_memory for similar incidents.
func (d *diagnoser) queryPastIncidents(ctx context.Context, ruleID, eventName string) []pastIncidentSummary {
	addr := d.memoryAddr
	if addr == "" {
		addr = config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	}
	if addr == "" {
		return nil
	}

	cc, err := grpc.Dial(addr,
		globular.InternalDialOption(),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		return nil
	}
	defer cc.Close()

	client := ai_memorypb.NewAiMemoryServiceClient(cc)
	resp, err := client.Query(ctx, &ai_memorypb.QueryRqst{
		Project:    "globular-services",
		Type:       ai_memorypb.MemoryType_DEBUG,
		Tags:       []string{"incident", ruleID},
		TextSearch: eventName,
		Limit:      5,
	})
	if err != nil || resp == nil {
		return nil
	}

	var results []pastIncidentSummary
	for _, mem := range resp.Memories {
		if rc, ok := mem.Metadata["root_cause"]; ok {
			results = append(results, pastIncidentSummary{
				rootCause: rc,
				action:    mem.Metadata["action"],
			})
		}
	}
	return results
}

// getClusterHealth queries the cluster controller for current health.
func (d *diagnoser) getClusterHealth(ctx context.Context) (*cluster_controllerpb.GetClusterHealthResponse, error) {
	addr := d.controllerAddr
	if addr == "" {
		addr = config.ResolveServiceAddr("clustercontroller.ClusterControllerService", "")
	}
	if addr == "" {
		return nil, fmt.Errorf("cluster controller not found")
	}

	cc, err := grpc.Dial(addr,
		globular.InternalDialOption(),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	return client.GetClusterHealth(ctx, &cluster_controllerpb.GetClusterHealthRequest{})
}
