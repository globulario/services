package main

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestArtifactKeyWithBuild_Exact(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "globular",
		Name:        "echo",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
	}
	key := artifactKeyWithBuild(ref, 7)
	want := "globular%echo%1.2.3%linux_amd64%7"
	if key != want {
		t.Errorf("artifactKeyWithBuild = %q, want %q", key, want)
	}
}

func TestArtifactKeyWithBuild_DifferentBuilds(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "globular",
		Name:        "echo",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
	}
	key1 := artifactKeyWithBuild(ref, 1)
	key2 := artifactKeyWithBuild(ref, 2)
	if key1 == key2 {
		t.Errorf("two builds of same version should produce different keys: %q == %q", key1, key2)
	}
}

func TestArtifactKeyLegacy_NoBuildNumber(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "globular",
		Name:        "echo",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
	}
	key := artifactKeyLegacy(ref)
	want := "globular%echo%1.2.3%linux_amd64"
	if key != want {
		t.Errorf("artifactKeyLegacy = %q, want %q", key, want)
	}
}

func TestArtifactKeyWithBuild_ZeroBuild(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "acme",
		Name:        "gateway",
		Version:     "2.0.0",
		Platform:    "linux_arm64",
	}
	key := artifactKeyWithBuild(ref, 0)
	want := "acme%gateway%2.0.0%linux_arm64%0"
	if key != want {
		t.Errorf("artifactKeyWithBuild(0) = %q, want %q", key, want)
	}
	// Legacy key should differ from build=0 key.
	legacy := artifactKeyLegacy(ref)
	if key == legacy {
		t.Error("build=0 key should differ from legacy key (legacy has no trailing %0)")
	}
}
