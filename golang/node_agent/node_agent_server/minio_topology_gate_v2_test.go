package main

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/config"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeSlimMember builds an ObjectStoreMemberSlim for a fully admitted node.
func makeSlimMember(nodeID string, gen uint64) config.ObjectStoreMemberSlim {
	return config.ObjectStoreMemberSlim{
		NodeID:           nodeID,
		IntentGeneration: gen,
		Admitted:         true,
	}
}

// makeSlimMemberBlocked builds an ObjectStoreMemberSlim for a tracked but
// not-yet-admitted node.
func makeSlimMemberBlocked(nodeID string, gen uint64, reason string) config.ObjectStoreMemberSlim {
	return config.ObjectStoreMemberSlim{
		NodeID:           nodeID,
		IntentGeneration: gen,
		Admitted:         false,
		BlockedReason:    reason,
	}
}

// v2StateWith returns an ObjectStoreDesiredState with AuthorizedMembers set
// (v2 mode) at the given generation.
func v2StateWith(gen int64, members ...config.ObjectStoreMemberSlim) *config.ObjectStoreDesiredState {
	return &config.ObjectStoreDesiredState{
		Mode:              config.ObjectStoreModeDistributed,
		Generation:        gen,
		Nodes:             []string{},
		AuthorizedMembers: members,
	}
}

// legacyState returns an ObjectStoreDesiredState with nil AuthorizedMembers
// (legacy mode) containing the given IPs.
func legacyState(gen int64, ips ...string) *config.ObjectStoreDesiredState {
	return &config.ObjectStoreDesiredState{
		Mode:              config.ObjectStoreModeDistributed,
		Generation:        gen,
		Nodes:             ips,
		AuthorizedMembers: nil, // nil = legacy mode
	}
}

// ── Test 1: matching intent + desired generation allows MinIO membership ──────

func TestTopologyMemberV2_MatchingIntentAndGenerationAllowed(t *testing.T) {
	state := v2StateWith(3, makeSlimMember("node-a", 3))
	allowed, reason := nodeIsTopologyMember("node-a", "10.0.0.1", state)
	if !allowed {
		t.Errorf("matching admitted member at correct generation must be allowed; reason=%q", reason)
	}
	if reason != "" {
		t.Errorf("allowed result must have empty reason, got %q", reason)
	}
}

// ── Test 2: intent member false blocks MinIO membership ───────────────────────

func TestTopologyMemberV2_IntentMemberFalseBlocks(t *testing.T) {
	// Controller marks node as not_admitted because ObjectStoreIntent.Member=false.
	state := v2StateWith(3, makeSlimMemberBlocked("node-b", 3, "objectstore_intent:not_member"))
	allowed, reason := nodeIsTopologyMember("node-b", "10.0.0.2", state)
	if allowed {
		t.Error("node with intent member=false must be blocked")
	}
	if !strings.Contains(reason, "blocked") {
		t.Errorf("reason must contain 'blocked', got %q", reason)
	}
	if !strings.Contains(reason, "objectstore_intent:not_member") {
		t.Errorf("reason must identify intent cause, got %q", reason)
	}
}

// ── Test 3: node missing from desired members blocks MinIO membership ─────────

func TestTopologyMemberV2_NodeMissingFromDesiredBlocks(t *testing.T) {
	// AuthorizedMembers is set (v2 mode) but does not include node-c.
	state := v2StateWith(2, makeSlimMember("node-a", 2))
	allowed, reason := nodeIsTopologyMember("node-c", "10.0.0.3", state)
	if allowed {
		t.Error("node not listed in v2 AuthorizedMembers must be blocked")
	}
	if reason != "node not listed in approved objectstore topology" {
		t.Errorf("unexpected reason: %q", reason)
	}
}

// ── Test 4: generation mismatch blocks MinIO membership ───────────────────────

func TestTopologyMemberV2_GenerationMismatchBlocks(t *testing.T) {
	// Node has intent_generation=1 but state.Generation is 2 → stale.
	state := v2StateWith(2, makeSlimMember("node-a", 1)) // gen mismatch: 1 != 2
	allowed, reason := nodeIsTopologyMember("node-a", "10.0.0.1", state)
	if allowed {
		t.Error("generation mismatch must block MinIO membership")
	}
	if !strings.Contains(reason, "objectstore generation mismatch") {
		t.Errorf("reason must describe generation mismatch, got %q", reason)
	}
	if !strings.Contains(reason, "topology not applied") {
		t.Errorf("reason must mention 'topology not applied', got %q", reason)
	}
}

// ── Test 5: blocked/removing/removed lifecycle blocks MinIO membership ────────

func TestTopologyMemberV2_BlockedLifecycleBlocks(t *testing.T) {
	cases := []struct {
		name          string
		blockedReason string
	}{
		{"blocked status", "status:blocked"},
		{"removed status", "status:removed"},
		{"bootstrap failed", "bootstrap:failed"},
		{"bootstrapping lifecycle", "lifecycle:bootstrapping"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := v2StateWith(5, makeSlimMemberBlocked("node-x", 5, tc.blockedReason))
			allowed, reason := nodeIsTopologyMember("node-x", "10.0.0.5", state)
			if allowed {
				t.Errorf("%s: must be blocked, but was allowed", tc.name)
			}
			if !strings.Contains(reason, "blocked") {
				t.Errorf("%s: reason must contain 'blocked', got %q", tc.name, reason)
			}
		})
	}
}

