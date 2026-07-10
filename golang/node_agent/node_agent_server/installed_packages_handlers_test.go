package main

import "testing"

func TestInstalledPackageLookupKindsCoverAllAuthoritativeKinds(t *testing.T) {
	got := map[string]bool{}
	for _, kind := range authoritativeInstalledPackageKinds {
		got[kind] = true
	}
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "APPLICATION", "COMMAND"} {
		if !got[kind] {
			t.Fatalf("installed package omitted-kind lookup must cover %s; got %v", kind, authoritativeInstalledPackageKinds)
		}
	}
}
