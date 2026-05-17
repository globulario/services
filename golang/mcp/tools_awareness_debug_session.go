package main

// tools_awareness_debug_session.go — stub after debugsession package was removed
// from standalone awareness module.

import (
	"context"
)

// registerAwarenessDebugSessionTool registers a stub for awareness.debug_session.
func registerAwarenessDebugSessionTool(s *server, _ *awarenessState) {
	s.register(toolDef{
		Name: "awareness.debug_session",
		Description: "Produce a guided debugging plan for an AI agent " +
			"[not available — debugsession package removed from standalone awareness module]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {Type: "string", Description: "Task description (required)"},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "debugsession package was removed from standalone awareness module",
		}, nil
	})
}
