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
		"agent_context_cache", "graph_builds", "preflight_audits",
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

// TestCodeSmellsForInvariants verifies that pattern nodes linked to invariants
// via requires edges surface their code_smells when queried by invariant ID.
func TestCodeSmellsForInvariants(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	// Invariant node.
	_ = g.AddNode(ctx, graph.Node{
		ID:   "invariant:inv.test",
		Type: graph.NodeTypeInvariant,
		Name: "inv.test",
	})

	// Pattern node with code_smells in metadata.
	_ = g.AddNode(ctx, graph.Node{
		ID:   "pattern:test_pattern",
		Type: graph.NodeTypePattern,
		Name: "test_pattern",
		Metadata: map[string]any{
			"code_smells": []any{"foo_bad_thing", "bar_also_bad"},
		},
	})

	// requires edge: pattern → invariant.
	_ = g.AddEdge(ctx, graph.Edge{
		Src:  "pattern:test_pattern",
		Kind: graph.EdgeRequires,
		Dst:  "invariant:inv.test",
	})

	smells, err := g.CodeSmellsForInvariants(ctx, []string{"invariant:inv.test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(smells) != 2 {
		t.Errorf("want 2 code smells, got %d: %v", len(smells), smells)
	}
	// Results should be sorted.
	if smells[0] != "bar_also_bad" || smells[1] != "foo_bad_thing" {
		t.Errorf("unexpected order: %v", smells)
	}
}

func TestCodeSmellsForInvariantsEmpty(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	smells, err := g.CodeSmellsForInvariants(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if smells != nil {
		t.Errorf("expected nil for empty input, got %v", smells)
	}
}

func TestCodeSmellsDeduplication(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	for _, id := range []string{"invariant:a", "invariant:b"} {
		_ = g.AddNode(ctx, graph.Node{ID: id, Type: graph.NodeTypeInvariant, Name: id})
	}

	// Two pattern nodes sharing a code smell both pointing at invariant:a.
	for i, pid := range []string{"pattern:p1", "pattern:p2"} {
		extraSmell := "smell_" + string(rune('a'+i))
		_ = g.AddNode(ctx, graph.Node{
			ID:   pid,
			Type: graph.NodeTypePattern,
			Name: pid,
			Metadata: map[string]any{
				"code_smells": []any{"shared_smell", extraSmell},
			},
		})
		_ = g.AddEdge(ctx, graph.Edge{Src: pid, Kind: graph.EdgeRequires, Dst: "invariant:a"})
	}

	smells, err := g.CodeSmellsForInvariants(ctx, []string{"invariant:a"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, s := range smells {
		seen[s]++
	}
	if seen["shared_smell"] != 1 {
		t.Errorf("shared_smell should appear exactly once, count=%d", seen["shared_smell"])
	}
}

func TestInsertAndQueryPreflightAudit(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	rec := graph.PreflightAuditRecord{
		ID:             "audit-001",
		Task:           "fix desired_hash drift",
		GitSHA:         "abc123",
		Files:          []string{"golang/cluster_controller/convergence.go"},
		ForbiddenFixes: []string{"use_raw_digest"},
		Invariants:     []string{"infra.desired_hash_consistency"},
		CodeSmells:     []string{"raw_artifact_digest_as_desired_hash"},
	}

	if err := g.InsertPreflightAudit(ctx, rec); err != nil {
		t.Fatalf("InsertPreflightAudit: %v", err)
	}

	results, err := g.QueryPreflightAudits(ctx, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 audit record, got %d", len(results))
	}

	got := results[0]
	if got.ID != "audit-001" {
		t.Errorf("ID: got %s, want audit-001", got.ID)
	}
	if got.Task != "fix desired_hash drift" {
		t.Errorf("Task: got %s, want 'fix desired_hash drift'", got.Task)
	}
	if got.GitSHA != "abc123" {
		t.Errorf("GitSHA: got %s, want abc123", got.GitSHA)
	}
	if len(got.Files) != 1 || got.Files[0] != "golang/cluster_controller/convergence.go" {
		t.Errorf("Files: got %v", got.Files)
	}
	if len(got.ForbiddenFixes) != 1 || got.ForbiddenFixes[0] != "use_raw_digest" {
		t.Errorf("ForbiddenFixes: got %v", got.ForbiddenFixes)
	}
	if len(got.CodeSmells) != 1 || got.CodeSmells[0] != "raw_artifact_digest_as_desired_hash" {
		t.Errorf("CodeSmells: got %v", got.CodeSmells)
	}
}

func TestQueryPreflightAuditsBySHA(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()

	for i, sha := range []string{"sha-aaa", "sha-bbb", "sha-aaa"} {
		_ = g.InsertPreflightAudit(ctx, graph.PreflightAuditRecord{
			ID:     "audit-" + string(rune('a'+i)),
			Task:   "task",
			GitSHA: sha,
		})
	}

	results, err := g.QueryPreflightAudits(ctx, 0, "sha-aaa")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 records for sha-aaa, got %d", len(results))
	}

	all, err := g.QueryPreflightAudits(ctx, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("want 3 records total, got %d", len(all))
	}
}

