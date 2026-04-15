package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	eventpb "github.com/globulario/services/golang/event/eventpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
	Utility "github.com/globulario/utility"
)

// remediationConsumer subscribes to operation.restart_requested events from the
// AI executor and orchestrates restarts through the node agent, tracked as
// workflow runs. This is the controller's role as the policy and orchestration
// authority: validate → workflow → execute → verify → emit outcome.
type remediationConsumer struct {
	srv *server

	// Cooldown: prevent duplicate restarts for the same unit within a window.
	mu        sync.Mutex
	inflight  map[string]time.Time // unit → last restart time
	cooldown  time.Duration
}

const (
	remediationCooldown   = 60 * time.Second
	remediationVerifyWait = 5 * time.Second
)

func newRemediationConsumer(srv *server) *remediationConsumer {
	return &remediationConsumer{
		srv:      srv,
		inflight: make(map[string]time.Time),
		cooldown: remediationCooldown,
	}
}

// Start subscribes to operation events from the event service.
// Runs in background; safe to call before the event client is connected.
func (rc *remediationConsumer) Start() {
	go func() {
		// Wait for the event client to be available.
		for i := 0; i < 120; i++ {
			if rc.srv.eventClient != nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if rc.srv.eventClient == nil {
			log.Printf("remediation-consumer: event client unavailable, not subscribing")
			return
		}

		uuid := Utility.RandomUUID()
		err := rc.srv.eventClient.Subscribe("operation.restart_requested", uuid, rc.handleRestartRequested)
		if err != nil {
			log.Printf("remediation-consumer: subscribe failed: %v", err)
			return
		}
		log.Printf("remediation-consumer: subscribed to operation.restart_requested")
	}()
}

// handleRestartRequested is the event callback. It validates the request,
// starts a workflow, executes the restart, verifies health, and emits the
// outcome.
func (rc *remediationConsumer) handleRestartRequested(evt *eventpb.Event) {
	// Parse event payload.
	var payload struct {
		Target     string `json:"target"`
		Source     string `json:"source"`
		IncidentID string `json:"incident_id"`
		RootCause  string `json:"root_cause"`
	}
	if err := json.Unmarshal(evt.GetData(), &payload); err != nil {
		log.Printf("remediation-consumer: invalid payload: %v", err)
		return
	}

	// Extract unit name from target (format: "restart_service:globular-rbac.service")
	unit := payload.Target
	if idx := strings.Index(unit, ":"); idx >= 0 {
		unit = unit[idx+1:]
	}
	if unit == "" {
		log.Printf("remediation-consumer: empty unit in target %q", payload.Target)
		return
	}

	// Ensure .service suffix.
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}

	log.Printf("remediation-consumer: received restart request for %s (incident=%s, cause=%s)",
		unit, payload.IncidentID, payload.RootCause)

	// ── Validate ──────────────────────────────────────────────────────

	// 1. Only allow globular-* units (safety boundary).
	if !strings.HasPrefix(unit, "globular-") {
		log.Printf("remediation-consumer: REJECTED — unit %q outside allowed prefix", unit)
		rc.emitOutcome(unit, payload.IncidentID, false, "rejected: unit outside allowed prefix")
		return
	}

	// 2. Cooldown: no duplicate in-flight restart.
	rc.mu.Lock()
	if last, ok := rc.inflight[unit]; ok && time.Since(last) < rc.cooldown {
		rc.mu.Unlock()
		log.Printf("remediation-consumer: SKIPPED — %s in cooldown (last restart %s ago)", unit, time.Since(last).Round(time.Second))
		rc.emitOutcome(unit, payload.IncidentID, false, "skipped: cooldown active")
		return
	}
	rc.inflight[unit] = time.Now()
	rc.mu.Unlock()

	// 3. Find which node runs this unit.
	nodeID, agentEndpoint, hostname := rc.findNodeForUnit(unit)
	if agentEndpoint == "" {
		log.Printf("remediation-consumer: REJECTED — no node found running %s", unit)
		rc.emitOutcome(unit, payload.IncidentID, false, "rejected: no node found for unit")
		return
	}

	// ── Workflow: track the restart as an auditable run ────────────────

	ctx := context.Background()
	runID := ""
	if rc.srv.workflowRec != nil {
		runID = rc.srv.workflowRec.StartRun(ctx, &workflow.RunParams{
			NodeID:        nodeID,
			NodeHostname:  hostname,
			ReleaseKind:   "AIRemediation",
			TriggerReason: workflow.TriggerRepair,
			CorrelationID: fmt.Sprintf("ai-remediation/%s/%s", unit, payload.IncidentID),
			WorkflowName:  "ai.remediate.restart",
		})
		log.Printf("remediation-consumer: workflow run started (run=%s, node=%s, unit=%s)", runID, hostname, unit)
	}

	// ── Execute: restart via node agent ───────────────────────────────

	var stepSeq int32
	if runID != "" && rc.srv.workflowRec != nil {
		stepSeq = rc.srv.workflowRec.RecordStep(ctx, runID, &workflow.StepParams{
			StepKey: "restart_service",
			Title:   fmt.Sprintf("Restart %s on %s", unit, hostname),
			Actor:   workflowpb.WorkflowActor_ACTOR_NODE_AGENT,
			Phase:   workflowpb.WorkflowPhaseKind_PHASE_START,
			Status:  workflowpb.StepStatus_STEP_STATUS_RUNNING,
			Message: fmt.Sprintf("Restarting %s via ControlService on %s", unit, agentEndpoint),
		})
	}

	restartErr := rc.restartUnit(ctx, agentEndpoint, unit)

	if restartErr != nil {
		log.Printf("remediation-consumer: FAILED — restart %s on %s: %v", unit, hostname, restartErr)
		if runID != "" && rc.srv.workflowRec != nil {
			rc.srv.workflowRec.RecordStep(ctx, runID, &workflow.StepParams{
				StepKey: "restart_service",
				Title:   fmt.Sprintf("Restart %s on %s", unit, hostname),
				Status:  workflowpb.StepStatus_STEP_STATUS_FAILED,
				Message: restartErr.Error(),
			})
			rc.srv.workflowRec.FinishRun(ctx, runID, workflowpb.RunStatus_RUN_STATUS_FAILED,
				fmt.Sprintf("Failed to restart %s", unit), restartErr.Error(), workflowpb.FailureClass_FAILURE_CLASS_SYSTEMD)
		}
		rc.emitOutcome(unit, payload.IncidentID, false, "restart failed: "+restartErr.Error())
		return
	}

	log.Printf("remediation-consumer: restart command sent for %s on %s", unit, hostname)

	// ── Verify: check that the service is healthy ─────────────────────

	time.Sleep(remediationVerifyWait)

	verified, detail := rc.verifyUnit(ctx, agentEndpoint, unit)

	if runID != "" && rc.srv.workflowRec != nil {
		verifyStatus := workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
		if !verified {
			verifyStatus = workflowpb.StepStatus_STEP_STATUS_FAILED
		}
		// Update the restart step to succeeded before recording verify.
		if stepSeq > 0 {
			rc.srv.workflowRec.RecordStep(ctx, runID, &workflow.StepParams{
				StepKey: "restart_service",
				Title:   fmt.Sprintf("Restart %s on %s", unit, hostname),
				Status:  workflowpb.StepStatus_STEP_STATUS_SUCCEEDED,
				Message: "systemctl restart completed",
			})
		}
		rc.srv.workflowRec.RecordStep(ctx, runID, &workflow.StepParams{
			StepKey: "verify_health",
			Title:   fmt.Sprintf("Verify %s health on %s", unit, hostname),
			Actor:   workflowpb.WorkflowActor_ACTOR_NODE_AGENT,
			Phase:   workflowpb.WorkflowPhaseKind_PHASE_VERIFY,
			Status:  verifyStatus,
			Message: detail,
		})

		runStatus := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
		runSummary := fmt.Sprintf("Restarted %s on %s — %s", unit, hostname, detail)
		runErr := ""
		failClass := workflowpb.FailureClass_FAILURE_CLASS_UNKNOWN
		if !verified {
			runStatus = workflowpb.RunStatus_RUN_STATUS_FAILED
			runErr = detail
			failClass = workflowpb.FailureClass_FAILURE_CLASS_SYSTEMD
		}
		rc.srv.workflowRec.FinishRun(ctx, runID, runStatus, runSummary, runErr, failClass)
	}

	log.Printf("remediation-consumer: %s restart %s (verified=%v: %s)", unit, map[bool]string{true: "SUCCEEDED", false: "FAILED"}[verified], verified, detail)
	rc.emitOutcome(unit, payload.IncidentID, verified, detail)
}

