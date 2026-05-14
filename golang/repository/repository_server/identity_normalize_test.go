package main

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestNormalizePlatform(t *testing.T) {
	// Canonical form is the underscore form ("linux_amd64") so the function's
	// output can be substituted directly into storage keys and prefix
	// lookups. See identity_normalize.go for the diagnosis that pinned this
	// direction (the prior slash form broke joined sync lookups).
	cases := map[string]string{
		"linux_amd64":  "linux_amd64",
		"linux-amd64":  "linux_amd64",
		"linux/amd64":  "linux_amd64",
		" Linux\\ARM ": "linux_arm",
		"linux//amd64": "linux_amd64",
		"_linux_amd64": "linux_amd64",
		"linux_amd64_": "linux_amd64",
	}
	for in, want := range cases {
		if got := NormalizePlatform(in); got != want {
			t.Fatalf("NormalizePlatform(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestNormalizeChecksum(t *testing.T) {
	raw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if got := NormalizeChecksum(raw); got != "sha256:"+raw {
		t.Fatalf("NormalizeChecksum raw=%q", got)
	}
	withPrefix := "sha256:" + raw
	if got := NormalizeChecksum(withPrefix); got != withPrefix {
		t.Fatalf("NormalizeChecksum prefixed=%q", got)
	}
}

func TestValidateBuildID(t *testing.T) {
	if err := ValidateBuildID("1234"); err == nil {
		t.Fatal("expected numeric-only build_id to fail")
	}
	if err := ValidateBuildID("ab"); err == nil {
		t.Fatal("expected too-short build_id to fail")
	}
	if err := ValidateBuildID("build-01"); err != nil {
		t.Fatalf("expected valid build_id, got %v", err)
	}
}

func TestCanonicalArtifactKeyAndLegacyAliasKey(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "globular",
		Name:        "dns",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
	}
	if got := CanonicalArtifactKey(ref, "01JABCDEF0123456789"); got != "globular%dns%1.2.3%linux_amd64%01JABCDEF0123456789" {
		t.Fatalf("CanonicalArtifactKey=%q", got)
	}
	if got := CanonicalArtifactStorageKeyByBuildNumber(ref, 7); got != "globular%dns%1.2.3%linux_amd64%7" {
		t.Fatalf("CanonicalArtifactStorageKeyByBuildNumber=%q", got)
	}
	if got := LegacyArtifactIdentityKey(ref); got != "globular%dns%1.2.3%linux_amd64" {
		t.Fatalf("LegacyArtifactIdentityKey=%q", got)
	}
	if got := LegacyBuildAliasKey(ref, 171, "v1.2.44"); got != "globular%dns%1.2.3%linux_amd64%v1.2.44%171" {
		t.Fatalf("LegacyBuildAliasKey=%q", got)
	}
}
