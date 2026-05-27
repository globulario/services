package component_catalog

import (
	"sort"
	"testing"
)

func TestProfilePackages_NonEmpty(t *testing.T) {
	if len(ProfilePackages) == 0 {
		t.Fatal("ProfilePackages must not be empty")
	}
	for profile, pkgs := range ProfilePackages {
		if len(pkgs) == 0 {
			t.Errorf("profile %q has no packages — a profile with no services is not a profile", profile)
		}
	}
}

func TestProfilePackages_Sorted(t *testing.T) {
	for profile, pkgs := range ProfilePackages {
		sorted := append([]string(nil), pkgs...)
		sort.Strings(sorted)
		for i := range pkgs {
			if pkgs[i] != sorted[i] {
				t.Errorf("profile %q packages not sorted (review profilemap.go)", profile)
				break
			}
		}
	}
}

func TestProfilePackages_NoDuplicates(t *testing.T) {
	for profile, pkgs := range ProfilePackages {
		seen := make(map[string]bool)
		for _, p := range pkgs {
			if seen[p] {
				t.Errorf("profile %q has duplicate package %q", profile, p)
			}
			seen[p] = true
		}
	}
}

func TestProfileNames_AlphabeticalAndComplete(t *testing.T) {
	got := ProfileNames()
	want := make([]string, 0, len(ProfilePackages))
	for k := range ProfilePackages {
		want = append(want, k)
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("ProfileNames length mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ProfileNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPackagesForProfiles_Empty(t *testing.T) {
	if got := PackagesForProfiles(nil); len(got) != 0 {
		t.Errorf("nil profiles → expected empty, got %v", got)
	}
	if got := PackagesForProfiles([]string{}); len(got) != 0 {
		t.Errorf("empty profiles → expected empty, got %v", got)
	}
	if got := PackagesForProfiles([]string{"", " ", "\t"}); len(got) != 0 {
		t.Errorf("whitespace-only profiles → expected empty, got %v", got)
	}
}

func TestPackagesForProfiles_UnknownProfile(t *testing.T) {
	if got := PackagesForProfiles([]string{"nonexistent-profile"}); len(got) != 0 {
		t.Errorf("unknown profile → expected empty, got %v", got)
	}
}

func TestPackagesForProfiles_Single(t *testing.T) {
	got := PackagesForProfiles([]string{"gateway"})
	want := []string{"envoy", "gateway", "keepalived", "xds"}
	if !equalSorted(got, want) {
		t.Errorf("gateway profile: got %v, want %v", got, want)
	}
}

func TestPackagesForProfiles_Union(t *testing.T) {
	got := PackagesForProfiles([]string{"gateway", "dns"})
	// gateway: envoy, gateway, keepalived, xds
	// dns:     dns
	// union:   dns, envoy, gateway, keepalived, xds
	want := []string{"dns", "envoy", "gateway", "keepalived", "xds"}
	if !equalSorted(got, want) {
		t.Errorf("union [gateway, dns]: got %v, want %v", got, want)
	}
}

func TestPackagesForProfiles_Dedup(t *testing.T) {
	// gateway and control-plane both contain envoy, gateway, xds.
	// The union must deduplicate.
	got := PackagesForProfiles([]string{"gateway", "control-plane"})
	seen := make(map[string]int)
	for _, p := range got {
		seen[p]++
	}
	for p, count := range seen {
		if count != 1 {
			t.Errorf("package %q appears %d times in union (must be 1)", p, count)
		}
	}
}

func TestPackagesForProfiles_CaseInsensitive(t *testing.T) {
	a := PackagesForProfiles([]string{"GATEWAY"})
	b := PackagesForProfiles([]string{"gateway"})
	if !equalSorted(a, b) {
		t.Errorf("case-insensitive lookup failed: GATEWAY=%v gateway=%v", a, b)
	}
}

func TestHasProfile(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"core", true},
		{"control-plane", true},
		{"  storage  ", true}, // trimmed
		{"GATEWAY", true},     // case-insensitive
		{"unknown", false},
		{"", false},
	}
	for _, c := range cases {
		if got := HasProfile(c.in); got != c.want {
			t.Errorf("HasProfile(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeProfiles_ExpandsInheritanceAndCanonicalizes(t *testing.T) {
	got := NormalizeProfiles([]string{" Control-Plane ", "gateway", "GATEWAY"})
	want := []string{"control-plane", "core", "gateway"}
	if !equalSorted(got, want) {
		t.Fatalf("NormalizeProfiles mismatch: got %v want %v", got, want)
	}
}

func TestUnknownProfiles(t *testing.T) {
	got := UnknownProfiles([]string{"core", "unknown", "  ", "UNKNOWN"})
	want := []string{"unknown"}
	if !equalSorted(got, want) {
		t.Fatalf("UnknownProfiles mismatch: got %v want %v", got, want)
	}
}

func equalSorted(a, b []string) bool {
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