// unitProfileMap maps systemd unit prefixes to the profiles that run them.
// This allows the controller to target the correct node when the unit isn't
// in the node's active Units list (e.g. because it was just killed).
var unitProfileMap = map[string]string{
	"globular-cluster-controller": "control-plane",
	"globular-etcd":               "control-plane",
	"globular-workflow":           "control-plane",
	"globular-cluster-doctor":     "control-plane",
	"globular-dns":                "control-plane",
	"globular-authentication":     "control-plane",
	"globular-rbac":               "control-plane",
	"globular-resource":           "control-plane",
	"globular-discovery":          "control-plane",
	"globular-event":              "control-plane",
	"globular-log":                "control-plane",
	"globular-xds":                "control-plane",
	"globular-envoy":              "control-plane",
	"globular-gateway":            "gateway",
	"globular-minio":              "storage",
	"globular-repository":         "storage",
	"globular-monitoring":         "storage",
	"globular-prometheus":         "storage",
	"globular-alertmanager":       "storage",
	"globular-backup-manager":     "storage",
	"globular-ai-memory":          "ai",
	"globular-ai-executor":        "ai",
	"globular-ai-watcher":         "ai",
	"globular-ai-router":          "ai",
	"globular-mcp":                "ai",
}

// profileForUnit returns the profile that owns a unit, or "" if unknown.
func profileForUnit(unit string) string {
	base := strings.TrimSuffix(unit, ".service")
	if p, ok := unitProfileMap[base]; ok {
		return p
	}
	return ""
}

