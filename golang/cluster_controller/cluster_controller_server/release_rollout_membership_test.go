package main

import (
	"context"
	"testing"

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
	srv, enqueued := newSweepTestServer(t, &nodeState{
		NodeID:              "joiner",
		BootstrapPhase:      BootstrapWorkloadReady,
		Status:              "ready",
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
		AppliedServicesHash: "sha256:partial",
	})
	srv.enqueueReleasesForConvergingNodes(context.Background())
	if len(*enqueued) == 0 {
		t.Fatalf("expected ServiceRelease enqueue for a converging node, got none")
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
