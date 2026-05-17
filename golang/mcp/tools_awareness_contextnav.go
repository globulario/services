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
			"an agent has the rest of the preflight cached and only wants the navigation layer. " +
			"Capped at max_traces (default 5) to protect context budget.",
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
				"max_traces": {
					Type:        "number",
					Description: "Maximum decision traces to return. Default: 5. Use 0 for unlimited (forensic only).",
					Default:     5,
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

		maxTraces := intArgDefault(args, "max_traces", 5)
		traces := r.DecisionTraces
		totalTraces := len(traces)
		if maxTraces > 0 && len(traces) > maxTraces {
			traces = traces[:maxTraces]
		}

		result := map[string]interface{}{
			"task":            task,
			"decision_traces": traces,
			"trust":           r.Trust,
			"graph_freshness": r.GraphFreshness,
			"live_overlay":    r.LiveOverlay,
			"total_traces":    totalTraces,
		}
		if totalTraces > maxTraces && maxTraces > 0 {
			result["truncated"] = true
			result["truncation_reason"] = fmt.Sprintf("%d traces omitted; increase max_traces or use forensic preflight for all", totalTraces-maxTraces)
		}
		return result, nil
	})
}

func registerAwarenessFindingContextTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.finding_context",
		Description: "Return the per-finding decision trace for an explicit prefixed finding id " +
			"(failure_mode:X | invariant:Y | forbidden_fix:Z). Produces owner inference, ranked " +
			"pivots, falsifiers, and next actions without running a full preflight.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"finding": {
					Type:        "string",
					Description: "Prefixed finding id (failure_mode:X | invariant:Y | forbidden_fix:Z)",
				},
				"task": {
					Type:        "string",
					Description: "Task or symptom description (optional; drives falsifier generation)",
				},
				"files": {
					Type:        "array",
					Description: "Files in scope (drives owner inference)",
					Items:       &propSchema{Type: "string"},
				},
			},
			Required: []string{"finding"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		findingID, _ := args["finding"].(string)
		if findingID == "" {
			return nil, fmt.Errorf("finding is required")
		}

		kind, id, err := contextnav.ParseFindingID(findingID)
		if err != nil {
			return nil, fmt.Errorf("invalid finding id: %w", err)
		}

		opts := contextnav.FindingContextOptions{
			Kind:  kind,
			ID:    id,
			Graph: st.g,
			Task:  strArg(args, "task"),
			Files: getStrSlice(args, "files"),
		}

		trace, err := contextnav.BuildForFinding(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("finding_context: %w", err)
		}

		return map[string]interface{}{
			"finding":       findingID,
			"finding_id":    id,
			"finding_type":  kind,
			"decision_trace": trace,
		}, nil
	})
}
