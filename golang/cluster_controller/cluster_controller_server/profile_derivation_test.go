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
