package main

// minio_independence_test.go — acceptance tests proving the repository does not
// depend on MinIO for correctness. Packages live ONLY in the local POSIX CAS;
// the repository has no MinIO dependency at all.
//
// These tests validate:
//   A. RequireHealthy() only blocks on ScyllaDB — never on storage.
//   B. Publish cannot create PUBLISHED metadata with missing local blob.

import (
	"context"
	"sync/atomic"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Test A: RequireHealthy() gates only on ScyllaDB ──────────────────────────

// TestDepHealth_ScyllaOKAllowsRPCs: ScyllaDB OK → RequireHealthy must return nil.
func TestDepHealth_ScyllaOKAllowsRPCs(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(true) // ScyllaDB is OK

	w := &depHealthWatchdog{healthy: healthy}

	if err := w.RequireHealthy(); err != nil {
		t.Fatalf("RequireHealthy() must return nil when ScyllaDB is OK, got: %v", err)
	}
}

// TestDepHealth_ScyllaDownBlocksRPCs: ScyllaDB down → RequireHealthy() MUST return an error.
func TestDepHealth_ScyllaDownBlocksRPCs(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(false) // ScyllaDB is DOWN

	w := &depHealthWatchdog{healthy: healthy}

	err := w.RequireHealthy()
	if err == nil {
		t.Fatal("RequireHealthy() must return an error when ScyllaDB is down")
	}
	// Error must mention ScyllaDB metadata, never storage.
	msg := err.Error()
	if !containsSubstring(msg, "ScyllaDB") && !containsSubstring(msg, "scylladb") && !containsSubstring(msg, "metadata") {
		t.Errorf("error should mention ScyllaDB, got: %s", msg)
	}
}

func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Test B: publish cannot create PUBLISHED metadata with missing local blob ──

// TestPublish_MissingLocalBlobBlocksPromote verifies that promoteToPublished
// refuses to advance when the local blob is missing.
func TestPublish_MissingLocalBlobBlocksPromote(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Seed a minimal manifest (no binary written to storage).
	m := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "test-publisher",
			Name:        "test-pkg",
			Platform:    "linux_amd64",
			Version:     "1.0.0",
		},
		BuildNumber: 42,
		SizeBytes:   1024,
	}
	key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())

	// Pre-seed to LEDGER_WRITTEN (the state just before PUBLISHED).
	srv.cacheArtifactState(key, artifactStateRecord{State: PipelineLedgerWritten})

	// Do NOT write the binary blob — it's missing.
	// Attempt to promote to PUBLISHED.
	err := srv.promoteToPublished(ctx, key, m)
	if err == nil {
		t.Fatal("promoteToPublished must return an error when local blob is missing")
	}

	// Verify the artifact did NOT advance to PUBLISHED.
	state := srv.readArtifactState(ctx, key)
	if state == PipelinePublished {
		t.Errorf("artifact must not be PUBLISHED when local blob is missing, got PUBLISHED")
	}
}
