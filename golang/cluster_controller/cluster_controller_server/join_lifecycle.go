// @awareness namespace=globular.platform
// @awareness component=platform_controller.join_lifecycle
// @awareness file_role=join_fsm_eight_phases_eligible_for_cluster_decisions_gates_active_only
// @awareness implements=globular.platform:intent.controller.join_lifecycle_fsm_gates_cluster_decisions
// @awareness risk=critical
package main

// JoinLifecyclePhase is the typed lifecycle state of a join request or node.
//
// The phases form a directed flow from initial request through admission to
// active membership. Non-admitted phases are explicitly excluded from cluster
// decisions (RF, topology, workflow scheduling, dependency calculations).
//
//	JOIN_REQUESTED → JOIN_AUTHORIZED → BOOTSTRAPPING → NODE_AGENT_REGISTERED
//	→ ADMISSION_PENDING → ADMITTED → CONVERGING → ACTIVE
//
// Terminal states: REJECTED, QUARANTINED, REMOVED, STALE_GHOST.
// Error states: BLOCKED (recoverable), QUARANTINED (requires manual intervention).
type JoinLifecyclePhase string

const (
	JoinPhaseRequested           JoinLifecyclePhase = "join_requested"
	JoinPhaseAuthorized          JoinLifecyclePhase = "join_authorized"
	JoinPhaseBootstrapping       JoinLifecyclePhase = "bootstrapping"
	JoinPhaseNodeAgentRegistered JoinLifecyclePhase = "node_agent_registered"
	JoinPhaseAdmissionPending    JoinLifecyclePhase = "admission_pending"
	JoinPhaseAdmitted            JoinLifecyclePhase = "admitted"
	JoinPhaseConverging          JoinLifecyclePhase = "converging"
	JoinPhaseActive              JoinLifecyclePhase = "active"
	JoinPhaseBlocked             JoinLifecyclePhase = "blocked"
	JoinPhaseRejected            JoinLifecyclePhase = "rejected"
	JoinPhaseQuarantined         JoinLifecyclePhase = "quarantined"
	JoinPhaseRemoving            JoinLifecyclePhase = "removing"
	JoinPhaseRemoved             JoinLifecyclePhase = "removed"
	JoinPhaseStaleGhost          JoinLifecyclePhase = "stale_ghost"
)

// Terminal returns true when the phase is a dead-end that will not advance
// further. Terminal nodes must not be retried without explicit operator action.
func (p JoinLifecyclePhase) Terminal() bool {
	switch p {
	case JoinPhaseRejected, JoinPhaseRemoved, JoinPhaseQuarantined, JoinPhaseStaleGhost:
		return true
	}
	return false
}

// Admitted returns true when the node has been admitted into the cluster,
// regardless of whether it has finished converging.
func (p JoinLifecyclePhase) Admitted() bool {
	switch p {
	case JoinPhaseAdmitted, JoinPhaseConverging, JoinPhaseActive:
		return true
	}
	return false
}

// Active returns true when the node is in full active membership with runtime
// proof verified.
func (p JoinLifecyclePhase) Active() bool {
	return p == JoinPhaseActive
}

// EligibleForClusterDecisions returns true when a node in this phase may
// participate in RF calculations, topology membership, workflow scheduling,
// and dependency decisions. Only admitted or active nodes are eligible.
//
// This is the single gate for cluster-decision eligibility. Callers MUST
// check this before counting a node toward RF or topology quorum.
func (p JoinLifecyclePhase) EligibleForClusterDecisions() bool {
	switch p {
	case JoinPhaseAdmitted, JoinPhaseConverging, JoinPhaseActive:
		return true
	}
	return false
}

// normalizeJoinLifecyclePhase converts a raw string to a JoinLifecyclePhase,
// mapping legacy joinRequestRecord.Status values to their typed equivalents.
// Returns "" when the input is empty or unrecognized — callers use this as
// "no lifecycle data present" to preserve backward compatibility.
func normalizeJoinLifecyclePhase(raw string) JoinLifecyclePhase {
	switch JoinLifecyclePhase(raw) {
	case JoinPhaseRequested, JoinPhaseAuthorized, JoinPhaseBootstrapping,
		JoinPhaseNodeAgentRegistered, JoinPhaseAdmissionPending,
		JoinPhaseAdmitted, JoinPhaseConverging, JoinPhaseActive,
		JoinPhaseBlocked, JoinPhaseRejected, JoinPhaseQuarantined,
		JoinPhaseRemoving, JoinPhaseRemoved, JoinPhaseStaleGhost:
		return JoinLifecyclePhase(raw)
	}
	// Legacy Status string → typed lifecycle mapping.
	switch raw {
	case "pending":
		return JoinPhaseRequested
	case "approved":
		return JoinPhaseAuthorized
	case "blocked":
		return JoinPhaseBlocked
	case "rejected":
		return JoinPhaseRejected
	case "converging":
		return JoinPhaseConverging
	case "ready":
		return JoinPhaseActive
	}
	return ""
}

// effectiveLifecyclePhase returns the authoritative lifecycle phase for a
// joinRequestRecord, deriving it from LifecyclePhase when set, falling back
// to normalizing the legacy Status string.
func effectiveLifecyclePhase(jr *joinRequestRecord) JoinLifecyclePhase {
	if jr == nil {
		return ""
	}
	if jr.LifecyclePhase != "" {
		return jr.LifecyclePhase
	}
	return normalizeJoinLifecyclePhase(jr.Status)
}
