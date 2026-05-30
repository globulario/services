package manual_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/extractors/manual"
	"github.com/globulario/services/golang/awareness/graph"
)

// fixture writes a tiny incident_patterns.yaml + supporting nodes for the
// pattern's references to land cleanly. Returns the YAML file path.
func writeIncidentPatternFixture(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "incident_patterns.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return path
}

// seedReferent puts the kinds of nodes a pattern's refs typically resolve to
// (invariant, failure_mode, forbidden_fix) into the graph so the extractor's
// existence checks pass and the corresponding edges land.
func seedReferent(t *testing.T, g *graph.Graph, ctx context.Context, nodeID, name, nodeType string) {
	t.Helper()
	if err := g.AddNode(ctx, graph.Node{ID: nodeID, Type: nodeType, Name: name}); err != nil {
		t.Fatalf("seed %s: %v", nodeID, err)
	}
}

// TestLoadIncidentPatterns_OneNodePerPattern pins the headline contract:
// every YAML entry produces exactly one graph node of type incident_pattern,
// with the correct id/name/path. If anyone refactors the loader to merge
// patterns or skip them silently, this test fails immediately.
func TestLoadIncidentPatterns_OneNodePerPattern(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	path := writeIncidentPatternFixture(t, t.TempDir(), `incident_patterns:
  - id: pat.example_first
    title: First example pattern
    severity: warning
  - id: pat.example_second
    title: Second example pattern
    severity: critical
`)
	if err := manual.LoadIncidentPatterns(ctx, g, path); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	for _, want := range []struct {
		id   string
		name string
	}{
		{"incident_pattern:pat.example_first", "pat.example_first"},
		{"incident_pattern:pat.example_second", "pat.example_second"},
	} {
		n, err := g.FindNode(ctx, want.id)
		if err != nil {
			t.Fatalf("FindNode(%s): %v", want.id, err)
		}
		if n == nil {
			t.Fatalf("expected node %s to exist", want.id)
		}
		if n.Type != graph.NodeTypeIncidentPattern {
			t.Errorf("%s: type = %q, want %q", want.id, n.Type, graph.NodeTypeIncidentPattern)
		}
		if n.Name != want.name {
			t.Errorf("%s: name = %q, want %q", want.id, n.Name, want.name)
		}
		if !strings.HasSuffix(n.Path, "incident_patterns.yaml") {
			t.Errorf("%s: path %q should end with incident_patterns.yaml", want.id, n.Path)
		}
	}
}

// TestLoadIncidentPatterns_FilesCreateBothEdgeDirections pins the rule that
// makes patterns surface as Direct matches in awareness_impact_file. Without
// the reverse source_file → implements → incident_pattern edge, the impact
// partition can't pick up patterns the same way it picks up invariants.
func TestLoadIncidentPatterns_FilesCreateBothEdgeDirections(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	path := writeIncidentPatternFixture(t, t.TempDir(), `incident_patterns:
  - id: pat.with_file
    title: Pattern with a protected file
    severity: warning
    files:
      - golang/example/foo.go
`)
	if err := manual.LoadIncidentPatterns(ctx, g, path); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	patID := "incident_pattern:pat.with_file"
	fileID := "source_file:golang/example/foo.go"

	out, err := g.Neighbors(ctx, patID, "out")
	if err != nil {
		t.Fatalf("pat → out edges: %v", err)
	}
	if !hasEdge(out, graph.EdgeProtects, fileID) {
		t.Errorf("missing edge %s → protects → %s; got %s", patID, fileID, edgesSummary(out))
	}

	rev, err := g.Neighbors(ctx, fileID, "out")
	if err != nil {
		t.Fatalf("file → out edges: %v", err)
	}
	if !hasEdge(rev, graph.EdgeImplements, patID) {
		t.Errorf("missing reverse edge %s → implements → %s; without this, impact's directIncidentPatternIDsForFile cannot land patterns. got %s",
			fileID, patID, edgesSummary(rev))
	}
}

// TestLoadIncidentPatterns_RelatedInvariantBecomesEdge — when the referenced
// invariant exists, an affects edge lands. The "exists" gate avoids
// materialising orphan edges that point at nothing.
func TestLoadIncidentPatterns_RelatedInvariantBecomesEdge(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	seedReferent(t, g, ctx, "invariant:example.invariant", "example.invariant", graph.NodeTypeInvariant)

	path := writeIncidentPatternFixture(t, t.TempDir(), `incident_patterns:
  - id: pat.with_invariant
    title: links to an invariant
    severity: warning
    related_invariants:
      - example.invariant
`)
	if err := manual.LoadIncidentPatterns(ctx, g, path); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	out, err := g.Neighbors(ctx, "incident_pattern:pat.with_invariant", "out")
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if !hasEdge(out, graph.EdgeAffects, "invariant:example.invariant") {
		t.Errorf("missing edge pat → affects → invariant; got %s", edgesSummary(out))
	}
}

