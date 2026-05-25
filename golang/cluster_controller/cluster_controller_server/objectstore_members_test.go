package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeObjectStoreMember(nodeID, hostname, address string) ObjectStoreMember {
	return ObjectStoreMember{
		NodeID:           nodeID,
		Hostname:         hostname,
		Address:          address,
		AddedAt:          time.Now(),
		Source:           "test",
		IntentGeneration: 1,
	}
}

func makeMinioNode(nodeID string) *nodeState {
	n := makeStorageNode(nodeID)
	n.NodeID = nodeID
	return n
}

// addMinioUnit appends a globular-minio.service unit record with the given state.
func addMinioUnit(node *nodeState, state string) {
	node.Units = append(node.Units, unitStatusRecord{
		Name:  "globular-minio.service",
		State: state,
	})
}

// ── Test 1: profile alone does not grant membership when desired list is set ──

func TestObjectStoreMember_ProfileAloneNotSufficientWhenDesiredSet(t *testing.T) {
	node := makeMinioNode("profile-only-node")
	// Desired list is non-nil but does not contain this node.
	desired := []ObjectStoreMember{
		makeObjectStoreMember("other-node", "other", "10.0.0.5"),
	}
	status := objectStoreMembershipStatus(node, desired)
	if status != "not_listed" {
		t.Errorf("expected not_listed, got %q", status)
	}
	if nodeIsExplicitObjectStoreMember(node, desired) {
		t.Error("profile-only node must not be eligible when not in desired list")
	}
}

// ── Test 2: node in DesiredObjectStoreMembers is eligible ─────────────────────

func TestObjectStoreMember_ExplicitlyListedNodeIsEligible(t *testing.T) {
	node := makeMinioNode("listed-node")
	desired := []ObjectStoreMember{
		makeObjectStoreMember("listed-node", "listed", "10.0.0.6"),
	}
	status := objectStoreMembershipStatus(node, desired)
	if status != "explicit_desired_state" {
		t.Errorf("expected explicit_desired_state, got %q", status)
	}
	if !nodeIsExplicitObjectStoreMember(node, desired) {
		t.Error("node in desired list must be eligible")
	}
}

// ── Test 3: nil desired list triggers legacy profile-derived mode ─────────────

func TestObjectStoreMember_NilDesiredListIsLegacyMode(t *testing.T) {
	node := makeMinioNode("legacy-node")
	status := objectStoreMembershipStatus(node, nil)
	if status != "legacy_profile_derived" {
		t.Errorf("expected legacy_profile_derived for nil desired list, got %q", status)
	}
	if !nodeIsExplicitObjectStoreMember(node, nil) {
		t.Error("nil desired list must return true (caller's profile guard applies)")
	}
}

// ── Test 4: ObjectStoreIntent.Member=false blocks even with matching profile ──

func TestObjectStoreMember_IntentMemberFalseBlocksEvenWithProfile(t *testing.T) {
	node := makeMinioNode("excluded-node")
	node.ObjectStoreIntent = &ObjectStoreIntent{
		Member: false,
		Reason: "explicitly excluded by controller",
	}
	// Even if in desired list, intent=false must block.
	desired := []ObjectStoreMember{
		makeObjectStoreMember("excluded-node", "excluded", "10.0.0.7"),
	}
	status := objectStoreMembershipStatus(node, desired)
	if status != "intent_not_member" {
		t.Errorf("expected intent_not_member, got %q", status)
	}
	if nodeIsExplicitObjectStoreMember(node, desired) {
		t.Error("ObjectStoreIntent.Member=false must block membership")
	}
}

// ── Test 5: nil node returns not_listed ───────────────────────────────────────

func TestObjectStoreMember_NilNodeReturnsNotListed(t *testing.T) {
	status := objectStoreMembershipStatus(nil, []ObjectStoreMember{})
	if status != "not_listed" {
		t.Errorf("nil node must return not_listed, got %q", status)
	}
	if nodeIsExplicitObjectStoreMember(nil, []ObjectStoreMember{}) {
		t.Error("nil node must not be eligible")
	}
}

// ── Test 6: non-nil empty desired list → not_listed (not legacy) ─────────────

func TestObjectStoreMember_EmptyNonNilDesiredIsNotListed(t *testing.T) {
	node := makeMinioNode("any-node")
	// Non-nil but empty desired list — different from nil.
	desired := []ObjectStoreMember{}
	status := objectStoreMembershipStatus(node, desired)
	if status != "not_listed" {
		t.Errorf("empty non-nil desired must return not_listed, got %q", status)
	}
}

