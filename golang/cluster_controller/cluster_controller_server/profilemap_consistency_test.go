package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	cc "github.com/globulario/services/golang/component_catalog"
)

// TestProfileMap_ConsistentWithCatalog enforces the architectural
// invariant that the shared profile→packages map (component_catalog
// package, used by node-agent at Day-0) matches the controller's rich
// catalog. Any drift between the two means the node-agent installs a
// different set than the controller expects, which is exactly the
// Day-0 bypass bug we're fixing.
//
// On failure, the test prints the corrected map so the maintainer can
// paste it into profilemap.go.
func TestProfileMap_ConsistentWithCatalog(t *testing.T) {
	// Build expected: for every profile in ProfileCapabilities, gather
	// every component whose Profiles list contains that profile. Include
	// all kinds (Infrastructure, Workload, Command) — Day-0 installs
	// them all.
	expected := make(map[string][]string)
	for profile := range ProfileCapabilities {
		var pkgs []string
		for _, c := range catalog {
			for _, p := range c.Profiles {
				if p == profile {
					pkgs = append(pkgs, c.Name)
					break
				}
			}
		}
		sort.Strings(pkgs)
		expected[profile] = pkgs
	}

	// Compare key sets first.
	expectedKeys := make([]string, 0, len(expected))
	for k := range expected {
		expectedKeys = append(expectedKeys, k)
	}
	sort.Strings(expectedKeys)

	actualKeys := cc.ProfileNames()

	if !equalStringSlices(expectedKeys, actualKeys) {
		t.Errorf("profile name set mismatch:\n  expected: %v\n  shared:   %v", expectedKeys, actualKeys)
	}

	// Compare per-profile package lists.
	mismatched := false
	for _, profile := range expectedKeys {
		want := expected[profile]
		got := append([]string(nil), cc.ProfilePackages[profile]...)
		sort.Strings(got)
		if !equalStringSlices(want, got) {
			t.Errorf("profile %q packages drift:\n  catalog:  %v\n  shared:   %v", profile, want, got)
			mismatched = true
		}
	}

	// On any mismatch, dump the corrected map so the maintainer can
	// update profilemap.go in a single paste.
	if mismatched || t.Failed() {
		t.Log(renderProfileMap(expected))
	}
}

// TestPackagesForProfiles_ReturnsCatalogUnion sanity-checks the union
// query against a representative profile combination.
func TestPackagesForProfiles_ReturnsCatalogUnion(t *testing.T) {
	// A founding node has core+control-plane+storage. The shared map
	// should return the union of those three lists, deduplicated, with
	// nothing else.
	got := cc.PackagesForProfiles([]string{"core", "control-plane", "storage"})

	// Build expected from the catalog the same way.
	wantSet := make(map[string]struct{})
	for _, p := range []string{"core", "control-plane", "storage"} {
		for _, c := range catalog {
			for _, cp := range c.Profiles {
				if cp == p {
					wantSet[c.Name] = struct{}{}
					break
				}
			}
		}
	}
	want := make([]string, 0, len(wantSet))
	for n := range wantSet {
		want = append(want, n)
	}
	sort.Strings(want)

	if !equalStringSlices(want, got) {
		t.Errorf("union mismatch for [core, control-plane, storage]:\n  expected: %v\n  got:      %v", want, got)
	}
}

func equalStringSlices(a, b []string) bool {
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

// renderProfileMap formats the expected map as a Go literal suitable for
// pasting into profilemap.go — emitted on test failure so the fix is
// trivial.
func renderProfileMap(m map[string][]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("\n--- corrected ProfilePackages literal ---\n")
	sb.WriteString("var ProfilePackages = map[string][]string{\n")
	for _, k := range keys {
		fmt.Fprintf(&sb, "\t%q: {\n", k)
		for _, p := range m[k] {
			fmt.Fprintf(&sb, "\t\t%q,\n", p)
		}
		sb.WriteString("\t},\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}
