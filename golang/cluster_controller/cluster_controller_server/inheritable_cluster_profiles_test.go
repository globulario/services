package main

import (
	"sort"
	"testing"
)

// TestInheritableClusterProfiles verifies that a joining node inherits the
// cluster's real, assignable catalog profiles while excluding:
//   - hardware-gated profiles (control-plane, storage, gateway),
//   - opt-in workloads (media-server),
//   - non-catalog / derived labels (e.g. "ai").
func TestInheritableClusterProfiles(t *testing.T) {
	nodes := map[string]*nodeState{
		"founder": {Profiles: []string{
			"core", "control-plane", "storage", // founding trio (control-plane/storage hardware-gated)
			"ai",           // derived label, NOT a catalog profile
			"media-server", // opt-in workload
			"dns",          // real catalog software profile -> inherit
		}},
		"peer": {Profiles: []string{
			"core", "compute", // compute is a real catalog profile -> inherit
			"gateway", // hardware-gated -> not inherited
		}},
		"nil-guard": nil,
	}

	got := inheritableClusterProfiles(nodes)
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}

	wantPresent := []string{"core", "dns", "compute"}
	for _, p := range wantPresent {
		if !set[p] {
			sort.Strings(got)
			t.Errorf("expected profile %q to be inherited; got %v", p, got)
		}
	}

	wantAbsent := []string{"ai", "media-server", "control-plane", "storage", "gateway"}
	for _, p := range wantAbsent {
		if set[p] {
			sort.Strings(got)
			t.Errorf("profile %q must NOT be inherited; got %v", p, got)
		}
	}
}

// TestInheritableClusterProfilesEmpty verifies an empty cluster inherits nothing
// (no panic, no spurious profiles).
func TestInheritableClusterProfilesEmpty(t *testing.T) {
	if got := inheritableClusterProfiles(map[string]*nodeState{}); len(got) != 0 {
		t.Errorf("empty cluster must inherit no profiles; got %v", got)
	}
	if got := inheritableClusterProfiles(nil); len(got) != 0 {
		t.Errorf("nil node map must inherit no profiles; got %v", got)
	}
}
