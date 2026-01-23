package main

import (
	"context"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
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
		kv:        kv,
		planStore: ps,
	}
	desired := &clustercontrollerpb.DesiredState{
		Generation:      1,
		Network:         &clustercontrollerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80},
		ServiceVersions: map[string]string{serviceUnitForCanonical("gateway"): "1"},
	}
	if err := srv.saveDesiredState(context.Background(), desired); err != nil {
		t.Fatalf("saveDesiredState: %v", err)
	}
	hashNet, _ := hashDesiredNetwork(desired.GetNetwork())
	_ = srv.putNodeAppliedHash(context.Background(), "n1", hashNet)
	hashSvc := stableServiceDesiredHash(map[string]string{canonicalServiceName("gateway"): "1"})
	_ = srv.putNodeAppliedServiceHash(context.Background(), "n1", hashSvc)

	resp, err := srv.GetClusterHealthV1(context.Background(), &clustercontrollerpb.GetClusterHealthV1Request{})
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
