package main

import (
	"strings"
	"testing"
)

// scyllaLocations must yield an object-store location whenever MinIO is
// available, even with only a local destination configured — otherwise the
// scylla provider is judged "unavailable" and ScyllaDB (ai-memory, behavioral
// memory) is silently never backed up.
func TestScyllaLocations_SynthesizesFromMinioWhenOnlyLocalDestination(t *testing.T) {
	srv := &server{
		Destinations:  []DestinationConfig{{Name: "local", Type: "local", Path: "/var/backups/globular", Primary: true}},
		MinioEndpoint: "10.0.0.63:9000",
	}
	got := srv.scyllaLocations()
	want := "s3:" + scyllaBackupBucket
	if len(got) != 1 || got[0] != want {
		t.Fatalf("scyllaLocations() = %v, want [%q]", got, want)
	}
}

func TestScyllaLocations_EmptyWhenNoMinioAndNoObjectStore(t *testing.T) {
	srv := &server{
		Destinations: []DestinationConfig{{Name: "local", Type: "local", Path: "/var/backups/globular", Primary: true}},
		// MinioEndpoint deliberately empty
	}
	if got := srv.scyllaLocations(); len(got) != 0 {
		t.Fatalf("scyllaLocations() = %v, want empty (no object store available)", got)
	}
}

// An explicitly-configured object-store destination must win over the
// synthesized MinIO fallback.
func TestScyllaLocations_PrefersExplicitObjectStoreDestination(t *testing.T) {
	srv := &server{
		Destinations: []DestinationConfig{
			{Name: "local", Type: "local", Path: "/var/backups/globular", Primary: true},
			{Name: "offsite", Type: "s3", Path: "my-bucket/cluster-01"},
		},
		MinioEndpoint: "10.0.0.63:9000",
	}
	got := srv.scyllaLocations()
	if len(got) != 1 || got[0] != "s3:my-bucket/cluster-01" {
		t.Fatalf("scyllaLocations() = %v, want [s3:my-bucket/cluster-01]", got)
	}
}

// coverageImpact must name ai-memory for the scylla provider so a skipped
// ScyllaDB backup is unmistakable in the logs.
func TestCoverageImpact_ScyllaMentionsAiMemory(t *testing.T) {
	impact := coverageImpact("scylla")
	if !strings.Contains(strings.ToLower(impact), "ai-memory") {
		t.Errorf("coverageImpact(scylla) = %q, expected it to name ai-memory", impact)
	}
}
