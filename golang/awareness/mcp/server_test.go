package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
	mcpaware "github.com/globulario/services/golang/awareness/mcp"
)

// requiredTools is the full list of tools the spec mandates.
var requiredTools = []string{
	"awareness.preflight",
	"awareness.agent_context",
	"awareness.impact_file",
	"awareness.did_we_fix",
	"awareness.pattern_status",
	"awareness.runtime_snapshot",
	"awareness.validate_package",
	"awareness.package_context",
	"awareness.propose_from_incident",
	"awareness.validate_proposal",
	"awareness.approve_proposal",
	"awareness.fix_status",
	// Phase 11: graph integrity and trust management.
	"awareness.graph_integrity_check",
	"awareness.impact_path",
}

// forbiddenTools must never be exposed over MCP.
var forbiddenTools = []string{
	"awareness.promote_proposal",
	"awareness.promote-proposal",
}

func makeTestServer(t *testing.T) (*mcpaware.Server, string) {
	t.Helper()
	docsDir := setupTestDocsDir(t)
	g := setupTestGraph(t)
	s := mcpaware.NewWithGraph(mcpaware.Config{DocsDir: docsDir}, g)
	t.Cleanup(func() { s.Close() })
	return s, docsDir
}

func TestToolRegistryContainsAllRequiredTools(t *testing.T) {
	s, _ := makeTestServer(t)
	names := s.ToolNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range requiredTools {
		if !nameSet[want] {
			t.Errorf("missing required tool: %q", want)
		}
	}
}

func TestPromoteProposalIsNotExposedAsMCPTool(t *testing.T) {
	s, _ := makeTestServer(t)
	for _, forbidden := range forbiddenTools {
		if s.HasTool(forbidden) {
			t.Errorf("forbidden tool %q must not be registered as MCP tool", forbidden)
		}
	}
}

func TestServerDegradesMissingGraphDB(t *testing.T) {
	docsDir := setupTestDocsDir(t)
	// Server with no graph (nil) — should not panic or error on New.
	s := mcpaware.NewWithGraph(mcpaware.Config{DocsDir: docsDir}, nil)
	defer s.Close()

	result, err := s.CallTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch",
	})
	if err != nil {
		t.Fatalf("preflight should degrade gracefully, got error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	// warnings must mention missing graph
	if warnings, ok := m["warnings"].([]interface{}); !ok || len(warnings) == 0 {
		t.Error("expected warnings in degraded preflight result")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func setupTestDocsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "context_aliases.yaml"), []byte(`aliases:
  infra.desired_hash_consistency:
    - desired_hash
    - checksum mismatch
`), 0o644)

	_ = os.WriteFile(filepath.Join(dir, "fix_cases.yaml"), []byte(`fix_cases:
  - id: desired_hash_consistency
    title: "Desired hash fix"
    status: PARTIAL
    pattern: "desired_hash"
    target_invariants:
      - infra.desired_hash_consistency
    remaining_files:
      - golang/awareness/analysis/hash.go
    required_tests:
      - TestDriftWorkflowUsesDesiredHash
`), 0o644)

	_ = os.WriteFile(filepath.Join(dir, "guardrails.yaml"), []byte("guardrails: []\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "proposals"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "incidents"), 0o755)
	return dir
}

func setupTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	_ = g.UpsertInvariant(ctx, graph.Invariant{
		ID: "infra.desired_hash_consistency", Title: "Desired hash must be stable",
		Severity: "critical", Status: "active",
	})
	_ = g.AddNode(ctx, graph.Node{
		ID: "invariant:infra.desired_hash_consistency", Type: graph.NodeTypeInvariant,
		Name: "infra.desired_hash_consistency",
	})
	_ = g.AddNode(ctx, graph.Node{
		ID: "forbidden_fix:use_raw_digest", Type: graph.NodeTypeForbiddenFix,
		Name: "use raw artifact digest as desired_hash",
	})
	_ = g.AddEdge(ctx, graph.Edge{
		Src: "invariant:infra.desired_hash_consistency", Kind: graph.EdgeForbids,
		Dst: "forbidden_fix:use_raw_digest",
	})
	for _, svc := range []string{"envoy", "cluster-controller", "node-agent"} {
		_ = g.AddNode(ctx, graph.Node{ID: "service:" + svc, Type: graph.NodeTypeGlobularService, Name: svc})
	}
	return g
}
