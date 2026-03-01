package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	planpb "github.com/globulario/services/golang/plan/planpb"
)

func TestNodePlanSuccess_Succeeded(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "node-1"}},
		PlanStatuses: map[string]*planpb.NodePlanStatus{
			"node-1": {PlanId: "plan-abc", NodeId: "node-1", State: planpb.PlanState_PLAN_SUCCEEDED},
		},
	}
	findings := (nodePlanSuccess{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for succeeded plan, got %d", len(findings))
	}
}

func TestNodePlanSuccess_Failed(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "node-1"}},
		PlanStatuses: map[string]*planpb.NodePlanStatus{
			"node-1": {
				PlanId:       "plan-abc",
				NodeId:       "node-1",
				State:        planpb.PlanState_PLAN_FAILED,
				ErrorMessage: "install failed",
				ErrorStepId:  "step-3",
			},
		},
	}
	findings := (nodePlanSuccess{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for failed plan, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "node.plan.last_apply_success" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
	// Evidence must include error fields
	if len(f.Evidence) == 0 || f.Evidence[0].KeyValues["error_message"] != "install failed" {
		t.Error("expected error_message in evidence")
	}
}

func TestNodePlanSuccess_StepFailed(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "node-1"}},
		PlanStatuses: map[string]*planpb.NodePlanStatus{
			"node-1": {
				PlanId: "plan-xyz",
				NodeId: "node-1",
				State:  planpb.PlanState_PLAN_RUNNING,
				Steps: []*planpb.StepStatus{
					{Id: "step-1", State: planpb.StepState_STEP_OK},
					{Id: "step-2", State: planpb.StepState_STEP_FAILED, Message: "timeout"},
				},
			},
		},
	}
	findings := (nodePlanSuccess{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for step failure, got %d", len(findings))
	}
	if findings[0].Evidence[0].KeyValues["failed_step_ids"] != "step-2" {
		t.Errorf("expected failed_step_ids=step-2, got %s", findings[0].Evidence[0].KeyValues["failed_step_ids"])
	}
}
