package main

// tools_awareness_debug_session.go — awareness.debug_session MCP tool.
//
// Produces a guided debugging plan for an AI agent using the debugsession package.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/debugsession"
)

// registerAwarenessDebugSessionTool registers the awareness.debug_session tool.
func registerAwarenessDebugSessionTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.debug_session",
		Description: "Produce a guided debugging plan for an AI agent. " +
			"Composes preflight, semantic navigation, runtime evidence, fix-ledger, and node context " +
			"into a single actionable DebugSessionReport. Read-only — never mutates state.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task":            {Type: "string", Description: "Task description (required)"},
				"files":           {Type: "array", Items: &propSchema{Type: "string"}, Description: "Changed or relevant files"},
				"package_path":    {Type: "string", Description: "Go package path (e.g. golang/cluster_controller)"},
				"phase":           {Type: "string", Description: "Operation phase (e.g. 'edit', 'diagnose')"},
				"include_runtime": {Type: "boolean", Description: "Include live runtime evidence"},
				"format":          {Type: "string", Description: "Output format: json (default) or markdown"},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task, _ := args["task"].(string)
		if strings.TrimSpace(task) == "" {
			return nil, fmt.Errorf("task is required")
		}

		opts := debugsession.Options{
			Task:          task,
			DocsDir:       st.docsDir,
			RuntimeWindow: 24 * time.Hour,
		}

		if files, ok := args["files"].([]interface{}); ok {
			for _, f := range files {
				if s, ok := f.(string); ok {
					opts.Files = append(opts.Files, s)
				}
			}
		}
		if pkg, ok := args["package_path"].(string); ok {
			opts.PackagePath = pkg
		}
		if phase, ok := args["phase"].(string); ok {
			opts.Phase = phase
		}
		if ir, ok := args["include_runtime"].(bool); ok {
			opts.IncludeRuntime = ir
		}

		report, err := debugsession.Run(ctx, opts, st.g)
		if err != nil {
			return nil, fmt.Errorf("debug_session: %w", err)
		}
		return report, nil
	})
}