// ── Test 7: reconcileMinioJoinPhases skips node not in desired list ───────────

func TestObjectStoreMember_ReconcileSkipsNodeNotInDesiredList(t *testing.T) {
	node := makeMinioNode("unlisted-node")
	node.MinioJoinPhase = MinioJoinNone
	addMinioUnit(node, "active")

	state := &controllerState{
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeObjectStoreMember("other-node", "other", "10.0.0.8"),
		},
		MinioPoolNodes: []string{},
	}

	m := newMinioPoolManager()
	dirty := m.reconcileMinioJoinPhases([]*nodeState{node}, state)
	if dirty {
		t.Error("reconcile must not touch a node not in desired list")
	}
	if node.MinioJoinPhase != MinioJoinNone {
		t.Errorf("node phase must remain None, got %v", node.MinioJoinPhase)
	}
}

// ── Test 8: reconcileMinioJoinPhases admits node in desired list ──────────────

func TestObjectStoreMember_ReconcileAdmitsNodeInDesiredList(t *testing.T) {
	node := makeMinioNode("listed-pool-node")
	node.MinioJoinPhase = MinioJoinNone
	node.Identity.Ips = []string{"10.0.1.1"}
	node.BootstrapPhase = BootstrapWorkloadReady
	addMinioUnit(node, "active")

	state := &controllerState{
		DesiredObjectStoreMembers: []ObjectStoreMember{
			makeObjectStoreMember("listed-pool-node", "listed", "10.0.1.1"),
		},
		MinioPoolNodes: []string{},
	}

	m := newMinioPoolManager()
	dirty := m.reconcileMinioJoinPhases([]*nodeState{node}, state)
	if !dirty {
		t.Error("reconcile must advance the node in desired list (Day-0 bootstrap)")
	}
	if node.MinioJoinPhase == MinioJoinNone {
		t.Errorf("node phase must advance from None, got %v", node.MinioJoinPhase)
	}
}

// ── Test 9: objectStoreDesiredMembersFromIntents builds from node intents ─────

func TestObjectStoreMember_FromIntentsBuildsCorrectly(t *testing.T) {
	nodes := map[string]*nodeState{
		"node-a": {
			NodeID: "node-a",
			Identity: storedIdentity{
				Hostname: "alpha",
				Ips:      []string{"10.1.0.1"},
			},
			Profiles:          []string{"storage"},
			ObjectStoreIntent: &ObjectStoreIntent{Member: true},
		},
		"node-b": {
			NodeID:            "node-b",
			ObjectStoreIntent: &ObjectStoreIntent{Member: false}, // excluded
		},
		"node-c": {
			NodeID:            "node-c",
			ObjectStoreIntent: nil, // no intent → not migrated
		},
	}

	result := objectStoreDesiredMembersFromIntents(nodes, 5)

	if len(result) != 1 {
		t.Fatalf("expected 1 member (node-a only), got %d", len(result))
	}
	if result[0].NodeID != "node-a" {
		t.Errorf("expected node-a, got %q", result[0].NodeID)
	}
	if result[0].IntentGeneration != 5 {
		t.Errorf("expected generation=5, got %d", result[0].IntentGeneration)
	}
	if result[0].Source != "migration" {
		t.Errorf("expected source=migration, got %q", result[0].Source)
	}
}

// ── Test 10: serialization round-trip preserves ObjectStoreMember fields ──────

func TestObjectStoreMember_SerializationRoundTrip(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Second)
	m := ObjectStoreMember{
		NodeID:           "rt-node",
		Hostname:         "rt-host",
		Address:          "10.2.0.1",
		AddedAt:          ts,
		Source:           "apply_topology",
		IntentGeneration: 42,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ObjectStoreMember
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.NodeID != "rt-node" || got.Hostname != "rt-host" || got.Address != "10.2.0.1" {
		t.Errorf("identity fields not preserved: %+v", got)
	}
	if got.Source != "apply_topology" || got.IntentGeneration != 42 {
		t.Errorf("metadata fields not preserved: %+v", got)
	}
	if !got.AddedAt.Equal(ts) {
		t.Errorf("AddedAt not preserved: got %v, want %v", got.AddedAt, ts)
	}
}
