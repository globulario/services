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
	if handled {
		t.Fatal("expected malformed request to be skipped")
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
