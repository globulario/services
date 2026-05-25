package main

import (
	"fmt"
	"sort"
)

// ObjectStoreTopologyMode describes the current topology authority mode.
type ObjectStoreTopologyMode string

const (
	// ObjectStoreTopologyModeLegacy means DesiredObjectStoreMembers is nil.
	// Membership is derived from MinioPoolNodes and profile checks — legacy v1 behavior.
	ObjectStoreTopologyModeLegacy ObjectStoreTopologyMode = "legacy_fallback"

	// ObjectStoreTopologyModeV2Empty means DesiredObjectStoreMembers is an
	// explicit empty slice. Explicit v2 mode is active but no nodes are desired.
	ObjectStoreTopologyModeV2Empty ObjectStoreTopologyMode = "explicit_empty"

	// ObjectStoreTopologyModeV2 means DesiredObjectStoreMembers is non-nil and
	// has at least one entry. Explicit v2 admission is active.
	ObjectStoreTopologyModeV2 ObjectStoreTopologyMode = "explicit_v2"
)

// Doctor finding IDs for objectstore topology.
const (
	FindingObjectStoreLegacyFallback         = "objectstore.desired_state_missing_legacy_fallback"
	FindingObjectStoreExplicitTopologyEmpty   = "objectstore.explicit_topology_empty"
	FindingObjectStoreGenerationMismatch      = "objectstore.generation_mismatch"
	FindingObjectStoreIntentMemberNotListed   = "objectstore.intent_member_not_listed"
	FindingObjectStoreListedMemberNotAdmitted = "objectstore.listed_member_not_admitted"
	FindingObjectStorePendingTransition       = "objectstore.pending_transition"
	FindingObjectStoreBlockedTransition       = "objectstore.blocked_transition"
	FindingObjectStoreNodeAgentHoldingMinio   = "objectstore.node_agent_holding_minio"
)

// ObjectStoreTopologyFinding is a doctor/invariant observation about the
// current objectstore topology state.
type ObjectStoreTopologyFinding struct {
	// ID is a machine-readable dot-namespaced identifier. Always non-empty.
	ID string `json:"id"`
	// Severity is "info", "warn", "error", or "critical".
	Severity string `json:"severity"`
	// Message is the operator-readable description.
	Message string `json:"message"`
	// NodeID is the relevant node, empty when not node-specific.
	NodeID string `json:"node_id,omitempty"`
}

// ObjectStoreMemberStatus is the per-node admission result in a topology status report.
type ObjectStoreMemberStatus struct {
	// NodeID is the controller-assigned node identifier.
	NodeID string `json:"node_id"`
	// Hostname is the node's registered hostname (informational).
	Hostname string `json:"hostname,omitempty"`
	// Address is the stable IP used in pool config.
	Address string `json:"address,omitempty"`
	// IntentMember is true when ObjectStoreIntent.Member=true for this node.
	// Necessary but not sufficient for topology membership.
	IntentMember bool `json:"intent_member"`
	// ListedInDesired is true when this node appears in DesiredObjectStoreMembers.
	ListedInDesired bool `json:"listed_in_desired"`
	// IntentGeneration is the generation stored in ObjectStoreMember.IntentGeneration.
	IntentGeneration uint64 `json:"intent_generation"`
	// GenerationMatch is true when IntentGeneration matches state.ObjectStoreGeneration.
	GenerationMatch bool `json:"generation_match"`
	// Admitted is true when the controller has confirmed all eligibility conditions.
	Admitted bool `json:"admitted"`
	// BlockedReason is why Admitted=false (empty when admitted).
	BlockedReason string `json:"blocked_reason,omitempty"`
	// HeldMessage is the exact message a node-agent would log when holding this node.
	HeldMessage string `json:"held_message,omitempty"`
}

