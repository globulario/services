package operator

import (
	"context"

	"github.com/globulario/services/golang/clustercontroller/clustercontroller_server/rolling"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
)

type EtcdOperator struct {
	ps    store.PlanStore
	nodes func() []string
}

func NewEtcdOperator(ps store.PlanStore, nodes func() []string) Operator {
	return &EtcdOperator{ps: ps, nodes: nodes}
}

func (o *EtcdOperator) Name() string        { return "etcd" }
func (o *EtcdOperator) DependsOn() []string { return nil }

func (o *EtcdOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	if o.ps == nil || o.nodes == nil {
		return AdmitDecision{Allowed: true}, nil
	}
	states := make([]rolling.NodeRollState, 0)
	for _, id := range o.nodes() {
		st := rolling.NodeRollState{NodeID: id, IsHealthy: true}
		plan, _ := o.ps.GetCurrentPlan(ctx, id)
		status, _ := o.ps.GetStatus(ctx, id)
		if plan != nil && isServicePlan(plan, "etcd") {
			if status != nil && (status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING) {
				st.IsUpgrading = true
			}
		}
		// Treat nodes without applied desired hash as unhealthy for quorum safety.
		if status != nil && status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
			st.IsHealthy = false
		}
		states = append(states, st)
	}
	allowed, reason := rolling.AdmitRolling(rolling.RollingPolicy{Serial: true, MaxUnavailable: 1}, states)
	if !allowed {
		return AdmitDecision{Allowed: false, Reason: "rolling gate: " + reason, RequeueAfterSeconds: 10}, nil
	}
	return AdmitDecision{Allowed: true}, nil
}

func (o *EtcdOperator) MutatePlan(ctx context.Context, req MutateRequest) (*planpb.NodePlan, error) {
	plan := req.Plan
	if plan == nil || plan.Spec == nil {
		return plan, nil
	}
	addLock(plan, "service:etcd:rolling")
	addProbe(plan, &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": "127.0.0.1:2379"})})
	// Day-0 Security: Use https and CA cert for etcd health checks (NO HTTP FALLBACK)
	// H2 Hardening: Prefer canonical PKI paths, fall back to legacy paths for compatibility
	addProbe(plan, &planpb.Probe{
		Type: "probe.exec",
		Args: structpbFromMap(map[string]interface{}{
			"cmd": "etcdctl endpoint health --endpoints=https://127.0.0.1:2379 --cacert=/var/lib/globular/pki/ca.pem || " +
				"etcdctl endpoint health --endpoints=https://127.0.0.1:2379 --cacert=/var/lib/globular/pki/ca.crt || " +
				"etcdctl endpoint health --endpoints=https://127.0.0.1:2379 --cacert=/var/lib/globular/config/tls/ca.pem || " +
				"etcdctl endpoint health --endpoints=https://127.0.0.1:2379 --cacert=/var/lib/globular/config/tls/work/ca.crt",
		}),
	})
	return plan, nil
}

func (o *EtcdOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
