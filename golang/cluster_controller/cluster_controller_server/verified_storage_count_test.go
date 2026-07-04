package main

import (
	"testing"
	"time"
)

// TestCountVerifiedNodesWithProfile_LabelIsNotCapacity is the P1 regression:
// a node that carries the "storage" (or "control-plane") LABEL but is not a
// verified cluster member must NOT count toward founding/storage/RF quorum
// (meta.limited_members_are_not_capacity /
// forbidden_fix:profile_label_counts_as_storage_capacity). A label is intent;
// verification is capacity.
func TestCountVerifiedNodesWithProfile_LabelIsNotCapacity(t *testing.T) {
	nodes := map[string]*nodeState{
		// Verified members (empty phases = legacy-eligible).
		"n1": {Profiles: []string{"storage", "control-plane"}},
		"n2": {Profiles: []string{"storage", "control-plane"}},
		// Labeled storage, but unreachable — not capacity.
		"n3": {Profiles: []string{"storage"}, Status: "unreachable"},
		// Labeled storage, but still joining the Scylla ring — not capacity.
		"n4": {
			Profiles:            []string{"storage"},
			ScyllaJoinStartedAt: time.Now(),
			ScyllaJoinPhase:     ScyllaJoinStarted,
		},
	}

	// The bug this guards against: the raw LABEL count is 4, which would falsely
	// satisfy MinQuorumNodes(3) even though only 2 nodes are real capacity.
	if raw := countNodesWithProfile(nodes, "storage"); raw != 4 {
		t.Fatalf("sanity: raw label count should be 4, got %d", raw)
	}

	// Verified capacity count excludes the unreachable and mid-join nodes.
	if got := countVerifiedNodesWithProfile(nodes, "storage"); got != 2 {
		t.Fatalf("expected 2 VERIFIED storage nodes (n3 unreachable, n4 mid-join excluded), got %d", got)
	}
	// So a 3-node storage quorum is NOT satisfied by 4 labels with only 2 verified.
	if countVerifiedNodesWithProfile(nodes, "storage") >= MinQuorumNodes {
		t.Fatal("2 verified storage nodes must NOT satisfy the 3-node storage quorum")
	}

	// control-plane: only the two verified nodes carry it.
	if got := countVerifiedNodesWithProfile(nodes, "control-plane"); got != 2 {
		t.Fatalf("expected 2 verified control-plane nodes, got %d", got)
	}
}
