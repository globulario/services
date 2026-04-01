package main

import (
	"fmt"
	"sort"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// NodeUnitPlan is a lightweight replacement for the deleted cluster_controllerpb.NodePlan.
// It describes which units should be running on a node based on its profiles.
type NodeUnitPlan struct {
	NodeId         string
	Profiles       []string
	UnitActions    []*cluster_controllerpb.UnitAction
	RenderedConfig map[string]string
}

func (p *NodeUnitPlan) GetUnitActions() []*cluster_controllerpb.UnitAction {
	if p == nil {
		return nil
	}
	return p.UnitActions
}

func (p *NodeUnitPlan) GetRenderedConfig() map[string]string {
	if p == nil {
		return nil
	}
	return p.RenderedConfig
}

// profileUnitMap maps profile name → infrastructure systemd units.
// Populated by component_catalog.go init() from the canonical catalog.
var profileUnitMap map[string][]string

// allManagedUnits is the complete set of units the controller ever manages,
// computed from all profileUnitMap entries. Used to detect removed units.
// Populated by component_catalog.go init().
var allManagedUnits []string

// buildPlanActions computes the ordered list of unit actions for the given profiles.
// Returns an error if any profile is unknown.
//
// Action ordering:
//  1. stop  (removed units, reverse priority — highest priority last)
//  2. disable (removed units, reverse priority)
//  3. enable  (desired units, forward priority — lowest first)
//  4. start   (desired units, forward priority)
func buildPlanActions(profiles []string) ([]*cluster_controllerpb.UnitAction, error) {
	normalized := normalizeProfiles(profiles)
	if len(normalized) == 0 {
		normalized = []string{"core"}
	}

	// Compute desired unit set and collect errors for unknown profiles.
	desiredSet := make(map[string]struct{})
	var errs []string
	for _, profile := range normalized {
		units, ok := profileUnitMap[profile]
		if !ok {
			errs = append(errs, fmt.Sprintf("unknown profile: %q", profile))
			continue
		}
		for _, u := range units {
			desiredSet[strings.ToLower(u)] = struct{}{}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	// Desired units slice (for ordering).
	desiredUnits := make([]string, 0, len(desiredSet))
	for u := range desiredSet {
		desiredUnits = append(desiredUnits, u)
	}

	// Removed units = allManagedUnits − desiredUnits.
	removedUnits := make([]string, 0)
	for _, u := range allManagedUnits {
		if _, wanted := desiredSet[u]; !wanted {
			removedUnits = append(removedUnits, u)
		}
	}

	var actions []*cluster_controllerpb.UnitAction

	// Phase A: stop then disable for removed units (reverse priority order).
	// Highest-priority units (lowest priority number) are stopped last.
	sort.Slice(removedUnits, func(i, j int) bool {
		pi := getUnitPriority(removedUnits[i])
		pj := getUnitPriority(removedUnits[j])
		if pi != pj {
			return pi > pj // reverse: higher number = lower priority = stopped first
		}
		return removedUnits[i] > removedUnits[j]
	})
	for _, u := range removedUnits {
		actions = append(actions,
			&cluster_controllerpb.UnitAction{UnitName: u, Action: "stop"},
			&cluster_controllerpb.UnitAction{UnitName: u, Action: "disable"},
		)
	}

	// Phase B: enable then start for desired units (forward priority order).
	sort.Slice(desiredUnits, func(i, j int) bool {
		pi := getUnitPriority(desiredUnits[i])
		pj := getUnitPriority(desiredUnits[j])
		if pi != pj {
			return pi < pj
		}
		return desiredUnits[i] < desiredUnits[j]
	})
	for _, u := range desiredUnits {
		actions = append(actions,
			&cluster_controllerpb.UnitAction{UnitName: u, Action: "enable"},
			&cluster_controllerpb.UnitAction{UnitName: u, Action: "start"},
		)
	}

	return actions, nil
}

// unitPriority maps systemd unit names to their start priority.
// Lower number = higher priority = starts first / stops last.
// Populated by component_catalog.go init() from the canonical catalog.
var unitPriority map[string]int

// ServiceTier classifies units for phased bootstrap.
type ServiceTier int

const (
	TierBootstrap      ServiceTier = 0 // node-agent only (not managed here)
	TierInfrastructure ServiceTier = 1 // must be running before workloads
	TierWorkload       ServiceTier = 2 // normal application services
)

// unitTier maps systemd unit names to their service tier.
// Units not listed default to TierWorkload.
// Populated by component_catalog.go init() from the canonical catalog.
var unitTier map[string]ServiceTier

// getUnitTier returns the tier for a unit, defaulting to TierWorkload.
func getUnitTier(unit string) ServiceTier {
	if t, ok := unitTier[strings.ToLower(unit)]; ok {
		return t
	}
	return TierWorkload
}

// filterActionsByMaxTier returns only actions whose unit is at or below maxTier.
// This lets the reconciler restrict plan dispatch to infrastructure-only during bootstrap.
func filterActionsByMaxTier(actions []*cluster_controllerpb.UnitAction, maxTier ServiceTier) []*cluster_controllerpb.UnitAction {
	var filtered []*cluster_controllerpb.UnitAction
	for _, a := range actions {
		if getUnitTier(a.UnitName) <= maxTier {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// getUnitPriority returns the priority for a unit, defaulting to 1000 for unknown units.
// Lower number = higher priority = starts first / stops last.
// Unknown units get lowest priority (1000) so they don't accidentally jump the queue.
func getUnitPriority(unit string) int {
	if p, ok := unitPriority[strings.ToLower(unit)]; ok {
		return p
	}
	return 1000
}

// desiredUnitsFromActions extracts the distinct set of unit names that have enable or start actions.
// These are the units the controller wants running on the node.
func desiredUnitsFromActions(actions []*cluster_controllerpb.UnitAction) []string {
	seen := make(map[string]struct{}, len(actions))
	var result []string
	for _, a := range actions {
		if a.Action == "enable" || a.Action == "start" {
			name := strings.ToLower(a.UnitName)
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				result = append(result, a.UnitName)
			}
		}
	}
	return result
}

// missingInstalledUnits returns the desired units that are explicitly reported as "unknown"
// by the node agent. systemd returns "unknown" when no unit file exists on disk for that name.
//
// A unit that is absent from the reported slice is NOT counted as missing — the node agent
// may not yet report a full unit-file inventory, only currently-running units. We only gate
// when the node agent has probed a unit and confirmed it is absent ("unknown" state).
//
// Returns nil if the units slice is empty (node hasn't reported yet — skip the check).
func missingInstalledUnits(desired []string, units []unitStatusRecord) []string {
	if len(units) == 0 {
		return nil
	}
	// Build a map: unit name → known state for units that have been explicitly probed.
	probed := make(map[string]string, len(units))
	for _, u := range units {
		probed[strings.ToLower(u.Name)] = strings.ToLower(u.State)
	}
	var missing []string
	for _, unit := range desired {
		state, known := probed[strings.ToLower(unit)]
		// Only block if the unit was probed AND returned "unknown" (no unit file on disk).
		if known && state == "unknown" {
			missing = append(missing, unit)
		}
	}
	return missing
}

