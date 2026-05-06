package enforce_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/enforce"
)

// Test: Audit with no errors passes.
func TestAuditPassesOnCleanSource(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "clean.go"), []byte(`package pkg

//globular:enforces infra.hash_consistency
//globular:state_transition DESIRED -> INSTALLED
//globular:tested_by TestHashConsistency
func ComputeHash() {}
`), 0o644)

	ctx := context.Background()
	result := enforce.Audit(ctx, nil, enforce.AuditOptions{
		SrcDir:      dir,
		SkipContracts: true,
		SkipTests:     true,
		SkipDrift:     true,
	})

	if !result.Pass {
		t.Errorf("expected audit to pass, got %d errors: %v", result.ErrorCount, result.Findings)
	}
}

// Test: Audit with a malformed annotation fails.
func TestAuditFailsOnMalformedAnnotation(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad.go"), []byte(`package pkg

//globular:state_transition NODASH
func Foo() {}
`), 0o644)

	ctx := context.Background()
	result := enforce.Audit(ctx, nil, enforce.AuditOptions{
		SrcDir:      dir,
		SkipContracts: true,
		SkipTests:     true,
		SkipDrift:     true,
	})

	if result.Pass {
		t.Error("expected audit to fail due to MALFORMED_STATE_TRANSITION")
	}
	if result.ErrorCount == 0 {
		t.Error("expected at least one ERROR finding")
	}
}

// Test: AuditFiles is annotation-only and does not require a graph.
func TestAuditFilesAnnotationOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.go")
	_ = os.WriteFile(path, []byte(`package pkg

//globular:hash_schema good_schema
//globular:state_transition A -> B
func Good() {}

//globular:tested_by notATestName
func Bad() {}
`), 0o644)

	result := enforce.AuditFiles([]string{path})
	if result.Pass {
		t.Error("expected audit to fail due to bad tested_by")
	}
	found := false
	for _, f := range result.Findings {
		if f.Code == "ANNOTATION_BAD_TEST_NAME" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ANNOTATION_BAD_TEST_NAME, got: %v", result.Findings)
	}
}
