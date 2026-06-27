package main

// registry_artifact_kind_test.go — Slice 4a of the package-classification
// single-source migration. The publish path used to hardcode Kind=SERVICE for
// every uploaded package, which is why the read-time inferCorrectKind correction
// existed. registryArtifactKind stamps the registry-authoritative kind at WRITE
// time (publish + sync), so the stored manifest kind is correct at the source.
// (The read-time correction is retained in 4a as a legacy net; removed in 4b.)

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestRegistryArtifactKind(t *testing.T) {
	cases := []struct {
		name     string
		fallback repopb.ArtifactKind
		want     repopb.ArtifactKind
	}{
		// Infrastructure — previously published as SERVICE (the bug); now stamped correctly.
		{"xds", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_INFRASTRUCTURE},
		{"gateway", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_INFRASTRUCTURE},
		{"etcd", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_INFRASTRUCTURE},
		{"scylladb", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_INFRASTRUCTURE},
		// Commands — previously SERVICE; now COMMAND.
		{"mc", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_COMMAND},
		{"yt-dlp", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_COMMAND},
		// Services — stay SERVICE.
		{"dns", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_SERVICE},
		{"mcp", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_SERVICE},
		// Unknown / third-party — fall open to the caller's fallback.
		{"acme-thirdparty", repopb.ArtifactKind_SERVICE, repopb.ArtifactKind_SERVICE},
		{"acme-thirdparty", repopb.ArtifactKind_APPLICATION, repopb.ArtifactKind_APPLICATION},
	}
	for _, c := range cases {
		got := registryArtifactKind(c.name, c.fallback)
		if got != c.want {
			t.Errorf("registryArtifactKind(%q, fallback=%v) = %v, want %v", c.name, c.fallback, got, c.want)
		}
	}
}

// TestPublishStampsRegistryKind documents the publish before/after: the same
// inputs that previously yielded SERVICE (the handlers.go hardcode) now yield the
// registry-authoritative kind for infra/command packages.
func TestPublishStampsRegistryKind(t *testing.T) {
	// Mirrors handlers.go: Kind: registryArtifactKind(d.Name, ArtifactKind_SERVICE).
	if k := registryArtifactKind("xds", repopb.ArtifactKind_SERVICE); k != repopb.ArtifactKind_INFRASTRUCTURE {
		t.Errorf("publish of xds must now stamp INFRASTRUCTURE, got %v (was hardcoded SERVICE)", k)
	}
	if k := registryArtifactKind("mc", repopb.ArtifactKind_SERVICE); k != repopb.ArtifactKind_COMMAND {
		t.Errorf("publish of mc must now stamp COMMAND, got %v (was hardcoded SERVICE)", k)
	}
	if k := registryArtifactKind("dns", repopb.ArtifactKind_SERVICE); k != repopb.ArtifactKind_SERVICE {
		t.Errorf("publish of dns must remain SERVICE, got %v", k)
	}
}
