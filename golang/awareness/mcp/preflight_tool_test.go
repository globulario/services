package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpaware "github.com/globulario/services/golang/awareness/mcp"
)

func TestPreflightReturnsValidJSON(t *testing.T) {
	s, _ := makeTestServer(t)

	result, err := s.CallTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch after deploy",
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b)
	}

	for _, key := range []string{"task", "classification", "invariants", "failure_modes", "forbidden_fixes", "did_we_fix"} {
		if _, ok := m[key]; !ok {
			t.Errorf("preflight JSON missing key %q", key)
		}
	}
}

func TestPreflightDegradesMissingGraph(t *testing.T) {
	docsDir := setupTestDocsDir(t)
	// No graph.
	s, _ := makeTestServerNoGraph(t, docsDir)

	result, err := s.CallTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch",
	})
	if err != nil {
		t.Fatalf("expected degraded result, not error: %v", err)
	}

	b, _ := json.Marshal(result)
	if !strings.Contains(string(b), "no graph DB") {
		t.Errorf("degraded preflight must include 'no graph DB' warning, got: %s", b)
	}
}

func TestPreflightWithIncludeRuntimeIncludesRuntimeSection(t *testing.T) {
	s, _ := makeTestServer(t)

	result, err := s.CallTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task":            "desired_hash mismatch",
		"include_runtime": true,
		"runtime_window":  "5m",
	})
	if err != nil {
		t.Fatalf("preflight with runtime: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	// runtime section should be present when include_runtime is true.
	if _, ok := m["runtime"]; !ok {
		t.Error("expected 'runtime' section when include_runtime=true")
	}
}

func TestPreflightClassifiesDesiredHashAsMismatch(t *testing.T) {
	s, _ := makeTestServer(t)

	result, err := s.CallTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch between controller and node-agent",
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, _ := json.Marshal(result)
	// Should contain STATE_MISMATCH in classification.
	if !strings.Contains(string(b), "STATE_MISMATCH") {
		t.Errorf("expected STATE_MISMATCH classification for desired_hash task, got: %s", b)
	}
}

func makeTestServerNoGraph(t *testing.T, docsDir string) (*mcpaware.Server, string) {
	t.Helper()
	s := mcpaware.NewWithGraph(mcpaware.Config{DocsDir: docsDir}, nil)
	t.Cleanup(func() { s.Close() })
	return s, docsDir
}
