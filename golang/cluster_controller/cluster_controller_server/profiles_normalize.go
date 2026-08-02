package main

import (
	"github.com/globulario/services/golang/component_catalog"
)

// normalizeProfiles deduplicates, lowercases, trims, sorts, and expands
// inherited profiles. For example:
//
//	["control-plane", "gateway"] → ["control-plane", "core", "gateway"]
func normalizeProfiles(raw []string) []string {
	return component_catalog.NormalizeProfiles(raw)
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
