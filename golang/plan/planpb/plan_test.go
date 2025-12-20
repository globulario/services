package planpb

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestNodePlanRoundTrip(t *testing.T) {
	step := &PlanStep{
		Id:     "step-1",
		Action: "service.start",
		Args: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"unit": structpb.NewStringValue("globular-dns.service"),
			},
		},
	}
	plan := &NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     "cluster-1",
		NodeId:        "node-1",
		PlanId:        "plan-1",
		Generation:    1,
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		Spec: &PlanSpec{
			Steps: []*PlanStep{step},
		},
	}
	if plan.String() == "" {
		t.Fatal("NodePlan String should not be empty")
	}
	status := &NodePlanStatus{
		PlanId:    plan.PlanId,
		NodeId:    plan.NodeId,
		Generation: plan.Generation,
		State:     PlanState_PLAN_PENDING,
		Steps: []*StepStatus{
			{
				Id:    step.Id,
				State: StepState_STEP_PENDING,
			},
		},
	}
	if status.PlanId != plan.PlanId {
		t.Fatalf("expected plan_id %s, got %s", plan.PlanId, status.PlanId)
	}
}
