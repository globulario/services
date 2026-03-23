package rules

import (
	"fmt"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	planpb "github.com/globulario/services/golang/plan/planpb"
)

// ── Plan stuck in terminal state ────────────────────────────────────────────
//
// Detects nodes whose last plan is in a terminal failure state (FAILED or
// ROLLED_BACK) and the controller has not dispatched a replacement. This
// means the node is stuck — no convergence is happening.
//
// This was observed during Day 1 testing: plans failed with "exit status 1"
// and the controller stopped dispatching new plans, leaving the node in limbo.

type planStuckTerminal struct{}

func (planStuckTerminal) ID() string       { return "node.plan_stuck_terminal" }
func (planStuckTerminal) Category() string { return "plan" }
func (planStuckTerminal) Scope() string    { return "node" }

func (planStuckTerminal) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		hostname := node.GetIdentity().GetHostname()

		status, ok := snap.PlanStatuses[nodeID]
		if !ok || status == nil {
			continue
		}

		state := status.GetState()
		isFailed := state == planpb.PlanState_PLAN_FAILED
		isRolledBack := state == planpb.PlanState_PLAN_ROLLED_BACK

		if !isFailed && !isRolledBack {
			continue
		}

		// Check if there's a newer plan waiting (different plan ID in NodePlans).
		currentPlan, hasPlan := snap.NodePlans[nodeID]
		if hasPlan && currentPlan != nil {
			// There IS a plan — check if it's the same failed one or a new one.
			planStatus := status.GetPlanId()
			if currentPlan.GetPlanId() != "" && currentPlan.GetPlanId() != planStatus {
				continue // A new plan exists, node should pick it up.
			}
		}

		// Node has a terminal plan and no replacement.
		stateStr := "FAILED"
		if isRolledBack {
			stateStr = "ROLLED_BACK"
		}

		errMsg := status.GetErrorMessage()
		errStep := status.GetErrorStepId()
		detail := fmt.Sprintf("state=%s", stateStr)
		if errMsg != "" {
			detail += fmt.Sprintf(" error=%q", errMsg)
		}
		if errStep != "" {
			detail += fmt.Sprintf(" failed_step=%s", errStep)
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("node.plan_stuck_terminal", nodeID, status.GetPlanId()),
			InvariantID: "node.plan_stuck_terminal",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "plan",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s (%s) stuck with terminal plan %s (%s). "+
				"No replacement plan dispatched — convergence halted.",
				hostname, nodeID, status.GetPlanId(), detail),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetPlanStatusV1", map[string]string{
					"node_id":       nodeID,
					"plan_id":       status.GetPlanId(),
					"state":         stateStr,
					"error_message": errMsg,
					"error_step_id": errStep,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check node-agent logs for the step failure",
					fmt.Sprintf("ssh %s journalctl -u globular-node-agent.service --since '10 min ago' | grep -i 'plan\\|step\\|fail\\|error'", hostname)),
				step(2, "Force the controller to recompute plans for this node",
					"sudo systemctl restart globular-cluster-controller.service"),
				step(3, "If the same plan keeps failing, investigate the specific step and fix the root cause", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}