// ObjectStoreTopologyStatus is the full read-only status of the objectstore
// topology authority. It is built by buildObjectStoreTopologyStatus from the
// current controllerState — no I/O, no mutations.
//
// Operators use this to answer:
//   - Is topology legacy or explicit v2?
//   - What generation is active?
//   - Which nodes are desired? Which are actually authorized?
//   - Which nodes are held, and why?
//   - Is a transition pending, approved, blocked, or applied?
//   - Are any nodes trying to join without being listed?
//   - Are any listed nodes blocked by lifecycle, intent, or generation?
type ObjectStoreTopologyStatus struct {
	// Mode describes the authority mode (legacy_fallback, explicit_empty, explicit_v2).
	Mode ObjectStoreTopologyMode `json:"mode"`
	// Generation is the current ObjectStoreGeneration.
	Generation int64 `json:"generation"`
	// Message is the human-readable summary.
	Message string `json:"message"`

	// DesiredMembers is a copy of DesiredObjectStoreMembers (nil when legacy mode).
	DesiredMembers []ObjectStoreMember `json:"desired_members,omitempty"`

	// MemberStatuses contains per-node admission results for all desired members.
	// Separate from DesiredMembers: desired is what the controller wants;
	// member_statuses is whether each desired node can be admitted right now.
	MemberStatuses []ObjectStoreMemberStatus `json:"member_statuses,omitempty"`

	// HeldNodes contains admission results for nodes that would be held by
	// the node-agent (Admitted=false), with the reason they are held.
	HeldNodes []ObjectStoreMemberStatus `json:"held_nodes,omitempty"`

	// IntentMembersNotListed lists node IDs that have ObjectStoreIntent.Member=true
	// but do not appear in DesiredObjectStoreMembers. Only populated in v2 mode.
	// These nodes believe they should be members but the controller disagrees.
	IntentMembersNotListed []string `json:"intent_members_not_listed,omitempty"`

	// GenerationMismatchedNodes lists node IDs in DesiredObjectStoreMembers whose
	// IntentGeneration does not match the current ObjectStoreGeneration.
	// The node-agent will refuse to render topology for these nodes.
	GenerationMismatchedNodes []string `json:"generation_mismatched_nodes,omitempty"`

	// PendingTransition is the current in-flight transition record, if any.
	PendingTransition *ObjectStoreTopologyTransition `json:"pending_transition,omitempty"`

	// Findings contains all doctor/invariant observations for this topology.
	Findings []ObjectStoreTopologyFinding `json:"findings,omitempty"`
}

