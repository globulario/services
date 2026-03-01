package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// Regression: desired keyed by unit name still aggregates correctly via canonical keys.
func TestClusterHealthCanonicalizesServiceKeys(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg: &clusterControllerConfig{},
		state: &controllerState{Nodes: map[string]*nodeState{
			"n1": {NodeID: "n1", Units: []unitStatusRecord{{Name: serviceUnitForCanonical("gateway")}}},
		}},
		kv:         kv,
		planStore:  ps,
		resources:  resourcestore.NewMemStore(),
		etcdClient: nil,
	}
	_, _ = srv.resources.Apply(context.Background(), "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80},
	})
	// Apply service with unit-style name to ensure canonicalization.
	unitName := serviceUnitForCanonical("gateway")
	_, _ = srv.resources.Apply(context.Background(), "ServiceDesiredVersion", &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: unitName, Generation: 1},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{ServiceName: unitName, Version: "1"},
	})
	hashNet, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80})
	_ = srv.putNodeAppliedHash(context.Background(), "n1", hashNet)
	hashSvc := stableServiceDesiredHash(map[string]string{canonicalServiceName("gateway"): "1"})
	_ = srv.putNodeAppliedServiceHash(context.Background(), "n1", hashSvc)

	resp, err := srv.GetClusterHealthV1(context.Background(), &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		t.Fatalf("GetClusterHealthV1: %v", err)
	}
	if len(resp.GetServices()) != 1 {
		t.Fatalf("expected 1 service summary, got %d", len(resp.GetServices()))
	}
	if resp.GetServices()[0].GetServiceName() != "gateway" {
		t.Fatalf("expected canonical service name gateway, got %s", resp.GetServices()[0].GetServiceName())
	}
	if resp.GetServices()[0].GetNodesAtDesired() != 1 {
		t.Fatalf("expected nodes_at_desired=1, got %d", resp.GetServices()[0].GetNodesAtDesired())
	}
}