// ── Test 6: nil desired members preserves legacy fallback ─────────────────────

func TestTopologyMemberV2_NilAuthorizedMembersIsLegacy(t *testing.T) {
	// Legacy state: no AuthorizedMembers, only IP list.
	state := legacyState(1, "10.0.0.63", "10.0.0.8")

	// Node with matching IP must be allowed in legacy mode.
	allowed, reason := nodeIsTopologyMember("any-node-id", "10.0.0.8", state)
	if !allowed {
		t.Errorf("nil AuthorizedMembers must use legacy IP check; reason=%q", reason)
	}

	// Non-matching IP must be blocked even in legacy mode.
	allowed2, _ := nodeIsTopologyMember("any-node-id", "10.0.0.99", state)
	if allowed2 {
		t.Error("non-matching IP in legacy mode must be blocked")
	}
}

// ── Test 7: empty explicit desired members disables fallback ──────────────────

func TestTopologyMemberV2_EmptyAuthorizedMembersDisablesFallback(t *testing.T) {
	// Non-nil but empty AuthorizedMembers: v2 mode with no admitted nodes.
	// Even if the IP is in Nodes, the node-agent must not use IP fallback.
	state := &config.ObjectStoreDesiredState{
		Generation:        2,
		Nodes:             []string{"10.0.0.1"}, // IP is here but AuthorizedMembers overrides
		AuthorizedMembers: []config.ObjectStoreMemberSlim{}, // explicitly empty, not nil
	}
	allowed, reason := nodeIsTopologyMember("node-a", "10.0.0.1", state)
	if allowed {
		t.Error("empty non-nil AuthorizedMembers must not fall back to IP check")
	}
	if reason != "node not listed in approved objectstore topology" {
		t.Errorf("unexpected reason: %q", reason)
	}
}

// ── Test 8: node-agent reports clear blocked reason ───────────────────────────

func TestTopologyMemberV2_BlockedReasonIsClear(t *testing.T) {
	// Verify the exact reason strings required by the spec.
	t.Run("not listed", func(t *testing.T) {
		state := v2StateWith(1, makeSlimMember("other-node", 1))
		_, reason := nodeIsTopologyMember("my-node", "10.0.0.1", state)
		if reason != "node not listed in approved objectstore topology" {
			t.Errorf("unexpected reason: %q", reason)
		}
	})
	t.Run("generation mismatch", func(t *testing.T) {
		state := v2StateWith(5, makeSlimMember("my-node", 3)) // gen 3 != state gen 5
		_, reason := nodeIsTopologyMember("my-node", "10.0.0.1", state)
		if !strings.HasPrefix(reason, "objectstore generation mismatch") {
			t.Errorf("unexpected reason: %q", reason)
		}
	})
}

// ── Test 9: package install/preparation can still happen without topology ─────

func TestTopologyMemberV2_PackageInstallNotGatedByTopology(t *testing.T) {
	// The topology gate (nodeIsTopologyMember) governs service START only.
	// MinIO package installation does not call nodeIsTopologyMember.
	// This test verifies the gate is a pure function with no side effects
	// on package state — it returns (false, reason) and does nothing else.
	state := v2StateWith(2, makeSlimMember("other-node", 2))
	allowed, reason := nodeIsTopologyMember("my-node", "10.0.0.9", state)

	// Gate correctly blocks service start.
	if allowed {
		t.Fatalf("gate must block non-listed node, but got allowed=true reason=%q", reason)
	}
	// No side effects: we can call the gate multiple times without state mutation.
	allowed2, reason2 := nodeIsTopologyMember("my-node", "10.0.0.9", state)
	if allowed != allowed2 || reason != reason2 {
		t.Error("nodeIsTopologyMember must be pure and idempotent")
	}
	// The gate does not clear, modify, or delete DesiredObjectStoreMembers.
	if len(state.AuthorizedMembers) != 1 {
		t.Error("gate must not mutate AuthorizedMembers")
	}
}

// ── Test 10: no direct DesiredObjectStoreMembers mutation from gate ───────────

func TestTopologyMemberV2_GateDoesNotMutateState(t *testing.T) {
	original := []config.ObjectStoreMemberSlim{
		makeSlimMember("node-a", 7),
		makeSlimMember("node-b", 7),
	}
	state := &config.ObjectStoreDesiredState{
		Generation:        7,
		Nodes:             []string{"10.0.0.1", "10.0.0.2"},
		AuthorizedMembers: original,
	}

	// Call the gate for an unknown node — must not mutate state.
	nodeIsTopologyMember("node-c", "10.0.0.3", state)
	nodeIsTopologyMember("node-a", "10.0.0.1", state)
	nodeIsTopologyMember("", "", state)

	// Verify no mutation.
	if len(state.AuthorizedMembers) != 2 {
		t.Errorf("AuthorizedMembers mutated: got %d entries, want 2", len(state.AuthorizedMembers))
	}
	if state.AuthorizedMembers[0].NodeID != "node-a" || state.AuthorizedMembers[1].NodeID != "node-b" {
		t.Errorf("AuthorizedMembers entries changed: %+v", state.AuthorizedMembers)
	}
	if state.Generation != 7 {
		t.Error("Generation must not be mutated by the gate")
	}
}
