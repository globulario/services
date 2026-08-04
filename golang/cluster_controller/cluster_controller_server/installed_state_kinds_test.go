package main

import "testing"

func TestAuthoritativeInstalledPackageKindsCoverAllInstalledStateKinds(t *testing.T) {
	got := map[string]bool{}
	for _, kind := range authoritativeInstalledPackageKinds {
		got[kind] = true
	}
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "APPLICATION", "COMMAND"} {
		if !got[kind] {
			t.Fatalf("authoritative installed-state scans must cover %s; got %v", kind, authoritativeInstalledPackageKinds)
		}
	}
}
