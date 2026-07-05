package main

import (
	"log"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/component_catalog"
)

// foundingNodeProfiles are the profiles that MUST be present on the first 3
// nodes of the cluster. These provide the quorum-capable infrastructure:
//   - etcd: needs all nodes (embedded in core/control-plane)
//   - ScyllaDB: minimum 3 nodes for replication
//   - MinIO: minimum 3 nodes for erasure coding
//
// Without these on the founding nodes, the cluster has single points of failure
// that cascade into workflow execution, artifact publishing, and reconciliation.
//
// INVARIANT: The first 3 nodes MUST have core + control-plane + storage.
// This is non-negotiable and enforced at join time.
// media-server is intentionally excluded here. The founding node must always
// carry quorum + core, but media workloads are opt-in so fresh nodes do not
// inherit content/media services by default.
var foundingNodeProfiles = []string{"core", "control-plane", "storage"}

// MinQuorumNodes is the minimum number of nodes required for infrastructure
// quorum (etcd, ScyllaDB, MinIO). Below this count, every node MUST have
// all foundational profiles.
const MinQuorumNodes = 3

// normalizeProfiles deduplicates, lowercases, trims, sorts, and expands
// inherited profiles. For example:
//
//	["control-plane", "gateway"] → ["control-plane", "core", "gateway"]
func normalizeProfiles(raw []string) []string {
	return component_catalog.NormalizeProfiles(raw)
}

// notInheritedProfiles are excluded from cluster-profile inheritance
// (inheritableClusterProfiles). Two categories:
//   - hardware-gated: control-plane, storage, gateway — governed per-node by
//     deduceProfiles from the node's own capabilities, so a resource-light node
//     never inherits e.g. "storage" it cannot serve. enforceFoundingProfiles
//     still guarantees these on the first MinQuorumNodes nodes for quorum.
//   - opt-in workloads: media-server — deliberately opt-in (see foundingNodeProfiles
//     comment above); fresh nodes must not auto-inherit content/media services.
//
// Everything else that is a real catalog profile (core, compute, dns, database,
// scylla, …) is inheritable so joining nodes come out identical to the founder.
var notInheritedProfiles = map[string]bool{
	"control-plane": true,
	"storage":       true,
	"gateway":       true,
	"media-server":  true,
}

// filterCatalogProfiles returns only the catalog-defined profiles from raw
// (case-insensitive via component_catalog.HasProfile), dropping unknown or
// derived labels. Returns nil if none remain, so a caller passing an
// operator-requested set degrades to suggested/inherited defaults rather than
// assigning an undefined profile. The controller — not the requester — decides;
// this upholds the profile↔component bijection invariant.
func filterCatalogProfiles(raw []string) []string {
	var out []string
	for _, p := range raw {
		if component_catalog.HasProfile(p) {
			out = append(out, p)
		}
	}
	return out
}

// inheritableClusterProfiles returns the union of assignable (catalog) profiles
// currently present across cluster nodes, excluding notInheritedProfiles and any
// non-catalog/derived label (e.g. "ai", which is derived from installed ai-*
// services in the status projection, not an assignable profile). A joining node
// inherits these when no explicit profile set was requested, so nodes come out
// identical to the founder for software/infrastructure profiles while hardware
// and opt-in profiles stay governed elsewhere.
//
// Only catalog-defined profiles are ever returned (component_catalog.HasProfile),
// which upholds the profile↔component bijection invariant — inheritance can never
// introduce an undefined profile.
//
// Caller must hold srv.lock (reads the passed srv.state.Nodes map).
func inheritableClusterProfiles(nodes map[string]*nodeState) []string {
	var out []string
	for _, n := range nodes {
		if n == nil {
			continue
		}
		for _, p := range n.Profiles {
			key := strings.ToLower(strings.TrimSpace(p))
			if notInheritedProfiles[key] {
				continue
			}
			if component_catalog.HasProfile(key) {
				out = append(out, key)
			}
		}
	}
	return out // dedup handled by normalizeProfiles at the callsite
}

// enforceFoundingProfiles ensures the founding node profiles are present
// when the cluster has fewer than MinQuorumNodes nodes with storage.
// Returns the merged profile set.
func enforceFoundingProfiles(profiles []string, storageNodeCount int) []string {
	if storageNodeCount >= MinQuorumNodes {
		return profiles // quorum already met, no enforcement needed
	}
	merged := make([]string, len(profiles))
	copy(merged, profiles)
	set := make(map[string]bool, len(merged))
	for _, p := range merged {
		set[p] = true
	}
	for _, fp := range foundingNodeProfiles {
		if !set[fp] {
			merged = append(merged, fp)
		}
	}
	return normalizeProfiles(merged)
}

