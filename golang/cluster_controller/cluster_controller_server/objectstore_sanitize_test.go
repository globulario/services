package main

import "testing"

func mkSanitizeNode(status string, ips ...string) *nodeState {
	n := &nodeState{Status: status}
	n.Identity.Ips = ips
	return n
}

// TestSanitizeMinioPoolNodes verifies stale-peer removal: pool IPs that belong to
// no eligible node (or to a removed/unreachable/blocked node) are dropped, order
// is preserved, and duplicates are collapsed.
func TestSanitizeMinioPoolNodes(t *testing.T) {
	nodes := map[string]*nodeState{
		"a": mkSanitizeNode("active", "10.0.0.1"),
		"b": mkSanitizeNode("removed", "10.0.0.2"),     // ineligible node → its IP is stale
		"c": mkSanitizeNode("active", "10.0.0.3"),      // eligible but not in pool
		"d": mkSanitizeNode("unreachable", "10.0.0.4"), // ineligible
	}
	// .2 = removed-node IP, .9 = belongs to no node, .1 duplicated.
	pool := []string{"10.0.0.1", "10.0.0.2", "10.0.0.9", "10.0.0.1"}

	after, removed := sanitizeMinioPoolNodes(pool, nodes)

	if got, want := after, []string{"10.0.0.1"}; !eqStrings(got, want) {
		t.Errorf("after = %v, want %v", got, want)
	}
	if got, want := removed, []string{"10.0.0.2", "10.0.0.9"}; !eqStrings(got, want) {
		t.Errorf("removed = %v, want %v", got, want)
	}
}

// TestSanitizeMinioPoolNodes_EmptyNodesDoesNotPrune proves the absence guard
// (meta.absence_scope_must_be_explicit): an empty node set is "unknown", not
// "every peer is stale", so a live pool is only de-duplicated, never wiped.
func TestSanitizeMinioPoolNodes_EmptyNodesDoesNotPrune(t *testing.T) {
	pool := []string{"10.0.0.1", "10.0.0.1", "not-an-ip", "10.0.0.2"}

	after, removed := sanitizeMinioPoolNodes(pool, nil)

	if got, want := after, []string{"10.0.0.1", "10.0.0.2"}; !eqStrings(got, want) {
		t.Errorf("after = %v, want %v (valid IPs kept, deduped)", got, want)
	}
	if got, want := removed, []string{"not-an-ip"}; !eqStrings(got, want) {
		t.Errorf("removed = %v, want %v (only the invalid entry)", got, want)
	}
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
