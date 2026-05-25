package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeOSTTNode returns a nodeState fully eligible for objectstore membership:
// admitted lifecycle, workload-ready bootstrap, ObjectStoreIntent.Member=true,
// and a routable IP.
func makeOSTTNode(nodeID, ip string) *nodeState {
	return &nodeState{
		NodeID: nodeID,
		Identity: storedIdentity{
			Hostname: nodeID,
			Ips:      []string{ip},
		},
		Profiles:           []string{"storage"},
		Status:             "active",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: JoinPhaseAdmitted,
		ObjectStoreIntent:  &ObjectStoreIntent{Member: true},
	}
}

// makeOSTTMember wraps a node's ID and address into an ObjectStoreMember.
func makeOSTTMember(nodeID, ip string) ObjectStoreMember {
	return ObjectStoreMember{
		NodeID:           nodeID,
		Hostname:         nodeID,
		Address:          ip,
		AddedAt:          time.Now(),
		Source:           "test",
		IntentGeneration: 1,
	}
}

// makeSingleNodeState returns a nodes map with one eligible node.
func makeSingleNodeState(nodeID, ip string) map[string]*nodeState {
	return map[string]*nodeState{nodeID: makeOSTTNode(nodeID, ip)}
}

// ── Test 1: valid transition from gen N to N+1 passes preflight ───────────────

func TestOSTT_ValidTransitionPassesPreflight(t *testing.T) {
	nodes := makeSingleNodeState("node-a", "10.0.0.1")
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 0, 1, "initial pool", nodes, 0,
	)
	if tr.Status != ObjectStoreTransitionPending {
		t.Errorf("expected pending, got %q; preflight reason=%q", tr.Status, tr.Preflight.Reason)
	}
	if !tr.Preflight.OK {
		t.Errorf("preflight must pass, reason=%q", tr.Preflight.Reason)
	}
	if tr.FromGeneration != 0 || tr.ToGeneration != 1 {
		t.Errorf("generations wrong: from=%d to=%d", tr.FromGeneration, tr.ToGeneration)
	}
}

// ── Test 2: stale from_generation is rejected ─────────────────────────────────

func TestOSTT_StaleFromGenerationRejected(t *testing.T) {
	nodes := makeSingleNodeState("node-a", "10.0.0.1")
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	// currentGeneration is 3 but transition says from_generation=1 → stale.
	tr := requestObjectStoreTopologyTransition(
		requested, nil, 1, 2, "stale request", nodes, 3,
	)
	if tr.Status != ObjectStoreTransitionBlocked {
		t.Errorf("expected blocked, got %q", tr.Status)
	}
	if tr.Preflight.Reason != "stale_transition:from_generation_mismatch" {
		t.Errorf("unexpected preflight reason: %q", tr.Preflight.Reason)
	}
}

// ── Test 3: unapproved transition does not update DesiredObjectStoreMembers ───

func TestOSTT_UnapprovedTransitionDoesNotMutate(t *testing.T) {
	nodes := makeSingleNodeState("node-a", "10.0.0.1")
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	state := &controllerState{
		ObjectStoreGeneration:     0,
		DesiredObjectStoreMembers: nil,
	}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 0, 1, "test", nodes, 0,
	)
	// Do NOT call approve — call apply directly.
	err := applyObjectStoreTransition(tr, state)
	if err == nil {
		t.Fatal("applying unapproved transition must fail")
	}
	if state.DesiredObjectStoreMembers != nil {
		t.Error("DesiredObjectStoreMembers must not be mutated by unapproved transition")
	}
	if state.ObjectStoreGeneration != 0 {
		t.Error("ObjectStoreGeneration must not advance for unapproved transition")
	}
}

// ── Test 4: approved transition updates DesiredObjectStoreMembers and bumps gen

func TestOSTT_ApprovedTransitionUpdatesDesiredAndBumpsGeneration(t *testing.T) {
	nodes := makeSingleNodeState("node-a", "10.0.0.1")
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	state := &controllerState{
		ObjectStoreGeneration:     0,
		DesiredObjectStoreMembers: nil,
	}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 0, 1, "bootstrap", nodes, 0,
	)
	if err := approveObjectStoreTransition(tr, "test-operator"); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if err := applyObjectStoreTransition(tr, state); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if state.ObjectStoreGeneration != 1 {
		t.Errorf("expected generation=1, got %d", state.ObjectStoreGeneration)
	}
	if len(state.DesiredObjectStoreMembers) != 1 || state.DesiredObjectStoreMembers[0].NodeID != "node-a" {
		t.Errorf("DesiredObjectStoreMembers not updated: %+v", state.DesiredObjectStoreMembers)
	}
	if tr.Status != ObjectStoreTransitionApplied {
		t.Errorf("transition status must be applied, got %q", tr.Status)
	}
}

