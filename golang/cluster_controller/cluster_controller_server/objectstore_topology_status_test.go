package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeStatusNode returns a nodeState with ObjectStoreIntent.Member=true and
// lifecycle fully eligible (admitted, workload_ready, active).
func makeStatusNode(nodeID, ip string) *nodeState {
	return &nodeState{
		NodeID: nodeID,
		Identity: storedIdentity{
			Hostname: nodeID + ".host",
			Ips:      []string{ip},
		},
		Profiles:           []string{"storage"},
		Status:             "active",
		BootstrapPhase:     BootstrapWorkloadReady,
		JoinLifecyclePhase: JoinPhaseAdmitted,
		ObjectStoreIntent:  &ObjectStoreIntent{Member: true},
	}
}

// makeStatusMember returns an ObjectStoreMember with the given generation.
func makeStatusMember(nodeID, ip string, gen uint64) ObjectStoreMember {
	return ObjectStoreMember{
		NodeID:           nodeID,
		Hostname:         nodeID + ".host",
		Address:          ip,
		AddedAt:          time.Now(),
		Source:           "test",
		IntentGeneration: gen,
	}
}

// ── Test 1: nil DesiredObjectStoreMembers reports legacy fallback ─────────────

func TestTopologyStatus_NilDesiredReportsLegacyFallback(t *testing.T) {
	state := &controllerState{
		Nodes:                    map[string]*nodeState{},
		DesiredObjectStoreMembers: nil, // legacy mode
		ObjectStoreGeneration:    0,
	}
	status := buildObjectStoreTopologyStatus(state)

	if status.Mode != ObjectStoreTopologyModeLegacy {
		t.Errorf("nil desired must produce legacy mode, got %q", status.Mode)
	}
	if !status.HasFinding(FindingObjectStoreLegacyFallback) {
		t.Error("legacy fallback finding must be present")
	}
	if !strings.Contains(status.Message, "legacy") {
		t.Errorf("message must mention legacy, got %q", status.Message)
	}
	if status.DesiredMembers != nil {
		t.Error("DesiredMembers must be nil in legacy mode")
	}
}

// ── Test 2: empty DesiredObjectStoreMembers reports explicit empty (not legacy) ─

func TestTopologyStatus_EmptyDesiredReportsExplicitEmptyNotLegacy(t *testing.T) {
	state := &controllerState{
		Nodes:                    map[string]*nodeState{},
		DesiredObjectStoreMembers: []ObjectStoreMember{}, // explicit empty v2 mode
		ObjectStoreGeneration:    1,
	}
	status := buildObjectStoreTopologyStatus(state)

	if status.Mode != ObjectStoreTopologyModeV2Empty {
		t.Errorf("empty non-nil desired must produce explicit_empty mode, got %q", status.Mode)
	}
	if status.HasFinding(FindingObjectStoreLegacyFallback) {
		t.Error("explicit empty must not produce legacy fallback finding")
	}
	if !status.HasFinding(FindingObjectStoreExplicitTopologyEmpty) {
		t.Error("explicit empty must produce explicit_topology_empty finding")
	}
	if status.DesiredMembers == nil {
		t.Error("DesiredMembers must be non-nil (empty slice) in v2 empty mode")
	}
	if len(status.DesiredMembers) != 0 {
		t.Errorf("DesiredMembers must be empty, got %d entries", len(status.DesiredMembers))
	}
}

// ── Test 3: blocked desired member appears with blocked reason ────────────────

func TestTopologyStatus_BlockedDesiredMemberAppearsWithReason(t *testing.T) {
	node := makeStatusNode("node-a", "10.0.0.1")
	node.Status = "blocked" // forces nodeIsObjectStoreMemberAdmitted=false

	state := &controllerState{
		Nodes:    map[string]*nodeState{"node-a": node},
		ObjectStoreGeneration: 2,
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeStatusMember("node-a", "10.0.0.1", 2),
		},
	}
	status := buildObjectStoreTopologyStatus(state)

	if status.Mode != ObjectStoreTopologyModeV2 {
		t.Errorf("expected explicit_v2 mode, got %q", status.Mode)
	}
	if len(status.MemberStatuses) != 1 {
		t.Fatalf("expected 1 member status, got %d", len(status.MemberStatuses))
	}
	ms := status.MemberStatuses[0]
	if ms.Admitted {
		t.Error("blocked node must not be admitted")
	}
	if !strings.Contains(ms.BlockedReason, "blocked") {
		t.Errorf("blocked reason must mention blocked, got %q", ms.BlockedReason)
	}
	if len(status.HeldNodes) != 1 {
		t.Errorf("blocked node must appear in HeldNodes, got %d", len(status.HeldNodes))
	}
	if !status.HasFinding(FindingObjectStoreListedMemberNotAdmitted) {
		t.Error("must have listed_member_not_admitted finding")
	}
}

// ── Test 4: intent member not in desired list is reported ─────────────────────

