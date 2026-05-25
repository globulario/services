package main

import "time"

// ObjectStoreMember is the controller-authorized record of a node's desired
// membership in the MinIO erasure pool.
//
// Design rule: "storage" profile is a capability label — it does NOT imply
// MinIO pool membership. The controller must explicitly add a node to
// DesiredObjectStoreMembers (via apply-topology or Day-0 bootstrap) before
// reconcileMinioJoinPhases will advance that node beyond MinioJoinNonMember.
//
// Legacy nodes (clusters with nil DesiredObjectStoreMembers) continue to use
// profile-derived membership for backward compatibility. The status function
// returns "legacy_profile_derived" in that case so callers can log accordingly.
//
// INVARIANT: ObjectStoreIntent.Member=true is a necessary precondition but
// not sufficient — the node must also appear in DesiredObjectStoreMembers when
// that list is present. An intent alone without a desired-state entry is rejected.
type ObjectStoreMember struct {
	// NodeID is the controller-assigned node identifier.
	NodeID string `json:"node_id"`
	// Hostname is the node's registered hostname (informational).
	Hostname string `json:"hostname,omitempty"`
	// Address is the stable IP address used in MinIO pool config.
	// Must not be the cluster VIP — use the node's primary interface IP.
	Address string `json:"address"`
	// AddedAt is when the controller added this node to the desired member list.
	AddedAt time.Time `json:"added_at"`
	// Source describes how this entry was created.
	//   "day0_bootstrap"  — added during Day-0 bootstrap (auto)
	//   "apply_topology"  — added by an explicit apply-topology operator call
	//   "migration"       — migrated from legacy MinioPoolNodes IP list
	Source string `json:"source,omitempty"`
	// IntentGeneration is the ObjectStoreGeneration at which this node was admitted.
	IntentGeneration uint64 `json:"intent_generation"`
}

// objectStoreMembershipStatus returns the membership status for n under the
// Phase E-lite contract. It never returns ""; the empty-desired-list and
// legacy cases are distinguished.
//
// Return values:
//
//	"explicit_desired_state"  — node appears in desiredMembers (v2 mode); eligible
//	"legacy_profile_derived"  — desiredMembers is nil; profile check governs (v1 mode)
//	"not_listed"              — desiredMembers present, node not found; ineligible
//	"intent_not_member"       — ObjectStoreIntent.Member=false; explicit exclusion
func objectStoreMembershipStatus(node *nodeState, desiredMembers []ObjectStoreMember) string {
	if node == nil {
		return "not_listed"
	}
	// Explicit controller exclusion: intent beats everything.
	if node.ObjectStoreIntent != nil && !node.ObjectStoreIntent.Member {
		return "intent_not_member"
	}
	// nil desired list → legacy mode. Profile check in caller governs.
	if desiredMembers == nil {
		return "legacy_profile_derived"
	}
	for _, m := range desiredMembers {
		if m.NodeID == node.NodeID {
			return "explicit_desired_state"
		}
	}
	return "not_listed"
}

// nodeIsExplicitObjectStoreMember returns true when the node is authorized to
// participate in MinIO pool reconciliation.
//
// v2 mode (desiredMembers non-nil): node must appear by NodeID.
// legacy mode (desiredMembers nil): returns true unconditionally — the caller's
// profile guard (profilesForMinio) is the effective gate in legacy mode.
func nodeIsExplicitObjectStoreMember(node *nodeState, desiredMembers []ObjectStoreMember) bool {
	status := objectStoreMembershipStatus(node, desiredMembers)
	return status == "explicit_desired_state" || status == "legacy_profile_derived"
}

// nodeIsObjectStoreMemberAdmitted returns true when the node is in a lifecycle
// state that allows it to render active MinIO topology config.
//
// Used by buildObjectStoreDesiredStateLocked to set Admitted in ObjectStoreMemberSlim
// records published to etcd for node-agent enforcement (Phase E.2).
func nodeIsObjectStoreMemberAdmitted(node *nodeState) bool {
	if node == nil {
		return false
	}
	if node.ObjectStoreIntent == nil || !node.ObjectStoreIntent.Member {
		return false
	}
	switch node.Status {
	case "removed", "blocked":
		return false
	}
	if node.BootstrapPhase == BootstrapFailed {
		return false
	}
	if lp := node.JoinLifecyclePhase; lp != "" && !lp.EligibleForClusterDecisions() {
		return false
	}
	return true
}

// objectStoreMemberBlockedReason returns a human-readable reason why a node is
// not admitted, for embedding in ObjectStoreMemberSlim.BlockedReason.
func objectStoreMemberBlockedReason(node *nodeState) string {
	if node == nil {
		return "node_not_found"
	}
	if node.ObjectStoreIntent == nil || !node.ObjectStoreIntent.Member {
		return "objectstore_intent:not_member"
	}
	switch node.Status {
	case "removed":
		return "status:removed"
	case "blocked":
		return "status:blocked"
	}
	if node.BootstrapPhase == BootstrapFailed {
		return "bootstrap:failed"
	}
	if lp := node.JoinLifecyclePhase; lp != "" && !lp.EligibleForClusterDecisions() {
		return "lifecycle:" + string(lp)
	}
	return "not_admitted"
}

// objectStoreDesiredMembersFromIntents builds the initial DesiredObjectStoreMembers
// list from all nodes that have ObjectStoreIntent.Member=true and a known routable
// IP. Used as the migration path when upgrading a cluster to Phase E-lite: the
// controller populates DesiredObjectStoreMembers from existing node intents so that
// the v2 gate does not lock out already-admitted nodes on the first boot after upgrade.
func objectStoreDesiredMembersFromIntents(nodes map[string]*nodeState, generation uint64) []ObjectStoreMember {
	var result []ObjectStoreMember
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.ObjectStoreIntent == nil || !n.ObjectStoreIntent.Member {
			continue
		}
		ip := nodeRoutableIP(n)
		if ip == "" {
			continue
		}
		result = append(result, ObjectStoreMember{
			NodeID:           n.NodeID,
			Hostname:         n.Identity.Hostname,
			Address:          ip,
			AddedAt:          time.Now(),
			Source:           "migration",
			IntentGeneration: generation,
		})
	}
	return result
}
