package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Transition status constants for ObjectStoreTopologyTransition.Status.
const (
	ObjectStoreTransitionPending    = "pending"
	ObjectStoreTransitionApproved   = "approved"
	ObjectStoreTransitionApplying   = "applying"
	ObjectStoreTransitionApplied    = "applied"
	ObjectStoreTransitionBlocked    = "blocked"
	ObjectStoreTransitionRejected   = "rejected"
	ObjectStoreTransitionFailed     = "failed"
	ObjectStoreTransitionRolledBack = "rolled_back"
)

// ObjectStoreTopologyPreflightResult is the result of validating a proposed
// topology transition before it is approved or applied.
type ObjectStoreTopologyPreflightResult struct {
	// OK is true when all preflight checks passed.
	OK bool `json:"ok"`
	// Reason is the first failing check, empty when OK=true.
	Reason string `json:"reason,omitempty"`
	// Details holds per-check diagnostic key/value pairs.
	Details map[string]string `json:"details,omitempty"`
	// CheckedAt is when the preflight ran.
	CheckedAt time.Time `json:"checked_at"`
}

// ObjectStoreTopologyTransition is the generation-based record of a requested
// change to the MinIO erasure pool membership.
//
// Design rules (v2 Phase E.1):
//   - DesiredObjectStoreMembers must not be mutated directly when an explicit
//     desired state already exists. All mutations go through a transition record.
//   - from_generation must match the current ObjectStoreGeneration in state; the
//     controller rejects stale transitions that arrived after another was applied.
//   - Unapproved transitions must not update DesiredObjectStoreMembers.
//   - Only an approved transition may be applied. Applying atomically updates
//     DesiredObjectStoreMembers and increments ObjectStoreGeneration.
//   - Storage profile alone does not imply topology membership (Phase E invariant).
//   - ObjectStoreIntent.Member=true is necessary but not sufficient; the node must
//     also appear in the transition's requested_members.
type ObjectStoreTopologyTransition struct {
	// TransitionID is a controller-assigned unique identifier.
	TransitionID string `json:"transition_id"`
	// FromGeneration is the current ObjectStoreGeneration the transition was
	// calculated from. The controller rejects this transition if the state has
	// already advanced past this generation.
	FromGeneration uint64 `json:"from_generation"`
	// ToGeneration is the generation that will result after applying this
	// transition. Must equal FromGeneration+1.
	ToGeneration uint64 `json:"to_generation"`
	// RequestedMembers is the desired pool membership after applying the transition.
	RequestedMembers []ObjectStoreMember `json:"requested_members"`
	// PreviousMembers is a snapshot of DesiredObjectStoreMembers at the time the
	// transition was created. Used for audit and rollback analysis.
	PreviousMembers []ObjectStoreMember `json:"previous_members,omitempty"`
	// Approved is true when an operator has explicitly approved this transition.
	Approved bool `json:"approved"`
	// ApprovedBy records who approved the transition (username or service account).
	ApprovedBy string `json:"approved_by,omitempty"`
	// Reason is the human-readable motivation for this change.
	Reason string `json:"reason,omitempty"`
	// CreatedAt is when the transition was requested.
	CreatedAt time.Time `json:"created_at"`
	// ApprovedAt is when an operator approved the transition.
	ApprovedAt time.Time `json:"approved_at,omitempty"`
	// Status is the current lifecycle state of the transition.
	Status string `json:"status"`
	// BlockedReason is the preflight failure reason when Status==blocked.
	BlockedReason string `json:"blocked_reason,omitempty"`
	// Preflight is the result of the last preflight check run.
	Preflight ObjectStoreTopologyPreflightResult `json:"preflight"`
}

// newTransitionID returns a short random identifier for a transition record.
func newTransitionID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "ostt-" + hex.EncodeToString(b)
}

// requestObjectStoreTopologyTransition creates a new topology transition record,
// runs preflight against the current cluster nodes and generation, and returns
// the record with its initial status.
//
// Status is ObjectStoreTransitionPending when all preflight checks pass.
// Status is ObjectStoreTransitionBlocked when preflight fails.
//
// The caller is responsible for:
//   - Ensuring no other pending transition is in progress (or explicitly replacing it)
//   - Storing the result in state.PendingObjectStoreTransition
//   - Persisting state to etcd
func requestObjectStoreTopologyTransition(
	requested []ObjectStoreMember,
	previous []ObjectStoreMember,
	fromGen uint64,
	toGen uint64,
	reason string,
	nodes map[string]*nodeState,
	currentGeneration int64,
) *ObjectStoreTopologyTransition {
	t := &ObjectStoreTopologyTransition{
		TransitionID:     newTransitionID(),
		FromGeneration:   fromGen,
		ToGeneration:     toGen,
		RequestedMembers: requested,
		PreviousMembers:  previous,
		Reason:           reason,
		CreatedAt:        time.Now(),
		Status:           ObjectStoreTransitionPending,
	}
	t.Preflight = preflightObjectStoreTransition(t, nodes, currentGeneration)
	if !t.Preflight.OK {
		t.Status = ObjectStoreTransitionBlocked
		t.BlockedReason = t.Preflight.Reason
	}
	return t
}

