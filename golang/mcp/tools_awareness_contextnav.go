package main

// tools_awareness_contextnav.go — Phase 10 MCP surface for the
// context-navigation effort. Two tools:
//
//   - awareness.decision_trace
//     Runs a full preflight and returns ONLY the decision_traces slice.
//     Useful when an agent has the rest of the preflight output cached
//     and just wants the navigation layer.
//
//   - awareness.finding_context
//     Takes an explicit prefixed finding id (e.g.
//     `failure_mode:workflow.resume_poisoning`) and returns the single
//     DecisionTrace for that finding without running full preflight.
//
// Both tools are registered alongside the other awareness navigation
// tools (node_context, neighborhood, explain_node) in
// registerAwarenessTools.

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/analysis/contextnav"
	"github.com/globulario/services/golang/awareness/preflight"
)

func registerAwarenessContextNavTools(s *server, st *awarenessState) {
	registerAwarenessDecisionTraceTool(s, st)
	registerAwarenessFindingContextTool(s, st)
}

func registerAwarenessDecisionTraceTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.decision_trace",
		Description: "Run a preflight for the given task and return ONLY the per-finding decision " +
			"traces (matched_by / owner / pivots / next_actions / falsifiers). The fast path when " +
			"an agent has the rest of the preflight cached and only wants the navigation layer.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Task description (required)",
				},
				"files": {
					Type:        "array",
					Description: "Files you plan to edit (drives owner inference + impact analysis)",
					Items:       &propSchema{Type: "string"},
				},
				"include_runtime": {
					Type:        "boolean",
					Description: "Collect live runtime snapshot and merge runtime evidence into traces",
					Default:     false,
				},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := strArg(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}
		if st.g == nil {
			return map[string]interface{}{
				"error": "awareness graph not available",
				"hint":  "run 'globular awareness build' to build the graph",
			}, nil
		}
		opts := preflight.Options{
			Task:           task,
			Files:          getStrSlice(args, "files"),
			DocsDir:        st.docsDir,
			RepoRoot:       st.repoRoot,
			IncludeRuntime: getBool(args, "include_runtime", false),
		}
		if opts.IncludeRuntime {
			opts.Bridge = newLiveBridge(st)
		}
		r, err := preflight.Run(ctx, opts, st.g)
		if err != nil {
			return nil, fmt.Errorf("preflight run: %w", err)
		}
		return map[string]interface{}{
			"task":            task,
			"decision_traces": r.DecisionTraces,
			"trust":           r.Trust,
			"graph_freshness": r.GraphFreshness,
			"live_overlay":    r.LiveOverlay,
		}, nil
	})
}

func registerAwarenessFindingContextTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.finding_context",
		Description: "Return the per-finding decision trace for an explicit prefixed finding id " +
			"(failure_mode:X | invariant:Y | forbidden_fix:Z). Skips full preflight — runs only " +
			"the contextnav.Build path on the supplied finding plus a graph walk for owner " +
			"inference and ranked pivots. Useful when the agent already knows which finding it " +
			"wants to dig into.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"finding": {
					Type:        "string",
					Description: "Prefixed finding id (failure_mode:X | invariant:Y | forbidden_fix:Z)",
				},
				"task": {
					Type:        "string",
					Description: "Optional task description used for owner-inference tiebreakers",
				},
				"files": {
					Type:        "array",
					Description: "Optional files for owner inference and file-hint enrichment",
					Items:       &propSchema{Type: "string"},
				},
				"include_runtime": {
					Type:        "boolean",
					Description: "Include runtime-flavoured pivots and evidence",
					Default:     false,
				},
			},
			Required: []string{"finding"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		finding := strArg(args, "finding")
		if finding == "" {
			return nil, fmt.Errorf("finding is required (form: failure_mode:X | invariant:Y | forbidden_fix:Z)")
		}
		kind, id, err := contextnav.ParseFindingID(finding)
		if err != nil {
			return nil, err
		}
		tr, err := contextnav.BuildForFinding(ctx, contextnav.FindingContextOptions{
			Kind:           kind,
			ID:             id,
			Graph:          st.g, // may be nil — BuildForFinding handles that
			Task:           strArg(args, "task"),
			Files:          getStrSlice(args, "files"),
			IncludeRuntime: getBool(args, "include_runtime", false),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"finding":        finding,
			"decision_trace": tr,
		}, nil
	})
}