func TestTopologyStatus_IntentMemberNotListedIsReported(t *testing.T) {
	nodeA := makeStatusNode("node-a", "10.0.0.1")
	nodeB := makeStatusNode("node-b", "10.0.0.2") // intent=true but not in desired

	state := &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": nodeA,
			"node-b": nodeB,
		},
		ObjectStoreGeneration: 1,
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeStatusMember("node-a", "10.0.0.1", 1),
			// node-b is NOT listed
		},
	}
	status := buildObjectStoreTopologyStatus(state)

	found := false
	for _, id := range status.IntentMembersNotListed {
		if id == "node-b" {
			found = true
		}
	}
	if !found {
		t.Errorf("node-b must appear in IntentMembersNotListed; got %v", status.IntentMembersNotListed)
	}
	// node-a is listed so must NOT appear in IntentMembersNotListed
	for _, id := range status.IntentMembersNotListed {
		if id == "node-a" {
			t.Error("node-a is listed and must not appear in IntentMembersNotListed")
		}
	}
	if !status.HasFinding(FindingObjectStoreIntentMemberNotListed) {
		t.Error("must have intent_member_not_listed finding")
	}
}

// ── Test 5: generation mismatch is reported ───────────────────────────────────

func TestTopologyStatus_GenerationMismatchIsReported(t *testing.T) {
	node := makeStatusNode("node-a", "10.0.0.1")
	state := &controllerState{
		Nodes:                 map[string]*nodeState{"node-a": node},
		ObjectStoreGeneration: 5,
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeStatusMember("node-a", "10.0.0.1", 3), // gen 3 != state gen 5
		},
	}
	status := buildObjectStoreTopologyStatus(state)

	found := false
	for _, id := range status.GenerationMismatchedNodes {
		if id == "node-a" {
			found = true
		}
	}
	if !found {
		t.Errorf("node-a must appear in GenerationMismatchedNodes; got %v", status.GenerationMismatchedNodes)
	}
	if !status.HasFinding(FindingObjectStoreGenerationMismatch) {
		t.Error("must have generation_mismatch finding")
	}
	// The finding must identify the mismatch in its message.
	for _, f := range status.FindingsByID(FindingObjectStoreGenerationMismatch) {
		if !strings.Contains(f.Message, "node_gen=3") || !strings.Contains(f.Message, "desired_gen=5") {
			t.Errorf("generation mismatch finding must show node_gen and desired_gen, got: %q", f.Message)
		}
	}
}

// ── Test 6: pending transition is reported ────────────────────────────────────

func TestTopologyStatus_PendingTransitionIsReported(t *testing.T) {
	state := &controllerState{
		Nodes:                    map[string]*nodeState{},
		ObjectStoreGeneration:    1,
		DesiredObjectStoreMembers: []ObjectStoreMember{},
		PendingObjectStoreTransition: &ObjectStoreTopologyTransition{
			TransitionID:   "txn-abc123",
			Status:         ObjectStoreTransitionPending,
			FromGeneration: 1,
			ToGeneration:   2,
		},
	}
	status := buildObjectStoreTopologyStatus(state)

	if status.PendingTransition == nil {
		t.Fatal("PendingTransition must be non-nil when transition is in progress")
	}
	if status.PendingTransition.TransitionID != "txn-abc123" {
		t.Errorf("TransitionID mismatch: got %q", status.PendingTransition.TransitionID)
	}
	if !status.HasFinding(FindingObjectStorePendingTransition) {
		t.Error("must have pending_transition finding")
	}
	findings := status.FindingsByID(FindingObjectStorePendingTransition)
	if len(findings) == 0 || !strings.Contains(findings[0].Message, "txn-abc123") {
		t.Errorf("pending transition finding must reference the transition ID, got %+v", findings)
	}
}

// ── Test 7: blocked transition includes preflight reason ──────────────────────

