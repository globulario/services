package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

type recordingEtcdMembershipManager struct {
	removeCalls []string
	lastDesired []memberNode
	removeErr   error
}

func (m *recordingEtcdMembershipManager) snapshotEtcdMembers(context.Context) (*etcdMemberState, error) {
	return &etcdMemberState{}, nil
}

func (m *recordingEtcdMembershipManager) reconcileEtcdJoinPhases(context.Context, []*nodeState) bool {
	return false
}

func (m *recordingEtcdMembershipManager) removeStaleMembers(_ context.Context, desiredEtcdNodes []memberNode) error {
	ids := make([]string, 0, len(desiredEtcdNodes))
	m.lastDesired = append([]memberNode(nil), desiredEtcdNodes...)
	for _, node := range desiredEtcdNodes {
		ids = append(ids, node.NodeID)
	}
	m.removeCalls = append(m.removeCalls, strings.Join(ids, ","))
	if m.removeErr != nil {
		return m.removeErr
	}
	return nil
}

// newTestServer creates a server with a writable temp state path for tests.
func newTestServer(t *testing.T, state *controllerState) *server {
	t.Helper()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	srv := newServer(defaultClusterControllerConfig(), "", statePath, state, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234")
	return srv
}

func TestRestartUnitsForSpecChanges(t *testing.T) {
	httpSpec := &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "http",
	}
	httpsSpec := &cluster_controllerpb.ClusterNetworkSpec{
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
	srv.setLeader(true, "test", "127.0.0.1:1234")
	opID := "op-complete"
	nodeID := "node-1"
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "running", 10, false, ""))

	_, err := srv.CompleteOperation(context.Background(), &cluster_controllerpb.CompleteOperationRequest{
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
	if op.last.GetPhase() != cluster_controllerpb.OperationPhase_OP_SUCCEEDED {
		t.Fatalf("expected succeeded phase, got %s", op.last.GetPhase())
	}
}

func TestCleanupTimedOutOperationsFails(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234")
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
	if op.last.GetPhase() != cluster_controllerpb.OperationPhase_OP_FAILED {
		t.Fatalf("expected failed phase, got %s", op.last.GetPhase())
	}
	if !strings.Contains(op.last.GetMessage(), "timed out") {
		t.Fatalf("expected timeout message, got %q", op.last.GetMessage())
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	state := newControllerState()
	srv := newTestServer(t, state)

	_, err := srv.RemoveNode(context.Background(), &cluster_controllerpb.RemoveNodeRequest{
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
	srv.setLeader(true, "test", "127.0.0.1:1234")

	resp, err := srv.RemoveNode(context.Background(), &cluster_controllerpb.RemoveNodeRequest{
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

	srv.lock("test")
	if _, exists := srv.state.Nodes["node-1"]; exists {
		t.Fatal("expected node to be removed from state")
	}
	srv.unlock()
}

func TestRemoveNodeRemovesEtcdMembershipBeforeDeletingState(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID: "node-1",
		Identity: storedIdentity{
			Hostname: "host-1",
			Ips:      []string{"10.0.0.1"},
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	state.Nodes["node-2"] = &nodeState{
		NodeID: "node-2",
		Identity: storedIdentity{
			Hostname: "host-2",
			Ips:      []string{"10.0.0.2"},
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)
	mgr := &recordingEtcdMembershipManager{}
	srv.etcdMembers = mgr

	resp, err := srv.RemoveNode(context.Background(), &cluster_controllerpb.RemoveNodeRequest{
		NodeId: "node-2",
		Force:  true,
		Drain:  false,
	})
	if err != nil {
		t.Fatalf("RemoveNode error: %v", err)
	}
	if resp.GetOperationId() == "" {
		t.Fatal("expected operation_id in response")
	}
	if len(mgr.removeCalls) != 1 {
		t.Fatalf("expected 1 etcd prune call, got %d", len(mgr.removeCalls))
	}
	if len(mgr.lastDesired) != 1 || mgr.lastDesired[0].NodeID != "node-1" {
		t.Fatalf("unexpected desired etcd membership: %+v", mgr.lastDesired)
	}

	srv.lock("test")
	_, removedExists := srv.state.Nodes["node-2"]
	_, remainingExists := srv.state.Nodes["node-1"]
	srv.unlock()
	if removedExists {
		t.Fatal("expected removed etcd node to be absent from state")
	}
	if !remainingExists {
		t.Fatal("expected remaining node to stay in state")
	}
}

func TestRemoveNodeAbortsWhenEtcdMembershipCleanupFails(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID: "node-1",
		Identity: storedIdentity{
			Hostname: "host-1",
			Ips:      []string{"10.0.0.1"},
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	state.Nodes["node-2"] = &nodeState{
		NodeID: "node-2",
		Identity: storedIdentity{
			Hostname: "host-2",
			Ips:      []string{"10.0.0.2"},
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)
	srv.etcdMembers = &recordingEtcdMembershipManager{removeErr: fmt.Errorf("quorum failure")}

	_, err := srv.RemoveNode(context.Background(), &cluster_controllerpb.RemoveNodeRequest{
		NodeId: "node-2",
		Force:  true,
		Drain:  false,
	})
	if err == nil {
		t.Fatal("expected RemoveNode to fail when etcd cleanup fails")
	}
	if !strings.Contains(err.Error(), "remove node etcd membership") {
		t.Fatalf("unexpected error: %v", err)
	}

	srv.lock("test")
	_, removedExists := srv.state.Nodes["node-2"]
	srv.unlock()
	if !removedExists {
		t.Fatal("expected node state to remain when etcd cleanup fails")
	}
}

func TestReconcileAdvanceInfraJoinsPrunesStaleEtcdMembers(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID: "node-1",
		Identity: storedIdentity{
			Hostname: "host-1",
			Ips:      []string{"10.0.0.1"},
		},
		Profiles: []string{"core"},
		Status:   "healthy",
	}
	srv := newTestServer(t, state)
	mgr := &recordingEtcdMembershipManager{}
	srv.etcdMembers = mgr

	if err := srv.reconcileAdvanceInfraJoins(context.Background(), state.ClusterId); err != nil {
		t.Fatalf("reconcileAdvanceInfraJoins error: %v", err)
	}
	if len(mgr.removeCalls) != 1 {
		t.Fatalf("expected stale-member prune during reconcile, got %d calls", len(mgr.removeCalls))
	}
	if len(mgr.lastDesired) != 1 || mgr.lastDesired[0].NodeID != "node-1" {
		t.Fatalf("unexpected desired etcd membership during reconcile: %+v", mgr.lastDesired)
	}
}

func TestGetClusterHealthEmpty(t *testing.T) {
	state := newControllerState()
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &cluster_controllerpb.GetClusterHealthRequest{})
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

	state.Nodes["healthy-node"] = &nodeState{
		NodeID:   "healthy-node",
		Identity: storedIdentity{Hostname: "healthy"},
		Status:   "healthy",
		LastSeen: now.Add(-30 * time.Second),
	}
	state.Nodes["unhealthy-node"] = &nodeState{
		NodeID:    "unhealthy-node",
		Identity:  storedIdentity{Hostname: "unhealthy"},
		Status:    "unhealthy",
		LastSeen:  now.Add(-5 * time.Minute),
		LastError: "connection refused",
	}

	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	resp, err := srv.GetClusterHealth(context.Background(), &cluster_controllerpb.GetClusterHealthRequest{})
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

	resp, err := srv.GetClusterHealth(context.Background(), &cluster_controllerpb.GetClusterHealthRequest{})
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
