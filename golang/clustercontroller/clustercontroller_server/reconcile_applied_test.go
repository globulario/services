package main

import (
	"context"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/proto"
)

func newTestServerWithNode(kv *mapKV, ps *fakePlanStore) *server {
	return &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1"},
		}},
		kv:        kv,
		planStore: ps,
	}
}

func desiredNetworkForTests() *clustercontrollerpb.DesiredNetwork {
	return &clustercontrollerpb.DesiredNetwork{
		Domain:   "example.com",
		Protocol: "http",
		PortHttp: 80,
	}
}

func TestReconcileDoesNotMarkAppliedOnEmit(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	desired := &clustercontrollerpb.DesiredState{
		Generation: 1,
		Network:    desiredNetworkForTests(),
	}
	if err := srv.saveDesiredState(context.Background(), desired); err != nil {
		t.Fatalf("saveDesiredState: %v", err)
	}
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected 1 plan emission, got %d", ps.count)
	}
	applied, err := srv.getNodeAppliedHash(context.Background(), "n1")
	if err != nil {
		t.Fatalf("get applied hash: %v", err)
	}
	if applied != "" {
		t.Fatalf("expected no applied hash after emit, got %s", applied)
	}
}

func TestReconcileDoesNotReemitWhileRunning(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	desired := &clustercontrollerpb.DesiredState{Generation: 1, Network: desiredNetworkForTests()}
	if err := srv.saveDesiredState(context.Background(), desired); err != nil {
		t.Fatalf("saveDesiredState: %v", err)
	}
	srv.reconcileNodes(context.Background())
	firstPlan := proto.Clone(ps.lastPlan).(*planpb.NodePlan)
	meta := &planMeta{PlanId: firstPlan.GetPlanId(), Generation: firstPlan.GetGeneration(), DesiredHash: mustHash(t, desired.GetNetwork()), LastEmit: time.Now().UnixMilli()}
	if err := srv.putNodePlanMeta(context.Background(), "n1", meta); err != nil {
		t.Fatalf("put meta: %v", err)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_RUNNING,
	})
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected no additional plan while running, got %d", ps.count)
	}
}

func TestReconcileMarksAppliedOnSuccess(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	desired := &clustercontrollerpb.DesiredState{Generation: 1, Network: desiredNetworkForTests()}
	if err := srv.saveDesiredState(context.Background(), desired); err != nil {
		t.Fatalf("saveDesiredState: %v", err)
	}
	srv.reconcileNodes(context.Background())
	plan := ps.lastPlan
	hash := mustHash(t, desired.GetNetwork())
	meta := &planMeta{PlanId: plan.GetPlanId(), Generation: plan.GetGeneration(), DesiredHash: hash, LastEmit: time.Now().UnixMilli()}
	if err := srv.putNodePlanMeta(context.Background(), "n1", meta); err != nil {
		t.Fatalf("put meta: %v", err)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     plan.GetPlanId(),
		NodeId:     "n1",
		Generation: plan.GetGeneration(),
		State:      planpb.PlanState_PLAN_SUCCEEDED,
	})
	srv.reconcileNodes(context.Background())
	applied, err := srv.getNodeAppliedHash(context.Background(), "n1")
	if err != nil {
		t.Fatalf("get applied: %v", err)
	}
	if applied != hash {
		t.Fatalf("expected applied hash %s, got %s", hash, applied)
	}
}

func TestReconcileReemitsAfterFailure(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	desired := &clustercontrollerpb.DesiredState{Generation: 1, Network: desiredNetworkForTests()}
	if err := srv.saveDesiredState(context.Background(), desired); err != nil {
		t.Fatalf("saveDesiredState: %v", err)
	}
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
	hash := mustHash(t, desired.GetNetwork())
	meta := &planMeta{PlanId: firstPlan.GetPlanId(), Generation: firstPlan.GetGeneration(), DesiredHash: hash, LastEmit: time.Now().Add(-time.Minute).UnixMilli()}
	if err := srv.putNodePlanMeta(context.Background(), "n1", meta); err != nil {
		t.Fatalf("put meta: %v", err)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_FAILED,
	})
	srv.reconcileNodes(context.Background())
	if ps.count < 2 {
		t.Fatalf("expected re-emit after failure, got %d", ps.count)
	}
}

func mustHash(t *testing.T, net *clustercontrollerpb.DesiredNetwork) string {
	t.Helper()
	h, err := hashDesiredNetwork(net)
	if err != nil {
		t.Fatalf("hashDesiredNetwork: %v", err)
	}
	return h
}
