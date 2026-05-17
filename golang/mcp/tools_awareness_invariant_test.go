package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

func buildInvariantTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

// populateInvariantFixture creates a well-formed invariant graph for testing.
func populateInvariantFixture(t *testing.T, g *graph.Graph, invID string) {
	t.Helper()
	ctx := context.Background()
	nodeID := "invariant:" + invID
	fileID := "source_file:golang/auth/validate.go"

	_ = g.AddNode(ctx, graph.Node{
		ID: nodeID, Type: graph.NodeTypeInvariant, Name: invID,
		Summary: "Token must be validated.",
	})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: invID, Title: "Token validation", Severity: "critical", Status: "active"})

	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: "validate.go", Path: "golang/auth/validate.go"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: nodeID,
		Metadata: map[string]any{"trust_level": "strict_verified"}})

	testID := "test:TestTokenValidated"
	_ = g.AddNode(ctx, graph.Node{ID: testID, Type: graph.NodeTypeTest, Name: "TestTokenValidated"})
	_ = g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeTestedBy, Dst: testID})
	_ = g.AddEdge(ctx, graph.Edge{Src: testID, Kind: graph.EdgeVerifies, Dst: nodeID})

	fixID := "forbidden_fix:skip_validation"
	_ = g.AddNode(ctx, graph.Node{ID: fixID, Type: graph.NodeTypeForbiddenFix, Name: "skip_validation"})
	_ = g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeForbids, Dst: fixID})
	_ = g.AddEdge(ctx, graph.Edge{Src: fixID, Kind: graph.EdgeBlocksForbiddenAction, Dst: nodeID})

	authID := "authority:/globular/auth/keys"
	_ = g.AddNode(ctx, graph.Node{ID: authID, Type: "authority_source", Name: "/globular/auth/keys"})
	_ = g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeReadsAuthority, Dst: authID})

	fmID := "failure_mode:auth.token.bypass"
	_ = g.AddNode(ctx, graph.Node{ID: fmID, Type: graph.NodeTypeFailureMode, Name: "auth.token.bypass"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fmID, Kind: graph.EdgeViolates, Dst: nodeID})
	_ = g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeAffects, Dst: fmID})
}

func TestExplainInvariant_ReturnsImplementations(t *testing.T) {
	g := buildInvariantTestGraph(t)
	invID := "auth.token_validation"
	populateInvariantFixture(t, g, invID)

	result, err := buildExplainInvariant(context.Background(), g, invID)
	if err != nil {
		t.Fatalf("buildExplainInvariant: %v", err)
	}

	impls, _ := result["implementations"].([]map[string]interface{})
	if len(impls) == 0 {
		t.Error("expected at least one implementation")
	}
}

func TestExplainInvariant_ReturnsTests(t *testing.T) {
	g := buildInvariantTestGraph(t)
	invID := "auth.token_validation2"
	populateInvariantFixture(t, g, invID)

	result, err := buildExplainInvariant(context.Background(), g, invID)
	if err != nil {
		t.Fatalf("buildExplainInvariant: %v", err)
	}

	tests, _ := result["tests"].([]map[string]interface{})
	hasVerifies := false
	for _, tv := range tests {
		if tv["edge_kind"] == "verifies" {
			hasVerifies = true
		}
	}
	if !hasVerifies {
		t.Error("expected a test with edge_kind=verifies")
	}
}

func TestExplainInvariant_ReturnsForbiddenActions(t *testing.T) {
	g := buildInvariantTestGraph(t)
	invID := "auth.token_validation3"
	populateInvariantFixture(t, g, invID)

	result, err := buildExplainInvariant(context.Background(), g, invID)
	if err != nil {
		t.Fatalf("buildExplainInvariant: %v", err)
	}

	fixes, _ := result["forbidden_actions"].([]string)
	if len(fixes) == 0 {
		t.Error("expected at least one forbidden action")
	}
}

func TestExplainInvariant_NotFound(t *testing.T) {
	g := buildInvariantTestGraph(t)

	result, err := buildExplainInvariant(context.Background(), g, "does.not.exist")
	if err != nil {
		t.Fatalf("buildExplainInvariant: %v", err)
	}

	if _, hasErr := result["error"]; !hasErr {
		t.Error("expected error key for unknown invariant")
	}
}

func TestExplainInvariant_ReportsGapsWhenEmpty(t *testing.T) {
	g := buildInvariantTestGraph(t)
	ctx := context.Background()
	invID := "empty.invariant"
	nodeID := "invariant:" + invID

	_ = g.AddNode(ctx, graph.Node{ID: nodeID, Type: graph.NodeTypeInvariant, Name: invID, Summary: "Empty."})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: invID, Title: "Empty", Severity: "low", Status: "active"})

	result, err := buildExplainInvariant(ctx, g, invID)
	if err != nil {
		t.Fatalf("buildExplainInvariant: %v", err)
	}

	gaps, _ := result["gaps"].([]string)
	if len(gaps) == 0 {
		t.Error("expected gap findings for invariant with no edges")
	}
}

func TestFileInvariantContext_ReturnsLinkedInvariants(t *testing.T) {
	g := buildInvariantTestGraph(t)
	invID := "auth.token_validation_ctx"
	populateInvariantFixture(t, g, invID)

	result, err := buildFileInvariantContext(context.Background(), g, "golang/auth/validate.go")
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	links, _ := result["invariants"].([]map[string]interface{})
	if len(links) == 0 {
		t.Error("expected at least one invariant linked to validate.go")
	}
}

func TestFileInvariantContext_ReturnsEditWarnings(t *testing.T) {
	g := buildInvariantTestGraph(t)
	invID := "auth.token_validation_warn"
	populateInvariantFixture(t, g, invID)

	result, err := buildFileInvariantContext(context.Background(), g, "golang/auth/validate.go")
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	warnings, _ := result["edit_warnings"].([]string)
	if len(warnings) == 0 {
		t.Error("expected at least one edit warning for file with forbidden_fix-linked invariant")
	}
}

func TestFileInvariantContext_UnknownFile(t *testing.T) {
	g := buildInvariantTestGraph(t)

	result, err := buildFileInvariantContext(context.Background(), g, "golang/nonexistent.go")
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	if result["warning"] == "" {
		t.Error("expected a warning for file not in graph")
	}
	links, _ := result["invariants"].([]interface{})
	if links != nil && len(links) > 0 {
		t.Error("expected empty invariants for unknown file")
	}
}

func TestFileInvariantContext_RequiredTests(t *testing.T) {
	g := buildInvariantTestGraph(t)
	invID := "auth.token_validation_tests"
	populateInvariantFixture(t, g, invID)

	result, err := buildFileInvariantContext(context.Background(), g, "golang/auth/validate.go")
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	tests, _ := result["required_tests"].([]string)
	if len(tests) == 0 {
		t.Error("expected required_tests in file invariant context")
	}
}
