package main

// tools_awareness_node_context.go — stubs after the context package was removed
// from standalone awareness module. The node_context, neighborhood, and explain_node
// MCP tools are not available in this build.

import (
	"context"
)

// registerAwarenessNodeContextTools registers stubs for the three node-centric navigation tools.
func registerAwarenessNodeContextTools(s *server, _ *awarenessState) {
	registerAwarenessNodeContext(s)
	registerAwarenessNeighborhood(s)
	registerAwarenessExplainNode(s)
}

func registerAwarenessNodeContext(s *server) {
	s.register(toolDef{
		Name:        "awareness.node_context",
		Description: "Show full architectural context for a graph node [not available — context package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {Type: "string", Description: "Node ID, service name, symbol name, file path, invariant ID, or failure mode ID"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "context package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessNeighborhood(s *server) {
	s.register(toolDef{
		Name:        "awareness.neighborhood",
		Description: "Show the BFS neighborhood of a graph node [not available — context package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node":  {Type: "string", Description: "Node ID or name (required)"},
				"depth": {Type: "integer", Description: "BFS depth (max 4)", Default: 1},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "context package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessExplainNode(s *server) {
	s.register(toolDef{
		Name:        "awareness.explain_node",
		Description: "Explain a graph node's role, risks, and edit warnings [not available — context package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {Type: "string", Description: "Node ID or name (required)"},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "context package was removed from standalone awareness module",
		}, nil
	})
}
