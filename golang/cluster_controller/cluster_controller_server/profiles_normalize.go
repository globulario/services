package main

import (
	"sort"
	"strings"
)

// normalizeProfiles deduplicates, lowercases, trims, and sorts profiles.
// For example: ["Core", " gateway", "core"] → ["core", "gateway"]
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
	}
	sort.Strings(result)
	return result
}
