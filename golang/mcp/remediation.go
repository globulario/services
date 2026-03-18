package main

import "sort"

// ── Remediation Types ────────────────────────────────────────────────────────

// RemediationPlan is an actionable, ordered sequence of steps to make a
// blocked operation safe.
type RemediationPlan struct {
	TargetOperation  string            `json:"target_operation"`
	Status           string            `json:"status"` // "ready", "blocked", "ambiguous"
	Reason           string            `json:"reason,omitempty"`
	OrderedSteps     []RemediationStep `json:"ordered_steps"`
	AffectedServices []string          `json:"affected_services,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
}

// RemediationStep is a single step in a remediation plan.
type RemediationStep struct {
	Order    int    `json:"order"`
	Action   string `json:"action"` // "remove", "disable", "reconfigure", "install"
	Target   string `json:"target"`
	Reason   string `json:"reason"`
	Blocking bool   `json:"blocking,omitempty"`
}

// ── Removal Order Planner ────────────────────────────────────────────────────

// BuildRemovalPlan computes a safe, leaf-first removal order for a target
// service given the set of installed services and a dependency source.
//
// It builds the reverse dependency subgraph reachable from the target,
// then performs a topological sort (leaves first) so that no service is
// removed before its dependents.
//
// Returns a RemediationPlan with status:
//   - "ready"    — valid topological order found
//   - "blocked"  — cycle detected, manual intervention required
func BuildRemovalPlan(target string, installed []string, deps DependencySource) *RemediationPlan {
	plan := &RemediationPlan{
		TargetOperation: "remove " + target,
	}

	// Build the subgraph of services that transitively depend on target.
	// reverseGraph[X] = services that X depends on (within the impacted set)
	impacted := collectImpactedServices(target, installed, deps)

	if len(impacted) == 0 {
		// No dependents — target can be removed directly
		plan.Status = "ready"
		plan.OrderedSteps = []RemediationStep{
			{Order: 1, Action: "remove", Target: target, Reason: "no dependents"},
		}
		return plan
	}

	// All impacted services + the target itself
	allNodes := make([]string, 0, len(impacted)+1)
	allNodes = append(allNodes, impacted...)
	allNodes = append(allNodes, target)
	plan.AffectedServices = impacted

	// Build forward dependency edges within the impacted set for topo sort.
	// edge: dependent → dependency (must remove dependent first)
	nodeSet := make(map[string]bool, len(allNodes))
	for _, n := range allNodes {
		nodeSet[n] = true
	}

	// inDegree counts how many services within the set depend on each node
	inDegree := make(map[string]int, len(allNodes))
	// dependsOn[A] = [B, C] means A depends on B and C (within the set)
	dependsOn := make(map[string][]string)

	for _, n := range allNodes {
		inDegree[n] = 0
	}

	for _, svc := range allNodes {
		svcDeps := deps.Dependencies(svc)
		for _, d := range svcDeps {
			if nodeSet[d.Name] && d.Required {
				// svc depends on d.Name → svc must be removed before d.Name
				// so d.Name has an incoming edge from svc
				dependsOn[svc] = append(dependsOn[svc], d.Name)
				inDegree[d.Name]++
			}
		}
	}

	// Kahn's algorithm for topological sort (leaf-first = zero in-degree first)
	var queue []string
	for _, n := range allNodes {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}
	// Sort queue for deterministic output
	sort.Strings(queue)

	var ordered []string
	for len(queue) > 0 {
		// Pick alphabetically first for determinism
		sort.Strings(queue)
		node := queue[0]
		queue = queue[1:]
		ordered = append(ordered, node)

		// For each dependency of this node within the set,
		// decrement its in-degree
		for _, dep := range dependsOn[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Cycle detection: if ordered doesn't contain all nodes, there's a cycle
	if len(ordered) != len(allNodes) {
		plan.Status = "blocked"
		plan.Reason = "dependency cycle detected — manual intervention required"

		// Report which nodes are in the cycle
		orderedSet := make(map[string]bool, len(ordered))
		for _, n := range ordered {
			orderedSet[n] = true
		}
		var cycleNodes []string
		for _, n := range allNodes {
			if !orderedSet[n] {
				cycleNodes = append(cycleNodes, n)
			}
		}
		sort.Strings(cycleNodes)
		plan.Warnings = append(plan.Warnings, "services involved in cycle: "+joinStrings(cycleNodes, ", "))
		return plan
	}

	// Build steps
	plan.Status = "ready"
	for i, svc := range ordered {
		reason := "leaf — no remaining dependents"
		if svc == target {
			reason = "target service — all dependents removed"
		}
		plan.OrderedSteps = append(plan.OrderedSteps, RemediationStep{
			Order:  i + 1,
			Action: "remove",
			Target: svc,
			Reason: reason,
		})
	}

	return plan
}

// collectImpactedServices finds all services that transitively depend on target
// among the installed services (BFS over reverse dependency graph).
func collectImpactedServices(target string, installed []string, deps DependencySource) []string {
	visited := map[string]bool{target: true}
	queue := []string{target}
	var impacted []string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Find installed services that depend on current
		dependents := deps.ReverseDeps(current, installed)
		for _, dep := range dependents {
			if !visited[dep] {
				visited[dep] = true
				impacted = append(impacted, dep)
				queue = append(queue, dep)
			}
		}
	}

	sort.Strings(impacted)
	return impacted
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
