package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Node Intent — the resolved Day 1 state for a node
// ---------------------------------------------------------------------------

// NodeIntent is the complete resolved desired state for a node, derived from
// its profiles and the component catalog.
type NodeIntent struct {
	// NodeID is the node this intent was resolved for.
	NodeID string `json:"node_id"`

	// Profiles is the node's assigned profiles.
	Profiles []string `json:"profiles"`

	// RequiredCapabilities is the union of capabilities required by the profiles.
	RequiredCapabilities []Capability `json:"required_capabilities"`

	// DesiredInfra lists infrastructure components to install (ordered by priority).
	// Includes components directly from profiles AND those pulled in by
	// capability requirements or transitive dependencies.
	DesiredInfra []*Component `json:"-"`

	// DesiredInfraNames is the serializable form of DesiredInfra.
	DesiredInfraNames []string `json:"desired_infra"`

	// DesiredWorkloads lists workload components eligible for installation
	// (ordered by priority). Only includes workloads whose runtime local
	// dependencies are satisfied.
	DesiredWorkloads []*Component `json:"-"`

	// DesiredWorkloadNames is the serializable form of DesiredWorkloads.
	DesiredWorkloadNames []string `json:"desired_workloads"`

	// BlockedWorkloads lists workloads that cannot start because their
	// runtime local dependencies are not healthy.
	BlockedWorkloads []BlockedWorkload `json:"blocked_workloads,omitempty"`

	// ResolvedComponents is the full list of canonical component names
	// (infra + workloads + blocked) for diagnostics.
	ResolvedComponents []string `json:"resolved_components"`

	// MaterializedDesired records infra desired-state entries that were
	// auto-created by the controller during this reconcile cycle.
	// Populated by materializeMissingInfraDesired, not by ResolveNodeIntent.
	MaterializedDesired []MaterializedInfra `json:"materialized_desired,omitempty"`
}

// BlockedWorkload records why a workload cannot start.
type BlockedWorkload struct {
	Name        string   `json:"name"`
	MissingDeps []string `json:"missing_deps"`
	Reason      string   `json:"reason"`
}

// ---------------------------------------------------------------------------
// Resolver
// ---------------------------------------------------------------------------

// ResolveNodeIntent computes the full Day 1 intent for a node given its
// profiles and current unit status. The algorithm:
//
//  1. Collect capabilities required by profiles.
//  2. Find infra components that provide those capabilities.
//  3. Collect all components whose profiles match the node.
//  4. Expand transitive install and runtime dependencies.
//  5. Classify into infra vs workload.
//  6. Gate workloads on runtime dependency health.
func ResolveNodeIntent(nodeID string, profiles []string, units []unitStatusRecord) (*NodeIntent, error) {
	normalized := normalizeProfiles(profiles)
	if len(normalized) == 0 {
		normalized = []string{"core"}
	}

	// Validate profiles.
	for _, p := range normalized {
		if err := ValidateProfile(p); err != nil {
			return nil, err
		}
	}

	intent := &NodeIntent{
		NodeID:   nodeID,
		Profiles: normalized,
	}

	// Step 1: Collect required capabilities from profiles.
	capSet := make(map[Capability]bool)
	for _, p := range normalized {
		for _, cap := range ProfileCapabilities[p] {
			capSet[cap] = true
		}
	}
	for cap := range capSet {
		intent.RequiredCapabilities = append(intent.RequiredCapabilities, cap)
	}
	sort.Slice(intent.RequiredCapabilities, func(i, j int) bool {
		return intent.RequiredCapabilities[i] < intent.RequiredCapabilities[j]
	})

	// Step 2-4: Collect and expand components.
	selected := make(map[string]*resolvedEntry)

	// 2a: Direct profile membership.
	for _, c := range catalog {
		if profilesOverlap(c.Profiles, normalized) {
			selected[c.Name] = &resolvedEntry{component: c, via: "profile"}
		}
	}

	// 2b: Capability-driven infra selection.
	// If a capability is required but no selected component provides it,
	// find one and add it.
	for cap := range capSet {
		if capSatisfiedBy(cap, selected) {
			continue
		}
		providers := ComponentsProvidingCapability(cap)
		if len(providers) > 0 {
			c := providers[0] // pick first (lowest priority)
			selected[c.Name] = &resolvedEntry{component: c, via: fmt.Sprintf("capability:%s", cap)}
		}
	}

	// Step 3: Expand transitive dependencies.
	// Iterate until no new entries are added.
	for {
		added := false
		for _, entry := range selected {
			for _, dep := range entry.component.InstallDependencies {
				if _, ok := selected[dep]; !ok {
					c := CatalogByName(dep)
					if c == nil {
						return nil, fmt.Errorf("component %q: install dep %q not in catalog", entry.component.Name, dep)
					}
					selected[dep] = &resolvedEntry{component: c, via: fmt.Sprintf("install-dep-of:%s", entry.component.Name)}
					added = true
				}
			}
			for _, dep := range entry.component.RuntimeLocalDependencies {
				if _, ok := selected[dep]; !ok {
					c := CatalogByName(dep)
					if c == nil {
						return nil, fmt.Errorf("component %q: runtime dep %q not in catalog", entry.component.Name, dep)
					}
					selected[dep] = &resolvedEntry{component: c, via: fmt.Sprintf("runtime-dep-of:%s", entry.component.Name)}
					added = true
				}
			}
		}
		if !added {
			break
		}
	}

	// Step 5: Classify into infra and workload.
	var infra, workloads []*Component
	for _, entry := range selected {
		if entry.component.Kind == KindInfrastructure {
			infra = append(infra, entry.component)
		} else {
			workloads = append(workloads, entry.component)
		}
	}

	// Sort by priority (lower = first).
	sort.Slice(infra, func(i, j int) bool {
		if infra[i].Priority != infra[j].Priority {
			return infra[i].Priority < infra[j].Priority
		}
		return infra[i].Name < infra[j].Name
	})
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].Priority != workloads[j].Priority {
			return workloads[i].Priority < workloads[j].Priority
		}
		return workloads[i].Name < workloads[j].Name
	})

	intent.DesiredInfra = infra
	intent.DesiredInfraNames = componentNames(infra)

	// Step 6: Gate workloads on runtime dependency health.
	healthyUnits := buildHealthySet(units)
	var readyWorkloads []*Component
	for _, w := range workloads {
		missing := checkRuntimeDeps(w, healthyUnits)
		if len(missing) > 0 {
			intent.BlockedWorkloads = append(intent.BlockedWorkloads, BlockedWorkload{
				Name:        w.Name,
				MissingDeps: missing,
				Reason:      fmt.Sprintf("waiting for: %s", strings.Join(missing, ", ")),
			})
		} else {
			readyWorkloads = append(readyWorkloads, w)
		}
	}
	intent.DesiredWorkloads = readyWorkloads
	intent.DesiredWorkloadNames = componentNames(readyWorkloads)

	// Build full resolved list.
	allNames := make([]string, 0, len(selected))
	for name := range selected {
		allNames = append(allNames, name)
	}
	sort.Strings(allNames)
	intent.ResolvedComponents = allNames

	return intent, nil
}

