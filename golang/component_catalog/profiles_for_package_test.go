package component_catalog

import (
	"reflect"
	"testing"
)

// TestProfilesForPackage pins the package→required-profiles inverse used by the
// cluster-doctor orphaned-install finding. It must agree with ProfilePackages
// (which the controller-side consistency test keeps aligned with the catalog).
func TestProfilesForPackage(t *testing.T) {
	cases := []struct {
		name string
		want []string
	}{
		// torrent is a media-server workload — the orphan case on a
		// control-plane/core/storage node that lacks media-server.
		{"torrent", []string{"media-server"}},
		// mcp is a control-plane service.
		{"mcp", []string{"control-plane"}},
		// gateway participates in both control-plane and gateway profiles.
		{"gateway", []string{"control-plane", "gateway"}},
		// case/whitespace insensitive.
		{"  Torrent  ", []string{"media-server"}},
		// unknown package → empty (DISTINCT from a profile orphan).
		{"does-not-exist", nil},
		{"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ProfilesForPackage(tc.name)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ProfilesForPackage(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestProfilesForPackage_InverseOfPackagesForProfiles checks the two views are
// mutually consistent: a package required by profile P must appear in
// PackagesForProfiles([P]), and vice versa.
func TestProfilesForPackage_InverseConsistency(t *testing.T) {
	for profile, pkgs := range ProfilePackages {
		for _, pkg := range pkgs {
			profs := ProfilesForPackage(pkg)
			found := false
			for _, p := range profs {
				if p == profile {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ProfilesForPackage(%q) = %v does not contain profile %q that claims it", pkg, profs, profile)
			}
		}
	}
}