// countNodesWithProfile counts how many nodes in the map carry the given profile
// LABEL. A label is placement intent, not proof of realized capacity — use this
// only for intent/placement decisions (e.g. deducing suggested profiles), never
// to answer a quorum/RF/capacity question. For that, use
// countVerifiedNodesWithProfile.
func countNodesWithProfile(nodes map[string]*nodeState, profile string) int {
	count := 0
	for _, n := range nodes {
		for _, p := range n.Profiles {
			if p == profile {
				count++
				break
			}
		}
	}
	return count
}

// countVerifiedNodesWithProfile counts nodes that BOTH carry the given profile
// label AND are verified-eligible cluster members (IsNodeVerifiedStorageEligible).
// This is the capacity/quorum count: a label is intent, verification is capacity
// (meta.limited_members_are_not_capacity /
// forbidden_fix:profile_label_counts_as_storage_capacity). A node labeled
// "storage" but still bootstrapping, failed, or mid-join is NOT storage capacity
// and must not satisfy founding/storage quorum. Mirrors the gate already used by
// storageControlPlaneNodeCount.
func countVerifiedNodesWithProfile(nodes map[string]*nodeState, profile string) int {
	count := 0
	for _, n := range nodes {
		if !IsNodeVerifiedStorageEligible(n) {
			continue
		}
		for _, p := range n.Profiles {
			if p == profile {
				count++
				break
			}
		}
	}
	return count
}

// enforceStorageQuorumLocked checks that at least MinQuorumNodes have the
// storage profile. If a storage node has left (or was removed), this
// auto-promotes the best available non-storage node to restore quorum.
//
// Selection criteria for promotion:
//  1. Prefer nodes with control-plane profile (already run etcd/ScyllaDB)
//  2. Prefer nodes that are healthy (recent heartbeat)
//  3. Never promote a node that is blocked or in bootstrap
//
// MUST be called under srv.lock(). Returns true if state was modified.
func (srv *server) enforceStorageQuorumLocked() bool {
	// Quorum is a capacity question: count only VERIFIED storage members, not
	// nodes that merely carry the label (forbidden_fix:profile_label_counts_as_storage_capacity).
	storageCount := countVerifiedNodesWithProfile(srv.state.Nodes, "storage")
	// The storage-node floor is policy-derived: durable/undeclared -> MinQuorumNodes
	// (3); a declared degraded policy lowers it to 2 or 1. cachedMinStorageNodes is
	// the no-I/O accessor (this runs under srv.lock) and resolves to the durable
	// floor on a cold cache — never degraded by accident
	// (intent:degraded_is_explicit_not_hidden).
	minNodes := cachedMinStorageNodes()
	if storageCount >= minNodes {
		return false
	}
	needed := minNodes - storageCount

	// Collect candidates: nodes without storage, preferring control-plane.
	type candidate struct {
		id              string
		hasControlPlane bool
		lastSeen        time.Time
	}
	var candidates []candidate
	for id, n := range srv.state.Nodes {
		if n.BlockedReason != "" {
			continue
		}
		if n.BootstrapPhase != "" && n.BootstrapPhase != BootstrapWorkloadReady && n.BootstrapPhase != "BootstrapNone" {
			continue // still bootstrapping
		}
		hasStorage := false
		hasCP := false
		for _, p := range n.Profiles {
			if p == "storage" {
				hasStorage = true
			}
			if p == "control-plane" {
				hasCP = true
			}
		}
		if hasStorage {
			continue // already has storage
		}
		candidates = append(candidates, candidate{id: id, hasControlPlane: hasCP, lastSeen: n.LastSeen})
	}

	// Sort: control-plane first, then by most recently seen.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].hasControlPlane != candidates[j].hasControlPlane {
			return candidates[i].hasControlPlane
		}
		return candidates[i].lastSeen.After(candidates[j].lastSeen)
	})

	modified := false
	for i := 0; i < needed && i < len(candidates); i++ {
		node := srv.state.Nodes[candidates[i].id]
		node.Profiles = enforceFoundingProfiles(node.Profiles, 0) // force all foundational
		log.Printf("storage-quorum: auto-promoted node %s (%s) to storage profile (was %d/%d, need %d)",
			candidates[i].id, node.Identity.Hostname, storageCount+i, minNodes, minNodes)
		modified = true
	}

	if modified {
		srv.emitClusterEvent("controller.storage_quorum_enforced", map[string]interface{}{
			"previous_count": storageCount,
			"promoted_count": needed,
		})
	}

	return modified
}
