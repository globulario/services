package main

import (
	"sort"
	"strings"
)

// profileInheritance defines implicit profile inclusions.
// A control-plane node IS a core node — it needs all core infra and services.
var profileInheritance = map[string][]string{
	"control-plane": {"core"},
	"compute":       {"core"},
}

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
