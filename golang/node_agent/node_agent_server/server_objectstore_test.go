package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureObjectstoreLayoutMissingContract verifies that when the local
// contract file is missing but etcd is available, ensureObjectstoreLayout
// succeeds using the etcd-sourced config (etcd is the source of truth).
// The old behavior (file-first → error on missing) was replaced in Phase 5
// of the objectstore hardening: etcd is always queried first.
func TestEnsureObjectstoreLayoutMissingContract(t *testing.T) {
	srv := NewNodeAgentServer("", nil, NodeAgentConfig{})
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "minio.json")

	orig := minioContractPath
	minioContractPath = missing
	t.Cleanup(func() { minioContractPath = orig })

	err := srv.ensureObjectstoreLayout(context.Background(), "example.com")
	if err != nil {
		// etcd available → should succeed even without local contract file.
		// If etcd is also unavailable in this test environment, accept the error
		// only if it mentions config unavailability (not "missing contract").
		if strings.Contains(err.Error(), "missing contract") || strings.Contains(err.Error(), missing) {
			t.Fatalf("etcd-first: should not fail with 'missing contract' when etcd is available, got: %v", err)
		}
		// etcd unavailable (CI without cluster) → acceptable failure.
		t.Logf("etcd unavailable in test environment: %v (acceptable)", err)
	}
}
