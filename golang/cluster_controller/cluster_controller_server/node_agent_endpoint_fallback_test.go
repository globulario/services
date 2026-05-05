package main

import "testing"

func TestFallbackNodeAgentEndpointFromState(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.state.Nodes["n1"] = &nodeState{
		NodeID: "n1",
		Identity: storedIdentity{
			Ips: []string{"10.0.0.63"},
		},
	}
	got := srv.fallbackNodeAgentEndpointFromState("n1", "node-1.globular.internal:11000")
	if got != "10.0.0.63:11000" {
		t.Fatalf("fallback endpoint = %q, want %q", got, "10.0.0.63:11000")
	}
}
