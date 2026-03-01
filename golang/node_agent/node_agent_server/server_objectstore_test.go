package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureObjectstoreLayoutMissingContract(t *testing.T) {
	srv := NewNodeAgentServer("", nil)
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "minio.json")
	t.Setenv("NODE_AGENT_MINIO_CONTRACT", missing)

	err := srv.ensureObjectstoreLayout(context.Background(), "example.com")
	if err == nil {
		t.Fatalf("expected error for missing contract")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error should mention contract path, got: %v", err)
	}
}
