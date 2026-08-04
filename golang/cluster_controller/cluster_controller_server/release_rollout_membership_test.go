package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// newSweepTestServer builds a server holding one AVAILABLE ServiceRelease in a
// memstore plus a recording releaseEnqueue, so enqueueReleasesForConvergingNodes
// can be exercised in isolation.
func newSweepTestServer(t *testing.T, nodes ...*nodeState) (*server, *[]string) {
	t.Helper()
	srv := &server{state: &controllerState{Nodes: map[string]*nodeState{}}}
	for _, n := range nodes {
		srv.state.Nodes[n.NodeID] = n
	}
	srv.resources = resourcestore.NewMemStore()
	if _, err := srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta:   &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/mcp"},
		Spec:   &cluster_controllerpb.ServiceReleaseSpec{PublisherID: "core@globular.io", ServiceName: "mcp"},
		Status: &cluster_controllerpb.ServiceReleaseStatus{Phase: cluster_controllerpb.ReleasePhaseAvailable},
	}); err != nil {
		t.Fatalf("seed ServiceRelease: %v", err)
	}
	var enqueued []string
	srv.releaseEnqueue = func(name string) { enqueued = append(enqueued, name) }
	return srv, &enqueued
}

// TestEnqueueReleasesForConvergingNodes_ReadyButUnconverged guards the
// rollout.partial_not_converged fix: a node that has reached Status "ready" on
// infra/hash convergence but never stamped a services hash (AppliedServicesHash
// == "") must still trigger the sweep, so AVAILABLE ServiceReleases (e.g. mcp,
// which reached AVAILABLE against the founding node before this node joined) are
// re-enqueued and dispatched to the late joiner. Before the fix the gate only
// fired for Status=="converging", so a "ready"-but-unconverged joiner was never
// served and applied_hash never stamped.
func TestEnqueueReleasesForConvergingNodes_ReadyButUnconverged(t *testing.T) {
	// Profiles + LastSeen make this a faithful "ready" joiner: the sweep now
	// gates on hasUnservedNodes, which (correctly) only counts a node as unserved
	// when it carries the release's target profile (mcp → control-plane) and is
	// actively heartbeating. A node cannot legitimately be "ready" without a
	// recent heartbeat, so setting these is realism, not test-gaming.
	srv, enqueued := newSweepTestServer(t, &nodeState{
		NodeID:              "joiner",
		BootstrapPhase:      BootstrapWorkloadReady,
		Status:              "ready",
		Profiles:            []string{"control-plane"},
		LastSeen:            time.Now(),
		AppliedServicesHash: "", // never converged — joined after release went AVAILABLE
	})
	srv.enqueueReleasesForConvergingNodes(context.Background())
	if len(*enqueued) == 0 {
		t.Fatalf("expected AVAILABLE ServiceRelease to be re-enqueued for a ready-but-unconverged joiner, got none")
	}
}

// TestEnqueueReleasesForConvergingNodes_AllConvergedNoChurn asserts the sweep is
// a no-op when every bootstrap-ready node has stamped a converged services hash
// — the widened gate must not introduce perpetual re-enqueue churn.
func TestEnqueueReleasesForConvergingNodes_AllConvergedNoChurn(t *testing.T) {
	srv, enqueued := newSweepTestServer(t, &nodeState{
		NodeID:              "converged",
		BootstrapPhase:      BootstrapWorkloadReady,
		Status:              "ready",
		AppliedServicesHash: "sha256:deadbeef",
	})
	srv.enqueueReleasesForConvergingNodes(context.Background())
	if len(*enqueued) != 0 {
		t.Fatalf("expected no enqueue for a fully-converged cluster, got %v", *enqueued)
	}
}

// TestEnqueueReleasesForConvergingNodes_ConvergingStillFires is a regression
// guard: the original trigger (Status=="converging") must keep firing.
func TestEnqueueReleasesForConvergingNodes_ConvergingStillFires(t *testing.T) {
	srv, enqueued := newSweepTestServer(t, &nodeState{
		NodeID:              "converging",
		BootstrapPhase:      BootstrapWorkloadReady,
		Status:              "converging",
		Profiles:            []string{"control-plane"},
		LastSeen:            time.Now(),
		AppliedServicesHash: "sha256:partial",
	})
	srv.enqueueReleasesForConvergingNodes(context.Background())
	if len(*enqueued) == 0 {
		t.Fatalf("expected ServiceRelease enqueue for a converging node, got none")
	}
}

// TestEnqueueReleasesForConvergingNodes_SkipsWhenNoUnservedNode pins the churn
// reduction: the sweep now gates each ServiceRelease on hasUnservedNodes, so a
// release with no dispatchable unserved node is NOT re-enqueued even while a node
// is unconverged. Here the sole unconverged node lacks the mcp release's target
// profile (control-plane), so dispatch could never serve it — re-enqueuing every
// 120s was pure churn that fed the workflow dispatch circuit breaker on Day-0.
func TestEnqueueReleasesForConvergingNodes_SkipsWhenNoUnservedNode(t *testing.T) {
	srv, enqueued := newSweepTestServer(t, &nodeState{
		NodeID:              "storage-only",
		BootstrapPhase:      BootstrapWorkloadReady,
		Status:              "converging",
		Profiles:            []string{"storage"}, // not control-plane → mcp never targets it
		LastSeen:            time.Now(),
		AppliedServicesHash: "",
	})
	srv.enqueueReleasesForConvergingNodes(context.Background())
	if len(*enqueued) != 0 {
		t.Fatalf("expected no enqueue when no node is a dispatchable target for the release, got %v", *enqueued)
	}
}

// TestEnqueueReleasesForConvergingNodes_UnreachableSkipped ensures a node that
// cannot receive a dispatch (unreachable) with an empty hash does not trigger
// pointless churn — hasUnservedNodes would skip it anyway.
func TestEnqueueReleasesForConvergingNodes_UnreachableSkipped(t *testing.T) {
	srv, enqueued := newSweepTestServer(t, &nodeState{
		NodeID:              "dead",
		BootstrapPhase:      BootstrapWorkloadReady,
		Status:              "unreachable",
		AppliedServicesHash: "",
	})
	srv.enqueueReleasesForConvergingNodes(context.Background())
	if len(*enqueued) != 0 {
		t.Fatalf("expected no enqueue for an unreachable node, got %v", *enqueued)
	}
}
