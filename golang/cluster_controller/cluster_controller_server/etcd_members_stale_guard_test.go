package main

import "testing"

// TestRemoveStaleMembersRefusesEmptyDesiredSet and
// TestRemoveStaleMembersPreservesQuorumFloor are the regression guards for the
// CRITICAL fail-open bug in removeStaleMembers: an empty/ambiguous desired etcd
// set, or a batch that would break quorum, must NEVER authorize MemberRemove.
// The decision lives in the pure etcdRemovalRefusalReason(); a non-empty reason
// means "refuse — remove nothing".

// refusal returns true when the batch is refused (no members removed).
func refusal(desiredNodes, desiredPeerURLs, desiredHostnames, started, candidates int) bool {
	return etcdRemovalRefusalReason(desiredNodes, desiredPeerURLs, desiredHostnames, started, candidates) != ""
}

func TestRemoveStaleMembersRefusesEmptyDesiredSet(t *testing.T) {
	// Empty desired set + 3 live members all flagged stale → refuse (0 removes).
	if !refusal(0, 0, 0, 3, 3) {
		t.Fatal("empty desired set with live members MUST be refused (would remove all → quorum death)")
	}
	// Desired set has 3 nodes but ALL have empty StableIP AND empty hostname, so
	// no usable identity survived → every live member looks stale → refuse.
	if !refusal(3, 0, 0, 3, 3) {
		t.Fatal("desired nodes present but with no usable identities MUST be refused")
	}
}

func TestRemoveStaleMembersPreservesQuorumFloor(t *testing.T) {
	// 3 started members, 2 flagged stale → would leave 1, below quorum floor 2 → refuse.
	if !refusal(3, 3, 3, 3, 2) {
		t.Fatal("removing 2 of 3 members (leaves 1 < quorum 2) MUST be refused")
	}
	// 5 started, 3 flagged stale → would leave 2, below quorum floor 3 → refuse.
	if !refusal(5, 5, 5, 5, 3) {
		t.Fatal("removing 3 of 5 members (leaves 2 < quorum 3) MUST be refused")
	}
}

func TestRemoveStaleMembersAllowsQuorumSafeRemoval(t *testing.T) {
	// Normal case: 3 started, 1 genuinely stale → leaves 2 == quorum floor 2 → proceed.
	if refusal(3, 3, 3, 3, 1) {
		t.Fatalf("a single quorum-safe stale removal must proceed, got refusal: %q",
			etcdRemovalRefusalReason(3, 3, 3, 3, 1))
	}
	// 5 started, 2 stale → leaves 3 == quorum floor 3 → proceed.
	if refusal(5, 5, 5, 5, 2) {
		t.Fatalf("removing 2 of 5 (leaves 3 == quorum 3) must proceed, got refusal: %q",
			etcdRemovalRefusalReason(5, 5, 5, 5, 2))
	}
	// Nothing stale → trivially safe (no refusal even if desired set were empty).
	if refusal(0, 0, 0, 3, 0) {
		t.Fatal("zero removal candidates must never be refused")
	}
}