// ── Test 5: duplicate node IDs are rejected ────────────────────────────────────

func TestOSTT_DuplicateNodeIDsRejected(t *testing.T) {
	nodes := makeSingleNodeState("node-a", "10.0.0.1")
	requested := []ObjectStoreMember{
		makeOSTTMember("node-a", "10.0.0.1"),
		makeOSTTMember("node-a", "10.0.0.2"), // same NodeID
	}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 0, 1, "dup test", nodes, 0,
	)
	if tr.Status != ObjectStoreTransitionBlocked {
		t.Errorf("expected blocked, got %q", tr.Status)
	}
	if tr.Preflight.Reason != "invalid_transition:duplicate_node_id" {
		t.Errorf("unexpected preflight reason: %q", tr.Preflight.Reason)
	}
}

// ── Test 6: bootstrapping/blocked/removed node is rejected ────────────────────

func TestOSTT_BootstrappingBlockedRemovedNodeRejected(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*nodeState)
		wantReason string
	}{
		{
			name:   "bootstrapping lifecycle",
			mutate: func(n *nodeState) { n.JoinLifecyclePhase = JoinPhaseBootstrapping },
			wantReason: "invalid_transition:node_not_eligible",
		},
		{
			name:   "blocked status",
			mutate: func(n *nodeState) { n.Status = "blocked" },
			wantReason: "invalid_transition:node_excluded_by_status",
		},
		{
			name:   "removed status",
			mutate: func(n *nodeState) { n.Status = "removed" },
			wantReason: "invalid_transition:node_excluded_by_status",
		},
		{
			name:   "bootstrap_failed phase",
			mutate: func(n *nodeState) { n.BootstrapPhase = BootstrapFailed },
			wantReason: "invalid_transition:node_bootstrap_failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := makeOSTTNode("node-x", "10.0.0.5")
			tc.mutate(node)
			nodes := map[string]*nodeState{"node-x": node}
			requested := []ObjectStoreMember{makeOSTTMember("node-x", "10.0.0.5")}

			tr := requestObjectStoreTopologyTransition(
				requested, nil, 0, 1, tc.name, nodes, 0,
			)
			if tr.Status != ObjectStoreTransitionBlocked {
				t.Errorf("expected blocked, got %q", tr.Status)
			}
			if tr.Preflight.Reason != tc.wantReason {
				t.Errorf("expected %q, got %q", tc.wantReason, tr.Preflight.Reason)
			}
		})
	}
}

// ── Test 7: node without ObjectStoreIntent.Member=true is rejected ────────────

func TestOSTT_NodeWithoutObjectStoreIntentRejected(t *testing.T) {
	node := makeOSTTNode("node-a", "10.0.0.1")
	node.ObjectStoreIntent = nil // no intent
	nodes := map[string]*nodeState{"node-a": node}
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 0, 1, "no intent", nodes, 0,
	)
	if tr.Status != ObjectStoreTransitionBlocked {
		t.Errorf("expected blocked, got %q", tr.Status)
	}
	if tr.Preflight.Reason != "invalid_transition:node_missing_objectstore_intent" {
		t.Errorf("unexpected preflight reason: %q", tr.Preflight.Reason)
	}
}

// ── Test 8: storage profile alone is not sufficient ───────────────────────────

func TestOSTT_StorageProfileAloneNotSufficient(t *testing.T) {
	node := makeOSTTNode("node-a", "10.0.0.1")
	// Has storage profile but intent.Member=false — explicit exclusion.
	node.ObjectStoreIntent = &ObjectStoreIntent{Member: false}
	nodes := map[string]*nodeState{"node-a": node}
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 0, 1, "profile only", nodes, 0,
	)
	if tr.Status != ObjectStoreTransitionBlocked {
		t.Errorf("expected blocked (storage profile alone must not grant membership), got %q", tr.Status)
	}
	if tr.Preflight.Reason != "invalid_transition:node_missing_objectstore_intent" {
		t.Errorf("unexpected preflight reason: %q", tr.Preflight.Reason)
	}
}

