package main

import (
	"reflect"
	"testing"
)

// TestClassifyProfileHygiene pins the publish-time drift classifier against the
// REAL component catalog. It mirrors the Step-0 drift survey: torrent/mcp
// over-broad, mcp/gateway/xds null→missing, dns under-broad, repository match,
// unknown→skip. WARN-first: classification only — no blocking, no mutation.
func TestClassifyProfileHygiene(t *testing.T) {
	cases := []struct {
		desc     string
		pkg      string
		manifest []string
		want     profileHygieneStatus
	}{
		// over-broad: manifest declares more profiles than the catalog claims.
		// torrent catalog is [media-server]; [media-server compute] is a strict superset.
		{"torrent over-broad", "torrent", []string{"media-server", "compute"}, profileHygieneOverBroad},
		{"mcp old over-broad", "mcp", []string{"core", "compute", "control-plane"}, profileHygieneOverBroad},
		// missing: manifest empty/null but catalog has profiles.
		{"mcp null manifest", "mcp", nil, profileHygieneMissing},
		{"gateway null manifest", "gateway", nil, profileHygieneMissing},
		{"xds null manifest", "xds", []string{}, profileHygieneMissing},
		// match: manifest equals catalog.
		{"repository matches", "repository", []string{"core", "compute"}, profileHygieneOK},
		// under-broad: manifest omits a profile the catalog claims (dns profile).
		{"dns under-broad", "dns", []string{"core", "compute", "control-plane"}, profileHygieneUnderBroad},
		// mismatch: disjoint — neither subset (torrent catalog is [media-server]).
		{"disjoint mismatch", "torrent", []string{"gateway", "control-plane"}, profileHygieneMismatch},
		// skip: unknown to the catalog — distinct condition, never a profile drift.
		{"unknown package skipped", "totally-unknown-pkg", []string{"core"}, profileHygieneSkipUnknown},
		{"unknown package empty skipped", "totally-unknown-pkg", nil, profileHygieneSkipUnknown},
		// case/whitespace insensitivity.
		{"case-insensitive match", "repository", []string{"  Core ", "COMPUTE"}, profileHygieneOK},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got, _ := classifyProfileHygiene(tc.pkg, tc.manifest)
			if got != tc.want {
				_, cat := classifyProfileHygiene(tc.pkg, tc.manifest)
				t.Errorf("classifyProfileHygiene(%q, %v) = %q, want %q (catalog=%v)", tc.pkg, tc.manifest, got, tc.want, cat)
			}
		})
	}
}

// TestClassifyProfileHygiene_ReadOnly confirms the classifier does not mutate
// its input — manifest profiles stay informational and untouched (no derive,
// no placement side effect).
func TestClassifyProfileHygiene_ReadOnly(t *testing.T) {
	in := []string{"core", "compute"}
	orig := append([]string(nil), in...)
	_, _ = classifyProfileHygiene("torrent", in)
	if !reflect.DeepEqual(in, orig) {
		t.Errorf("classifyProfileHygiene mutated its input: got %v, want %v", in, orig)
	}
}
