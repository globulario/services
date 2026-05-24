package main

import (
	"context"
	"testing"
)

func TestHandleNodeRemovalRequest_StaleRequestNotFoundIsClean(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	handled, err := srv.handleNodeRemovalRequest(context.Background(),
		nodeRemovalRequestPrefix+"missing-node", nil)
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if err != nil {
		t.Fatalf("expected not-found request to be non-fatal, got: %v", err)
	}
}

func TestHandleNodeRemovalRequest_MalformedSkipped(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	handled, err := srv.handleNodeRemovalRequest(context.Background(),
		"/globular/controller/node_removals/requests/", []byte(`{"bad":"payload"}`))
	if !handled {
		t.Fatal("expected malformed request to be consumed")
	}
	if err != nil {
		t.Fatalf("expected nil error for malformed request, got: %v", err)
	}
}

func TestHandleNodeRemovalRequest_NodeIDFromPayload(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	handled, err := srv.handleNodeRemovalRequest(context.Background(),
		"/globular/controller/node_removals/requests/", []byte(`{"node_id":"missing-node"}`))
	if !handled {
		t.Fatal("expected payload-based request to be handled")
	}
	if err != nil {
		t.Fatalf("expected payload-based missing node to be non-fatal, got: %v", err)
	}
}

func TestHandleNodeRemovalRequest_ResolveByHostname(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-abc"] = &nodeState{
		NodeID: "node-abc",
		Identity: storedIdentity{
			Hostname: "globule-lenovo",
			Ips:      []string{"10.0.0.102"},
		},
		Profiles: []string{"storage"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)

	handled, err := srv.handleNodeRemovalRequest(context.Background(),
		nodeRemovalRequestPrefix, []byte(`{"hostname":"globule-lenovo"}`))
	if !handled {
		t.Fatal("expected hostname request to be handled")
	}
	if err != nil {
		t.Fatalf("hostname request should succeed, got: %v", err)
	}
	srv.lock("test")
	_, exists := srv.state.Nodes["node-abc"]
	srv.unlock()
	if exists {
		t.Fatal("expected node to be removed by hostname selector")
	}
}
