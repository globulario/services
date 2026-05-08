package manual_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/extractors/manual"
)

func TestWalkUnindexedFindsUnknownKeys(t *testing.T) {
	dir := t.TempDir()

	// Known graph type — must NOT appear in results.
	writeYAML(t, dir, "inv.yaml", "invariants:\n  - id: x\n    title: t\n    severity: critical\n    status: active\n")
	// Config-only type — must NOT appear (intentionally excluded).
	writeYAML(t, dir, "aliases.yaml", "aliases:\n  foo:\n    - bar\n")
	// Another config-only type in a subdirectory — must NOT appear.
	sub := filepath.Join(dir, "knowledge")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, sub, "config.yaml", "trust:\n  strict_verified: 40\n")
	// Truly unknown type — must appear.
	writeYAML(t, dir, "mystery.yaml", "unknown_future_type:\n  - id: x\n")

	files, err := manual.WalkUnindexed(dir)
	if err != nil {
		t.Fatalf("WalkUnindexed: %v", err)
	}

	byKey := make(map[string]string) // topKey → path
	for _, f := range files {
		byKey[f.TopKey] = f.Path
	}

	if _, ok := byKey["aliases"]; ok {
		t.Error("aliases: is config-only and must NOT appear in unindexed list")
	}
	if _, ok := byKey["trust"]; ok {
		t.Error("trust: is config-only and must NOT appear in unindexed list")
	}
	if _, ok := byKey["invariants"]; ok {
		t.Error("invariants: is a known graph type and must not appear in unindexed list")
	}
	if _, ok := byKey["unknown_future_type"]; !ok {
		t.Error("expected unknown_future_type to be reported as unindexed — truly unknown keys must surface")
	}
}

func TestWalkUnindexedMissingDirReturnsEmpty(t *testing.T) {
	files, err := manual.WalkUnindexed("/nonexistent/docs/awareness")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestWalkUnindexedReturnsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "knowledge")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, sub, "weights.yaml", "unknown_future_key:\n  verified: 30\n")

	files, err := manual.WalkUnindexed(dir)
	if err != nil {
		t.Fatalf("WalkUnindexed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if filepath.IsAbs(files[0].Path) {
		t.Errorf("path should be relative, got %q", files[0].Path)
	}
}
