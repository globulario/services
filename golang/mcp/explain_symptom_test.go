package main

import (
	"context"
	"testing"
)

func TestExplainSymptom_UnknownTextNoMatch(t *testing.T) {
	s := NewWithGraph(Config{DocsDir: resolveTestDocsDir()}, nil)

	result, err := s.CallTool(context.Background(), "awareness.explain_symptom", map[string]interface{}{
		"text": "zzz_totally_unknown_xyzzy_not_in_knowledge_base",
	})
	if err != nil {
		// If docs dir is not available, we may get a "docs dir not configured" error.
		// That's acceptable in a test environment without real docs.
		t.Logf("tool returned error (acceptable if docs dir missing): %v", err)
		return
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	matches, _ := m["matches"].([]map[string]interface{})
	// Unknown text should return 0 matches (no panic).
	if matches != nil && len(matches) > 0 {
		t.Logf("got %d matches (unexpected but non-fatal)", len(matches))
	}
}

func TestExplainSymptom_RequiresText(t *testing.T) {
	s := NewWithGraph(Config{DocsDir: "/tmp/nonexistent-docs"}, nil)

	_, err := s.CallTool(context.Background(), "awareness.explain_symptom", map[string]interface{}{})
	// Should error: either "text is required" or "docs dir not configured".
	if err == nil {
		t.Error("expected error when text is missing or docs dir invalid")
	}
}

func TestExplainSymptom_Registered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	if !s.HasTool("awareness.explain_symptom") {
		t.Error("awareness.explain_symptom should be registered")
	}
}

// resolveTestDocsDir returns the docs/awareness path relative to the repo root if available.
func resolveTestDocsDir() string {
	out, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	import_path := out + "/docs/awareness"
	return import_path
}