// ── Test 9: generation mismatch prevents a second apply ───────────────────────

func TestOSTT_GenerationMismatchPreventsSecondApply(t *testing.T) {
	nodes := makeSingleNodeState("node-a", "10.0.0.1")
	requested := []ObjectStoreMember{makeOSTTMember("node-a", "10.0.0.1")}

	state := &controllerState{ObjectStoreGeneration: 0}

	// First transition: gen 0→1.
	tr1 := requestObjectStoreTopologyTransition(requested, nil, 0, 1, "first", nodes, 0)
	approveObjectStoreTransition(tr1, "op")
	if err := applyObjectStoreTransition(tr1, state); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if state.ObjectStoreGeneration != 1 {
		t.Fatalf("expected gen=1 after first apply, got %d", state.ObjectStoreGeneration)
	}

	// Attempting to apply tr1 again must fail: state is now at gen 1
	// but tr1.FromGeneration=0.
	err := applyObjectStoreTransition(tr1, state)
	if err == nil {
		t.Fatal("second apply of same transition must fail (stale from_generation)")
	}
	if state.ObjectStoreGeneration != 1 {
		t.Error("ObjectStoreGeneration must not change on stale apply")
	}
}

// ── Test 10: legacy nil DesiredObjectStoreMembers still falls back ────────────

func TestOSTT_LegacyNilDesiredMembersFallback(t *testing.T) {
	node := makeOSTTNode("legacy-node", "10.0.0.2")
	// nil DesiredObjectStoreMembers → legacy_profile_derived
	status := objectStoreMembershipStatus(node, nil)
	if status != "legacy_profile_derived" {
		t.Errorf("expected legacy_profile_derived, got %q", status)
	}
	if !nodeIsExplicitObjectStoreMember(node, nil) {
		t.Error("nil desired must return true (caller's profile guard applies)")
	}
}

// ── Test 11: explicit empty DesiredObjectStoreMembers is not legacy ────────────

func TestOSTT_ExplicitEmptyDesiredIsNotLegacy(t *testing.T) {
	node := makeOSTTNode("node-b", "10.0.0.3")
	// Non-nil but empty → not_listed (v2 mode with no members)
	status := objectStoreMembershipStatus(node, []ObjectStoreMember{})
	if status != "not_listed" {
		t.Errorf("empty non-nil desired must return not_listed, got %q", status)
	}
	if nodeIsExplicitObjectStoreMember(node, []ObjectStoreMember{}) {
		t.Error("empty non-nil desired must not be eligible")
	}
}

// ── Test 12: transition records serialize/deserialize correctly ────────────────

func TestOSTT_SerializationRoundTrip(t *testing.T) {
	nodes := makeSingleNodeState("node-c", "10.0.0.9")
	requested := []ObjectStoreMember{makeOSTTMember("node-c", "10.0.0.9")}

	tr := requestObjectStoreTopologyTransition(
		requested, nil, 5, 6, "round-trip test", nodes, 5,
	)
	approveObjectStoreTransition(tr, "test-op")

	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ObjectStoreTopologyTransition
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.TransitionID != tr.TransitionID {
		t.Errorf("TransitionID not preserved: got %q", got.TransitionID)
	}
	if got.FromGeneration != 5 || got.ToGeneration != 6 {
		t.Errorf("generations not preserved: from=%d to=%d", got.FromGeneration, got.ToGeneration)
	}
	if got.Status != ObjectStoreTransitionApproved {
		t.Errorf("status not preserved: got %q", got.Status)
	}
	if !got.Approved || got.ApprovedBy != "test-op" {
		t.Errorf("approval fields not preserved: approved=%v by=%q", got.Approved, got.ApprovedBy)
	}
	if len(got.RequestedMembers) != 1 || got.RequestedMembers[0].NodeID != "node-c" {
		t.Errorf("RequestedMembers not preserved: %+v", got.RequestedMembers)
	}
	if !got.Preflight.OK {
		t.Errorf("Preflight.OK not preserved")
	}
	if got.Preflight.CheckedAt.IsZero() {
		t.Error("Preflight.CheckedAt must be non-zero")
	}
}
