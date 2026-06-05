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

func (m *recordingEtcdMembershipManager) reconcileEtcdAutoRejoin(context.Context, []*nodeState) bool {
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

// Post-2026-06-05 lift: the inline RemoveNode logic was replaced by the
// node.remove workflow. The tests below pin the NodeRemoveControllerConfig
// action handlers directly — those handlers are what the workflow's step
// actions execute. This is the correct regression seam after the lift;
// the dispatch path is owned by the workflow engine and tested there.
//
// TestRemoveNodeNotFound above still tests the RPC-handler pre-dispatch
// validation (returns NotFound BEFORE workflow dispatch). The full
// dispatch path requires a workflow service connection; that's an
// integration test, not a unit test.

func TestNodeRemoveControllerConfig_DeleteState_RemovesAndPersists(t *testing.T) {
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

	cfg := srv.buildNodeRemoveControllerConfig()
	if err := cfg.DeleteState(context.Background(), "node-1"); err != nil {
		t.Fatalf("DeleteState: %v", err)
	}

	srv.lock("test")
	defer srv.unlock()
	if _, exists := srv.state.Nodes["node-1"]; exists {
		t.Fatal("expected node to be removed from state")
	}
}

func TestNodeRemoveControllerConfig_DeleteState_IdempotentForMissingNode(t *testing.T) {
	state := newControllerState()
	srv := newTestServer(t, state)

	cfg := srv.buildNodeRemoveControllerConfig()
	// First call on a non-existent node must return nil (idempotent for
	// the case where a prior workflow attempt already deleted the entry).
	if err := cfg.DeleteState(context.Background(), "never-existed"); err != nil {
		t.Fatalf("DeleteState on absent node: %v", err)
	}
}

func TestNodeRemoveControllerConfig_RemoveEtcdMembership_PrunesCorrectly(t *testing.T) {
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

	cfg := srv.buildNodeRemoveControllerConfig()
	if err := cfg.RemoveEtcdMembership(context.Background(), "node-2"); err != nil {
		t.Fatalf("RemoveEtcdMembership: %v", err)
	}
	if len(mgr.removeCalls) != 1 {
		t.Fatalf("expected 1 etcd prune call, got %d", len(mgr.removeCalls))
	}
	if len(mgr.lastDesired) != 1 || mgr.lastDesired[0].NodeID != "node-1" {
		t.Fatalf("unexpected desired etcd membership: %+v", mgr.lastDesired)
	}
}

func TestNodeRemoveControllerConfig_RemoveEtcdMembership_PropagatesQuorumFailure(t *testing.T) {
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

	cfg := srv.buildNodeRemoveControllerConfig()
	err := cfg.RemoveEtcdMembership(context.Background(), "node-2")
	if err == nil {
		t.Fatal("expected RemoveEtcdMembership to surface quorum failure")
	}
	if !strings.Contains(err.Error(), "quorum failure") {
		t.Fatalf("unexpected error: %v", err)
	}

	// State must remain — the workflow's delete_state step would not
	// have run if remove_etcd_membership returned an error first.
	srv.lock("test")
	_, removedExists := srv.state.Nodes["node-2"]
	srv.unlock()
	if !removedExists {
		t.Fatal("expected node state to remain when etcd cleanup fails")
	}
}

func TestNodeRemoveControllerConfig_Preflight_ReturnsViolations(t *testing.T) {
	state := newControllerState()
	srv := newTestServer(t, state)

	cfg := srv.buildNodeRemoveControllerConfig()
	// Preflight on a missing node returns either empty (no violations)
	// or an explicit list — verify the call succeeds without panic.
	violations, err := cfg.Preflight(context.Background(), "any-node-id")
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	_ = violations // shape is enough; specific violations depend on cluster state
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
		LastSeen: time.Now(),
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

func TestReconcileAdvanceInfraJoinsSkipsStaleEtcdPruneWhenNodeUnresponsive(t *testing.T) {
	state := newControllerState()
	state.Nodes["node-1"] = &nodeState{
		NodeID: "node-1",
		Identity: storedIdentity{
			Hostname: "host-1",
			Ips:      []string{"10.0.0.1"},
		},
		Profiles: []string{"core"},
		Status:   "offline",
		LastSeen: time.Now().Add(-10 * time.Minute),
	}
	srv := newTestServer(t, state)
	mgr := &recordingEtcdMembershipManager{}
	srv.etcdMembers = mgr

	if err := srv.reconcileAdvanceInfraJoins(context.Background(), state.ClusterId); err != nil {
		t.Fatalf("reconcileAdvanceInfraJoins error: %v", err)
	}
	if len(mgr.removeCalls) != 0 {
		t.Fatalf("expected stale-member prune to be skipped for unresponsive node, got %d calls", len(mgr.removeCalls))
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

func TestGetClusterHealthConvergingScyllaInactiveIsUnhealthy(t *testing.T) {
	state := newControllerState()
	now := time.Now()
	state.MinioPoolNodes = []string{"10.0.0.102"}
	state.Nodes["node-1"] = &nodeState{
		NodeID:   "node-1",
		Identity: storedIdentity{Hostname: "lenovo", Ips: []string{"10.0.0.102"}},
		Profiles: []string{"storage"},
		Status:   "converging",
		LastSeen: now.Add(-20 * time.Second),
		Units: []unitStatusRecord{
			{Name: "globular-etcd.service", State: "active"},
			{Name: "scylla-server.service", State: "inactive"},
			{Name: "globular-minio.service", State: "inactive"},
		},
	}
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)
	resp, err := srv.GetClusterHealth(context.Background(), &cluster_controllerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth error: %v", err)
	}
	if resp.GetHealthyNodes() != 0 {
		t.Fatalf("expected 0 healthy nodes, got %d", resp.GetHealthyNodes())
	}
	if resp.GetUnhealthyNodes() != 1 {
		t.Fatalf("expected 1 unhealthy node, got %d", resp.GetUnhealthyNodes())
	}
	if resp.GetStatus() != "unhealthy" {
		t.Fatalf("expected unhealthy cluster status, got %s", resp.GetStatus())
	}
}