// TestLoadIncidentPatterns_FailureModeBecomesEdge — same contract for the
// failure_mode field.
func TestLoadIncidentPatterns_FailureModeBecomesEdge(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	seedReferent(t, g, ctx, "failure_mode:example.fm", "example.fm", graph.NodeTypeFailureMode)

	path := writeIncidentPatternFixture(t, t.TempDir(), `incident_patterns:
  - id: pat.with_fm
    title: links to a failure_mode
    severity: warning
    failure_mode: example.fm
`)
	if err := manual.LoadIncidentPatterns(ctx, g, path); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	out, err := g.Neighbors(ctx, "incident_pattern:pat.with_fm", "out")
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if !hasEdge(out, graph.EdgeAffects, "failure_mode:example.fm") {
		t.Errorf("missing edge pat → affects → failure_mode; got %s", edgesSummary(out))
	}
}

// TestLoadIncidentPatterns_DanglingReferenceSkipsEdgeButLoadsNode —
// the pattern itself still gets a node even when its refs are unknown, but
// no orphan edges are created. Validation of dangling refs is the
// knowledge.Load layer's job; the extractor is silent about them.
func TestLoadIncidentPatterns_DanglingReferenceSkipsEdgeButLoadsNode(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	path := writeIncidentPatternFixture(t, t.TempDir(), `incident_patterns:
  - id: pat.with_dangling
    title: references things that do not exist
    severity: warning
    related_invariants:
      - never.declared.invariant
    failure_mode: never.declared.fm
    wrong_fixes:
      - never_declared_forbidden_fix
`)
	if err := manual.LoadIncidentPatterns(ctx, g, path); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	patID := "incident_pattern:pat.with_dangling"
	n, err := g.FindNode(ctx, patID)
	if err != nil || n == nil {
		t.Fatalf("expected pattern node to load despite dangling refs; got node=%v err=%v", n, err)
	}

	out, err := g.Neighbors(ctx, patID, "out")
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	for _, e := range out {
		// No edge should point at any of the never-declared targets.
		if e.Dst == "invariant:never.declared.invariant" ||
			e.Dst == "failure_mode:never.declared.fm" ||
			e.Dst == "forbidden_fix:never_declared_forbidden_fix" {
			t.Errorf("orphan edge %s → %s → %s; dangling refs must not create edges", patID, e.Kind, e.Dst)
		}
	}
}

// TestLoadIncidentPatterns_NoLongBodyTextInGraphJSON is the anti-bloat
// guard. The extractor must not mirror root_cause / lesson / wrong_fixes
// long-form text into graph node fields. The body lives in the YAML; the
// graph node is an index.
func TestLoadIncidentPatterns_NoLongBodyTextInGraphJSON(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	// A ~5 KB root_cause body and a ~5 KB lesson body. If either ends up
	// inside the graph node, the marshaled size will balloon past the
	// 1 KB anti-bloat ceiling.
	bigBody := strings.Repeat("This is a long narrative root_cause body. ", 120) // ~5 KB
	body := `incident_patterns:
  - id: pat.with_long_body
    title: A perfectly normal title
    severity: warning
    root_cause: |
      ` + bigBody + `
    lesson: |
      ` + bigBody + `
    wrong_fixes:
      - ` + strings.Repeat("a_long_wrong_fix_name_that_should_not_be_inlined_in_node_fields_", 20) + `
`
	path := writeIncidentPatternFixture(t, t.TempDir(), body)
	if err := manual.LoadIncidentPatterns(ctx, g, path); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	n, err := g.FindNode(ctx, "incident_pattern:pat.with_long_body")
	if err != nil || n == nil {
		t.Fatalf("expected pattern node to exist; got node=%v err=%v", n, err)
	}
	blob, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(blob) > 1024 {
		t.Errorf("incident_pattern graph node is %d bytes — anti-bloat ceiling is 1024. Long-form fields (root_cause/lesson/wrong_fixes) leaked into the node; they should stay in the YAML.\n  node: %s", len(blob), string(blob))
	}
}

// helpers shared with other manual extractor tests; kept package-local so
// the rest of the suite doesn't have to import a helpers package.
func hasEdge(edges []graph.Edge, kind, dst string) bool {
	for _, e := range edges {
		if e.Kind == kind && e.Dst == dst {
			return true
		}
	}
	return false
}

func edgesSummary(edges []graph.Edge) string {
	parts := make([]string, 0, len(edges))
	for _, e := range edges {
		parts = append(parts, e.Kind+"→"+e.Dst)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
