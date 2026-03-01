package operator

import (
	"context"

	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
)

type MinioOperator struct {
	ps    store.PlanStore
	nodes func() []string
}

func NewMinioOperator(ps store.PlanStore, nodes func() []string) Operator {
	return &MinioOperator{ps: ps, nodes: nodes}
}

func (o *MinioOperator) Name() string        { return "minio" }
func (o *MinioOperator) DependsOn() []string { return nil }

func (o *MinioOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	// Serial by default; best-effort check for another upgrading plan.
	if o.ps == nil || o.nodes == nil {
		return AdmitDecision{Allowed: true}, nil
	}
	for _, id := range o.nodes() {
		plan, _ := o.ps.GetCurrentPlan(ctx, id)
		status, _ := o.ps.GetStatus(ctx, id)
		if plan != nil && isServicePlan(plan, "minio") && status != nil {
			if status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING {
				return AdmitDecision{Allowed: false, Reason: "minio rolling: another node upgrading", RequeueAfterSeconds: 10}, nil
			}
		}
	}
	return AdmitDecision{Allowed: true}, nil
}

func (o *MinioOperator) MutatePlan(ctx context.Context, req MutateRequest) (*planpb.NodePlan, error) {
	plan := req.Plan
	if plan == nil || plan.Spec == nil {
		return plan, nil
	}
	addLock(plan, "service:minio:rolling")
	addProbe(plan, &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": "127.0.0.1:9000"})})
	// Enforce layout via existing action.
	plan.Spec.Steps = append(plan.Spec.Steps, &planpb.PlanStep{
		Action: "ensure_objectstore_layout",
		Args: structpbFromMap(map[string]interface{}{
			"domain":         req.DesiredDomain,
			"contract_path":  "/var/lib/globular/objectstore/minio.json",
			"bucket":         "globular",
			"retry":          10,
			"retry_delay_ms": 500,
			"sentinel_name":  ".keep",
		}),
	})
	return plan, nil
}

func (o *MinioOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