// preflightObjectStoreTransition validates a transition against the current
// cluster state. Returns OK=true only when all checks pass.
//
// Checks performed:
//  1. from_generation matches currentGeneration (stale detection)
//  2. to_generation == from_generation + 1
//  3. no duplicate node IDs in requested members
//  4. all requested members have non-empty NodeID and Address
//  5. each requested node exists in cluster nodes
//  6. each requested node is in an eligible lifecycle phase
//  7. each requested node has ObjectStoreIntent.Member=true
//  8. no requested node has Status==removed/blocked or BootstrapPhase==failed
//  9. resulting pool has at least 1 node
func preflightObjectStoreTransition(
	t *ObjectStoreTopologyTransition,
	nodes map[string]*nodeState,
	currentGeneration int64,
) ObjectStoreTopologyPreflightResult {
	details := make(map[string]string)
	fail := func(reason string) ObjectStoreTopologyPreflightResult {
		return ObjectStoreTopologyPreflightResult{
			OK:        false,
			Reason:    reason,
			Details:   details,
			CheckedAt: time.Now(),
		}
	}

	// 1. from_generation must match current state generation.
	if uint64(currentGeneration) != t.FromGeneration {
		details["current_generation"] = fmt.Sprintf("%d", currentGeneration)
		details["from_generation"] = fmt.Sprintf("%d", t.FromGeneration)
		return fail("stale_transition:from_generation_mismatch")
	}

	// 2. to_generation must be exactly from_generation + 1.
	if t.ToGeneration != t.FromGeneration+1 {
		details["from_generation"] = fmt.Sprintf("%d", t.FromGeneration)
		details["to_generation"] = fmt.Sprintf("%d", t.ToGeneration)
		return fail("invalid_transition:to_generation_must_be_from_plus_one")
	}

	// 3. Duplicate node IDs in requested members.
	seen := make(map[string]struct{}, len(t.RequestedMembers))
	for _, m := range t.RequestedMembers {
		if _, ok := seen[m.NodeID]; ok {
			details["duplicate_node_id"] = m.NodeID
			return fail("invalid_transition:duplicate_node_id")
		}
		seen[m.NodeID] = struct{}{}
	}

	// 4, 5, 6, 7, 8: per-member checks.
	for _, m := range t.RequestedMembers {
		if m.NodeID == "" {
			return fail("invalid_transition:empty_node_id")
		}
		if m.Address == "" {
			details["node_missing_address"] = m.NodeID
			return fail("invalid_transition:missing_address")
		}
		node, ok := nodes[m.NodeID]
		if !ok || node == nil {
			details["unknown_node"] = m.NodeID
			return fail("invalid_transition:node_not_found")
		}
		// 6. Lifecycle phase: non-empty typed phase must be admitted or active.
		if lp := node.JoinLifecyclePhase; lp != "" && !lp.EligibleForClusterDecisions() {
			details["ineligible_node"] = m.NodeID
			details["ineligible_lifecycle"] = string(lp)
			return fail("invalid_transition:node_not_eligible")
		}
		// 8. Status exclusions.
		switch node.Status {
		case "removed", "blocked":
			details["excluded_node"] = m.NodeID
			details["excluded_status"] = node.Status
			return fail("invalid_transition:node_excluded_by_status")
		}
		if node.BootstrapPhase == BootstrapFailed {
			details["excluded_node"] = m.NodeID
			details["excluded_bootstrap"] = "bootstrap_failed"
			return fail("invalid_transition:node_bootstrap_failed")
		}
		// 7. ObjectStoreIntent.Member=true required. Storage profile alone is
		// not sufficient — the controller must have explicitly authorized the node.
		if node.ObjectStoreIntent == nil || !node.ObjectStoreIntent.Member {
			details["no_objectstore_intent"] = m.NodeID
			return fail("invalid_transition:node_missing_objectstore_intent")
		}
	}

	// 9. At least one member in the resulting pool.
	if len(t.RequestedMembers) == 0 {
		return fail("invalid_transition:empty_member_list")
	}

	return ObjectStoreTopologyPreflightResult{
		OK:        true,
		CheckedAt: time.Now(),
	}
}

// approveObjectStoreTransition marks a pending transition as approved by
// approvedBy. Returns an error when the transition is not in the pending state.
func approveObjectStoreTransition(t *ObjectStoreTopologyTransition, approvedBy string) error {
	if t == nil {
		return fmt.Errorf("nil transition")
	}
	if t.Status != ObjectStoreTransitionPending {
		return fmt.Errorf("cannot approve transition %s: status=%s (must be pending)", t.TransitionID, t.Status)
	}
	t.Approved = true
	t.ApprovedBy = approvedBy
	t.ApprovedAt = time.Now()
	t.Status = ObjectStoreTransitionApproved
	return nil
}

// applyObjectStoreTransition applies an approved transition to controller state,
// atomically updating DesiredObjectStoreMembers and advancing ObjectStoreGeneration.
//
// Returns an error when:
//   - the transition is not approved
//   - from_generation has drifted (another transition was applied in the meantime)
//
// On success the transition status advances to ObjectStoreTransitionApplied.
func applyObjectStoreTransition(t *ObjectStoreTopologyTransition, state *controllerState) error {
	if t == nil {
		return fmt.Errorf("nil transition")
	}
	if t.Status != ObjectStoreTransitionApproved {
		return fmt.Errorf("transition %s is not approved (status=%s)", t.TransitionID, t.Status)
	}
	if uint64(state.ObjectStoreGeneration) != t.FromGeneration {
		return fmt.Errorf("stale transition %s: from_generation=%d does not match current=%d",
			t.TransitionID, t.FromGeneration, state.ObjectStoreGeneration)
	}
	state.DesiredObjectStoreMembers = t.RequestedMembers
	state.ObjectStoreGeneration = int64(t.ToGeneration)
	t.Status = ObjectStoreTransitionApplied
	return nil
}
