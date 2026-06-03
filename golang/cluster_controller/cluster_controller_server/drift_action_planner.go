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
//
// Topology-classified actions (INFRASTRUCTURE kind) are now NARROWED to the
// violation kinds the specific package's subsystem actually owns:
//
//   - MinIO/objectstore upgrade is gated by storage_quorum (correct: an
//     upgrade that survives only because erasure-coding is degraded would
//     be incorrect).
//   - cluster-controller upgrade is gated by controller_placement.
//   - Envoy/xds/keepalived/dns upgrades are gated by ingress_participant.
//   - node-agent / repository / authentication / observability / AI services
//     are control-plane infrastructure that does NOT mutate storage,
//     ingress, or controller topology. Blocking these on storage_quorum
//     was an over-application of the gate documented in the Phase 30+
//     incident chain (see docs/awareness/reports/envoy_lds_cds_wedge.md
//     and Phase 35 commit).
//   - Unknown packages default conservative: any violation blocks.
//
// Phase 35: this scoping prevents the live cluster's storage_quorum
// violation (single-node cluster, MinIO needs ≥3) from indefinitely
// blocking node-agent rollouts whose binary identity has nothing to do
// with MinIO erasure-coding state.
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
//                          topology dimension blocks it.
//   - (map[k]bool, true) — known topology-sensitive package; blocked
//                          only on listed violation kinds.
//   - (nil, false)       — unknown package; caller treats as conservative
//                          (any violation blocks).
//
// The classification is explicit (not based on name substrings) so new
// packages cannot slip through unclassified — the unknown branch surfaces
// them via continued blocking.
func packageTopologySensitivities(packageName string) (map[string]bool, bool) {
	name := strings.ToLower(strings.TrimSpace(packageName))
	switch name {
	// ── Storage-topology-affecting packages ─────────────────────────────
	// MinIO directly: erasure-coding depends on storage_quorum; an upgrade
	// could reformat or change drive count, so must wait for healthy
	// topology. sidekick = MinIO sidekick (drive-count health checks).
	case "minio", "objectstore", "sidekick":
		return map[string]bool{
			"storage_quorum":                true,
			"objectstore_topology_mismatch": true,
		}, true
	// Scylla and backup-manager touch on storage durability assumptions
	// (Scylla has its own quorum but its operational guarantees overlap
	// with the storage tier; backup-manager depends on object store).
	case "scylladb", "scylla", "scylla-manager", "backup-manager":
		return map[string]bool{"storage_quorum": true}, true

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
	// topology. Blocking them on storage_quorum (the over-application
	// observed in the live INC) is incorrect — their bytes are
	// independent of the storage subsystem's health.
	case "node-agent", "repository", "authentication", "rbac", "resource",
		"event", "log", "monitoring", "prometheus", "alertmanager",
		"ai-executor", "ai-memory", "ai-router", "ai-watcher",
		"mcp", "media", "file", "search", "title", "persistence",
		"workflow", "torrent", "awareness-graph", "ldap", "mail",
		"globular-cli", "node-exporter", "scylla-manager-agent",
		"cluster-doctor", "etcd", "blog", "catalog", "conversation",
		"compute", "domain", "globular-cli-bin":
		return nil, true
	}
	return nil, false
}
