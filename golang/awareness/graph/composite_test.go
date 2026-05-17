package graph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

// Regression tests for the writable-runtime-alongside-immutable-bundle
// architecture.
//
// OpenComposite opens a graph that reads static knowledge from a bundle
// directory (read-only) and writes mutable runtime data to a separate
// runtime directory.

func TestOpenComposite_ReadsBundleAndWritesRuntime(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundle")
	runtimeDir := filepath.Join(dir, "runtime")

	// Seed a bundle with a node, then close it.
	b, err := graph.Open(bundleDir)
	if err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	ctx := context.Background()
	if err := b.AddNode(ctx, graph.Node{
		ID:   "bundle-n1",
		Type: graph.NodeTypeGlobularService,
		Name: "from-bundle",
	}); err != nil {
		t.Fatalf("seed bundle node: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("seed bundle close: %v", err)
	}

	// Open composite: writable runtime + read-only bundle.
	g, err := graph.OpenComposite(bundleDir, runtimeDir)
	if err != nil {
		t.Fatalf("OpenComposite: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	// Bundle data must be readable through the composite handle.
	n, err := g.FindNode(ctx, "bundle-n1")
	if err != nil {
		t.Fatalf("FindNode bundle-n1: %v", err)
	}
	if n == nil {
		t.Fatal("expected bundle-n1 to be visible through composite")
	}
	if n.Name != "from-bundle" {
		t.Errorf("bundle node name = %q, want %q", n.Name, "from-bundle")
	}

	// Writing to a runtime-mutable table succeeds.
	if err := g.InsertPreflightAudit(ctx, graph.PreflightAuditRecord{
		ID:   "pflt-1",
		Task: "composite-write-test",
	}); err != nil {
		t.Fatalf("runtime write: %v", err)
	}
	results, err := g.QueryPreflightAudits(ctx, 0, "")
	if err != nil {
		t.Fatalf("read after runtime write: %v", err)
	}
	if len(results) != 1 || results[0].ID != "pflt-1" {
		t.Errorf("preflight audit not visible after write: %v", results)
	}
}

func TestOpenComposite_RefusesWritesToBundleData(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundle")
	runtimeDir := filepath.Join(dir, "runtime")

	// Create empty bundle.
	b, err := graph.Open(bundleDir)
	if err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	b.Close()

	g, err := graph.OpenComposite(bundleDir, runtimeDir)
	if err != nil {
		t.Fatalf("OpenComposite: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	// AddNode must fail because the composite graph is read-only for static data.
	err = g.AddNode(context.Background(), graph.Node{
		ID:   "illegal",
		Type: graph.NodeTypeGlobularService,
		Name: "x",
	})
	if err == nil {
		t.Fatal("writing a bundle-only node via composite graph must fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "read-only") &&
		!strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Errorf("error should mention read-only; got: %v", err)
	}
}

func TestOpenComposite_PersistsRuntimeDataAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundle")
	runtimeDir := filepath.Join(dir, "runtime")

	// Create empty bundle.
	b, err := graph.Open(bundleDir)
	if err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	b.Close()

	ctx := context.Background()

	// First open: write a runtime incident record.
	g1, err := graph.OpenComposite(bundleDir, runtimeDir)
	if err != nil {
		t.Fatalf("OpenComposite #1: %v", err)
	}
	if err := g1.UpsertIncident(ctx, graph.IncidentRecord{
		ID:    "persist1",
		Title: "test incident",
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	g1.Close()

	// Re-open: record must still be there.
	g2, err := graph.OpenComposite(bundleDir, runtimeDir)
	if err != nil {
		t.Fatalf("OpenComposite #2: %v", err)
	}
	t.Cleanup(func() { g2.Close() })

	inc, err := g2.FindIncident(ctx, "persist1")
	if err != nil {
		t.Fatalf("read after reopen: %v", err)
	}
	if inc == nil {
		t.Fatal("incident record not visible after reopen")
	}
	if inc.Title != "test incident" {
		t.Errorf("incident title = %q, want %q", inc.Title, "test incident")
	}
}

func TestOpenComposite_CreatesRuntimeParentDir(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundle")
	if b, err := graph.Open(bundleDir); err != nil {
		t.Fatalf("seed bundle: %v", err)
	} else {
		b.Close()
	}

	// Runtime directory under a path that doesn't yet exist — must be auto-created.
	nested := filepath.Join(dir, "deep", "nested", "runtime")
	g, err := graph.OpenComposite(bundleDir, nested)
	if err != nil {
		t.Fatalf("OpenComposite: %v", err)
	}
	defer g.Close()

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("runtime dir not created at %s: %v", nested, err)
	}
}

func TestOpenComposite_RejectsMissingBundle(t *testing.T) {
	dir := t.TempDir()
	_, err := graph.OpenComposite(
		filepath.Join(dir, "no-such-bundle"),
		filepath.Join(dir, "runtime"),
	)
	if err == nil {
		t.Fatal("OpenComposite with missing bundle must error")
	}
	if !strings.Contains(err.Error(), "stat bundle") &&
		!strings.Contains(err.Error(), "bundle") {
		t.Errorf("error should mention bundle; got: %v", err)
	}
}

func TestOpenComposite_RejectsEmptyPaths(t *testing.T) {
	cases := []struct {
		name        string
		bundlePath  string
		runtimePath string
		wantSubstr  string
	}{
		{"empty bundle", "", "/tmp/r", "bundlePath is empty"},
		{"empty runtime", "/tmp/b", "", "runtimePath is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := graph.OpenComposite(tc.bundlePath, tc.runtimePath)
			if err == nil {
				t.Fatal("expected error for empty path")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error should contain %q; got: %v", tc.wantSubstr, err)
			}
		})
	}
}

func TestOpenComposite_BundleNodesVisibleRuntimeMutable(t *testing.T) {
	// Verifies that bundle-static and runtime-mutable data coexist correctly.
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundle")
	runtimeDir := filepath.Join(dir, "runtime")

	ctx := context.Background()

	// Seed bundle with nodes, edges, and invariants.
	b, err := graph.Open(bundleDir)
	if err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	for _, node := range []graph.Node{
		{ID: "service:controller", Type: graph.NodeTypeGlobularService, Name: "controller"},
		{ID: "service:node-agent", Type: graph.NodeTypeGlobularService, Name: "node-agent"},
	} {
		_ = b.AddNode(ctx, node)
	}
	_ = b.AddEdge(ctx, graph.Edge{
		Src:  "service:controller",
		Kind: graph.EdgeDependsOn,
		Dst:  "service:node-agent",
	})
	_ = b.UpsertInvariant(ctx, graph.Invariant{ID: "inv.test", Title: "Test Invariant"})
	b.Close()

	g, err := graph.OpenComposite(bundleDir, runtimeDir)
	if err != nil {
		t.Fatalf("OpenComposite: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	// Bundle nodes are visible.
	nodes, err := g.FindNodesByType(ctx, graph.NodeTypeGlobularService)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 bundle nodes, got %d", len(nodes))
	}

	// Bundle edges are visible.
	edges, err := g.EdgesByKind(ctx, graph.EdgeDependsOn)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 bundle edge, got %d", len(edges))
	}

	// Bundle invariants are visible.
	invs, err := g.AllInvariants(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 {
		t.Errorf("expected 1 bundle invariant, got %d", len(invs))
	}

	// Runtime writes work (preflight audit, incident, proposal).
	if err := g.UpsertIncident(ctx, graph.IncidentRecord{
		ID: "inc-composite-test", Title: "composite test",
	}); err != nil {
		t.Fatalf("UpsertIncident: %v", err)
	}
	inc, err := g.FindIncident(ctx, "inc-composite-test")
	if err != nil || inc == nil {
		t.Fatalf("FindIncident: err=%v, inc=%v", err, inc)
	}
}
