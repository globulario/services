package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// A node carrying a reported error may never be counted healthy or rendered
// green.
//
// The defect, observed live on a 5-node cluster: `globular cluster health`
// printed "Cluster Status: HEALTHY / Healthy: 5 / Unhealthy: 0" while showing
//
//	✅ a166b992… (node-3)
//	   Error: globular-file.service is inactive
//
// The classifier tested `node.LastError != ""` only in its THIRD switch arm,
// which a "ready" node with a fresh heartbeat could never reach — the first arm
// matched first. The error string was still attached to the response, so the
// operator saw a green check and an error on consecutive lines.
//
// The rule these pin: a node with active reported errors from the selected
// evidence snapshot cannot be displayed as healthy or counted in the healthy
// aggregate. It may be labelled converging or degraded — never silently green.
//
// Scope note: this is a classification and presentation change only. It does not
// reinterpret doctor severity, and cluster health and doctor report remain
// different instruments answering different questions. They are simply not
// permitted to issue logically incompatible verdicts from the same evidence.

func healthNode(id, hostname, status, lastErr string, seenAgo time.Duration) *nodeState {
	return &nodeState{
		NodeID:    id,
		Identity:  storedIdentity{Hostname: hostname},
		Status:    status,
		LastSeen:  time.Now().Add(-seenAgo),
		LastError: lastErr,
	}
}

func healthOf(t *testing.T, nodes ...*nodeState) *cluster_controllerpb.GetClusterHealthResponse {
	t.Helper()
	state := newControllerState()
	for _, n := range nodes {
		state.Nodes[n.NodeID] = n
	}
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)
	resp, err := srv.GetClusterHealth(context.Background(), &cluster_controllerpb.GetClusterHealthRequest{})
	if err != nil {
		t.Fatalf("GetClusterHealth: %v", err)
	}
	return resp
}

func nodeStatusOf(t *testing.T, resp *cluster_controllerpb.GetClusterHealthResponse, id string) *cluster_controllerpb.NodeHealthStatus {
	t.Helper()
	for _, n := range resp.GetNodeHealth() {
		if n.GetNodeId() == id {
			return n
		}
	}
	t.Fatalf("node %s absent from health response", id)
	return nil
}

//  1. A ready node with no reported error is healthy. The fix must not make
//     ordinary nodes look broken.
func TestHealth_ReadyNoError_IsHealthy(t *testing.T) {
	resp := healthOf(t, healthNode("n1", "node-1", "ready", "", 30*time.Second))
	if got := nodeStatusOf(t, resp, "n1").GetStatus(); got != "healthy" {
		t.Fatalf("status = %q, want healthy", got)
	}
	if resp.GetHealthyNodes() != 1 || resp.GetUnhealthyNodes() != 0 {
		t.Fatalf("healthy=%d unhealthy=%d, want 1/0", resp.GetHealthyNodes(), resp.GetUnhealthyNodes())
	}
	if resp.GetStatus() != "healthy" {
		t.Fatalf("cluster status = %q, want healthy", resp.GetStatus())
	}
}

//  2. THE DEFECT. A ready node with an active error is neither green nor counted
//     healthy, even though its heartbeat is fresh and its coarse status says
//     "ready".
func TestHealth_ReadyWithActiveError_NotGreenNotCounted(t *testing.T) {
	resp := healthOf(t, healthNode("n1", "node-1", "ready", "globular-file.service is inactive", 30*time.Second))

	n := nodeStatusOf(t, resp, "n1")
	if n.GetStatus() == "healthy" {
		t.Fatalf("a node reporting %q was classified healthy", n.GetLastError())
	}
	if resp.GetHealthyNodes() != 0 {
		t.Fatalf("healthy count = %d, want 0 — a node with an active error must not be counted healthy",
			resp.GetHealthyNodes())
	}
	if n.GetLastError() == "" {
		t.Error("the error must still be reported, not swallowed by the reclassification")
	}
	if n.GetStatus() != "degraded" {
		t.Errorf("status = %q, want degraded (ready node claiming done while reporting an error)", n.GetStatus())
	}
}

//  3. A node still converging keeps the softer label. Stale unit evidence during
//     a join is expected and must not be escalated to degraded.
func TestHealth_ConvergingWithError_IsConvergingNotDegraded(t *testing.T) {
	resp := healthOf(t, healthNode("n1", "node-1", "converging", "globular-scylla-manager.service missing", 20*time.Second))

	n := nodeStatusOf(t, resp, "n1")
	if n.GetStatus() != "converging" {
		t.Fatalf("status = %q, want converging — a settling node must not be labelled degraded", n.GetStatus())
	}
	if resp.GetHealthyNodes() != 0 {
		t.Errorf("converging-with-error must not be counted healthy, got %d", resp.GetHealthyNodes())
	}
}

