package main

import (
	"testing"
	"time"
)

// TestBootstrap_LearnerWithNoVoterWaits: a promoting learner that has NO healthy
// voter to defer to must hold in etcd_joining (a genuine wait) and must NOT fail
// bootstrap on the phase timeout — its etcd is a functional member. This is the
// "no write authority yet" branch.
func TestBootstrap_LearnerWithNoVoterWaits(t *testing.T) {
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
	nodes := []*nodeState{node} // no other node → no healthy voter

	reconcileBootstrapPhases(nodes, nil, emitter)

	if node.BootstrapPhase == BootstrapFailed {
		t.Fatal("a learner with no voter must NOT fail bootstrap on the etcd-joining timeout")
	}
	if node.BootstrapPhase != BootstrapEtcdJoining {
		t.Fatalf("a learner with no voter must hold in etcd_joining, got %s", node.BootstrapPhase)
	}
	if node.BootstrapError != "etcd learner awaiting a healthy voter before bootstrap can proceed" {
		t.Fatalf("expected the awaiting-voter diagnostic, got %q", node.BootstrapError)
	}

	// Once the controller promotes the learner to a voter, bootstrap advances.
	node.EtcdJoinPhase = EtcdJoinVerified
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("after promotion to voter, bootstrap must advance to etcd_ready, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_HealthyLearnerWithVoterProceedsWithoutDeclaredPolicy is the core
// buildout-contract regression: a healthy non-voting learner that defers to a
// healthy voter must PROCEED through bootstrap as an explicit non-voter WITHOUT any
// declared two_node_degraded storage policy (bootstrap is not steady state; you
// cannot reach 3 voters without first passing through 2). It must:
//   - advance past etcd_joining to etcd_ready,
//   - be marked etcd_degraded / not-HA (surfaced, not hidden),
//   - remain a LEARNER (EtcdJoinPhase stays EtcdJoinPromoting) — Policy A' must not
//     be short-circuited into a 2-voter promotion.
func TestBootstrap_HealthyLearnerWithVoterProceedsWithoutDeclaredPolicy(t *testing.T) {
	emitter := &mockEmitter{}
	now := time.Now()

	// Founder / existing voter: empty EtcdJoinPhase, recently seen, already done
	// bootstrapping (skipped by reconcile, but counts as a healthy voter).
	founder := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "n1", Ips: []string{"10.0.0.1"}},
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapWorkloadReady,
		LastSeen:       now,
	}
	learner := &nodeState{
		NodeID:             "n2",
		Identity:           storedIdentity{Hostname: "n2", Ips: []string{"10.0.0.2"}},
		Profiles:           []string{"core"},
		BootstrapPhase:     BootstrapEtcdJoining,
		EtcdJoinPhase:      EtcdJoinPromoting,
		LastSeen:           now,
		BootstrapStartedAt: now.Add(-1 * time.Hour),
	}
	nodes := []*nodeState{founder, learner}

	reconcileBootstrapPhases(nodes, nil, emitter)

	if learner.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("a healthy learner with a healthy voter must advance to etcd_ready without a declared policy, got %s", learner.BootstrapPhase)
	}
	if !learner.EtcdBootstrapDegraded {
		t.Fatal("the proceeding learner must be marked EtcdBootstrapDegraded (etcd_ha=false, surfaced not hidden)")
	}
	if learner.EtcdDegradedReason == "" {
		t.Fatal("the proceeding learner must carry a not-HA reason")
	}
	if learner.BootstrapError != "" {
		t.Fatalf("BootstrapError must be cleared once the learner proceeds, got %q", learner.BootstrapError)
	}
	// Policy A' invariant: the node MUST still be a learner — never auto-promoted to
	// a 2-voter etcd. Real voter promotion happens only when a 3rd node arrives.
	if learner.EtcdJoinPhase != EtcdJoinPromoting {
		t.Fatalf("proceeding must NOT promote the learner to a voter; EtcdJoinPhase must stay EtcdJoinPromoting, got %s", learner.EtcdJoinPhase)
	}
}
