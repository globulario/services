package rules

import (
	"fmt"
	"strings"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	planpb "github.com/globulario/services/golang/plan/planpb"
)

type nodePlanSuccess struct{}

func (nodePlanSuccess) ID() string       { return "node.plan.last_apply_success" }
func (nodePlanSuccess) Category() string { return "plan" }
func (nodePlanSuccess) Scope() string    { return "node" }

func (nodePlanSuccess) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		status, ok := snap.PlanStatuses[nodeID]
		if !ok {
			continue
		}

		state := status.GetState()
		failed := state == planpb.PlanState_PLAN_FAILED || state == planpb.PlanState_PLAN_ROLLING_BACK

		// Also check individual step failures even if plan-level state is not FAILED.
		var failedStepIDs []string
		for _, s := range status.GetSteps() {
			if s.GetState() == planpb.StepState_STEP_FAILED {
				failedStepIDs = append(failedStepIDs, s.GetId())
			}
		}
		if len(failedStepIDs) > 0 {
			failed = true
		}

		if !failed {
			continue
		}

		kv := map[string]string{
			"node_id":      nodeID,
			"plan_id":      status.GetPlanId(),
			"state":        state.String(),
			"error_message": status.GetErrorMessage(),
			"error_step_id": status.GetErrorStepId(),
		}
		if len(failedStepIDs) > 0 {
			kv["failed_step_ids"] = strings.Join(failedStepIDs, ",")
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("node.plan.last_apply_success", nodeID, status.GetPlanId()),
			InvariantID: "node.plan.last_apply_success",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "plan",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s last plan %s failed (state=%s)",
				nodeID, status.GetPlanId(), state),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetPlanStatusV1", kv),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Inspect plan execution logs for node "+nodeID, "globular doctor node "+nodeID),
				step(2, "Check nodeagent journal for details: journalctl -u globular-nodeagent -n 200", ""),
				step(3, "If safe, trigger a new reconciliation to generate a corrective plan", ""),
				step(4, "If plan is stuck rolling back, investigate the failed step: "+kv["error_step_id"], ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
