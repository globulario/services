// @awareness namespace=globular.platform
// @awareness component=platform_ai_executor.diagnoser
// @awareness file_role=ai_diagnosis_with_memory_first_fallback_and_high_risk_safety_gate
// @awareness implements=globular.platform:intent.ai.high_risk_diagnosis_always_escalates_to_notify_admin
// @awareness implements=globular.platform:intent.ai.memory_queried_before_claude_to_boost_confidence
// @awareness implements=globular.platform:intent.ai.supplementary_not_required
// @awareness risk=high
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
// Prefers direct Anthropic API when configured, falls back to CLI.
type diagnoser struct {
	controllerAddr string
	memoryAddr     string
	claude         *claudeClient
	anthropic      *anthropicClient
	ledger         *incidentLedger
}

func newDiagnoser(cfg AnthropicConfig) *diagnoser {
	return &diagnoser{
		claude:    newClaudeClient(),
		anthropic: newAnthropicClient(cfg),
		ledger:    &incidentLedger{},
	}
}

// sendPrompt calls the configured Anthropic API backend, or errors so the
// caller falls back to deterministic analysis.
//
// The autonomous incident path deliberately does NOT fall back to the Claude
// CLI: the CLI spends whatever interactive subscription happens to be logged in
// on the host (a developer's personal Max account), and an incident storm turns
// that into an unbounded, silent drain. AI diagnosis here is opt-in via an
// explicit, separately-billed API key in service config (Anthropic.ApiKey).
// With no key, isAvailable() is false and diagnose() uses the deterministic
// analyzer — AI is supplementary, never required.
func (d *diagnoser) sendPrompt(ctx context.Context, prompt string) (string, error) {
	if d.anthropic != nil && d.anthropic.isAvailable() {
		return d.anthropic.sendPrompt(ctx, prompt)
	}
	return "", fmt.Errorf("no AI backend available (set Anthropic.ApiKey in service config)")
}

