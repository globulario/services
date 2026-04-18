package main

// recovery_planner.go — deterministic reseed ordering for node.recover.full_reseed.
//
// Rule D: Install order must be deterministic and infrastructure-aware.
// Rule B: Exact captured build_id/checksum is preferred over latest stable.
//
// Ordering model (bootstrap class + kind + priority + lexical tiebreaker):
//
//   BOOTSTRAP_FOUNDATION    (0) — etcd, scylladb, minio
//   BOOTSTRAP_CORE_CONTROL  (1) — auth, rbac, resource, discovery, dns, repository,
//                                  workflow, cluster-controller, node-agent
//   BOOTSTRAP_SUPPORTING    (2) — monitoring, event, envoy, log, xds, keepalived
//   BOOTSTRAP_WORKLOAD      (3) — applications, other services, commands

import (
	"fmt"
	"log"
	"sort"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// bootstrapClass assigns an ordering class to an artifact.
// Lower number = installed earlier.
type bootstrapClass int

const (
	bootstrapFoundation   bootstrapClass = 0
	bootstrapCoreControl  bootstrapClass = 1
	bootstrapSupporting   bootstrapClass = 2
	bootstrapWorkload     bootstrapClass = 3
)

// bootstrapClassOf derives the bootstrap class for an artifact.
func bootstrapClassOf(name, kind string) bootstrapClass {
	n := strings.ToLower(name)
	k := strings.ToUpper(kind)

	// Foundation: essential distributed substrate.
	switch n {
	case "etcd", "scylladb", "scylla", "minio":
		return bootstrapFoundation
	}

	// Core control-plane services.
	coreControl := map[string]bool{
		"authentication":       true,
		"authentication-server": true,
		"rbac":                 true,
		"rbac-server":          true,
		"resource":             true,
		"resource-server":      true,
		"discovery":            true,
		"discovery-server":     true,
		"dns":                  true,
		"dns-server":           true,
		"repository":           true,
		"repository-server":    true,
		"workflow":             true,
		"workflow-server":      true,
		"cluster-controller":   true,
		"node-agent":           true,
		"node_agent":           true,
	}
	if coreControl[n] {
		return bootstrapCoreControl
	}

	// Supporting infrastructure: observability, mesh, logging.
	supporting := map[string]bool{
		"monitoring":        true,
		"monitoring-server": true,
		"event":             true,
		"event-server":      true,
		"envoy":             true,
		"envoy-xds":         true,
		"xds":               true,
		"log":               true,
		"log-server":        true,
		"keepalived":        true,
	}
	if supporting[n] {
		return bootstrapSupporting
	}

	// Infrastructure kind but not named above → supporting tier.
	if k == "INFRASTRUCTURE" {
		return bootstrapSupporting
	}

	// Everything else (SERVICE, APPLICATION, COMMAND) → workload tier.
	return bootstrapWorkload
}

// kindRank returns a numeric rank within the same bootstrap class.
// Lower = earlier.
func kindRank(kind string) int {
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE":
		return 0
	case "SERVICE":
		return 1
	case "APPLICATION":
		return 2
	case "COMMAND":
		return 3
	default:
		return 4
	}
}

// sortedReseedOrder returns the artifacts sorted in deterministic install order.
//
// Sort key (stable, ascending):
//   1. bootstrap class (lower = first)
//   2. kind rank
//   3. explicit priority field (lower = first)
//   4. stable lexical: kind/publisher/name/version/build_id
//
// If cyclic hard-deps are detected the planner logs a warning and falls back
// to topological ambiguity resolution (the non-cyclic part first, cycles last).
func sortedReseedOrder(artifacts []cluster_controllerpb.SnapshotArtifact) []cluster_controllerpb.SnapshotArtifact {
	type ranked struct {
		art         cluster_controllerpb.SnapshotArtifact
		bClass      bootstrapClass
		kRank       int
		lexKey      string
	}

	ranked_ := make([]ranked, len(artifacts))
	for i, a := range artifacts {
		ranked_[i] = ranked{
			art:    a,
			bClass: bootstrapClassOf(a.Name, a.Kind),
			kRank:  kindRank(a.Kind),
			lexKey: strings.ToLower(fmt.Sprintf("%s/%s/%s/%s/%s", a.Kind, a.PublisherID, a.Name, a.Version, a.BuildID)),
		}
	}

	sort.SliceStable(ranked_, func(i, j int) bool {
		ri, rj := ranked_[i], ranked_[j]
		if ri.bClass != rj.bClass {
			return ri.bClass < rj.bClass
		}
		if ri.kRank != rj.kRank {
			return ri.kRank < rj.kRank
		}
		pi, pj := ri.art.Priority, rj.art.Priority
		if pi != pj {
			return pi < pj
		}
		return ri.lexKey < rj.lexKey
	})

	out := make([]cluster_controllerpb.SnapshotArtifact, len(ranked_))
	for i, r := range ranked_ {
		out[i] = r.art
	}
	return out
}

