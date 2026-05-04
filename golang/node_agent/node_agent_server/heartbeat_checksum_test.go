package main

import (
	"testing"

	"github.com/globulario/services/golang/repository/repositorypb"
)

func TestRuntimeChecksumFromManifest_PrefersEntrypoint(t *testing.T) {
	m := &repositorypb.ArtifactManifest{
		Checksum:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EntrypointChecksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	got := runtimeChecksumFromManifest(m)
	want := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got != want {
		t.Fatalf("runtimeChecksumFromManifest() = %q, want %q", got, want)
	}
}

func TestRuntimeChecksumFromManifest_FallsBackToArchive(t *testing.T) {
	m := &repositorypb.ArtifactManifest{
		Checksum: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	got := runtimeChecksumFromManifest(m)
	want := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if got != want {
		t.Fatalf("runtimeChecksumFromManifest() = %q, want %q", got, want)
	}
}

