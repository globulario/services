package main

import "testing"

// Regression for the scylla-manager auto-registration bug: detectScyllaClusters
// emits synthetic "native:<name>" AND "scylla_host:<ip>" markers for a ScyllaDB
// that is reachable but NOT registered. The registration path counted
// "scylla_host:" as a managed (registered) cluster, so it believed registration
// was already done, skipped `sctool cluster add`, and left the cluster
// unregistered forever (doctor scylla_manager.cluster_registered, cluster_count=0).
func TestIsRegisteredScyllaCluster(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"globular.internal", true},          // a real registered cluster name
		{"native:globular.internal", false},  // synthetic native-detection marker
		{"scylla_host:10.0.0.63", false},      // synthetic host marker — the bug
	}
	for _, tc := range cases {
		if got := isRegisteredScyllaCluster(tc.name); got != tc.want {
			t.Errorf("isRegisteredScyllaCluster(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}

	// The exact reachable-but-unregistered shape must count as ZERO registered
	// clusters, so the caller proceeds to `sctool cluster add`.
	reachableUnregistered := []string{"native:globular.internal", "scylla_host:10.0.0.63"}
	managed := 0
	for _, c := range reachableUnregistered {
		if isRegisteredScyllaCluster(c) {
			managed++
		}
	}
	if managed != 0 {
		t.Fatalf("reachable-but-unregistered ScyllaDB must count as 0 registered clusters, got %d — registration would be skipped", managed)
	}
}
