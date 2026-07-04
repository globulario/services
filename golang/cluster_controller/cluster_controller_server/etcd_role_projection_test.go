package main

import (
	"context"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

// etcdNodeState builds an etcd-capable node (core profile) at the given IP in the
// given join phase.
func etcdNodeState(id, hostname, ip string, phase EtcdJoinPhase) *nodeState {
	return &nodeState{
		NodeID:        id,
		Identity:      storedIdentity{Hostname: hostname, Ips: []string{ip}},
		Profiles:      []string{"core"},
		EtcdJoinPhase: phase,
	}
}

// TestEtcdRoleProjection is the task#17(e) regression: a node's etcd join phase is
// projected from the LIVE member role, never from script assumptions. A learner
// must never be reported as verified.
func TestEtcdRoleProjection(t *testing.T) {
	ctx := context.Background()

	t.Run("script-added learner -> promoting, EtcdMemberID captured, NOT verified", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{etcdMember(42, "n2", "10.0.0.2", true)}}
		m := &etcdMemberManager{client: f}
		node := etcdNodeState("n2", "n2", "10.0.0.2", EtcdJoinNone)

		m.reconcileEtcdJoinPhases(ctx, []*nodeState{node})

		if node.EtcdJoinPhase != EtcdJoinPromoting {
			t.Fatalf("a learner must project to EtcdJoinPromoting, got %s", node.EtcdJoinPhase)
		}
		if node.EtcdJoinPhase == EtcdJoinVerified {
			t.Fatal("a learner must NEVER be verified")
		}
		if node.EtcdMemberID != 42 {
			t.Fatalf("EtcdMemberID must be captured for the learner (want 42), got %d", node.EtcdMemberID)
		}
	})

	t.Run("voter -> verified", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{etcdMember(7, "n1", "10.0.0.1", false)}}
		m := &etcdMemberManager{client: f}
		node := etcdNodeState("n1", "n1", "10.0.0.1", EtcdJoinNone)

		m.reconcileEtcdJoinPhases(ctx, []*nodeState{node})

		if node.EtcdJoinPhase != EtcdJoinVerified {
			t.Fatalf("a voting member must project to EtcdJoinVerified, got %s", node.EtcdJoinPhase)
		}
	})

	t.Run("promoted voter -> verified, EtcdMemberID survives until verified", func(t *testing.T) {
		// Node is mid-promotion (learner captured id 42), and etcd has now promoted
		// it to a voter (same id, IsLearner=false).
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{etcdMember(42, "n2", "10.0.0.2", false)}}
		m := &etcdMemberManager{client: f}
		node := etcdNodeState("n2", "n2", "10.0.0.2", EtcdJoinPromoting)
		node.EtcdMemberID = 42

		m.reconcileEtcdJoinPhases(ctx, []*nodeState{node})

		if node.EtcdJoinPhase != EtcdJoinVerified {
			t.Fatalf("a promoted learner must become verified, got %s", node.EtcdJoinPhase)
		}
		if node.EtcdMemberID != 42 {
			t.Fatalf("EtcdMemberID must be preserved through promotion->verified, got %d", node.EtcdMemberID)
		}
	})

	t.Run("still-learner stays promoting (never verified across cycles)", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{etcdMember(42, "n2", "10.0.0.2", true)}}
		m := &etcdMemberManager{client: f}
		node := etcdNodeState("n2", "n2", "10.0.0.2", EtcdJoinPromoting)
		node.EtcdMemberID = 42

		for i := 0; i < 3; i++ {
			m.reconcileEtcdJoinPhases(ctx, []*nodeState{node})
			if node.EtcdJoinPhase != EtcdJoinPromoting {
				t.Fatalf("a still-learner must stay promoting (cycle %d), got %s", i, node.EtcdJoinPhase)
			}
		}
	})

	t.Run("missing member does not fabricate verified", func(t *testing.T) {
		// etcd has NO member for this node's IP.
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{etcdMember(7, "other", "10.0.0.99", false)}}
		m := &etcdMemberManager{client: f}
		node := etcdNodeState("n2", "n2", "10.0.0.2", EtcdJoinNone)

		m.reconcileEtcdJoinPhases(ctx, []*nodeState{node})

		if node.EtcdJoinPhase == EtcdJoinVerified {
			t.Fatal("a node with no matching etcd member must NOT be marked verified")
		}
	})

	t.Run("promoting node whose member vanished is not faked to verified", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{}} // learner gone (failed pre-promotion)
		m := &etcdMemberManager{client: f}
		node := etcdNodeState("n2", "n2", "10.0.0.2", EtcdJoinPromoting)
		node.EtcdMemberID = 42

		m.reconcileEtcdJoinPhases(ctx, []*nodeState{node})

		if node.EtcdJoinPhase == EtcdJoinVerified {
			t.Fatal("a promoting node whose member disappeared must NOT be marked verified")
		}
	})
}
