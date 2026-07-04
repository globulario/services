package main

import (
	"context"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

// TestRemoveStaleMembersRefusesEmptyDesiredSet and
// TestRemoveStaleMembersPreservesQuorumFloor are the regression guards for the
// CRITICAL fail-open bug in removeStaleMembers: an empty/ambiguous desired etcd
// set, or a batch that would break quorum, must NEVER authorize MemberRemove.
// The decision lives in the pure etcdRemovalRefusalReason(); a non-empty reason
// means "refuse — remove nothing".
//
// The quorum floor is a VOTER concept (meta.limited_members_are_not_capacity):
// learners never raise it and, when removed, never lower the voter-survivor
// count. The pure-function args are (desiredNodes, desiredPeerURLs,
// desiredHostnames, startedVoters, removalCandidates, votingRemovalCandidates).

// refusal returns true when the batch is refused (no members removed).
func refusal(desiredNodes, desiredPeerURLs, desiredHostnames, startedVoters, candidates, votingCandidates int) bool {
	return etcdRemovalRefusalReason(desiredNodes, desiredPeerURLs, desiredHostnames, startedVoters, candidates, votingCandidates) != ""
}

func TestRemoveStaleMembersRefusesEmptyDesiredSet(t *testing.T) {
	// Empty desired set + 3 live voters all flagged stale → refuse (0 removes).
	if !refusal(0, 0, 0, 3, 3, 3) {
		t.Fatal("empty desired set with live members MUST be refused (would remove all → quorum death)")
	}
	// Desired set has 3 nodes but ALL have empty StableIP AND empty hostname, so
	// no usable identity survived → every live member looks stale → refuse.
	if !refusal(3, 0, 0, 3, 3, 3) {
		t.Fatal("desired nodes present but with no usable identities MUST be refused")
	}
}

func TestRemoveStaleMembersPreservesQuorumFloor(t *testing.T) {
	// 3 started voters, 2 flagged stale → would leave 1, below quorum floor 2 → refuse.
	if !refusal(3, 3, 3, 3, 2, 2) {
		t.Fatal("removing 2 of 3 voters (leaves 1 < quorum 2) MUST be refused")
	}
	// 5 started voters, 3 flagged stale → would leave 2, below quorum floor 3 → refuse.
	if !refusal(5, 5, 5, 5, 3, 3) {
		t.Fatal("removing 3 of 5 voters (leaves 2 < quorum 3) MUST be refused")
	}
}

func TestRemoveStaleMembersAllowsQuorumSafeRemoval(t *testing.T) {
	// Normal case: 3 started voters, 1 genuinely stale → leaves 2 == quorum floor 2 → proceed.
	if refusal(3, 3, 3, 3, 1, 1) {
		t.Fatalf("a single quorum-safe stale removal must proceed, got refusal: %q",
			etcdRemovalRefusalReason(3, 3, 3, 3, 1, 1))
	}
	// 5 started voters, 2 stale → leaves 3 == quorum floor 3 → proceed.
	if refusal(5, 5, 5, 5, 2, 2) {
		t.Fatalf("removing 2 of 5 voters (leaves 3 == quorum 3) must proceed, got refusal: %q",
			etcdRemovalRefusalReason(5, 5, 5, 5, 2, 2))
	}
	// Nothing stale → trivially safe (no refusal even if desired set were empty).
	if refusal(0, 0, 0, 3, 0, 0) {
		t.Fatal("zero removal candidates must never be refused")
	}
}

// TestRemovalRefusal_LearnerNeverCountsAsQuorumCapacity is the direct
// meta.limited_members_are_not_capacity guard on the pure decision. A learner
// present in the cluster must not change the voter-based math.
func TestRemovalRefusal_LearnerNeverCountsAsQuorumCapacity(t *testing.T) {
	// 2 voters + 1 learner, remove 1 VOTER. startedVoters=2 (learner excluded by
	// the caller), votingRemovalCandidates=1 → survivors 1 < quorum 2 → REFUSE.
	// If the learner were miscounted as a 3rd started voter, the floor would be 2
	// and survivors 2 → wrongly ALLOWED. Asserting refusal proves the ghost weight
	// is gone.
	if !refusal(3, 3, 3, /*startedVoters*/ 2, /*candidates*/ 1, /*votingCandidates*/ 1) {
		t.Fatal("with 2 real voters, removing 1 voter must be refused regardless of any learner present")
	}

	// Removing a stale LEARNER is always quorum-safe: votingRemovalCandidates=0 →
	// voter survivors unchanged. Even down to a single voter.
	if refusal(3, 3, 3, /*startedVoters*/ 3, /*candidates*/ 1, /*votingCandidates*/ 0) {
		t.Fatal("removing a stale learner (0 voting removals) must never be refused")
	}
	if refusal(1, 1, 1, /*startedVoters*/ 1, /*candidates*/ 1, /*votingCandidates*/ 0) {
		t.Fatal("removing a stale learner from a 1-voter cluster must never be refused")
	}
}

// etcdMember builds a started (named) member with a peer URL at the given IP.
func etcdMember(id uint64, name, ip string, learner bool) *etcdserverpb.Member {
	return &etcdserverpb.Member{
		ID:        id,
		Name:      name,
		PeerURLs:  []string{"https://" + ip + ":2380"},
		IsLearner: learner,
	}
}

// TestRemoveStaleMembers_LoopExcludesLearners proves the fix at the loop level
// (not just the pure guard): removeStaleMembers must compute startedVoters and
// votingRemovalCandidates with IsLearner excluded, driven against a fake etcd.
func TestRemoveStaleMembers_LoopExcludesLearners(t *testing.T) {
	ctx := context.Background()

	t.Run("stale learner is removed (learner removal is always quorum-safe)", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{
			etcdMember(1, "n1", "10.0.0.1", false), // voter, desired
			etcdMember(2, "n2", "10.0.0.2", false), // voter, desired
			etcdMember(3, "n3", "10.0.0.3", true),  // learner, STALE
		}}
		m := &etcdMemberManager{client: f}
		desired := []memberNode{
			{Hostname: "n1", IP: "10.0.0.1"},
			{Hostname: "n2", IP: "10.0.0.2"},
		}
		if err := m.removeStaleMembers(ctx, desired); err != nil {
			t.Fatalf("removeStaleMembers: %v", err)
		}
		if len(f.removeCalls) != 1 || f.removeCalls[0] != 3 {
			t.Fatalf("expected exactly the stale learner (id 3) removed; got %v", f.removeCalls)
		}
	})

	t.Run("learner is not counted as a voter — dropping a real voter below quorum is refused", func(t *testing.T) {
		// 2 voters {n1 desired, n2 STALE} + 1 learner {n3 desired}. Removing the
		// stale voter n2 would leave 1 of 2 voters < quorum 2 → the batch must be
		// REFUSED and nothing removed. If n3 (learner) were counted as a 3rd voter,
		// the guard would compute survivors 2 >= quorum 2 and wrongly allow it.
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{
			etcdMember(1, "n1", "10.0.0.1", false), // voter, desired
			etcdMember(2, "n2", "10.0.0.2", false), // voter, STALE
			etcdMember(3, "n3", "10.0.0.3", true),  // learner, desired
		}}
		m := &etcdMemberManager{client: f}
		desired := []memberNode{
			{Hostname: "n1", IP: "10.0.0.1"},
			{Hostname: "n3", IP: "10.0.0.3"},
		}
		err := m.removeStaleMembers(ctx, desired)
		if err == nil {
			t.Fatal("expected removal to be REFUSED (would drop voters below quorum), got nil error")
		}
		if len(f.removeCalls) != 0 {
			t.Fatalf("no member may be removed when the batch is refused; got %v", f.removeCalls)
		}
	})
}
