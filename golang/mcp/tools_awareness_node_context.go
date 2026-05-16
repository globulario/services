package main

import (
	"context"
	"encoding/json"
	"fmt"

	awarectx "github.com/globulario/awareness/context"
)

// registerAwarenessNodeContextTools registers the three node-centric navigation tools:
//   - awareness.node_context  — full context for a graph node
//   - awareness.neighborhood  — BFS neighborhood of a node
//   - awareness.explain_node  — natural-language explanation of a node's role
func registerAwarenessNodeContextTools(s *server, st *awarenessState) {
	registerAwarenessNodeContext(s, st)
	registerAwarenessNeighborhood(s, st)
	registerAwarenessExplainNode(s, st)
}

func registerAwarenessNodeContext(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.node_context",
		Description: "Return full architectural context for a graph node: invariants, failure modes, " +
			"forbidden fixes, state reads/writes, required tests, edit warnings, design decisions, " +
			"and recommended searches. Start from a node ID, symbol, file, invariant, or failure mode.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID, service name, symbol name, file path, invariant ID, or failure mode ID",
				},
				"zoom": {
					Type:        "string",
					Description: "Semantic zoom level",
					Enum:        []string{"local", "module", "service", "architecture", "runtime", "history", "all"},
					Default:     "all",
				},
				"depth": {
					Type:        "integer",
					Description: "Traversal depth (default 2)",
					Default:     2,
				},
				"max_items": {
					Type:        "integer",
					Description: "Maximum items per list (default 10)",
					Default:     10,
				},
				"format": {
					Type:        "string",
					Description: "Output format: agent, markdown, or json",
					Enum:        []string{"agent", "markdown", "json"},
					Default:     "agent",
				},
				"include_runtime": {
					Type:        "boolean",
					Description: "Include runtime bridge evidence (requires runtime data in graph)",
					Default:     false,
				},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		ref := strArg(args, "node")
		if ref == "" {
			return nil, fmt.Errorf("node is required")
		}
		if st.g == nil {
			return map[string]interface{}{
				"error":    "awareness graph not available",
				"hint":     "run 'globular awareness build' to build the graph",
				"node_ref": ref,
			}, nil
		}

		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}
		zoom := awarectx.Zoom(strArg(args, "zoom"))
		if zoom == "" {
			zoom = awarectx.ZoomAll
		}

		opts := awarectx.Options{
			Zoom:           zoom,
			MaxItems:       getInt(args, "max_items", 10),
			Depth:          getInt(args, "depth", 2),
			IncludeRuntime: getBool(args, "include_runtime", false),
		}

		r, err := awarectx.ResolveNode(ctx, st.g, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			candidates := make([]map[string]string, 0, len(r.Candidates))
			for _, c := range r.Candidates {
				candidates = append(candidates, map[string]string{
					"id": c.ID, "type": c.Type, "name": c.Name,
					"path": c.Path, "summary": c.Summary,
				})
			}
			return map[string]interface{}{
				"error":      "node not found",
				"ref":        ref,
				"candidates": candidates,
			}, nil
		}

		nc, err := awarectx.Build(ctx, st.g, r.Exact.ID, opts)
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

func registerAwarenessNeighborhood(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.neighborhood",
		Description: "Return the BFS neighborhood of a graph node up to a given depth, " +
			"partitioned by type: services, symbols, files, invariants, failure modes, tests, other.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name",
				},
				"depth": {
					Type:        "integer",
					Description: "BFS depth (default 1, max 4)",
					Default:     1,
				},
				"format": {
					Type:        "string",
					Description: "Output format: agent, markdown, or json",
					Enum:        []string{"agent", "markdown", "json"},
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
		if st.g == nil {
			return map[string]interface{}{
				"error": "awareness graph not available",
				"hint":  "run 'globular awareness build' to build the graph",
			}, nil
		}

		depth := getInt(args, "depth", 1)
		if depth > 4 {
			depth = 4
		}
		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		r, err := awarectx.ResolveNode(ctx, st.g, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			return map[string]interface{}{
				"error": "node not found",
				"ref":   ref,
			}, nil
		}

		nr, err := awarectx.Neighborhood(ctx, st.g, r.Exact.ID, depth)
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

func registerAwarenessExplainNode(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.explain_node",
		Description: "Generate a natural-language explanation of a graph node's role, what it protects, " +
			"the risks it carries, what fixes are forbidden, and what an AI agent should inspect next.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name to explain",
				},
				"format": {
					Type:        "string",
					Description: "Output format: agent, markdown, or json",
					Enum:        []string{"agent", "markdown", "json"},
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
		if st.g == nil {
			return map[string]interface{}{
				"error": "awareness graph not available",
				"hint":  "run 'globular awareness build' to build the graph",
			}, nil
		}

		format := strArg(args, "format")
		if format == "" {
			format = "markdown"
		}

		r, err := awarectx.ResolveNode(ctx, st.g, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", ref, err)
		}
		if r.Exact == nil {
			return map[string]interface{}{
				"error": "node not found",
				"ref":   ref,
			}, nil
		}

		ex, err := awarectx.ExplainNode(ctx, st.g, r.Exact.ID, awarectx.Options{MaxItems: 10})
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
