package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureObjectstoreLayoutMissingContract(t *testing.T) {
	srv := NewNodeAgentServer("", nil, NodeAgentConfig{})
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "minio.json")

	// Redirect contract path via package-level variable (same pattern as ActionBinDir).
	orig := minioContractPath
	minioContractPath = missing
	t.Cleanup(func() { minioContractPath = orig })

	err := srv.ensureObjectstoreLayout(context.Background(), "example.com")
	if err == nil {
		t.Fatalf("expected error for missing contract")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error should mention contract path, got: %v", err)
	}
}
