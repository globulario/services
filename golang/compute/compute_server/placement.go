// placement.go implements resource-aware node placement for compute units.
//
// Placement has two phases:
//   1. Hard filters: profile match, minimum CPU/memory/disk
//   2. Scoring: combine load (active units) with capacity (CPU, RAM, disk)
//
// The scorer is simple and transparent — placement decisions are fully
// explained in structured logs.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/compute/computepb"
)

// ─── Priority model ──────────────────────────────────────────────────────────

// Priority classes for compute jobs. Higher value = higher priority.
const (
	PriorityLow      = 1
	PriorityNormal   = 5
	PriorityHigh     = 8
	PriorityCritical = 10
)

// parsePriority converts a string priority to a numeric value.
// Default is PriorityNormal when unset or unrecognized.
func parsePriority(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return PriorityLow
	case "high":
		return PriorityHigh
	case "critical":
		return PriorityCritical
	case "normal", "":
		return PriorityNormal
	default:
		return PriorityNormal
	}
}

// placementResult holds the outcome of a placement decision.
type placementResult struct {
	Node  computeNodeInfo
	Score float64
	Load  int
}

// placementError is returned when no node satisfies the hard filters.
type placementError struct {
	Reason     string
	Candidates int
	Profiles   []string
}

func (e *placementError) Error() string {
	return fmt.Sprintf("placement failed: %s (candidates=%d, profiles=%v)", e.Reason, e.Candidates, e.Profiles)
}

// placeUnit selects the best node for a compute unit. It applies hard filters
// first (profiles, minimum resources), then scores eligible nodes by load +
// capacity. Higher-priority jobs get a scoring boost that effectively lets them
// tolerate more node load. Returns the chosen node with full observability.
func placeUnit(ctx context.Context, nodes []computeNodeInfo, def *computepb.ComputeDefinition, loadMap map[string]int, priorities ...int) (*placementResult, error) {
	jobPriority := PriorityNormal
	if len(priorities) > 0 {
		jobPriority = priorities[0]
	}
	if len(nodes) == 0 {
		return nil, &placementError{Reason: "no compute service instances", Candidates: 0}
	}

	// Phase 1: Hard filters.
	eligible := nodes

	// Filter by allowed_node_profiles.
	if len(def.GetAllowedNodeProfiles()) > 0 {
		eligible = filterByProfiles(eligible, def.AllowedNodeProfiles)
		if len(eligible) == 0 {
			return nil, &placementError{
				Reason:     fmt.Sprintf("no nodes match required profiles %v", def.AllowedNodeProfiles),
				Candidates: len(nodes),
				Profiles:   def.AllowedNodeProfiles,
			}
		}
	}

	// Default: prefer compute-profiled nodes.
	if cn := filterByProfiles(eligible, []string{"compute"}); len(cn) > 0 {
		eligible = cn
	}

	// Filter by minimum resource requirements.
	rp := def.GetResourceProfile()
	if rp != nil {
		eligible = filterByResources(eligible, rp)
		if len(eligible) == 0 {
			return nil, &placementError{
				Reason:     "no nodes meet minimum resource requirements",
				Candidates: len(nodes),
			}
		}
	}

	// Phase 2: Score eligible nodes with priority boost.
	best := scoreNodes(eligible, loadMap, jobPriority)

	slog.Info("compute placement: decision",
		"chosen", best.Node.Hostname,
		"address", best.Node.Address,
		"score", fmt.Sprintf("%.2f", best.Score),
		"load", best.Load,
		"eligible", len(eligible),
		"total", len(nodes))

	return best, nil
}

// filterByResources removes nodes that don't meet minimum CPU/memory/disk.
func filterByResources(nodes []computeNodeInfo, rp *computepb.ResourceProfile) []computeNodeInfo {
	if rp == nil {
		return nodes
	}
	var out []computeNodeInfo
	for _, n := range nodes {
		// CPU: min_cpu_millis is in millicores, node has core count.
		// Convert: node cores * 1000 = available millis.
		if rp.MinCpuMillis > 0 && n.CPUCount > 0 {
			if uint32(n.CPUCount)*1000 < rp.MinCpuMillis {
				continue
			}
		}
		// Memory.
		if rp.MinMemoryBytes > 0 && n.RAMBytes > 0 {
			if n.RAMBytes < rp.MinMemoryBytes {
				continue
			}
		}
		// Disk.
		if rp.LocalDiskBytes > 0 && n.DiskFreeBytes > 0 {
			if n.DiskFreeBytes < rp.LocalDiskBytes {
				continue
			}
		}
		out = append(out, n)
	}
	return out
}

// scoreNodes computes a placement score for each node and returns the best.
// Score formula: (capacity_score * priority_boost) / (1 + active_units)
// Higher is better. Capacity = normalized(CPU + RAM + disk).
// Priority boost: higher-priority jobs tolerate more load, effectively
// getting access to nodes that lower-priority jobs would avoid.
// Tie-break: round-robin.
func scoreNodes(nodes []computeNodeInfo, loadMap map[string]int, priority int) *placementResult {
	if len(nodes) == 0 {
		return nil
	}

	// Find max values for normalization.
	var maxCPU, maxDiskFree uint64
	var maxRAM uint64
	for _, n := range nodes {
		if uint64(n.CPUCount) > maxCPU {
			maxCPU = uint64(n.CPUCount)
		}
		if n.RAMBytes > maxRAM {
			maxRAM = n.RAMBytes
		}
		if n.DiskFreeBytes > maxDiskFree {
			maxDiskFree = n.DiskFreeBytes
		}
	}

	var bestScore float64 = -1
	var bestNodes []computeNodeInfo
	var bestLoad int

	for _, n := range nodes {
		// Normalize each dimension to [0, 1].
		cpuNorm := safeNorm(uint64(n.CPUCount), maxCPU)
		ramNorm := safeNorm(n.RAMBytes, maxRAM)
		diskNorm := safeNorm(n.DiskFreeBytes, maxDiskFree)

		// Capacity score: weighted average.
		capacity := 0.4*cpuNorm + 0.4*ramNorm + 0.2*diskNorm

		// Priority boost: higher priority tolerates more load.
		// Normal(5)=1.0x, High(8)=1.6x, Critical(10)=2.0x, Low(1)=0.2x
		priorityBoost := float64(priority) / float64(PriorityNormal)

		// Load penalty — check by both NodeID and Address since the
		// load map may be keyed by either depending on the source.
		load := 0
		if loadMap != nil {
			load = loadMap[n.Address]
			if l, ok := loadMap[n.NodeID]; ok && l > load {
				load = l
			}
		}
		score := (capacity * priorityBoost) / float64(1+load)

		if score > bestScore {
			bestScore = score
			bestNodes = []computeNodeInfo{n}
			bestLoad = load
		} else if score == bestScore {
			bestNodes = append(bestNodes, n)
		}
	}

	// Tie-break with round-robin.
	idx := roundRobinCounter.Add(1) - 1
	chosen := bestNodes[int(idx)%len(bestNodes)]

	return &placementResult{
		Node:  chosen,
		Score: bestScore,
		Load:  bestLoad,
	}
}

func safeNorm(val, max uint64) float64 {
	if max == 0 {
		return 1.0 // all equal when no data
	}
	return float64(val) / float64(max)
}
