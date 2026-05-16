package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/globulario/awareness/semantic"
)

// registerAwarenessSemanticTools registers the 5 semantic-distance / path-finding tools:
//   - awareness.related          — nodes ranked by semantic distance
//   - awareness.nearest          — nearest nodes of a specific type
//   - awareness.path             — shortest weighted path between two nodes
//   - awareness.why_related      — enriched explanation of why two nodes are related
//   - awareness.semantic_neighborhood — ranked neighbourhood across all types
func registerAwarenessSemanticTools(s *server, st *awarenessState) {
	registerAwarenessRelated(s, st)
	registerAwarenessNearest(s, st)
	registerAwarenessPath(s, st)
	registerAwarenessWhyRelated(s, st)
	registerAwarenessSemanticNeighborhood(s, st)
}

// ---- shared helpers ---------------------------------------------------------

var dimEnum = []string{
	semantic.DimensionCode,
	semantic.DimensionModule,
	semantic.DimensionService,
	semantic.DimensionPackage,
	semantic.DimensionState,
	semantic.DimensionWorkflow,
	semantic.DimensionArch,
	semantic.DimensionRuntime,
	semantic.DimensionHistory,
	semantic.DimensionTest,
	semantic.DimensionAll,
}

func graphUnavailable(ref string) map[string]interface{} {
	return map[string]interface{}{
		"error":    "awareness graph not available",
		"hint":     "run 'globular awareness build' to build the graph",
		"node_ref": ref,
	}
}

// ---- awareness.related ------------------------------------------------------

func registerAwarenessRelated(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.related",
		Description: "Return nodes semantically related to a given node, ranked by weighted distance. " +
			"Runs Dijkstra over the awareness graph in the requested semantic dimension. " +
			"Use 'architecture' to surface invariants and decisions; 'code' for symbols and files.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name to query from",
				},
				"dimension": {
					Type:        "string",
					Description: "Semantic dimension to optimise for",
					Enum:        dimEnum,
					Default:     semantic.DimensionAll,
				},
				"target_types": {
					Type:        "array",
					Description: "If non-empty, only return nodes of these types",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum results to return (default 10)",
					Default:     10,
				},
				"max_depth": {
					Type:        "integer",
					Description: "Maximum traversal depth (default 4)",
					Default:     4,
				},
				"max_cost": {
					Type:        "number",
					Description: "Maximum traversal cost (default 20)",
					Default:     20,
				},
				"format": {
					Type:    "string",
					Enum:    []string{"agent", "markdown", "json"},
					Default: "agent",
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
			return graphUnavailable(ref), nil
		}

		dim := strArg(args, "dimension")
		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		var targetTypes []string
		if raw, ok := args["target_types"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						targetTypes = append(targetTypes, s)
					}
				}
			}
		}

		opts := semantic.RelatedOptions{
			Dimension:   dim,
			TargetTypes: targetTypes,
			MaxResults:  getInt(args, "max_results", 10),
			MaxDepth:    getInt(args, "max_depth", 4),
			MaxCost:     getFloat(args, "max_cost", 20),
		}

		results, err := semantic.Related(ctx, st.g, ref, opts)
		if err != nil {
			return nil, fmt.Errorf("related: %w", err)
		}

		if dim == "" {
			dim = semantic.DimensionAll
		}

		if format == "json" {
			var out interface{}
			_ = json.Unmarshal([]byte(semantic.FormatRelated(results, ref, dim, "json")), &out)
			return out, nil
		}
		return semantic.FormatRelated(results, ref, dim, format), nil
	})
}

// ---- awareness.nearest ------------------------------------------------------

