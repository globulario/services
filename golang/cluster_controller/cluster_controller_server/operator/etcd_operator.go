package operator

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/rolling"
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
		// Treat nodes as unhealthy only if the CURRENT plan's status is non-succeeded.
		// A stale status from an old generation should not permanently block operations.
		if status != nil && plan != nil &&
			status.GetGeneration() == plan.GetGeneration() &&
			status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
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
	probeAddr := req.NodeIP + ":2379"
	if req.NodeIP == "" {
		probeAddr = "127.0.0.1:2379" // fallback only if controller has no node IP
	}
	addProbe(plan, &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": probeAddr})})
	addProbe(plan, &planpb.Probe{
		Type: "probe.exec",
		Args: structpbFromMap(map[string]interface{}{
			"cmd": fmt.Sprintf(
				"etcdctl endpoint health --endpoints=https://%s --cacert=/var/lib/globular/pki/ca.pem || "+
					"etcdctl endpoint health --endpoints=https://%s --cacert=/var/lib/globular/pki/ca.crt",
				probeAddr, probeAddr),
		}),
	})
	return plan, nil
}

func (o *EtcdOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
