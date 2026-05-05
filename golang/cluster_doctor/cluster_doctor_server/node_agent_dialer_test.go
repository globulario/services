package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestFallbackEndpointFromNodeRecord(t *testing.T) {
	n := &cluster_controllerpb.NodeRecord{
		NodeId:        "n1",
		AgentEndpoint: "node-1.globular.internal:11000",
		Identity: &cluster_controllerpb.NodeIdentity{
			AdvertiseIp: "10.0.0.8",
			Ips:         []string{"10.0.0.8"},
		},
	}
	got := fallbackEndpointFromNodeRecord(n)
	if got != "10.0.0.8:11000" {
		t.Fatalf("fallback endpoint = %q, want %q", got, "10.0.0.8:11000")
	}
}

