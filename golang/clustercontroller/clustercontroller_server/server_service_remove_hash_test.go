package main

import (
	"context"
	"strings"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
)

// Ensure removal uses stable desired hash and converges when removal flag enabled.
func TestServiceRemovalPlanHasStableHash(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}},
		}},
		kv:                   kv,
		planStore:            ps,
		resources:            resourcestore.NewMemStore(),
		enableServiceRemoval: true,
	}
	_, _ = srv.resources.Apply(context.Background(), "ClusterNetwork", &clustercontrollerpb.ClusterNetwork{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}

	srv.reconcileNodes(context.Background())
	plan := ps.lastPlan
	if plan == nil {
		t.Fatalf("expected removal plan emitted")
	}
	if plan.GetReason() != "service_remove" {
		t.Fatalf("expected service_remove reason, got %s", plan.GetReason())
	}
	if plan.GetDesiredHash() == "" || !strings.HasPrefix(plan.GetDesiredHash(), "services:") {
		t.Fatalf("expected stable desired hash with services: prefix, got %s", plan.GetDesiredHash())
	}

	// Simulate success and ensure applied hash is stored and no re-emit.
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     plan.GetPlanId(),
		NodeId:     "n1",
		Generation: plan.GetGeneration(),
		State:      planpb.PlanState_PLAN_SUCCEEDED,
	})
	srv.reconcileNodes(context.Background())
	appliedSvc, _ := srv.getNodeAppliedServiceHash(context.Background(), "n1")
	if appliedSvc != plan.GetDesiredHash() {
		t.Fatalf("expected applied service hash %s, got %s", plan.GetDesiredHash(), appliedSvc)
	}
	if ps.count > 1 {
		t.Fatalf("expected no re-emit after success, got %d plans", ps.count)
	}
}
