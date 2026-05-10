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

func TestWalkUnindexedExcludesExternallyHandledGraphKeys(t *testing.T) {
	dir := t.TempDir()

	// failuregraph_seeds/*.yaml — top key `id:`, loaded by failurelearning.RebuildFromSeeds.
	seeds := filepath.Join(dir, "failuregraph_seeds")
	if err := os.MkdirAll(seeds, 0o755); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, seeds, "cat.yaml", "id: ERRCAT-test\ntype: ErrorCategory\nname: test\nseverity: warning\nsummary: t\n")

	// detector_mapping.yaml — top key `detector_mappings:`, loaded by doctor mapping extractor.
	writeYAML(t, dir, "detector_mapping.yaml", "detector_mappings:\n  - id: dm.test\n    pattern: t\n")

	files, err := manual.WalkUnindexed(dir)
	if err != nil {
		t.Fatalf("WalkUnindexed: %v", err)
	}

	for _, f := range files {
		if f.TopKey == "id" {
			t.Errorf("id: keys (failuregraph_seeds) are loaded by another extractor and must NOT appear in unindexed list, got %q", f.Path)
		}
		if f.TopKey == "detector_mappings" {
			t.Errorf("detector_mappings: keys are loaded by another extractor and must NOT appear in unindexed list, got %q", f.Path)
		}
	}
}

func TestClassifyYAMLByTopKeyConsistencyWithWalkUnindexed(t *testing.T) {
	// Any key classified as Graph or Config must NOT surface as unindexed.
	// This guards against drift between the two checks.
	dir := t.TempDir()
	cases := map[string]string{
		"graph_invariants.yaml":    "invariants:\n  - id: x\n    title: t\n    severity: critical\n    status: active\n",
		"graph_external.yaml":      "id: ERRCAT-x\ntype: ErrorCategory\nname: x\nseverity: warning\nsummary: t\n",
		"graph_detector.yaml":      "detector_mappings:\n  - id: dm.x\n    pattern: x\n",
		"config_aliases.yaml":      "aliases:\n  foo:\n    - bar\n",
		"config_contracts.yaml":    "version: \"1\"\nschema: test/v1\ncontracts:\n  - id: c.x\n    summary: y\n",
		"unknown_truly_unknown.yaml": "not_a_recognized_top_key:\n  - x\n",
	}
	for name, body := range cases {
		writeYAML(t, dir, name, body)
	}

	files, err := manual.WalkUnindexed(dir)
	if err != nil {
		t.Fatalf("WalkUnindexed: %v", err)
	}
	got := make(map[string]bool)
	for _, f := range files {
		got[f.TopKey] = true
	}
	if !got["not_a_recognized_top_key"] {
		t.Error("truly unknown top key must surface in unindexed list")
	}
	for _, k := range []string{"invariants", "id", "detector_mappings", "aliases", "version"} {
		if got[k] {
			t.Errorf("key %q is recognized (graph or config) and must NOT appear in unindexed list", k)
		}
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