//  4. The classification follows the evidence: once a newer snapshot carries no
//     error, the node is healthy again. Nothing latches.
func TestHealth_ErrorClearsInNewerSnapshot_HealthyAgain(t *testing.T) {
	before := healthOf(t, healthNode("n1", "node-1", "ready", "globular-file.service is inactive", 15*time.Second))
	if before.GetHealthyNodes() != 0 {
		t.Fatalf("precondition: expected 0 healthy while erroring, got %d", before.GetHealthyNodes())
	}

	after := healthOf(t, healthNode("n1", "node-1", "ready", "", 15*time.Second))
	if after.GetHealthyNodes() != 1 {
		t.Fatalf("healthy count = %d after the error cleared, want 1 — classification must not latch",
			after.GetHealthyNodes())
	}
	if got := nodeStatusOf(t, after, "n1").GetStatus(); got != "healthy" {
		t.Fatalf("status = %q after clearing, want healthy", got)
	}
}

//  5. Absent evidence is not health. A node not seen within the freshness
//     threshold is unknown, never silently healthy.
func TestHealth_MissingEvidence_NotSilentlyHealthy(t *testing.T) {
	resp := healthOf(t, healthNode("n1", "node-1", "ready", "", 10*time.Minute))

	n := nodeStatusOf(t, resp, "n1")
	if n.GetStatus() == "healthy" {
		t.Fatal("a node with no recent evidence was classified healthy")
	}
	if resp.GetHealthyNodes() != 0 {
		t.Errorf("healthy count = %d, want 0", resp.GetHealthyNodes())
	}
	if n.GetLastError() == "" {
		t.Error("staleness should be explained, not silent")
	}
}

//  6. The displayed rows and the aggregate counters must describe the same
//     cluster. A green row that no counter accounts for is exactly how the
//     original defect stayed invisible.
func TestHealth_RowsAndAggregatesAgree(t *testing.T) {
	resp := healthOf(t,
		healthNode("n1", "node-1", "ready", "", 10*time.Second),
		healthNode("n2", "node-2", "ready", "unit down", 10*time.Second),
		healthNode("n3", "node-3", "converging", "still installing", 10*time.Second),
		healthNode("n4", "node-4", "ready", "", 10*time.Minute),
	)

	var healthy, notHealthy, unknown int
	for _, n := range resp.GetNodeHealth() {
		switch n.GetStatus() {
		case "healthy":
			healthy++
		case "unknown":
			unknown++
		default:
			notHealthy++
		}
	}

	if int(resp.GetTotalNodes()) != len(resp.GetNodeHealth()) {
		t.Fatalf("TotalNodes=%d but %d rows returned", resp.GetTotalNodes(), len(resp.GetNodeHealth()))
	}
	if healthy != int(resp.GetHealthyNodes()) {
		t.Errorf("%d green rows but HealthyNodes=%d", healthy, resp.GetHealthyNodes())
	}
	if notHealthy != int(resp.GetUnhealthyNodes()) {
		t.Errorf("%d non-healthy rows but UnhealthyNodes=%d", notHealthy, resp.GetUnhealthyNodes())
	}
	if unknown != int(resp.GetUnknownNodes()) {
		t.Errorf("%d unknown rows but UnknownNodes=%d", unknown, resp.GetUnknownNodes())
	}
	if sum := resp.GetHealthyNodes() + resp.GetUnhealthyNodes() + resp.GetUnknownNodes(); sum != resp.GetTotalNodes() {
		t.Errorf("counters sum to %d, total is %d — the buckets must partition the nodes", sum, resp.GetTotalNodes())
	}
}

//  7. The rollup may not claim an unqualified healthy verdict while any node is
//     actively reporting a problem. This is the headline an operator reads first.
func TestHealth_ClusterNotHealthyWhileAnyNodeReportsError(t *testing.T) {
	resp := healthOf(t,
		healthNode("n1", "node-1", "ready", "", 10*time.Second),
		healthNode("n2", "node-2", "ready", "", 10*time.Second),
		healthNode("n3", "node-3", "ready", "globular-file.service is inactive", 10*time.Second),
	)

	if resp.GetStatus() == "healthy" {
		t.Fatalf("cluster reported HEALTHY while node-3 reported %q",
			nodeStatusOf(t, resp, "n3").GetLastError())
	}
	if resp.GetHealthyNodes() >= resp.GetTotalNodes() {
		t.Errorf("healthy=%d of total=%d — the erroring node was counted",
			resp.GetHealthyNodes(), resp.GetTotalNodes())
	}
}
