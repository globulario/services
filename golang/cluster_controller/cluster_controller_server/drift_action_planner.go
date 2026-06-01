// @awareness namespace=globular.platform
// @awareness component=platform_controller.drift_action_planner
// @awareness file_role=per_action_safe_vs_topology_classification_replaces_all_or_nothing_halt
// @awareness implements=globular.platform:intent.controller.topology_safety_blocks_unsafe_drift_actions
// @awareness implements=globular.platform:intent.reconciliation.must_be_idempotent_and_bounded
// @awareness risk=high
package main

// drift_action_planner.go — per-action safety gate for drift reconciliation.
//
// The drift reconciler detects version mismatches between desired and installed
// state and emits cluster.drift_detected events. Before emitting an event that
// would trigger a remediation workflow, each action is classified as safe or
// topology-affecting.
//
// Safe actions (SERVICE, COMMAND version bumps) bypass the topology preflight
// and proceed even when cluster topology is degraded. Topology-affecting
// actions (INFRASTRUCTURE reconfigurations) are blocked when topologyPreflight
// returns violations.
//
// This replaces the previous all-or-nothing early return that halted ALL drift
// processing when any topology violation was present — a forbidden pattern
// per topology.reconciler_must_respect_safety_contract.
//
// Invariant: topology.reconciler_must_respect_safety_contract

import "strings"

// driftActionKind classifies a drift action by its safety profile with respect
// to cluster topology.
type driftActionKind string

const (
	// driftActionKindSafe covers actions that do not mutate cluster membership,
	// storage configuration, or ingress topology. These proceed even when the
	// topology preflight is degraded.
	driftActionKindSafe driftActionKind = "safe"

	// driftActionKindTopology covers actions that could affect cluster membership
	// or storage topology. These are blocked when topologyPreflight returns
	// violations.
	driftActionKindTopology driftActionKind = "topology"
)

// driftAction represents a single reconciliation action that the drift
// reconciler is considering dispatching.
type driftAction struct {
	NodeID     string
	PackageKey string         // "KIND/name"
	Kind       string         // SERVICE | COMMAND | INFRASTRUCTURE | ...
	ActionKind driftActionKind
}

// classifyDriftAction returns the safety classification for an action based on
// the package kind. SERVICE and COMMAND updates are always safe — they do not
// affect cluster membership or storage topology. INFRASTRUCTURE packages are
// topology-affecting and must not be applied when topology constraints are
// violated.
//
// The function is deterministic: the same kind always yields the same result.
func classifyDriftAction(kind string) driftActionKind {
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE":
		return driftActionKindTopology
	default:
		// SERVICE, COMMAND, and any future kinds default to safe.
		// Unknown kinds are conservatively classified as safe to avoid
		// blocking legitimate service updates on unrecognised labels.
		return driftActionKindSafe
	}
}

// driftActionSafe reports whether the action is safe to dispatch given the
// current topology violations. Safe-classified actions always return true.
// Topology-classified actions return false when any violation is present.
func driftActionSafe(action driftAction, violations []topologySafetyViolation) bool {
	if action.ActionKind == driftActionKindSafe {
		return true
	}
	return len(violations) == 0
}
