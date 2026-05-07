package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	awarectx "github.com/globulario/services/golang/awareness/context"
	"github.com/globulario/services/golang/awareness/graph"
)

func registerNodeContextTools(s *Server) {
	registerNodeContextTool(s)
	registerNeighborhoodTool(s)
	registerExplainNodeTool(s)
}

func registerNodeContextTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.node_context",
		Description: "Return full architectural context for a graph node: invariants, failure modes, forbidden fixes, state reads/writes, required tests, edit warnings, and recommended searches.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID, service name, symbol name, file path, invariant ID, or failure mode ID",
				},
				"format": {
					Type:        "string",
					Description: "Output format: markdown, json, or agent",
					Enum:        []string{"markdown", "json", "agent"},
					Default:     "agent",
				},
				"max_items": {
					Type:        "integer",
					Description: "Maximum items per list (default 20)",
					Default:     20,
				},
				"depth": {
					Type:        "integer",
					Description: "Traversal depth for related nodes (default 2)",
					Default:     2,
				},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		ref := strArg(args, "node")
		if ref == "" {
			return nil, fmt.Errorf("node is required")
		}
		if s.g == nil {
			return nil, fmt.Errorf("no graph DB — run 'globular awareness build' first")
		}

		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		opts := awarectx.Options{
			MaxItems: intArg(args, "max_items", 20),
			Depth:    intArg(args, "depth", 2),
		}

		r, err := awarectx.ResolveNode(ctx, s.g, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			return map[string]interface{}{
				"error":      "node not found",
				"ref":        ref,
				"candidates": nodeNames(r.Candidates),
			}, nil
		}

		nc, err := awarectx.Build(ctx, s.g, r.Exact.ID, opts)
		if err != nil {
			return nil, fmt.Errorf("build context: %w", err)
		}

		if format == "json" {
			var result interface{}
			out := awarectx.FormatNodeContext(nc, "json")
			_ = json.Unmarshal([]byte(out), &result)
			return result, nil
		}
		return awarectx.FormatNodeContext(nc, format), nil
	})
}

func registerNeighborhoodTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.neighborhood",
		Description: "Return the BFS neighborhood of a graph node up to a given depth, partitioned by type (services, symbols, files, invariants, failure modes, tests).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name to explore",
				},
				"depth": {
					Type:        "integer",
					Description: "BFS depth (default 1, max 4)",
					Default:     1,
				},
				"format": {
					Type:        "string",
					Description: "Output format: markdown, json, or agent",
					Enum:        []string{"markdown", "json", "agent"},
					Default:     "agent",
				},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		ref := strArg(args, "node")
		if ref == "" {
			return nil, fmt.Errorf("node is required")
		}
		if s.g == nil {
			return nil, fmt.Errorf("no graph DB — run 'globular awareness build' first")
		}

		depth := intArg(args, "depth", 1)
		if depth > 4 {
			depth = 4
		}
		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		r, err := awarectx.ResolveNode(ctx, s.g, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			return map[string]interface{}{
				"error":      "node not found",
				"ref":        ref,
				"candidates": nodeNames(r.Candidates),
			}, nil
		}

		nr, err := awarectx.Neighborhood(ctx, s.g, r.Exact.ID, depth)
		if err != nil {
			return nil, fmt.Errorf("neighborhood: %w", err)
		}

		if format == "json" {
			var result interface{}
			out := awarectx.FormatNeighborhood(nr, "json")
			_ = json.Unmarshal([]byte(out), &result)
			return result, nil
		}
		return awarectx.FormatNeighborhood(nr, format), nil
	})
}

func registerExplainNodeTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.explain_node",
		Description: "Generate a natural-language explanation of a graph node's role, what it protects, the risks it carries, and what edit warnings apply.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name to explain",
				},
				"format": {
					Type:        "string",
					Description: "Output format: markdown, json, or agent",
					Enum:        []string{"markdown", "json", "agent"},
					Default:     "markdown",
				},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		ref := strArg(args, "node")
		if ref == "" {
			return nil, fmt.Errorf("node is required")
		}
		if s.g == nil {
			return nil, fmt.Errorf("no graph DB — run 'globular awareness build' first")
		}

		format := strArg(args, "format")
		if format == "" {
			format = "markdown"
		}

		r, err := awarectx.ResolveNode(ctx, s.g, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			return map[string]interface{}{
				"error":      "node not found",
				"ref":        ref,
				"candidates": nodeNames(r.Candidates),
			}, nil
		}

		ex, err := awarectx.ExplainNode(ctx, s.g, r.Exact.ID, awarectx.Options{})
		if err != nil {
			return nil, fmt.Errorf("explain: %w", err)
		}

		if format == "json" {
			var result interface{}
			out := awarectx.FormatExplanation(ex, "json")
			_ = json.Unmarshal([]byte(out), &result)
			return result, nil
		}
		return awarectx.FormatExplanation(ex, format), nil
	})
}

// intArg extracts an int argument from an MCP args map, with a default.
func intArg(args map[string]interface{}, key string, def int) int {
	if v, ok := args[key]; ok {
		switch vv := v.(type) {
		case float64:
			return int(vv)
		case int:
			return vv
		case int64:
			return int(vv)
		}
	}
	return def
}

// nodeNames returns a slice of names from a node slice.
func nodeNames(nodes []*graph.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}
