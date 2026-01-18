package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

// newTestServer creates a server with a writable temp state path for tests.
func newTestServer(t *testing.T, state *controllerState) *server {
	t.Helper()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	return newServer(defaultClusterControllerConfig(), "", statePath, state, nil)
}

func TestRestartUnitsForSpecChanges(t *testing.T) {
	httpSpec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "http",
	}
	httpsSpec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
	}

	httpUnits := restartUnitsForSpec(httpSpec)
	if len(httpUnits) == 0 {
		t.Fatalf("expected restart units for http spec")
	}
	if !containsUnit(httpUnits, "globular-dns.service") {
		t.Fatalf("http restart units missing globular-dns.service")
	}
	httpsUnits := restartUnitsForSpec(httpsSpec)
	if len(httpsUnits) <= len(httpUnits) {
		t.Fatalf("expected https spec to include additional units")
	}
	found := false
	for _, unit := range httpsUnits {
		if unit == "globular-storage.service" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("https restart units missing globular-storage.service")
	}
}

func containsUnit(units []string, target string) bool {
	for _, u := range units {
		if u == target {
			return true
		}
	}
	return false
}

func TestCompleteOperationMarksDone(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	opID := "op-complete"
	nodeID := "node-1"
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "running", 10, false, ""))

	_, err := srv.CompleteOperation(context.Background(), &clustercontrollerpb.CompleteOperationRequest{
		OperationId: opID,
		NodeId:      nodeID,
		Success:     true,
		Message:     "done",
	})
	if err != nil {
		t.Fatalf("CompleteOperation error: %v", err)
	}
	op := srv.operations[opID]
	if op == nil || op.last == nil {
		t.Fatalf("operation state missing")
	}
	if !op.last.GetDone() {
		t.Fatalf("expected done=true, got %+v", op.last)
	}
	if op.last.GetPhase() != clustercontrollerpb.OperationPhase_OP_SUCCEEDED {
		t.Fatalf("expected succeeded phase, got %s", op.last.GetPhase())
	}
}

func TestCleanupTimedOutOperationsFails(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	opID := "op-timeout"
	srv.operations[opID] = &operationState{
		created: time.Now().Add(-(operationTimeout + time.Minute)),
		nodeID:  "node-x",
	}
	srv.cleanupTimedOutOperations()
	op := srv.operations[opID]
	if op == nil || op.last == nil {
		t.Fatalf("expected operation event after timeout")
	}
	if !op.last.GetDone() {
		t.Fatalf("expected done true after timeout, got %+v", op.last)
	}
	if op.last.GetPhase() != clustercontrollerpb.OperationPhase_OP_FAILED {
		t.Fatalf("expected failed phase, got %s", op.last.GetPhase())
	}
	if !strings.Contains(op.last.GetMessage(), "timed out") {
		t.Fatalf("expected timeout message, got %q", op.last.GetMessage())
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	state := newControllerState()
	srv := newTestServer(t, state)

	_, err := srv.RemoveNode(context.Background(), &clustercontrollerpb.RemoveNodeRequest{
		NodeId: "nonexistent-node",
	})
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestRemoveNodeSuccess(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID: "node-1",
		Identity: storedIdentity{
			Hostname: "test-host",
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)

	resp, err := srv.RemoveNode(context.Background(), &clustercontrollerpb.RemoveNodeRequest{
		NodeId: "node-1",
		Force:  true,
		Drain:  false,
	})
	if err != nil {
		t.Fatalf("RemoveNode error: %v", err)
	}
	if resp.GetOperationId() == "" {
		t.Fatal("expected operation_id in response")
	}
	if !strings.Contains(resp.GetMessage(), "removed") {
		t.Fatalf("expected 'removed' message, got: %s", resp.GetMessage())
	}

	// Verify node was actually removed
	srv.lock("test")
	if _, exists := srv.state.Nodes["node-1"]; exists {
		t.Fatal("expected node to be removed from state")
	}
	srv.unlock()
}

func TestGetClusterHealthEmpty(t *testing.T) {
	state := newControllerState()
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetTotalNodes() != 0 {
		t.Fatalf("expected 0 nodes, got %d", resp.GetTotalNodes())
	}
	if resp.GetStatus() != "unhealthy" {
		t.Fatalf("expected 'unhealthy' status for empty cluster, got %s", resp.GetStatus())
	}
}

func TestGetClusterHealthMixedNodes(t *testing.T) {
	state := newControllerState()
	now := time.Now()

	// Healthy node (seen recently)
	state.Nodes["healthy-node"] = &nodeState{
		NodeID:   "healthy-node",
		Identity: storedIdentity{Hostname: "healthy"},
		Status:   "healthy",
		LastSeen: now.Add(-30 * time.Second),
	}

	// Unhealthy node (seen long ago)
	state.Nodes["unhealthy-node"] = &nodeState{
		NodeID:    "unhealthy-node",
		Identity:  storedIdentity{Hostname: "unhealthy"},
		Status:    "unhealthy",
		LastSeen:  now.Add(-5 * time.Minute),
		LastError: "connection refused",
	}

	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetTotalNodes() != 2 {
		t.Fatalf("expected 2 nodes, got %d", resp.GetTotalNodes())
	}
	if resp.GetHealthyNodes() != 1 {
		t.Fatalf("expected 1 healthy node, got %d", resp.GetHealthyNodes())
	}
	if resp.GetUnhealthyNodes() != 1 {
		t.Fatalf("expected 1 unhealthy node, got %d", resp.GetUnhealthyNodes())
	}
	if resp.GetStatus() != "degraded" {
		t.Fatalf("expected 'degraded' status for mixed cluster, got %s", resp.GetStatus())
	}
}

func TestGetClusterHealthAllHealthy(t *testing.T) {
	state := newControllerState()
	now := time.Now()

	state.Nodes["node-1"] = &nodeState{
		NodeID:   "node-1",
		Identity: storedIdentity{Hostname: "node1"},
		Status:   "healthy",
		LastSeen: now.Add(-10 * time.Second),
	}
	state.Nodes["node-2"] = &nodeState{
		NodeID:   "node-2",
		Identity: storedIdentity{Hostname: "node2"},
		Status:   "healthy",
		LastSeen: now.Add(-20 * time.Second),
	}

	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetStatus() != "healthy" {
		t.Fatalf("expected 'healthy' status, got %s", resp.GetStatus())
	}
	if resp.GetHealthyNodes() != 2 {
		t.Fatalf("expected 2 healthy nodes, got %d", resp.GetHealthyNodes())
	}
}
