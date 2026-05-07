package docs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/extractors/docs"
	"github.com/globulario/services/golang/awareness/graph"
)

func openTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	dir := t.TempDir()
	g, err := graph.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open graph: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestDocExtract_DecisionNodeCreated verifies that a Markdown file with valid
// YAML front matter produces an architecture_decision node in the graph.
func TestDocExtract_DecisionNodeCreated(t *testing.T) {
	g := openTestGraph(t)
	root := t.TempDir()

	writeFile(t, root, "docs/awareness/my-decision.md", `---
id: my_decision
type: architecture_decision
status: accepted
summary: Test decision summary.
---

## My Decision

Body text.
`)

	warnings, err := docs.Extract(context.Background(), g, root)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}

	n, err := g.FindNodeByTypeAndName(context.Background(), graph.NodeTypeArchitectureDecision, "my_decision")
	if err != nil {
		t.Fatalf("FindNodeByTypeAndName: %v", err)
	}
	if n == nil {
		t.Fatal("expected architecture_decision node, got nil")
	}
	if n.Summary != "Test decision summary." {
		t.Errorf("unexpected summary: %q", n.Summary)
	}
}

// TestDocExtract_DecisionLinksToInvariant verifies that an invariant listed in
// front matter gets an edge from the decision node.
func TestDocExtract_DecisionLinksToInvariant(t *testing.T) {
	g := openTestGraph(t)
	root := t.TempDir()

	writeFile(t, root, "docs/awareness/dec-with-inv.md", `---
id: dec_with_invariant
type: architecture_decision
status: accepted
summary: Decision that references an invariant.
invariants:
  - my.invariant.id
---

Body.
`)

	if _, err := docs.Extract(context.Background(), g, root); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	ctx := context.Background()
	decNode, err := g.FindNodeByTypeAndName(ctx, graph.NodeTypeArchitectureDecision, "dec_with_invariant")
	if err != nil || decNode == nil {
		t.Fatalf("decision node not found: %v", err)
	}

	edges, err := g.OutgoingEdges(ctx, decNode.ID)
	if err != nil {
		t.Fatalf("OutgoingEdges: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Kind == graph.EdgeExplains {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an 'explains' edge from decision to invariant stub, got edges: %+v", edges)
	}
}

// TestDocExtract_DecisionLinksToFailureMode verifies that a failure_mode listed
// in front matter gets an edge from the decision node.
func TestDocExtract_DecisionLinksToFailureMode(t *testing.T) {
	g := openTestGraph(t)
	root := t.TempDir()

	writeFile(t, root, "docs/awareness/dec-with-fm.md", `---
id: dec_with_failure_mode
type: architecture_decision
status: accepted
summary: Decision that references a failure mode.
failure_modes:
  - my.failure.mode
---

Body.
`)

	if _, err := docs.Extract(context.Background(), g, root); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	ctx := context.Background()
	decNode, err := g.FindNodeByTypeAndName(ctx, graph.NodeTypeArchitectureDecision, "dec_with_failure_mode")
	if err != nil || decNode == nil {
		t.Fatalf("decision node not found: %v", err)
	}

	edges, err := g.OutgoingEdges(ctx, decNode.ID)
	if err != nil {
		t.Fatalf("OutgoingEdges: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Kind == graph.EdgeCausedBy {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 'caused_by' edge from decision to failure mode stub, got edges: %+v", edges)
	}
}

// TestDocExtract_DecisionLinksToForbiddenFix verifies that a forbidden_fix listed
// in front matter gets an edge from the decision node.
func TestDocExtract_DecisionLinksToForbiddenFix(t *testing.T) {
	g := openTestGraph(t)
	root := t.TempDir()

	writeFile(t, root, "docs/awareness/dec-with-fix.md", `---
id: dec_with_forbidden_fix
type: architecture_decision
status: accepted
summary: Decision that references a forbidden fix.
forbidden_fixes:
  - do_not_do_this
---

Body.
`)

	if _, err := docs.Extract(context.Background(), g, root); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	ctx := context.Background()
	decNode, err := g.FindNodeByTypeAndName(ctx, graph.NodeTypeArchitectureDecision, "dec_with_forbidden_fix")
	if err != nil || decNode == nil {
		t.Fatalf("decision node not found: %v", err)
	}

	edges, err := g.OutgoingEdges(ctx, decNode.ID)
	if err != nil {
		t.Fatalf("OutgoingEdges: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Kind == graph.EdgeForbids {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 'forbids' edge from decision to forbidden fix stub, got edges: %+v", edges)
	}
}

// TestDocExtract_FileWithNoFrontMatter verifies that a plain Markdown file
// (no front matter) still creates a documentation_section node.
func TestDocExtract_FileWithNoFrontMatter(t *testing.T) {
	g := openTestGraph(t)
	root := t.TempDir()

	writeFile(t, root, "docs/awareness/plain.md", `# Plain Doc

No front matter here.
`)

	if _, err := docs.Extract(context.Background(), g, root); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	ctx := context.Background()
	nodes, err := g.FindNodesByPath(ctx, "docs/awareness/plain.md")
	if err != nil {
		t.Fatalf("FindNodesByPath: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected documentation_section node for plain.md, got none")
	}
}
