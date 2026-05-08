package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/learning"
	"github.com/globulario/services/golang/awareness/preflight"
	"github.com/globulario/services/golang/awareness/runtime"
)

func registerAwarenessPreflightTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.preflight",
		Description: "Run a full architecture preflight before editing Globular code. Returns invariants, failure modes, forbidden fixes, did-we-fix status, required tests, and agent instruction.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Task description (required)",
				},
				"files": {
					Type:        "array",
					Description: "Files you plan to edit (run impact analysis)",
					Items:       &propSchema{Type: "string"},
				},
				"package_path": {
					Type:        "string",
					Description: "Path to package directory containing awareness.yaml",
				},
				"phase": {
					Type:        "string",
					Description: "Dependency phase for cycle detection (recovery, bootstrap, package_install, reconcile)",
				},
				"include_runtime": {
					Type:        "boolean",
					Description: "Collect live runtime snapshot and merge into preflight",
					Default:     false,
				},
				"runtime_window": {
					Type:        "string",
					Description: "Lookback window for runtime events/workflows (e.g. 15m, 1h)",
					Default:     "15m",
				},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := strArg(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}

		opts := preflight.Options{
			Task:        task,
			Files:       getStrSlice(args, "files"),
			PackagePath: strArg(args, "package_path"),
			Phase:       strArg(args, "phase"),
			DocsDir:     st.docsDir,
		}

		if getBool(args, "include_runtime", false) {
			opts.IncludeRuntime = true
			opts.Bridge = runtime.NewBridge(st.nodeID, "")
			if ws := strArg(args, "runtime_window"); ws != "" {
				if d, err := time.ParseDuration(ws); err == nil {
					opts.RuntimeWindow = d
				}
			}
			if opts.RuntimeWindow == 0 {
				opts.RuntimeWindow = 15 * time.Minute
			}
		}

		r, err := preflight.Run(ctx, opts, st.g)
		if err != nil {
			return nil, fmt.Errorf("preflight: %w", err)
		}

		out, err := preflight.Render(r, preflight.FormatJSON)
		if err != nil {
			return nil, fmt.Errorf("render: %w", err)
		}

		// Return as a raw JSON object (not a string) for clean MCP consumption.
		var result interface{}
		_ = json.Unmarshal([]byte(out), &result)
		return result, nil
	})

	s.register(toolDef{
		Name:        "awareness.agent_context",
		Description: "Generate architectural context for a task — invariants, failure modes, forbidden fixes, and required tests from the awareness graph.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Task description",
				},
				"files": {
					Type:        "array",
					Description: "Files being edited (narrows context)",
					Items:       &propSchema{Type: "string"},
				},
				"services": {
					Type:        "array",
					Description: "Service names to include (narrows context)",
					Items:       &propSchema{Type: "string"},
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
				"status":          "UNKNOWN_IMPACT",
				"invariants":      []string{},
				"failure_modes":   []string{},
				"forbidden_fixes": []string{},
				"required_tests":  []string{},
				"graph_available": false,
				"coverage": map[string]interface{}{
					"graph":          "not_checked",
					"raw_yaml":       "not_checked",
					"runtime":        "noop",
					"metrics":        "noop",
					"code_scan":      "not_checked",
					"incident_store": "not_checked",
				},
				"confidence":        "unknown",
				"confidence_reason": "graph DB not available — no architecture facts can be matched; run 'globular awareness build' first",
				"blind_spots": []string{
					"graph unavailable — static pattern matching only",
					"runtime snapshot not collected — no live cluster evidence",
					"metrics not available — resource saturation not assessed",
					"code violation scan not run — use awareness.scan_violations",
				},
				"warnings": []string{"no graph DB — run 'globular awareness build' first"},
			}, nil
		}

		var aliasMap learning.ContextAliasMap
		if st.docsDir != "" {
			aliasMap, _ = learning.LoadContextAliases(st.docsDir + "/context_aliases.yaml")
		}

		hints := analysis.AgentContextHints{
			Files:    getStrSlice(args, "files"),
			Services: getStrSlice(args, "services"),
		}

		_, result, err := analysis.GenerateAgentContext(ctx, st.g, task, hints, analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"invariants":        result.InvariantIDs,
			"failure_modes":     result.FailureModeIDs,
			"forbidden_fixes":   result.ForbiddenFixes,
			"required_tests":    result.RequiredTests,
			"required_searches": result.RequiredSearches,
			"services":          result.ServiceNames,
		}, nil
	})
}
