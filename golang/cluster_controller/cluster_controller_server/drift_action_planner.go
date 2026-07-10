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
	PackageKey string // "KIND/name"
	Kind       string // SERVICE | COMMAND | INFRASTRUCTURE | ...
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
//
// Topology-classified actions (INFRASTRUCTURE kind) are narrowed to concrete
// violation kinds the specific package's subsystem owns: objectstore topology
// mismatch, controller placement, or ingress participant placement. Observed
// quorum/capacity is report data, not a global drift admission floor.
func driftActionSafe(action driftAction, violations []topologySafetyViolation) bool {
	if action.ActionKind == driftActionKindSafe {
		return true
	}
	if len(violations) == 0 {
		return true
	}
	// Extract package name from "KIND/name".
	parts := strings.SplitN(action.PackageKey, "/", 2)
	name := parts[len(parts)-1]
	sensitivities, known := packageTopologySensitivities(name)
	if !known {
		// Unknown package — conservative: any violation blocks. New
		// packages should be classified explicitly before they reach
		// drift dispatch; the conservative default keeps integrity intact
		// until classification lands.
		return false
	}
	if sensitivities == nil {
		// Known control-plane / observability / AI service: does not
		// participate in any topology subsystem; not blocked by any
		// topology violation.
		return true
	}
	for _, v := range violations {
		if sensitivities[v.Kind] {
			return false
		}
	}
	return true
}

// packageTopologySensitivities returns the violation kinds that should
// block dispatch for the named package.
//
//   - (nil, true)        — known control-plane / service package; no
//     topology dimension blocks it.
//   - (map[k]bool, true) — known topology-sensitive package; blocked
//     only on listed violation kinds.
//   - (nil, false)       — unknown package; caller treats as conservative
//     (any violation blocks).
//
// The classification is explicit (not based on name substrings) so new
// packages cannot slip through unclassified — the unknown branch surfaces
// them via continued blocking.
func packageTopologySensitivities(packageName string) (map[string]bool, bool) {
	name := strings.ToLower(strings.TrimSpace(packageName))
	switch name {
	// ── Objectstore-topology-affecting packages ─────────────────────────
	// These are gated only by concrete objectstore topology mismatch, not
	// by a preferred storage node count.
	case "minio", "objectstore", "sidekick":
		return map[string]bool{"objectstore_topology_mismatch": true}, true

	// Scylla and backup-manager have component-local safety checks; do not
	// block them on a platform-wide storage count floor.
	case "scylladb", "scylla", "scylla-manager", "backup-manager":
		return nil, true

	// ── Ingress-participating packages ──────────────────────────────────
	// Envoy and xds are the mesh; keepalived is the VIP; DNS publishes
	// records that depend on the ingress participant set.
	case "envoy", "xds", "keepalived", "dns":
		return map[string]bool{"ingress_participant": true}, true

	// ── Controller placement ───────────────────────────────────────────
	// The controller's own upgrade must respect placement constraints.
	case "cluster-controller", "controller":
		return map[string]bool{"controller_placement": true}, true

	// ── Control-plane / service / observability — no topology gates ────
	// These packages do NOT mutate storage, ingress, or controller
	// topology.
	case "node-agent", "repository", "authentication", "rbac", "resource",
		"event", "log", "monitoring", "prometheus", "alertmanager",
		"ai-executor", "ai-memory", "ai-router", "ai-watcher",
		"mcp", "media", "file", "search", "title", "persistence",
		"workflow", "torrent", "ldap", "mail",
		"globular-cli", "node-exporter", "scylla-manager-agent",
		"cluster-doctor", "etcd", "blog", "catalog", "conversation",
		"compute", "domain", "globular-cli-bin":
		return nil, true
	}
	return nil, false
}