// ---------------------------------------------------------------------------
// Filtering helpers for reconcile integration
// ---------------------------------------------------------------------------

// FilterDesiredByIntent returns only the desired services that are in the
// node's resolved component set (infra or workload, including blocked).
func FilterDesiredByIntent(desired map[string]string, intent *NodeIntent) map[string]string {
	if intent == nil {
		return desired
	}
	allowed := make(map[string]bool, len(intent.ResolvedComponents))
	for _, name := range intent.ResolvedComponents {
		allowed[name] = true
	}
	filtered := make(map[string]string, len(desired))
	for svc, ver := range desired {
		canon := normalizeComponentName(canonicalServiceName(svc))
		if allowed[canon] {
			filtered[svc] = ver
		}
	}
	return filtered
}

// GateDependencies removes workload services from the desired map whose
// runtime local dependencies are not healthy. Returns the filtered map
// and a list of blocked services.
func GateDependencies(desired map[string]string, units []unitStatusRecord) (map[string]string, []BlockedWorkload) {
	healthyUnits := buildHealthySet(units)
	filtered := make(map[string]string, len(desired))
	var blocked []BlockedWorkload

	for svc, ver := range desired {
		canon := normalizeComponentName(canonicalServiceName(svc))
		comp := CatalogByName(canon)
		if comp == nil || comp.Kind == KindInfrastructure {
			// Unknown or infra — pass through (infra gated by bootstrap phases).
			filtered[svc] = ver
			continue
		}
		missing := checkRuntimeDeps(comp, healthyUnits)
		if len(missing) > 0 {
			blocked = append(blocked, BlockedWorkload{
				Name:        svc,
				MissingDeps: missing,
				Reason:      fmt.Sprintf("waiting for: %s", strings.Join(missing, ", ")),
			})
		} else {
			filtered[svc] = ver
		}
	}
	return filtered, blocked
}

