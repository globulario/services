package main

// tools_awareness_semantic.go — stubs after semantic package was removed from
// standalone awareness module. The semantic distance/path-finding MCP tools
// are not available in this build.

import (
	"context"
)

// registerAwarenessSemanticTools registers stubs for the 5 semantic tools.
func registerAwarenessSemanticTools(s *server, _ *awarenessState) {
	registerAwarenessRelated(s)
	registerAwarenessNearest(s)
	registerAwarenessPath(s)
	registerAwarenessWhyRelated(s)
	registerAwarenessSemanticNeighborhood(s)
}

func registerAwarenessRelated(s *server) {
	s.register(toolDef{
		Name:        "awareness.related",
		Description: "Find nodes semantically related to a given node [not available — semantic package removed]",
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
			"reason": "semantic package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessNearest(s *server) {
	s.register(toolDef{
		Name:        "awareness.nearest",
		Description: "Find nearest nodes of a specific type [not available — semantic package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {Type: "string", Description: "Node ID or name (required)"},
				"type": {Type: "string", Description: "Target node type (required)"},
			},
			Required: []string{"node", "type"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "semantic package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessPath(s *server) {
	s.register(toolDef{
		Name:        "awareness.path",
		Description: "Find the lowest-cost semantic path between two nodes [not available — semantic package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"from": {Type: "string", Description: "Source node ID or name (required)"},
				"to":   {Type: "string", Description: "Destination node ID or name (required)"},
			},
			Required: []string{"from", "to"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "semantic package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessWhyRelated(s *server) {
	s.register(toolDef{
		Name:        "awareness.why_related",
		Description: "Explain why two nodes are semantically related [not available — semantic package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"from": {Type: "string", Description: "Source node ID or name (required)"},
				"to":   {Type: "string", Description: "Destination node ID or name (required)"},
			},
			Required: []string{"from", "to"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "semantic package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessSemanticNeighborhood(s *server) {
	s.register(toolDef{
		Name:        "awareness.semantic_neighborhood",
		Description: "Show all semantically related nodes ranked by distance [not available — semantic package removed]",
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
			"reason": "semantic package was removed from standalone awareness module",
		}, nil
	})
}
