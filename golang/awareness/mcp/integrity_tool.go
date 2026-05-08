package mcp

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/integrity"
)

func registerIntegrityTools(s *Server) {
	registerGraphIntegrityCheckTool(s)
	registerImpactPathTool(s)
}

// ── awareness.graph_integrity_check ──────────────────────────────────────────

func registerGraphIntegrityCheckTool(s *Server) {
	s.register(toolDef{
		Name: "awareness.graph_integrity_check",
		Description: "Validate that the awareness knowledge graph is descriptive, not aspirational. " +
			"Checks DONE fix cases for required tests, failure mode references, forbidden fix shapes, " +
			"causal rule contradictions (including etcd alarm ordering), and edge provenance. " +
			"Returns structured findings with status (healthy|warning|critical), counts, and exit_code. " +
			"Exit codes: 0=healthy, 1=warning, 2=critical, 3=tool failure.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"strict": {
					Type:        "boolean",
					Description: "If true, treat all warnings as critical (exit code 2).",
					Default:     false,
				},
				"test_results_file": {
					Type:        "string",
					Description: "Optional path to a CI test results JSON file (e.g. .awareness/test-results.json).",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		opts := integrity.Options{
			DocsDir:         s.resolvedDocsDir(),
			RepoRoot:        s.resolvedRepoRoot(),
			Strict:          boolArg(args, "strict"),
			TestResultsFile: strArg(args, "test_results_file"),
		}
		if opts.DocsDir == "" {
			return nil, fmt.Errorf("graph_integrity_check: docs dir not configured")
		}

		result, err := integrity.Check(ctx, opts, s.g)
		if err != nil {
			return nil, fmt.Errorf("graph_integrity_check: %w", err)
		}
		return result, nil
	})
}

// ── awareness.impact_path ─────────────────────────────────────────────────────

func registerImpactPathTool(s *Server) {
	s.register(toolDef{
		Name: "awareness.impact_path",
		Description: "Traverse the awareness graph from changed files to impacted invariants, tests, " +
			"and failure modes. Returns typed edge chains with trust levels. " +
			"Inferred (untyped) edges are labelled low-confidence. Requires an indexed awareness graph.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"changed_files": {
					Type:        "array",
					Description: "Repo-relative file paths to query impact for.",
					Items:       &propSchema{Type: "string"},
				},
				"max_depth": {
					Type:        "integer",
					Description: "Maximum edge hops to traverse (default: 6).",
					Default:     6,
				},
			},
			Required: []string{"changed_files"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		files := strSliceArg(args, "changed_files")
		if len(files) == 0 {
			return nil, fmt.Errorf("changed_files is required")
		}
		if s.g == nil {
			return nil, fmt.Errorf("impact_path: awareness graph not available — run 'globular awareness build' first")
		}

		maxDepth := 6
		if v, ok := args["max_depth"].(float64); ok && v > 0 {
			maxDepth = int(v)
		}

		q := integrity.ImpactPathQuery{
			ChangedFiles: files,
			MaxDepth:     maxDepth,
		}
		paths, err := integrity.TraverseImpactPaths(ctx, s.g, q)
		if err != nil {
			return nil, fmt.Errorf("impact_path traversal: %w", err)
		}

		return map[string]interface{}{
			"paths": paths,
			"count": len(paths),
		}, nil
	})
}
