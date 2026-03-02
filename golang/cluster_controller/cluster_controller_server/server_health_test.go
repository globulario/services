package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
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
			"n1": {NodeID: "n1", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}, InstalledVersions: map[string]string{"gateway": "1"}},
			"n2": {NodeID: "n2", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}, InstalledVersions: map[string]string{"gateway": "1"}},
		}},
		kv:         kv,
		planStore:  ps,
		resources:  resourcestore.NewMemStore(),
		etcdClient: nil,
	}
	// Seed desired resources.
	_, _ = srv.resources.Apply(context.Background(), "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	_, _ = srv.resources.Apply(context.Background(), "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "gateway", Generation: 1},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: "gateway", Version: "1"},
	})
	// Mark applied hashes
	hashNet, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(context.Background(), "n1", hashNet)
	_ = srv.putNodeAppliedHash(context.Background(), "n2", hashNet)
	hashSvc := stableServiceDesiredHash(map[string]string{canonicalServiceName("gateway"): "1"})
	_ = srv.putNodeAppliedServiceHash(context.Background(), "n1", hashSvc)
	_ = srv.putNodeAppliedServiceHash(context.Background(), "n2", hashSvc)
	ps.lastPlan = &planpb.NodePlan{PlanId: "p1", Generation: 1, DesiredHash: hashNet}
	ps.status = &planpb.NodePlanStatus{PlanId: "p1", Generation: 1, State: planpb.PlanState_PLAN_SUCCEEDED}

	resp, err := srv.GetClusterHealthV1(context.Background(), &cluster_controllerpb.GetClusterHealthV1Request{})
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

// TestGetClusterHealthV1PerServiceConvergence verifies that when N services
// match desired and 1 new service is added, the N existing services show
// nodesAtDesired=1 while the new one shows nodesAtDesired=0.
func TestGetClusterHealthV1PerServiceConvergence(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}

	// Node has 3 services installed at desired versions.
	installed := map[string]string{
		"ldap": "1.0.0", "media": "2.0.0", "dns": "1.0.0",
	}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1", InstalledVersions: installed, Capabilities: &storedCapabilities{CanApplyPrivileged: true}},
		}},
		kv:        kv,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	// Seed desired: same 3 + 1 new service "title" not yet installed.
	_, _ = srv.resources.Apply(context.Background(), "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "test.com", Protocol: "http", PortHttp: 80},
	})
	for _, s := range []struct{ name, ver string }{
		{"ldap", "1.0.0"}, {"media", "2.0.0"}, {"dns", "1.0.0"}, {"title", "1.0.0"},
	} {
		_, _ = srv.resources.Apply(context.Background(), "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
			Meta: &cluster_controllerpb.ObjectMeta{Name: s.name, Generation: 1},
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: s.name, Version: s.ver},
		})
	}

	resp, err := srv.GetClusterHealthV1(context.Background(), &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		t.Fatalf("GetClusterHealthV1: %v", err)
	}
	if len(resp.GetServices()) != 4 {
		t.Fatalf("expected 4 service summaries, got %d", len(resp.GetServices()))
	}
	for _, ss := range resp.GetServices() {
		switch ss.GetServiceName() {
		case "ldap", "media", "dns":
			if ss.GetNodesAtDesired() != 1 {
				t.Errorf("service %s: expected nodesAtDesired=1, got %d", ss.GetServiceName(), ss.GetNodesAtDesired())
			}
		case "title":
			if ss.GetNodesAtDesired() != 0 {
				t.Errorf("service title: expected nodesAtDesired=0, got %d", ss.GetNodesAtDesired())
			}
		default:
			t.Errorf("unexpected service %s", ss.GetServiceName())
		}
		if ss.GetNodesTotal() != 1 {
			t.Errorf("service %s: expected nodesTotal=1, got %d", ss.GetServiceName(), ss.GetNodesTotal())
		}
	}
}

// Ensure fakePlanStore supports cloning for tests.

func (f *fakePlanStore) clonePlan() *planpb.NodePlan {
	if f.lastPlan == nil {
		return nil
	}
	return proto.Clone(f.lastPlan).(*planpb.NodePlan)

}
