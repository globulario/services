package main

import (
	"testing"
	"time"
)

// TestBootstrap_LearnerAwaitingPromotionDoesNotFail is the task#17(f) regression:
// a node that joined etcd as a non-voting learner sits in EtcdJoinPromoting until
// the controller promotes it (which under Policy A′ may wait for a third node).
// Its bootstrap must NOT fail on the etcd-joining phase timeout while it waits —
// its etcd is a functional member. Once promoted to a voter (EtcdJoinVerified),
// bootstrap advances.
func TestBootstrap_LearnerAwaitingPromotionDoesNotFail(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n2",
		Identity:           storedIdentity{Hostname: "n2", Ips: []string{"10.0.0.2"}},
		Profiles:           []string{"core"},
		BootstrapPhase:     BootstrapEtcdJoining,
		EtcdJoinPhase:      EtcdJoinPromoting,               // learner awaiting promotion
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now().Add(-1 * time.Hour), // long past any phase timeout
	}
	nodes := []*nodeState{node}

	reconcileBootstrapPhases(nodes, nil, emitter)

	if node.BootstrapPhase == BootstrapFailed {
		t.Fatal("a learner awaiting promotion must NOT fail bootstrap on the etcd-joining timeout")
	}
	if node.BootstrapPhase != BootstrapEtcdJoining {
		t.Fatalf("a promoting learner must hold in etcd_joining, got %s", node.BootstrapPhase)
	}
	if node.BootstrapError != "etcd learner awaiting promotion to voter" {
		t.Fatalf("expected the learner-awaiting-promotion diagnostic, got %q", node.BootstrapError)
	}

	// Once the controller promotes the learner to a voter, bootstrap advances.
	node.EtcdJoinPhase = EtcdJoinVerified
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("after promotion to voter, bootstrap must advance to etcd_ready, got %s", node.BootstrapPhase)
	}
}
