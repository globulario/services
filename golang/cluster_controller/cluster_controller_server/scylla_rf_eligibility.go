package main

import "time"

// IsNodeVerifiedStorageEligible returns true when n should count toward the
// verified storage/control-plane node tally used by RF policy and the schema
// guard. It is the single authoritative gate for that count.
//
// Design rule: fail closed. Any signal that explicitly indicates the node is
// not ready, is offline, or has failed disqualifies it. Signals not yet
// available in nodeState are called out as TODOs; they do not silently pass.
func IsNodeVerifiedStorageEligible(n *nodeState) bool {
	return nodeStorageEligibilityReason(n) == ""
}

// nodeStorageEligibilityReason returns the first reason n is ineligible, or ""
// when the node is eligible. Used by tests to assert on specific exclusion
// reasons without duplicating predicate logic.
func nodeStorageEligibilityReason(n *nodeState) string {
	if n == nil {
		return "nil node"
	}

	// JoinLifecyclePhase gate (v2): when the node carries a typed lifecycle
	// phase, it must be admitted or active. Nodes in join_requested,
	// join_authorized, bootstrapping, node_agent_registered, or
	// admission_pending are not yet cluster members and must not count toward
	// RF, topology quorum, or workflow scheduling decisions.
	//
	// Legacy nodes with an empty JoinLifecyclePhase keep existing behavior —
	// backward compat: upgrading the controller must not instantly make all
	// existing nodes ineligible.
	if lp := n.JoinLifecyclePhase; lp != "" && !lp.EligibleForClusterDecisions() {
		return "lifecycle:" + string(lp)
	}

	// Hard status exclusions.
	switch n.Status {
	case "removed":
		return "status=removed"
	case "blocked":
		return "status=blocked"
	case "unreachable":
		return "status=unreachable"
	case "draining", "removing":
		// Pre-v2 lifecycle states; exclude preemptively if ever set.
		return "status=" + n.Status
	}

	// Bootstrap phase exclusions.
	//
	// BootstrapNone ("") is the legacy pre-phase value. State migration in
	// loadControllerState upgrades persisted nodes to BootstrapWorkloadReady,
	// but in-memory nodes constructed before migration may still carry "".
	// Treat it as ready (backward compat) — Phase B will close this gap.
	//
	// BootstrapFailed is always excluded. All other named phases that are not
	// one of the three "ready" values indicate the node is still joining.
	switch n.BootstrapPhase {
	case BootstrapFailed:
		return "bootstrap_failed"
	case BootstrapNone, BootstrapWorkloadReady, BootstrapStorageJoining:
		// eligible — fall through
	default:
		return "bootstrapping:" + string(n.BootstrapPhase)
	}

	// Scylla join phase exclusions.
	//
	// If the Scylla join state machine has started (non-zero ScyllaJoinStartedAt),
	// we have observable evidence and can apply strict rules:
	//   - Failed       → ineligible
	//   - in-progress  → ineligible (still joining; RF should not advance)
	//   - Verified     → eligible
	//   - None         → Scylla stopped after a prior run; eligible here because
	//                    the node was healthy before and may be temporarily down.
	//                    Phase D (Group 0 awareness) will harden this case.
	//
	// If ScyllaJoinStartedAt is zero the state machine has never tracked this
	// node (legacy cluster or non-Scylla node). Treat as eligible with a TODO.
	if n.ScyllaJoinPhase == ScyllaJoinFailed {
		return "scylla_join_failed"
	}
	if !n.ScyllaJoinStartedAt.IsZero() && n.ScyllaJoinStartedAt != (time.Time{}) {
		switch n.ScyllaJoinPhase {
		case ScyllaJoinPrepared, ScyllaJoinConfigured, ScyllaJoinStarted:
			return "scylla_join_in_progress:" + string(n.ScyllaJoinPhase)
		}
	}

	// v2-join Phase B: JoinLifecyclePhase gate at top of this function handles
	// registered-but-not-admitted nodes. The TODO here is resolved.

	// TODO(v2-join-Phase-C): exclude nodes whose AgentEndpoint is empty or
	// whose LastSeen is stale beyond a configurable threshold, indicating the
	// node-agent has disconnected. Currently omitted to avoid false exclusions
	// on clusters where LastSeen is not yet reliably updated.

	// TODO(v2-join-Phase-D): once Scylla Raft Group 0 voter state is tracked,
	// exclude nodes that are Group 0 voters but unreachable or schema-agreement-
	// blocked. ScyllaJoinNone is ambiguous today (legacy nodes that ran before
	// the state machine was introduced will have phase="" and no start time even
	// though they are healthy). Phase D resolves this by tracking voters directly.

	return ""
}