func registerAwarenessNearest(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.nearest",
		Description: "Find the nearest nodes of a specific type to a given node, ranked by semantic distance. " +
			"Useful for 'what invariants are closest to this service?' or 'what tests cover this symbol?'",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name to query from",
				},
				"target_type": {
					Type:        "string",
					Description: "Node type to search for (e.g. invariant, failure_mode, test, globular_service)",
				},
				"dimension": {
					Type:    "string",
					Enum:    dimEnum,
					Default: semantic.DimensionAll,
				},
				"max_results": {
					Type:    "integer",
					Default: 10,
				},
				"max_depth": {
					Type:    "integer",
					Default: 4,
				},
				"max_cost": {
					Type:    "number",
					Default: 20,
				},
				"format": {
					Type:    "string",
					Enum:    []string{"agent", "markdown", "json"},
					Default: "agent",
				},
			},
			Required: []string{"node", "target_type"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		ref := strArg(args, "node")
		targetType := strArg(args, "target_type")
		if ref == "" {
			return nil, fmt.Errorf("node is required")
		}
		if targetType == "" {
			return nil, fmt.Errorf("target_type is required")
		}
		if st.g == nil {
			return graphUnavailable(ref), nil
		}

		dim := strArg(args, "dimension")
		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		opts := semantic.RelatedOptions{
			Dimension:  dim,
			MaxResults: getInt(args, "max_results", 10),
			MaxDepth:   getInt(args, "max_depth", 4),
			MaxCost:    getFloat(args, "max_cost", 20),
		}

		results, err := semantic.Nearest(ctx, st.g, ref, targetType, opts)
		if err != nil {
			return nil, fmt.Errorf("nearest: %w", err)
		}

		if dim == "" {
			dim = semantic.DimensionAll
		}

		if format == "json" {
			var out interface{}
			_ = json.Unmarshal([]byte(semantic.FormatRelated(results, ref, dim, "json")), &out)
			return out, nil
		}
		return semantic.FormatRelated(results, ref, dim, format), nil
	})
}

// ---- awareness.path ---------------------------------------------------------

func registerAwarenessPath(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.path",
		Description: "Find the lowest-cost semantic path between two nodes in the awareness graph. " +
			"Returns the step-by-step traversal, total cost, and a plain-English explanation. " +
			"Use this to understand how a code symbol is connected to an invariant or failure mode.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"from": {
					Type:        "string",
					Description: "Source node ID or name",
				},
				"to": {
					Type:        "string",
					Description: "Destination node ID or name",
				},
				"dimension": {
					Type:    "string",
					Enum:    dimEnum,
					Default: semantic.DimensionAll,
				},
				"max_depth": {
					Type:    "integer",
					Default: 6,
				},
				"max_cost": {
					Type:    "number",
					Default: 30,
				},
				"avoid_weak_edges": {
					Type:        "boolean",
					Description: "Skip edges with base cost >= 6 for tighter structural paths",
					Default:     false,
				},
				"include_runtime": {
					Type:    "boolean",
					Default: false,
				},
				"format": {
					Type:    "string",
					Enum:    []string{"agent", "markdown", "json"},
					Default: "agent",
				},
			},
			Required: []string{"from", "to"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		fromID := strArg(args, "from")
		toID := strArg(args, "to")
		if fromID == "" {
			return nil, fmt.Errorf("from is required")
		}
		if toID == "" {
			return nil, fmt.Errorf("to is required")
		}
		if st.g == nil {
			return graphUnavailable(fromID), nil
		}

		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		opts := semantic.PathOptions{
			Dimension:      strArg(args, "dimension"),
			MaxDepth:       getInt(args, "max_depth", 6),
			MaxCost:        getFloat(args, "max_cost", 30),
			AvoidWeakEdges: getBool(args, "avoid_weak_edges", false),
			IncludeRuntime: getBool(args, "include_runtime", false),
		}

		p, err := semantic.ShortestPath(ctx, st.g, fromID, toID, opts)
		if err != nil {
			return nil, fmt.Errorf("path: %w", err)
		}

		if format == "json" {
			var out interface{}
			_ = json.Unmarshal([]byte(semantic.FormatPath(p, "json")), &out)
			return out, nil
		}
		return semantic.FormatPath(p, format), nil
	})
}

// ---- awareness.why_related --------------------------------------------------