// buildObjectStoreTopologyStatus computes the full read-only topology status
// from the current controllerState.
//
// Pure function: no etcd I/O, no state mutations. Safe to call from tests,
// status RPCs, and reconcile loop diagnostics.
func buildObjectStoreTopologyStatus(state *controllerState) *ObjectStoreTopologyStatus {
	if state == nil {
		return &ObjectStoreTopologyStatus{
			Mode:    ObjectStoreTopologyModeLegacy,
			Message: "objectstore topology: no controller state available",
			Findings: []ObjectStoreTopologyFinding{{
				ID:       FindingObjectStoreLegacyFallback,
				Severity: "warn",
				Message:  "objectstore topology: legacy fallback active; membership derived from node pool/profile",
			}},
		}
	}

	status := &ObjectStoreTopologyStatus{
		Generation:        state.ObjectStoreGeneration,
		PendingTransition: state.PendingObjectStoreTransition,
	}

	// ── 1. Determine topology authority mode ─────────────────────────────────

	switch {
	case state.DesiredObjectStoreMembers == nil:
		status.Mode = ObjectStoreTopologyModeLegacy
		status.Message = "objectstore topology: legacy fallback active; membership derived from node pool/profile"
		status.addFinding(ObjectStoreTopologyFinding{
			ID:       FindingObjectStoreLegacyFallback,
			Severity: "warn",
			Message:  status.Message,
		})

	case len(state.DesiredObjectStoreMembers) == 0:
		status.Mode = ObjectStoreTopologyModeV2Empty
		status.Message = fmt.Sprintf("objectstore topology: explicit v2 mode active; generation=%d (no desired members)", state.ObjectStoreGeneration)
		status.addFinding(ObjectStoreTopologyFinding{
			ID:       FindingObjectStoreExplicitTopologyEmpty,
			Severity: "warn",
			Message:  "objectstore topology: explicit v2 mode active but DesiredObjectStoreMembers is empty",
		})

	default:
		status.Mode = ObjectStoreTopologyModeV2
		status.Message = fmt.Sprintf("objectstore topology: explicit v2 mode active; generation=%d", state.ObjectStoreGeneration)
	}

	// ── 2. Copy desired members (nil is meaningful — preserve it) ────────────

	if state.DesiredObjectStoreMembers != nil {
		out := make([]ObjectStoreMember, len(state.DesiredObjectStoreMembers))
		copy(out, state.DesiredObjectStoreMembers)
		status.DesiredMembers = out
	}

	// ── 3. Per-member admission status ───────────────────────────────────────

	for _, m := range state.DesiredObjectStoreMembers {
		node := state.Nodes[m.NodeID]
		admitted := nodeIsObjectStoreMemberAdmitted(node)
		blockedReason := ""
		heldMessage := ""
		if !admitted {
			blockedReason = objectStoreMemberBlockedReason(node)
			heldMessage = fmt.Sprintf("node held: blocked: %s", blockedReason)
		}

		intentMember := node != nil && node.ObjectStoreIntent != nil && node.ObjectStoreIntent.Member
		genMatch := m.IntentGeneration == uint64(state.ObjectStoreGeneration)

		ms := ObjectStoreMemberStatus{
			NodeID:           m.NodeID,
			Hostname:         m.Hostname,
			Address:          m.Address,
			IntentMember:     intentMember,
			ListedInDesired:  true,
			IntentGeneration: m.IntentGeneration,
			GenerationMatch:  genMatch,
			Admitted:         admitted,
			BlockedReason:    blockedReason,
			HeldMessage:      heldMessage,
		}
		status.MemberStatuses = append(status.MemberStatuses, ms)

		if !admitted {
			status.HeldNodes = append(status.HeldNodes, ms)
			status.addFinding(ObjectStoreTopologyFinding{
				ID:       FindingObjectStoreListedMemberNotAdmitted,
				Severity: "warn",
				Message:  heldMessage,
				NodeID:   m.NodeID,
			})
		}

		if !genMatch {
			status.GenerationMismatchedNodes = append(status.GenerationMismatchedNodes, m.NodeID)
			status.addFinding(ObjectStoreTopologyFinding{
				ID:       FindingObjectStoreGenerationMismatch,
				Severity: "warn",
				Message: fmt.Sprintf("node held: objectstore generation mismatch; topology not applied (node_gen=%d desired_gen=%d)",
					m.IntentGeneration, state.ObjectStoreGeneration),
				NodeID: m.NodeID,
			})
		}
	}

	// ── 4. Intent members not listed (v2 mode only) ───────────────────────────

	if state.DesiredObjectStoreMembers != nil {
		desiredIDs := make(map[string]struct{}, len(state.DesiredObjectStoreMembers))
		for _, m := range state.DesiredObjectStoreMembers {
			desiredIDs[m.NodeID] = struct{}{}
		}
		// Iterate in a deterministic order so tests and operators get stable output.
		nodeIDs := make([]string, 0, len(state.Nodes))
		for id := range state.Nodes {
			nodeIDs = append(nodeIDs, id)
		}
		sort.Strings(nodeIDs)
		for _, nodeID := range nodeIDs {
			node := state.Nodes[nodeID]
			if node == nil || node.ObjectStoreIntent == nil || !node.ObjectStoreIntent.Member {
				continue
			}
			if _, listed := desiredIDs[nodeID]; listed {
				continue
			}
			status.IntentMembersNotListed = append(status.IntentMembersNotListed, nodeID)
			status.addFinding(ObjectStoreTopologyFinding{
				ID:       FindingObjectStoreIntentMemberNotListed,
				Severity: "warn",
				Message: fmt.Sprintf("node %s has ObjectStoreIntent.Member=true but is not listed in DesiredObjectStoreMembers",
					nodeID),
				NodeID: nodeID,
			})
		}
	}

	// ── 5. Pending transition findings ────────────────────────────────────────

	if pt := state.PendingObjectStoreTransition; pt != nil {
		switch pt.Status {
		case ObjectStoreTransitionBlocked:
			status.addFinding(ObjectStoreTopologyFinding{
				ID:       FindingObjectStoreBlockedTransition,
				Severity: "error",
				Message:  fmt.Sprintf("transition blocked: %s", pt.BlockedReason),
			})
		case ObjectStoreTransitionPending:
			status.addFinding(ObjectStoreTopologyFinding{
				ID:       FindingObjectStorePendingTransition,
				Severity: "info",
				Message:  fmt.Sprintf("transition pending approval: %s", pt.TransitionID),
			})
		}
	}

	return status
}

// addFinding appends a finding to the status, deduplicating by ID+NodeID.
func (s *ObjectStoreTopologyStatus) addFinding(f ObjectStoreTopologyFinding) {
	for _, existing := range s.Findings {
		if existing.ID == f.ID && existing.NodeID == f.NodeID {
			return // deduplicate
		}
	}
	s.Findings = append(s.Findings, f)
}

// FindingsByID returns all findings with the given ID.
func (s *ObjectStoreTopologyStatus) FindingsByID(id string) []ObjectStoreTopologyFinding {
	var out []ObjectStoreTopologyFinding
	for _, f := range s.Findings {
		if f.ID == id {
			out = append(out, f)
		}
	}
	return out
}

// HasFinding returns true when the status contains at least one finding with the given ID.
func (s *ObjectStoreTopologyStatus) HasFinding(id string) bool {
	return len(s.FindingsByID(id)) > 0
}
