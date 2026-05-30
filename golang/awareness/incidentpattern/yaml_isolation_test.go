package incidentpattern_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/extractors/manual"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/incidentpattern"
)

// TestListPatterns_DoesNotSurfaceYAMLAuthoredPatterns is the architectural
// boundary test: the two pattern stores (YAML-authored, written by the
// manual extractor as graph nodes; and runtime-recorded, written by
// incidentpattern.Store.RecordPattern as separate JSON files under
// <DataDir>/incident_patterns/) must remain isolated. Loading a YAML
// pattern into the graph MUST NOT cause incidentpattern.Store.ListPatterns
// to start returning it — that would conflate stay-fixed knowledge with
// runtime-recorded incidents and break consumers like
// assurance.countIncidentPatterns that count "incidents this failure_mode
// was learned from".
//
// If anyone wires a YAML→JSON-store bridge into the extractor, this test
// fails and the trade-off has to be made explicit.
func TestListPatterns_DoesNotSurfaceYAMLAuthoredPatterns(t *testing.T) {
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	// Write a YAML pattern + load it via the manual extractor. The extractor
	// writes an incident_pattern graph node — but it must NOT write into
	// the incidentpattern.Store JSON store.
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "incident_patterns.yaml")
	if err := os.WriteFile(yamlPath, []byte(`incident_patterns:
  - id: pat.yaml_authored
    title: Stay-fixed pattern authored in YAML
    severity: warning
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := manual.LoadIncidentPatterns(ctx, g, yamlPath); err != nil {
		t.Fatalf("LoadIncidentPatterns: %v", err)
	}

	// Sanity: the YAML pattern is present as a graph node.
	if n, err := g.FindNode(ctx, "incident_pattern:pat.yaml_authored"); err != nil || n == nil {
		t.Fatalf("expected YAML pattern to land as graph node; got node=%v err=%v", n, err)
	}

	// Now ListPatterns from the JSON store. Must not include the YAML pattern.
	store := incidentpattern.NewStore(g)
	patterns, err := store.ListPatterns(ctx)
	if err != nil {
		t.Fatalf("ListPatterns: %v", err)
	}
	for _, p := range patterns {
		if p.ID == "pat.yaml_authored" || p.IncidentID == "pat.yaml_authored" {
			t.Errorf("incidentpattern.Store.ListPatterns returned the YAML-authored pattern %q. The two stores must stay isolated: YAML patterns live in graph nodes; the JSON store is for runtime-recorded incidents only.", p.ID)
		}
	}
	if len(patterns) != 0 {
		t.Logf("ListPatterns returned %d entries; none should be from YAML. ids:", len(patterns))
		for _, p := range patterns {
			t.Logf("  - %s (incident %s)", p.ID, p.IncidentID)
		}
	}
}