func registerAwarenessWhyRelated(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.why_related",
		Description: "Explain why two nodes are semantically related: shortest path, relationship summary, " +
			"why it matters (invariant/failure mode/forbidden fix context), edit warnings, required tests, " +
			"and forbidden approaches. Use this before editing code that may affect a distant component.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"from": {
					Type:        "string",
					Description: "Source node ID or name",
				},
				"to": {
					Type:        "string",
					Description: "Destination node ID or name",
				},
				"dimension": {
					Type:    "string",
					Enum:    dimEnum,
					Default: semantic.DimensionAll,
				},
				"max_depth": {
					Type:    "integer",
					Default: 6,
				},
				"max_cost": {
					Type:    "number",
					Default: 30,
				},
				"include_runtime": {
					Type:    "boolean",
					Default: false,
				},
				"format": {
					Type:    "string",
					Enum:    []string{"agent", "markdown", "json"},
					Default: "agent",
				},
			},
			Required: []string{"from", "to"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		fromID := strArg(args, "from")
		toID := strArg(args, "to")
		if fromID == "" {
			return nil, fmt.Errorf("from is required")
		}
		if toID == "" {
			return nil, fmt.Errorf("to is required")
		}
		if st.g == nil {
			return graphUnavailable(fromID), nil
		}

		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		opts := semantic.WhyOptions{
			Dimension:      strArg(args, "dimension"),
			MaxDepth:       getInt(args, "max_depth", 6),
			MaxCost:        getFloat(args, "max_cost", 30),
			IncludeRuntime: getBool(args, "include_runtime", false),
		}

		r, err := semantic.WhyRelated(ctx, st.g, fromID, toID, opts)
		if err != nil {
			return nil, fmt.Errorf("why_related: %w", err)
		}

		if format == "json" {
			var out interface{}
			_ = json.Unmarshal([]byte(semantic.FormatWhy(r, "json")), &out)
			return out, nil
		}
		return semantic.FormatWhy(r, format), nil
	})
}

// ---- awareness.semantic_neighborhood ----------------------------------------

func registerAwarenessSemanticNeighborhood(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.semantic_neighborhood",
		Description: "Return all nodes semantically reachable from a given node, ranked by distance, " +
			"across all node types. Useful as an overview before diving into node-context or why-related. " +
			"Shows the semantic gravity — what does this node attract in each dimension?",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {
					Type:        "string",
					Description: "Node ID or name",
				},
				"dimension": {
					Type:    "string",
					Enum:    dimEnum,
					Default: semantic.DimensionAll,
				},
				"max_results": {
					Type:    "integer",
					Default: 20,
				},
				"max_depth": {
					Type:    "integer",
					Default: 3,
				},
				"max_cost": {
					Type:    "number",
					Default: 15,
				},
				"include_runtime": {
					Type:    "boolean",
					Default: false,
				},
				"format": {
					Type:    "string",
					Enum:    []string{"agent", "markdown", "json"},
					Default: "agent",
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
			return graphUnavailable(ref), nil
		}

		dim := strArg(args, "dimension")
		format := strArg(args, "format")
		if format == "" {
			format = "agent"
		}

		opts := semantic.RelatedOptions{
			Dimension:      dim,
			MaxResults:     getInt(args, "max_results", 20),
			MaxDepth:       getInt(args, "max_depth", 3),
			MaxCost:        getFloat(args, "max_cost", 15),
			IncludeRuntime: getBool(args, "include_runtime", false),
		}

		results, err := semantic.SemanticNeighborhood(ctx, st.g, ref, opts)
		if err != nil {
			return nil, fmt.Errorf("semantic_neighborhood: %w", err)
		}

		if dim == "" {
			dim = semantic.DimensionAll
		}

		if format == "json" {
			var out interface{}
			_ = json.Unmarshal([]byte(semantic.FormatSemanticNeighborhood(results, ref, dim, "json")), &out)
			return out, nil
		}
		return semantic.FormatSemanticNeighborhood(results, ref, dim, format), nil
	})
}