// diagnose gathers context and builds a diagnosis for an incident.
// Uses Claude API when available, deterministic fallback otherwise.
//
func (d *diagnoser) diagnose(ctx context.Context, req *ai_executorpb.ProcessIncidentRequest) (*ai_executorpb.Diagnosis, error) {
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
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

	// 3. Dedup gate: if we have already diagnosed this incident signature,
	// reuse the prior diagnosis and skip the LLM entirely. This is what stops
	// a workflow that fails every ~90s from re-diagnosing the identical
	// signature hundreds of times a day. fromLedger records whether the answer
	// came from a prior diagnosis (so we don't re-persist it below).
	fingerprint := incidentFingerprint(req)
	var proposedAction, actionReason string
	fromLedger := false
	if d.ledger != nil {
		if entry := d.ledger.lookup(callCtx, fingerprint); entry != nil {
			rootCause = entry.rootCause
			confidence = entry.confidence
			proposedAction = entry.proposedAction
			actionReason = entry.actionReason
			fromLedger = true
			evidence = append(evidence, fmt.Sprintf(
				"recurring incident signature (seen %dx) — reusing prior diagnosis, skipping AI call",
				entry.occurrences+1))
			d.ledger.recordRepeat(callCtx, entry)
		}
	}

	// 4. Try the AI backend for a first-time analysis (falls back to
	// deterministic). Only the dedicated Anthropic API key counts as available
	// here — the CLI subscription path is intentionally excluded.
	aiAvailable := d.anthropic != nil && d.anthropic.isAvailable()
	if aiAvailable && rootCause == "" {
		healthStr := ""
		if health != nil {
			healthStr = fmt.Sprintf("%d healthy, %d unhealthy, %d unknown nodes",
				health.HealthyNodes, health.UnhealthyNodes, health.UnknownNodes)
		}

		prompt := buildAnalysisPrompt(req, evidence, healthStr)
		response, err := d.sendPrompt(callCtx, prompt)
		if err == nil {
			analysis, parseErr := parseAnalysis(response)
			if parseErr == nil && analysis != nil {
				rootCause = analysis.RootCause
				confidence = float32(analysis.Confidence)
				proposedAction = analysis.ProposedAction
				actionReason = analysis.Rationale
				evidence = append(evidence, "ai_analysis: "+analysis.Summary)
				if analysis.Detail != "" {
					evidence = append(evidence, "ai_detail: "+analysis.Detail)
				}
				if analysis.RiskLevel == "high" && proposedAction != "observe_and_record" && proposedAction != "notify_admin" {
					actionReason = actionReason + " [SAFETY: high risk, escalated to notify]"
					proposedAction = "notify_admin"
				}
				logger.Info("ai_diagnosis",
					"incident", req.GetIncidentId(),
					"root_cause", rootCause,
					"confidence", confidence,
					"proposed_action", proposedAction,
					"risk_level", analysis.RiskLevel,
				)
			} else {
				// Got a response but couldn't parse structured output.
				evidence = append(evidence, "ai_response: "+response)
			}
		} else {
			logger.Warn("AI analysis failed, using deterministic fallback", "err", err)
			evidence = append(evidence, "ai: unavailable, using deterministic analysis")
		}
	}

	// 5. Deterministic fallback if no AI answer and not a recurring signature.
	if rootCause == "" {
		rootCause, confidence = d.analyzeEventPattern(eventName, eventPayload, req.GetEventBatch())
	}
	if proposedAction == "" {
		proposedAction, actionReason = d.proposeAction(ruleID, rootCause, eventPayload)
	}

	// Extract service identity from the event payload for human-readable messages.
	svcName, _ := eventPayload["service"].(string)
	unitName, _ := eventPayload["unit"].(string)
	if svcName == "" && unitName != "" {
		svcName = unitName
	}

	// Enrich root cause with the specific service name if not already included.
	if svcName != "" && !strings.Contains(rootCause, svcName) {
		rootCause = fmt.Sprintf("%s (%s)", rootCause, svcName)
	}

	summary := fmt.Sprintf("%s triggered by %s", ruleID, eventName)
	if svcName != "" {
		summary = fmt.Sprintf("%s on %s", summary, svcName)
	}
	summary = fmt.Sprintf("%s (%d events in batch)", summary, len(req.GetEventBatch()))

	detail := fmt.Sprintf("Rule: %s\nTrigger: %s\nService: %s\nBatch size: %d\nRoot cause: %s\nConfidence: %.0f%%\nEvidence:\n  - %s",
		ruleID, eventName, svcName, len(req.GetEventBatch()), rootCause, confidence*100,
		strings.Join(evidence, "\n  - "))

	diagnosis := &ai_executorpb.Diagnosis{
		IncidentId:     req.GetIncidentId(),
		Summary:        summary,
		Detail:         detail,
		Evidence:       evidence,
		RootCause:      rootCause,
		Confidence:     confidence,
		ProposedAction: proposedAction,
		ActionReason:   actionReason,
		DiagnosedAtMs:  time.Now().UnixMilli(),
	}

	// 6. Persist the diagnosis under its signature so the next occurrence
	// deduplicates — for every incident, regardless of tier or whether the AI
	// ran. This is the write half of the dedup contract; without it the ledger
	// stays empty and every repeat looks new (the bug that drove the storm).
	// Skipped when the answer itself came from the ledger (already recorded).
	if d.ledger != nil && !fromLedger && rootCause != "" {
		d.ledger.recordNew(ctx, fingerprint, diagnosis)
	}

	return diagnosis, nil
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

	case eventName == "service.restart_failed":
		unit, _ := payload["unit"].(string)
		attempts, _ := payload["attempts"].(float64) // JSON numbers are float64
		lastErr, _ := payload["last_error"].(string)
		return fmt.Sprintf("service_restart_exhausted: %s (%d attempts, last_error=%s)", unit, int(attempts), lastErr), 0.7

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

	case "service-restart-exhausted":
		unit, _ := payload["unit"].(string)
		svc, _ := payload["service"].(string)
		lastErr, _ := payload["last_error"].(string)
		// Diagnostic chain: check logs, certs, config, deps
		actions := []string{
			fmt.Sprintf("check_service_logs:%s", unit),
			"check_certificate_status",
		}
		if svc != "" {
			actions = append(actions, fmt.Sprintf("check_service_config:%s", svc))
		}
		actions = append(actions, "check_cluster_health")
		// Classify the error for targeted remediation
		switch {
		case strings.Contains(lastErr, "exit-code") || strings.Contains(lastErr, "203") || strings.Contains(lastErr, "126"):
			actions = append(actions, "escalate:requires_approval")
			return strings.Join(actions, " + "),
				"Service restart exhausted with permission/exec error — needs manual investigation"
		case strings.Contains(lastErr, "timeout"):
			actions = append(actions, "check_dependencies + retry_after_delay")
			return strings.Join(actions, " + "),
				"Service restart exhausted with timeout — likely dependency issue"
		default:
			actions = append(actions, "investigate_and_escalate")
			return strings.Join(actions, " + "),
				"Service restart exhausted — diagnose root cause via logs and config"
		}

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

	baseOpts, err := globular.InternalDialOptions()
	if err != nil {
		return nil
	}
	opts := append(baseOpts, grpc.WithTimeout(2*time.Second))
	cc, err := grpc.Dial(addr, opts...)
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

	baseOpts2, err := globular.InternalDialOptions()
	if err != nil {
		return nil, fmt.Errorf("internal TLS: %w", err)
	}
	opts2 := append(baseOpts2, grpc.WithTimeout(2*time.Second))
	cc, err := grpc.Dial(addr, opts2...)
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	return client.GetClusterHealth(ctx, &cluster_controllerpb.GetClusterHealthRequest{})
}
