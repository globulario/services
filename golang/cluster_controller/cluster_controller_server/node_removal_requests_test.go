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

// Post-2026-06-05 lift: removal itself runs in the node.remove workflow.
// This test now verifies the queue-consumer's resolution logic — turning a
// hostname-only request into the canonical node_id — which is unique to
// node_removal_requests.go. The full removal path is exercised by the
// NodeRemoveControllerConfig handler tests in server_test.go.
func TestResolveNodeRemovalTarget_ResolveByHostname(t *testing.T) {
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

	nodeID, err := srv.resolveNodeRemovalTarget(nodeRemovalRequest{Hostname: "globule-lenovo"})
	if err != nil {
		t.Fatalf("resolveNodeRemovalTarget: %v", err)
	}
	if nodeID != "node-abc" {
		t.Fatalf("nodeID = %q, want %q (hostname resolution failed)", nodeID, "node-abc")
	}
}

func TestResolveNodeRemovalTarget_ResolveByIP(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-xyz"] = &nodeState{
		NodeID: "node-xyz",
		Identity: storedIdentity{
			Hostname: "globule-lenovo",
			Ips:      []string{"10.0.0.102"},
		},
		Profiles: []string{"storage"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)

	nodeID, err := srv.resolveNodeRemovalTarget(nodeRemovalRequest{IP: "10.0.0.102"})
	if err != nil {
		t.Fatalf("resolveNodeRemovalTarget: %v", err)
	}
	if nodeID != "node-xyz" {
		t.Fatalf("nodeID = %q, want %q (IP resolution failed)", nodeID, "node-xyz")
	}
}

func TestResolveNodeRemovalTarget_EmptyForUnknownTarget(t *testing.T) {
	srv := newTestServer(t, newControllerState())

	nodeID, err := srv.resolveNodeRemovalTarget(nodeRemovalRequest{Hostname: "no-such-host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeID != "" {
		t.Fatalf("nodeID = %q, want empty (unknown target should not resolve)", nodeID)
	}
}