func TestTopologyStatus_BlockedTransitionIncludesPreflightReason(t *testing.T) {
	state := &controllerState{
		Nodes:                    map[string]*nodeState{},
		ObjectStoreGeneration:    1,
		DesiredObjectStoreMembers: []ObjectStoreMember{},
		PendingObjectStoreTransition: &ObjectStoreTopologyTransition{
			TransitionID:   "txn-blocked",
			Status:         ObjectStoreTransitionBlocked,
			FromGeneration: 1,
			ToGeneration:   2,
			BlockedReason:  "preflight_failed: insufficient_admitted_nodes (have 0, need ≥3)",
		},
	}
	status := buildObjectStoreTopologyStatus(state)

	if !status.HasFinding(FindingObjectStoreBlockedTransition) {
		t.Error("must have blocked_transition finding")
	}
	findings := status.FindingsByID(FindingObjectStoreBlockedTransition)
	if len(findings) == 0 {
		t.Fatal("must have at least one blocked_transition finding")
	}
	if findings[0].Severity != "error" {
		t.Errorf("blocked transition finding must be severity=error, got %q", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Message, "preflight_failed") {
		t.Errorf("blocked transition finding must include reason, got: %q", findings[0].Message)
	}
}

// ── Test 8: MemberStatuses separate from DesiredMembers ──────────────────────

func TestTopologyStatus_MemberStatusesSeparateFromDesiredMembers(t *testing.T) {
	nodeA := makeStatusNode("node-a", "10.0.0.1")  // admitted
	nodeB := makeStatusNode("node-b", "10.0.0.2")
	nodeB.Status = "removed"                         // not admitted

	state := &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": nodeA,
			"node-b": nodeB,
		},
		ObjectStoreGeneration: 3,
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeStatusMember("node-a", "10.0.0.1", 3),
			makeStatusMember("node-b", "10.0.0.2", 3),
		},
	}
	status := buildObjectStoreTopologyStatus(state)

	// DesiredMembers is a copy of the controller's desired list — both nodes present.
	if len(status.DesiredMembers) != 2 {
		t.Errorf("DesiredMembers must have 2 entries (all desired), got %d", len(status.DesiredMembers))
	}
	// MemberStatuses shows per-node admission results.
	if len(status.MemberStatuses) != 2 {
		t.Errorf("MemberStatuses must have 2 entries, got %d", len(status.MemberStatuses))
	}

	admittedCount := 0
	for _, ms := range status.MemberStatuses {
		if ms.Admitted {
			admittedCount++
		}
	}
	if admittedCount != 1 {
		t.Errorf("exactly 1 node must be admitted, got %d", admittedCount)
	}

	// HeldNodes is the subset of MemberStatuses where Admitted=false.
	if len(status.HeldNodes) != 1 {
		t.Errorf("HeldNodes must have 1 entry, got %d", len(status.HeldNodes))
	}
	if status.HeldNodes[0].NodeID != "node-b" {
		t.Errorf("HeldNodes must contain node-b, got %q", status.HeldNodes[0].NodeID)
	}
}

// ── Test 9: status output is stable and serializable ─────────────────────────

func TestTopologyStatus_OutputIsStableAndSerializable(t *testing.T) {
	nodeA := makeStatusNode("node-a", "10.0.0.1")
	nodeB := makeStatusNode("node-b", "10.0.0.2")
	nodeB.ObjectStoreIntent = &ObjectStoreIntent{Member: true}

	state := &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": nodeA,
			"node-b": nodeB,
		},
		ObjectStoreGeneration: 4,
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeStatusMember("node-a", "10.0.0.1", 4),
			// node-b has intent but is not listed → IntentMembersNotListed
		},
	}

	// Call twice to verify deterministic output.
	s1 := buildObjectStoreTopologyStatus(state)
	s2 := buildObjectStoreTopologyStatus(state)

	b1, err1 := json.Marshal(s1)
	b2, err2 := json.Marshal(s2)
	if err1 != nil || err2 != nil {
		t.Fatalf("status must be serializable: %v %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Error("buildObjectStoreTopologyStatus must produce stable output on repeated calls")
	}

	// Verify all required fields are present in the JSON output.
	var raw map[string]interface{}
	if err := json.Unmarshal(b1, &raw); err != nil {
		t.Fatalf("output must be valid JSON: %v", err)
	}
	for _, key := range []string{"mode", "generation", "message"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("required field %q missing from serialized status", key)
		}
	}
}

// ── Test 10: no mutation occurs from the observability path ──────────────────

func TestTopologyStatus_ObservabilityPathDoesNotMutateState(t *testing.T) {
	nodeA := makeStatusNode("node-a", "10.0.0.1")

	origMembers := []ObjectStoreMember{
		makeStatusMember("node-a", "10.0.0.1", 7),
		makeStatusMember("node-b", "10.0.0.2", 7),
	}
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": nodeA,
		},
		ObjectStoreGeneration:    7,
		DesiredObjectStoreMembers: origMembers,
		PendingObjectStoreTransition: &ObjectStoreTopologyTransition{
			TransitionID:   "txn-readonly",
			Status:         ObjectStoreTransitionPending,
			FromGeneration: 7,
			ToGeneration:   8,
		},
	}

	// Call multiple times; none should mutate state.
	buildObjectStoreTopologyStatus(state)
	buildObjectStoreTopologyStatus(state)
	buildObjectStoreTopologyStatus(state)

	if len(state.DesiredObjectStoreMembers) != 2 {
		t.Errorf("DesiredObjectStoreMembers must not be mutated: got %d entries, want 2",
			len(state.DesiredObjectStoreMembers))
	}
	if state.DesiredObjectStoreMembers[0].NodeID != "node-a" {
		t.Errorf("first entry must still be node-a, got %q", state.DesiredObjectStoreMembers[0].NodeID)
	}
	if state.ObjectStoreGeneration != 7 {
		t.Errorf("ObjectStoreGeneration must not be mutated: got %d, want 7", state.ObjectStoreGeneration)
	}
	if state.PendingObjectStoreTransition == nil {
		t.Error("PendingObjectStoreTransition must not be cleared by observability call")
	}
	if state.PendingObjectStoreTransition.TransitionID != "txn-readonly" {
		t.Errorf("PendingObjectStoreTransition.TransitionID must not change: got %q",
			state.PendingObjectStoreTransition.TransitionID)
	}
	if len(state.Nodes) != 1 {
		t.Errorf("Nodes map must not be mutated: got %d entries, want 1", len(state.Nodes))
	}
}
