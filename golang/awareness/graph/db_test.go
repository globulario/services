package graph_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

func openTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	dir := t.TempDir()
	g, err := graph.Open(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

// Test 1: migration creates all required tables and indexes.
func TestMigrationCreatesTablesAndIndexes(t *testing.T) {
	g := openTestGraph(t)
	db := g.DB()
	ctx := context.Background()

	tables := []string{
		"nodes", "edges", "invariants", "failure_modes",
		"agent_context_cache", "graph_builds",
	}
	for _, tbl := range tables {
		var name string
		err := db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", tbl, err)
		}
	}

	indexes := []string{
		"idx_nodes_type", "idx_nodes_name",
		"idx_edges_src", "idx_edges_dst", "idx_edges_kind", "idx_edges_phase",
	}
	for _, idx := range indexes {
		var name string
		err := db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q missing: %v", idx, err)
		}
	}
}

// Test 2: AddNode and AddEdge are idempotent.
func TestAddNodeAndEdgeIdempotent(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	n := graph.Node{
		ID:      "service:node-agent",
		Type:    graph.NodeTypeGlobularService,
		Name:    "node-agent",
		Summary: "Per-node executor",
	}
	for i := 0; i < 3; i++ {
		if err := g.AddNode(ctx, n); err != nil {
			t.Fatalf("AddNode attempt %d: %v", i, err)
		}
	}

	nodes, err := g.FindNodesByType(ctx, graph.NodeTypeGlobularService)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node after 3 upserts, got %d", len(nodes))
	}

	e := graph.Edge{
		Src:      "service:node-agent",
		Kind:     graph.EdgeDependsOn,
		Dst:      "service:repository",
		Phase:    "package_install",
		Required: true,
	}
	// Ensure dst node exists (foreign-key style — SQLite doesn't enforce it, but good practice).
	_ = g.AddNode(ctx, graph.Node{ID: "service:repository", Type: graph.NodeTypeGlobularService, Name: "repository"})

	for i := 0; i < 3; i++ {
		if err := g.AddEdge(ctx, e); err != nil {
			t.Fatalf("AddEdge attempt %d: %v", i, err)
		}
	}

	edges, err := g.EdgesByKind(ctx, graph.EdgeDependsOn)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge after 3 upserts, got %d", len(edges))
	}
	if !edges[0].Required {
		t.Error("expected Required=true")
	}
	if edges[0].Phase != "package_install" {
		t.Errorf("expected phase=package_install, got %q", edges[0].Phase)
	}
}

// Additional: FindNode returns nil for missing ID.
func TestFindNodeMissing(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	node, err := g.FindNode(ctx, "does:not:exist")
	if err != nil {
		t.Fatalf("FindNode: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil, got %+v", node)
	}
}

// Additional: Stats returns correct counts.
func TestStats(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	_ = g.AddNode(ctx, graph.Node{ID: "n1", Type: graph.NodeTypeSymbol, Name: "Foo"})
	_ = g.AddNode(ctx, graph.Node{ID: "n2", Type: graph.NodeTypeSymbol, Name: "Bar"})
	_ = g.AddEdge(ctx, graph.Edge{Src: "n1", Kind: graph.EdgeCalls, Dst: "n2"})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: "inv1", Title: "T1"})
	_ = g.UpsertFailureMode(ctx, graph.FailureMode{ID: "fm1", Title: "FM1"})

	s, err := g.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Nodes != 2 {
		t.Errorf("nodes: got %d, want 2", s.Nodes)
	}
	if s.Edges != 1 {
		t.Errorf("edges: got %d, want 1", s.Edges)
	}
	if s.Invariants != 1 {
		t.Errorf("invariants: got %d, want 1", s.Invariants)
	}
	if s.FailureModes != 1 {
		t.Errorf("failure_modes: got %d, want 1", s.FailureModes)
	}
}

// Additional: Open creates parent directory automatically.
func TestOpenCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "deep", "nested", "graph.db")
	g, err := graph.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	g.Close()
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("db file not created: %v", err)
	}
}

// Additional: second Open on same path re-uses existing schema.
func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "graph.db")
	ctx := context.Background()

	g1, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = g1.AddNode(ctx, graph.Node{ID: "x", Type: graph.NodeTypeSymbol, Name: "X"})
	g1.Close()

	g2, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer g2.Close()

	n, err := g2.FindNode(ctx, "x")
	if err != nil {
		t.Fatal(err)
	}
	if n == nil {
		t.Error("node written by g1 not visible via g2")
	}
}

