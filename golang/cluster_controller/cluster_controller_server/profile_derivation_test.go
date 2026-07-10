package main

import "testing"

func TestDeriveProfilesFromInstalled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		installed map[string]string
		want      []string
	}{
		{
			name: "media stack derives media-server and core",
			installed: map[string]string{
				"torrent": "1.2.257",
				"ffmpeg":  "7.0.2",
			},
			want: []string{"core", "media-server"},
		},
		{
			name: "qualified media package derives media-server",
			installed: map[string]string{
				"SERVICE/media": "1.2.257",
			},
			want: []string{"core", "media-server"},
		},
		{
			name: "founding stack plus media stays fully classified",
			installed: map[string]string{
				"dns":     "1.2.257",
				"minio":   "2025.1.0",
				"title":   "1.2.257",
				"gateway": "1.2.257",
			},
			want: []string{"control-plane", "core", "gateway", "media-server", "storage"},
		},
		{
			name: "unknown non-empty install falls back to compute",
			installed: map[string]string{
				"something-else": "0.0.1",
			},
			want: []string{"compute"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := deriveProfilesFromInstalled(tc.installed)
			if !sameStrings(got, tc.want) {
				t.Fatalf("deriveProfilesFromInstalled(%v) = %v, want %v", tc.installed, got, tc.want)
			}
		})
	}
}

// TestMergeDefaultProfilesIntoDerived guards the fix for the bootstrap-node
// media-server drop: a fresh node derives only [core,control-plane,storage]
// from its installed core services, and the operator-seeded default_profiles
// (core,media-server) must be merged in so the media stack is authorized and
// installed — otherwise media never installs (chicken-and-egg) and its packages
// orphan. Derived placement intent must survive the merge.
func TestMergeDefaultProfilesIntoDerived(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		derived  []string
		defaults []string
		want     []string
	}{
		{
			name:     "bootstrap node: seeded media-server merged onto derived profiles",
			derived:  []string{"control-plane", "core", "storage"},
			defaults: []string{"core", "media-server"},
			want:     []string{"control-plane", "core", "storage", "media-server"},
		},
		{
			name:     "no default_profiles: derived unchanged",
			derived:  []string{"control-plane", "core", "storage"},
			defaults: nil,
			want:     []string{"control-plane", "core", "storage"},
		},
		{
			name:     "duplicates deduped, derived profiles preserved",
			derived:  []string{"core", "control-plane", "storage"},
			defaults: []string{"core", "storage", "media-server"},
			want:     []string{"core", "control-plane", "storage", "media-server"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mergeDefaultProfilesIntoDerived(tc.derived, tc.defaults)
			if !sameStrings(got, tc.want) {
				t.Fatalf("mergeDefaultProfilesIntoDerived(%v, %v) = %v, want %v",
					tc.derived, tc.defaults, got, tc.want)
			}
			// Derived placement profiles must never be dropped by the merge when
			// present in the derived set.
			for _, q := range []string{"control-plane", "storage"} {
				inDerived := false
				for _, p := range tc.derived {
					if p == q {
						inDerived = true
					}
				}
				if inDerived && !sameStrings(filterHas(got, q), []string{q}) {
					t.Errorf("derived profile %q dropped by merge: got %v", q, got)
				}
			}
		})
	}
}

// filterHas returns [p] if p is in xs, else empty — a tiny helper for the
// placement-preservation assertion above.
func filterHas(xs []string, p string) []string {
	for _, x := range xs {
		if x == p {
			return []string{p}
		}
	}
	return nil
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, v := range got {
		seen[v]++
	}
	for _, v := range want {
		if seen[v] == 0 {
			return false
		}
		seen[v]--
	}
	for _, remaining := range seen {
		if remaining != 0 {
			return false
		}
	}
	return true
}