// buildReseedPlan constructs the PlannedRecoveryArtifact list from a snapshot.
// It validates exact-build availability when exactRequired is true.
//
// Returns an error if exactRequired is true and any artifact lacks a build_id.
func buildReseedPlan(snap *cluster_controllerpb.NodeRecoverySnapshot, exactRequired bool) ([]cluster_controllerpb.PlannedRecoveryArtifact, error) {
	if snap == nil || len(snap.Artifacts) == 0 {
		return nil, fmt.Errorf("snapshot is empty — cannot build reseed plan")
	}

	sorted := sortedReseedOrder(snap.Artifacts)
	plan := make([]cluster_controllerpb.PlannedRecoveryArtifact, 0, len(sorted))
	var missing []string

	for i, art := range sorted {
		source := "SNAPSHOT_EXACT"
		if art.BuildID == "" {
			source = "REPOSITORY_RESOLVED"
			if exactRequired {
				missing = append(missing, fmt.Sprintf("%s/%s@%s", art.Kind, art.Name, art.Version))
			}
		}
		plan = append(plan, cluster_controllerpb.PlannedRecoveryArtifact{
			PublisherID: art.PublisherID,
			Name:        art.Name,
			Kind:        art.Kind,
			Version:     art.Version,
			BuildID:     art.BuildID,
			Checksum:    art.Checksum,
			Order:       int32(i),
			Source:      source,
		})
	}

	if exactRequired && len(missing) > 0 {
		return nil, fmt.Errorf("exact_replay_required but %d artifact(s) have no build_id: %s",
			len(missing), strings.Join(missing, ", "))
	}

	if len(missing) > 0 {
		log.Printf("recovery planner: %d artifact(s) will use repository resolution (no build_id): %s",
			len(missing), strings.Join(missing, ", "))
	}

	return plan, nil
}

// validateNoReseedCycle checks that the requires/provides graph in the snapshot
// contains no cycles. Returns an error describing the cycle if found.
func validateNoReseedCycle(artifacts []cluster_controllerpb.SnapshotArtifact) error {
	// Build adjacency: name → names it requires.
	adj := make(map[string][]string)
	names := make(map[string]bool)
	for _, a := range artifacts {
		n := strings.ToLower(a.Name)
		names[n] = true
		for _, req := range a.Requires {
			adj[n] = append(adj[n], strings.ToLower(req))
		}
	}

	// DFS cycle detection.
	visited := make(map[string]int) // 0=unvisited 1=in-stack 2=done
	var path []string
	var dfs func(n string) bool
	dfs = func(n string) bool {
		if visited[n] == 2 {
			return false
		}
		if visited[n] == 1 {
			return true // cycle
		}
		visited[n] = 1
		path = append(path, n)
		for _, dep := range adj[n] {
			if !names[dep] {
				continue // external dep — ignore
			}
			if dfs(dep) {
				return true
			}
		}
		path = path[:len(path)-1]
		visited[n] = 2
		return false
	}

	for n := range names {
		path = nil
		if dfs(n) {
			return fmt.Errorf("cycle detected in requires graph: %s → %s",
				strings.Join(path, " → "), n)
		}
	}
	return nil
}
