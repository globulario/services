package main

import (
	"fmt"
	"sort"
	"strings"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

var coreUnits = []string{
	"globular-etcd.service",
	"globular-dns.service",
	"globular-discovery.service",
	"globular-event.service",
	"globular-rbac.service",
	"globular-file.service",
	"globular-minio.service",
}

var profileUnitMap = map[string][]string{
	"core":    coreUnits,
	"compute": coreUnits,
	"control-plane": {
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
	},
	"gateway": {
		"globular-gateway.service",
		"envoy.service",
	},
	"storage": {
		"globular-minio.service",
		"globular-file.service",
	},
	"dns": {
		"globular-dns.service",
	},
}

// allManagedUnits is the complete set of units the controller ever manages,
// computed from all profileUnitMap entries. Used to detect removed units.
var allManagedUnits []string

func init() {
	seen := make(map[string]struct{})
	for _, units := range profileUnitMap {
		for _, u := range units {
			seen[strings.ToLower(u)] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for u := range seen {
		result = append(result, u)
	}
	sort.Strings(result)
	allManagedUnits = result
}

// buildPlanActions computes the ordered list of unit actions for the given profiles.
// Returns an error if any profile is unknown.
//
// Action ordering:
//  1. stop  (removed units, reverse priority — highest priority last)
//  2. disable (removed units, reverse priority)
//  3. enable  (desired units, forward priority — lowest first)
//  4. start   (desired units, forward priority)
func buildPlanActions(profiles []string) ([]*clustercontrollerpb.UnitAction, error) {
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

	var actions []*clustercontrollerpb.UnitAction

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
			&clustercontrollerpb.UnitAction{UnitName: u, Action: "stop"},
			&clustercontrollerpb.UnitAction{UnitName: u, Action: "disable"},
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
			&clustercontrollerpb.UnitAction{UnitName: u, Action: "enable"},
			&clustercontrollerpb.UnitAction{UnitName: u, Action: "start"},
		)
	}

	return actions, nil
}

var unitPriority = map[string]int{
	"globular-etcd.service":      1,
	"etcd.service":               1,
	"globular-dns.service":       2,
	"dns.service":                2,
	"globular-discovery.service": 3,
	"discovery.service":          3,
	"globular-event.service":     4,
	"event.service":              4,
	"globular-rbac.service":      5,
	"rbac.service":               5,
	"globular-minio.service":     6,
	"minio.service":              6,
	"globular-file.service":      7,
	"file.service":               7,
	"globular-gateway.service":   8,
	"globular-xds.service":       8,
	"xds.service":                8,
	"envoy.service":              9,
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
func desiredUnitsFromActions(actions []*clustercontrollerpb.UnitAction) []string {
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

