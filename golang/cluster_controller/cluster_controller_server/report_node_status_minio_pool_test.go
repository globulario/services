package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestReportNodeStatus_AutoDerivedStorageAddsMinioPoolIP(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID:   "node-1",
		Identity: storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
	}
	srv := newTestServer(t, state)

	_, err := srv.ReportNodeStatus(context.Background(), &cluster_controllerpb.ReportNodeStatusRequest{
		Status: &cluster_controllerpb.NodeStatus{
			NodeId:        "node-1",
			AgentEndpoint: "10.0.0.63:11000",
			Identity: &cluster_controllerpb.NodeIdentity{
				Hostname: "globule-ryzen",
				Ips:      []string{"10.0.0.63"},
			},
			InstalledVersions: map[string]string{
				"minio":      "2025.1.0",
				"repository": "1.2.257",
			},
		},
	})
	if err != nil {
		t.Fatalf("ReportNodeStatus: %v", err)
	}

	node := srv.state.Nodes["node-1"]
	if node == nil {
		t.Fatal("node missing after ReportNodeStatus")
	}
	if got, want := node.AdvertiseFqdn, "globule-ryzen.globular.internal"; got != want {
		t.Fatalf("AdvertiseFqdn = %q, want %q", got, want)
	}
	if len(srv.state.MinioPoolNodes) != 1 {
		t.Fatalf("MinioPoolNodes length = %d, want 1 (%v)", len(srv.state.MinioPoolNodes), srv.state.MinioPoolNodes)
	}
	if got, want := srv.state.MinioPoolNodes[0], "10.0.0.63"; got != want {
		t.Fatalf("MinioPoolNodes[0] = %q, want stable IP %q", got, want)
	}
}