// findNodeForUnit scans controller state for a node that has the given
// systemd unit in its reported Units list, falling back to profile-based
// targeting when the unit isn't in any active report.
func (rc *remediationConsumer) findNodeForUnit(unit string) (nodeID, agentEndpoint, hostname string) {
	rc.srv.lock("remediation-consumer:find-node")
	defer rc.srv.unlock()

	// First pass: exact match in the node's active Units list.
	for _, node := range rc.srv.state.Nodes {
		for _, u := range node.Units {
			if u.Name == unit {
				return node.NodeID, node.AgentEndpoint, node.Identity.Hostname
			}
		}
	}

	// Second pass: profile-based targeting. The unit was stopped/killed
	// so it won't appear in the active list. Match by profile.
	requiredProfile := profileForUnit(unit)
	if requiredProfile != "" {
		for _, node := range rc.srv.state.Nodes {
			if node.AgentEndpoint == "" {
				continue
			}
			for _, p := range node.Profiles {
				if p == requiredProfile {
					return node.NodeID, node.AgentEndpoint, node.Identity.Hostname
				}
			}
		}
	}

	return "", "", ""
}

// restartUnit calls the node agent's ControlService RPC to restart a unit.
func (rc *remediationConsumer) restartUnit(ctx context.Context, agentEndpoint, unit string) error {
	conn, err := rc.srv.dialNodeAgent(agentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to node agent %s: %w", agentEndpoint, err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := client.ControlService(rctx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: "restart",
	})
	if err != nil {
		return fmt.Errorf("ControlService restart: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("restart rejected: %s", resp.GetMessage())
	}
	return nil
}

// verifyUnit checks if the unit is active after restart.
func (rc *remediationConsumer) verifyUnit(ctx context.Context, agentEndpoint, unit string) (bool, string) {
	conn, err := rc.srv.dialNodeAgent(agentEndpoint)
	if err != nil {
		return false, fmt.Sprintf("verify connect failed: %v", err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.ControlService(rctx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: "status",
	})
	if err != nil {
		return false, fmt.Sprintf("verify status failed: %v", err)
	}

	if resp.GetState() == "active" {
		return true, fmt.Sprintf("unit %s is active", unit)
	}
	return false, fmt.Sprintf("unit %s state: %s (%s)", unit, resp.GetState(), resp.GetMessage())
}

// emitOutcome publishes the result of the remediation attempt so the AI
// executor (and watcher) can update incident state from verifiable data,
// not assumptions.
func (rc *remediationConsumer) emitOutcome(unit, incidentID string, success bool, detail string) {
	status := "failed"
	if success {
		status = "succeeded"
	}
	rc.srv.emitClusterEvent("operation.restart_completed", map[string]interface{}{
		"unit":        unit,
		"incident_id": incidentID,
		"status":      status,
		"detail":      detail,
		"source":      "cluster_controller",
	})
}
