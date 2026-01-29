package main

import (
	"context"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
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
		resources: resourcestore.NewMemStore(),
	}
}

func desiredNetworkForTests() *clustercontrollerpb.DesiredNetwork {
	return &clustercontrollerpb.DesiredNetwork{
		Domain:   "example.com",
		Protocol: "http",
		PortHttp: 80,
	}
}

func applyDesiredForTests(t *testing.T, srv *server, net *clustercontrollerpb.DesiredNetwork, services map[string]string) {
	t.Helper()
	ctx := context.Background()
	if net != nil {
		_, err := srv.resources.Apply(ctx, "ClusterNetwork", &clustercontrollerpb.ClusterNetwork{
			Meta: &clustercontrollerpb.ObjectMeta{Name: "default", Generation: 1},
			Spec: &clustercontrollerpb.ClusterNetworkSpec{
				ClusterDomain:    net.GetDomain(),
				Protocol:         net.GetProtocol(),
				PortHttp:         net.GetPortHttp(),
				PortHttps:        net.GetPortHttps(),
				AlternateDomains: append([]string(nil), net.GetAlternateDomains()...),
				AcmeEnabled:      net.GetAcmeEnabled(),
				AdminEmail:       net.GetAdminEmail(),
			},
		})
		if err != nil {
			t.Fatalf("apply network: %v", err)
		}
	}
	for svc, ver := range services {
		canon := canonicalServiceName(svc)
		_, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", &clustercontrollerpb.ServiceDesiredVersion{
			Meta: &clustercontrollerpb.ObjectMeta{Name: canon, Generation: 1},
			Spec: &clustercontrollerpb.ServiceDesiredVersionSpec{
				ServiceName: canon,
				Version:     ver,
			},
		})
		if err != nil {
			t.Fatalf("apply service: %v", err)
		}
	}
}

func TestReconcileDoesNotMarkAppliedOnEmit(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	applyDesiredForTests(t, srv, desiredNetworkForTests(), nil)
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
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, nil)
	srv.reconcileNodes(context.Background())
	firstPlan := proto.Clone(ps.lastPlan).(*planpb.NodePlan)
	meta := &planMeta{PlanId: firstPlan.GetPlanId(), Generation: firstPlan.GetGeneration(), DesiredHash: mustHash(t, net), LastEmit: time.Now().UnixMilli()}
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
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, nil)
	srv.reconcileNodes(context.Background())
	plan := ps.lastPlan
	hash := mustHash(t, net)
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
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, nil)
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
	hash := mustHash(t, net)
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

func TestServiceReconcileMarksAppliedOnSuccess(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	srv.state.Nodes["n1"].Units = []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}
	applyDesiredForTests(t, srv, desiredNetworkForTests(), map[string]string{"globular-gateway.service": "1.2.3"})
	// Mark network converged so service reconcile can proceed.
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, desiredNetworkForTests())); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	plan := ps.lastPlan
	if plan == nil {
		t.Fatalf("expected service plan emitted")
	}
	svcHash := stableServiceDesiredHash(map[string]string{"gateway": "1.2.3"})
	if plan.GetDesiredHash() != svcHash {
		t.Fatalf("plan desired_hash mismatch: got %s want %s", plan.GetDesiredHash(), svcHash)
	}
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     plan.GetPlanId(),
		NodeId:     "n1",
		Generation: plan.GetGeneration(),
		State:      planpb.PlanState_PLAN_SUCCEEDED,
	})
	srv.reconcileNodes(context.Background())
	appliedSvc, err := srv.getNodeAppliedServiceHash(context.Background(), "n1")
	if err != nil {
		t.Fatalf("get applied svc hash: %v", err)
	}
	if appliedSvc != svcHash {
		t.Fatalf("expected applied service hash %s, got %s", svcHash, appliedSvc)
	}
}

func TestServiceReconcileDoesNotReemitWhileRunning(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	srv.state.Nodes["n1"].Units = []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, map[string]string{
		"globular-gateway.service": "1.2.3",
	})
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, net)); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
	svcHash := stableServiceDesiredHash(map[string]string{"gateway": "1.2.3"})
	ps.PutStatus(context.Background(), "n1", &planpb.NodePlanStatus{
		PlanId:     firstPlan.GetPlanId(),
		NodeId:     "n1",
		Generation: firstPlan.GetGeneration(),
		State:      planpb.PlanState_PLAN_RUNNING,
	})
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected no re-emit while running, got %d", ps.count)
	}
	if firstPlan.GetDesiredHash() != svcHash {
		t.Fatalf("expected desired hash set on plan")
	}
}

func TestServiceReconcileReemitsAfterFailure(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := newTestServerWithNode(kv, ps)
	srv.state.Nodes["n1"].Units = []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}
	net := desiredNetworkForTests()
	applyDesiredForTests(t, srv, net, map[string]string{
		"globular-gateway.service": "1.2.3",
	})
	if err := srv.putNodeAppliedHash(context.Background(), "n1", mustHash(t, net)); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	firstPlan := ps.lastPlan
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
