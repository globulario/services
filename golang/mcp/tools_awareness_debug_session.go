package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/awareness/debugsession"
)

// registerAwarenessDebugSessionTool registers the awareness.debug_session tool.
func registerAwarenessDebugSessionTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.debug_session",
		Description: "Produce a guided debugging plan for AI agents. Composes preflight, " +
			"semantic navigation, runtime evidence, fix-ledger, and node context into a " +
			"ranked, explainable report.\n\n" +
			"This tool is READ-ONLY: it never edits code, mutates runtime state, " +
			"promotes proposals, or dispatches remediation.\n\n" +
			"Call this at the START of a bug investigation before touching any code. " +
			"The report tells you where to start, what root-cause paths are likely, " +
			"what files and functions to inspect, what fixes are forbidden, and what " +
			"tests are required.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Natural-language description of the problem or task (required)",
				},
				"files": {
					Type:        "array",
					Description: "File paths to include in impact analysis",
				},
				"package_path": {
					Type:        "string",
					Description: "Path to a package directory containing awareness.yaml",
				},
				"phase": {
					Type:        "string",
					Description: "Dependency phase for cycle detection",
				},
				"include_runtime": {
					Type:        "boolean",
					Description: "Include live runtime snapshot evidence",
					Default:     false,
				},
				"runtime_window": {
					Type:        "string",
					Description: "Lookback window for runtime evidence (e.g. '15m', '1h')",
					Default:     "15m",
				},
				"format": {
					Type:        "string",
					Description: "Output format",
					Enum:        []string{"agent", "markdown", "json"},
					Default:     "agent",
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
				"task":  task,
			}, nil
		}

		// Resolve docs dir from repo root.
		repoRoot := awarGitRoot()
		docsDir := ""
		if repoRoot != "" {
			docsDir = filepath.Join(repoRoot, "docs", "awareness")
		}

		// Parse files array.
		var files []string
		if raw, ok := args["files"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						files = append(files, s)
					}
				}
			}
		}

		// Parse runtime window.
		window := 15 * time.Minute
		if wStr := strArg(args, "runtime_window"); wStr != "" {
			if d, err := time.ParseDuration(wStr); err == nil {
				window = d
			}
		}

		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		opts := debugsession.Options{
			Task:           task,
			Files:          files,
			PackagePath:    strArg(args, "package_path"),
			Phase:          strArg(args, "phase"),
			DocsDir:        docsDir,
			IncludeRuntime: getBool(args, "include_runtime", false),
			RuntimeWindow:  window,
		}

		report, err := debugsession.Run(ctx, opts, st.g)
		if err != nil {
			return nil, fmt.Errorf("debug_session: %w", err)
		}

		if format == "json" {
			var out interface{}
			_ = json.Unmarshal([]byte(debugsession.FormatReport(report, "json")), &out)
			return out, nil
		}
		return debugsession.FormatReport(report, format), nil
	})
}
