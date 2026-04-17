package main

import (
	"log"
	"sort"
	"strings"
	"time"
)

// profileInheritance defines implicit profile inclusions.
// A control-plane node IS a core node — it needs all core infra and services.
var profileInheritance = map[string][]string{
	"control-plane": {"core"},
	"compute":       {"core"},
}

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
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		normalized := strings.ToLower(strings.TrimSpace(p))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
		// Expand inherited profiles.
		for _, inherited := range profileInheritance[normalized] {
			if _, ok := seen[inherited]; !ok {
				seen[inherited] = struct{}{}
				result = append(result, inherited)
			}
		}
	}
	sort.Strings(result)
	return result
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

// countNodesWithProfile counts how many nodes in the map have the given profile.
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
	storageCount := countNodesWithProfile(srv.state.Nodes, "storage")
	if storageCount >= MinQuorumNodes {
		return false
	}
	needed := MinQuorumNodes - storageCount

	// Collect candidates: nodes without storage, preferring control-plane.
	type candidate struct {
		id             string
		hasControlPlane bool
		lastSeen       time.Time
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
			candidates[i].id, node.Identity.Hostname, storageCount+i, MinQuorumNodes, MinQuorumNodes)
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
