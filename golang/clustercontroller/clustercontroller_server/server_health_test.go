package main

import (
	"context"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/proto"
)

// Reuse mapKV and fakePlanStore from desired_state_test.go

func TestGetClusterHealthV1(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}},
			"n2": {NodeID: "n2", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}},
		}},
		kv:         kv,
		planStore:  ps,
		resources:  resourcestore.NewMemStore(),
		etcdClient: nil,
	}
	// Seed desired resources.
	_, _ = srv.resources.Apply(context.Background(), "ClusterNetwork", &clustercontrollerpb.ClusterNetwork{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	_, _ = srv.resources.Apply(context.Background(), "ServiceDesiredVersion", &clustercontrollerpb.ServiceDesiredVersion{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "gateway", Generation: 1},
		Spec: &clustercontrollerpb.ServiceDesiredVersionSpec{ServiceName: "gateway", Version: "1"},
	})
	// Mark applied hashes
	hashNet, _ := hashDesiredNetwork(&clustercontrollerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(context.Background(), "n1", hashNet)
	_ = srv.putNodeAppliedHash(context.Background(), "n2", hashNet)
	hashSvc := stableServiceDesiredHash(map[string]string{canonicalServiceName("gateway"): "1"})
	_ = srv.putNodeAppliedServiceHash(context.Background(), "n1", hashSvc)
	_ = srv.putNodeAppliedServiceHash(context.Background(), "n2", hashSvc)
	ps.lastPlan = &planpb.NodePlan{PlanId: "p1", Generation: 1, DesiredHash: hashNet}
	ps.status = &planpb.NodePlanStatus{PlanId: "p1", Generation: 1, State: planpb.PlanState_PLAN_SUCCEEDED}

	resp, err := srv.GetClusterHealthV1(context.Background(), &clustercontrollerpb.GetClusterHealthV1Request{})
	if err != nil {
		t.Fatalf("GetClusterHealthV1: %v", err)
	}
	if len(resp.GetNodes()) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(resp.GetNodes()))
	}
	if len(resp.GetServices()) != 1 {
		t.Fatalf("expected 1 service summary, got %d", len(resp.GetServices()))
	}
	svcSummary := resp.GetServices()[0]
	if svcSummary.GetServiceName() != "gateway" {
		t.Fatalf("expected service name gateway, got %s", svcSummary.GetServiceName())
	}
	if svcSummary.GetNodesAtDesired() != 2 {
		t.Fatalf("expected nodes_at_desired=2, got %d", svcSummary.GetNodesAtDesired())
	}
	if resp.GetNodes()[0].GetDesiredNetworkHash() != hashNet {
		t.Fatalf("node desired hash mismatch")
	}
	if resp.GetNodes()[0].GetAppliedServicesHash() != hashSvc {
		t.Fatalf("applied services hash mismatch")
	}
}

// Ensure fakePlanStore supports cloning for tests.

func (f *fakePlanStore) clonePlan() *planpb.NodePlan {
	if f.lastPlan == nil {
		return nil
	}
	return proto.Clone(f.lastPlan).(*planpb.NodePlan)

}
