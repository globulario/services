package operator

import (
	"context"

	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
)

type ScyllaOperator struct {
	ps    store.PlanStore
	nodes func() []string
}

func NewScyllaOperator(ps store.PlanStore, nodes func() []string) Operator {
	return &ScyllaOperator{ps: ps, nodes: nodes}
}

func (o *ScyllaOperator) Name() string        { return "scylla" }
func (o *ScyllaOperator) DependsOn() []string { return nil }

func (o *ScyllaOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	if o.ps == nil || o.nodes == nil {
		return AdmitDecision{Allowed: true}, nil
	}
	for _, id := range o.nodes() {
		plan, _ := o.ps.GetCurrentPlan(ctx, id)
		status, _ := o.ps.GetStatus(ctx, id)
		if plan != nil && isServicePlan(plan, "scylla") && status != nil {
			if status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING {
				return AdmitDecision{Allowed: false, Reason: "scylla rolling: another node upgrading", RequeueAfterSeconds: 10}, nil
			}
		}
	}
	return AdmitDecision{Allowed: true}, nil
}

func (o *ScyllaOperator) MutatePlan(ctx context.Context, req MutateRequest) (*planpb.NodePlan, error) {
	plan := req.Plan
	if plan == nil || plan.Spec == nil {
		return plan, nil
	}
	addLock(plan, "service:scylla:rolling")
	addProbe(plan, &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": "127.0.0.1:9042"})})
	// Prefer exec probe if available.
	addProbe(plan, &planpb.Probe{Type: "probe.exec", Args: structpbFromMap(map[string]interface{}{"cmd": "cqlsh -e \"SELECT now() FROM system.local\" || nodetool status"})})
	return plan, nil
}

func (o *ScyllaOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
