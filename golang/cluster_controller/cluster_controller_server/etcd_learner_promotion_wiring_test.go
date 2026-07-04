package main

import (
	"context"
	"fmt"
	"testing"
)

// TestReconcileAdvanceInfraJoins_WiresLearnerPromotion proves step (d): the
// reconcile loop drives reconcileLearnerPromotion once per cycle, with the target
// voter count equal to the admitted etcd membership (len(desiredEtcdNodes)). That
// target is what makes Policy A′ wait for a third node before promoting the
// second and never settle at a transient 2 voters.
func TestReconcileAdvanceInfraJoins_WiresLearnerPromotion(t *testing.T) {
	state := newControllerState()
	// Three admitted etcd-capable nodes (core profile) → target should be 3.
	for i, id := range []string{"n1", "n2", "n3"} {
		state.Nodes[id] = &nodeState{
			NodeID:   id,
			Identity: storedIdentity{Hostname: fmt.Sprintf("host-%d", i+1), Ips: []string{fmt.Sprintf("10.0.0.%d", i+1)}},
			Profiles: []string{"core"},
			Status:   "healthy",
		}
	}
	srv := newTestServer(t, state)
	mgr := &recordingEtcdMembershipManager{}
	srv.etcdMembers = mgr

	if err := srv.reconcileAdvanceInfraJoins(context.Background(), "test-cluster"); err != nil {
		t.Fatalf("reconcileAdvanceInfraJoins: %v", err)
	}

	if len(mgr.promotionTargets) != 1 {
		t.Fatalf("expected reconcileLearnerPromotion invoked exactly once, got %d calls", len(mgr.promotionTargets))
	}
	if got := mgr.promotionTargets[0]; got != 3 {
		t.Fatalf("expected promotion target = admitted etcd membership (3), got %d", got)
	}
}