// NodeIntentIncludesService checks if a node's resolved intent includes
// a service (by canonical name). Used by release pipeline scoping.
func NodeIntentIncludesService(node *nodeState, serviceName string) bool {
	if node == nil {
		return false
	}
	// If no intent resolved yet, allow all (backward compat).
	if node.ResolvedIntent == nil {
		return true
	}
	canon := normalizeComponentName(canonicalServiceName(serviceName))
	for _, name := range node.ResolvedIntent.ResolvedComponents {
		if name == canon {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

type resolvedEntry struct {
	component *Component
	via       string
}

func profilesOverlap(componentProfiles, nodeProfiles []string) bool {
	for _, cp := range componentProfiles {
		for _, np := range nodeProfiles {
			if cp == np {
				return true
			}
		}
	}
	return false
}

func capSatisfiedBy(cap Capability, selected map[string]*resolvedEntry) bool {
	for _, entry := range selected {
		for _, provided := range entry.component.ProvidesCapabilities {
			if provided == cap {
				return true
			}
		}
	}
	return false
}

func buildHealthySet(units []unitStatusRecord) map[string]bool {
	healthy := make(map[string]bool)
	for _, u := range units {
		if strings.ToLower(u.State) == "active" {
			healthy[strings.ToLower(u.Name)] = true
		}
	}
	return healthy
}

func checkRuntimeDeps(c *Component, healthyUnits map[string]bool) []string {
	var missing []string
	for _, dep := range c.RuntimeLocalDependencies {
		depComp := CatalogByName(dep)
		if depComp == nil {
			continue
		}
		if !healthyUnits[strings.ToLower(depComp.Unit)] {
			missing = append(missing, dep)
		}
	}
	return missing
}

func componentNames(comps []*Component) []string {
	names := make([]string, len(comps))
	for i, c := range comps {
		names[i] = c.Name
	}
	return names
}

// normalizeComponentName converts service names like "ai_memory" to catalog
// canonical form "ai-memory".
func normalizeComponentName(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "-")
}

// ---------------------------------------------------------------------------
// Day 1 phase computation
// ---------------------------------------------------------------------------

// ComputeDay1Phase derives the Day 1 lifecycle phase for a node by inspecting
// its bootstrap phase, resolved intent, and unit health. Returns the phase
// and a human-readable reason string.
func ComputeDay1Phase(node *nodeState) (Day1Phase, string) {
	if node == nil {
		return Day1Joined, "node state is nil"
	}

	// Map bootstrap phases to Day 1 phases.
	switch node.BootstrapPhase {
	case BootstrapAdmitted:
		return Day1Joined, "node admitted, awaiting identity setup"
	case BootstrapInfraPreparing:
		return Day1IdentityReady, "infra packages being installed"
	case BootstrapEtcdJoining:
		return Day1IdentityReady, "etcd join in progress"
	case BootstrapEtcdReady:
		return Day1ClusterConfigSynced, "etcd joined, cluster config available"
	case BootstrapXdsReady:
		return Day1ClusterConfigSynced, "xDS connected, awaiting envoy"
	case BootstrapEnvoyReady:
		return Day1ClusterConfigSynced, "envoy ready, awaiting workload readiness"
	case BootstrapFailed:
		return Day1InfraBlocked, fmt.Sprintf("bootstrap failed: %s", node.BootstrapError)
	}

	// From here, bootstrap is done (BootstrapNone or BootstrapWorkloadReady or BootstrapStorageJoining).
	intent := node.ResolvedIntent
	if intent == nil {
		return Day1ProfileResolved, "profiles not yet resolved"
	}

	// Profile resolved — check infra convergence.
	if len(intent.DesiredInfraNames) > 0 {
		healthyUnits := buildHealthySet(node.Units)
		allInfraHealthy := true
		allInfraInstalled := true
		var unhealthyInfra []string
		for _, c := range intent.DesiredInfra {
			if !healthyUnits[strings.ToLower(c.Unit)] {
				allInfraHealthy = false
				unhealthyInfra = append(unhealthyInfra, c.Name)
				// Check if unit exists at all (installed but not healthy vs not installed).
				found := false
				for _, u := range node.Units {
					if strings.EqualFold(u.Name, c.Unit) {
						found = true
						break
					}
				}
				if !found {
					allInfraInstalled = false
				}
			}
		}
		if !allInfraInstalled {
			return Day1InfraPlanned, fmt.Sprintf("infra not installed: %s", strings.Join(unhealthyInfra, ", "))
		}
		if !allInfraHealthy {
			return Day1InfraInstalled, fmt.Sprintf("infra not healthy: %s", strings.Join(unhealthyInfra, ", "))
		}
	}

	// Infra is healthy — check workload convergence.
	if len(intent.BlockedWorkloads) > 0 {
		names := make([]string, len(intent.BlockedWorkloads))
		for i, bw := range intent.BlockedWorkloads {
			names[i] = bw.Name
		}
		return Day1WorkloadBlocked, fmt.Sprintf("blocked workloads: %s", strings.Join(names, ", "))
	}

	// Check if all desired workloads are installed.
	if len(intent.DesiredWorkloadNames) > 0 {
		healthyUnits := buildHealthySet(node.Units)
		var notReady []string
		for _, c := range intent.DesiredWorkloads {
			if c.Kind == KindCommand {
				// CLI tools have no systemd unit — readiness is determined by
				// presence in InstalledVersions (version-marker written by node agent).
				if node.InstalledVersions[c.Name] == "" {
					notReady = append(notReady, c.Name)
				}
				continue
			}
			if !healthyUnits[strings.ToLower(c.Unit)] {
				notReady = append(notReady, c.Name)
			}
		}
		if len(notReady) > 0 {
			return Day1WorkloadsPlanned, fmt.Sprintf("workloads not ready: %s", strings.Join(notReady, ", "))
		}
	}

	return Day1Ready, "all infra healthy, all workloads converged"
}
