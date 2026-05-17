package graph

import (
	"context"
	"fmt"
)

// Neighbors returns edges connected to id.
// direction is "out" (outgoing), "in" (incoming), or "both".
func (g *Graph) Neighbors(ctx context.Context, id, direction string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Edge
	switch direction {
	case "in":
		for _, e := range g.byDst[id] {
			out = append(out, *e)
		}
	case "out":
		for _, e := range g.bySrc[id] {
			out = append(out, *e)
		}
	default:
		seen := make(map[edgeKey]bool)
		for _, e := range g.bySrc[id] {
			k := edgeKey{e.Src, e.Kind, e.Dst, e.Phase}
			if !seen[k] {
				seen[k] = true
				out = append(out, *e)
			}
		}
		for _, e := range g.byDst[id] {
			k := edgeKey{e.Src, e.Kind, e.Dst, e.Phase}
			if !seen[k] {
				seen[k] = true
				out = append(out, *e)
			}
		}
	}
	return out, nil
}

// NeighborsByClass returns outgoing edges from id filtered by edge_class.
func (g *Graph) NeighborsByClass(ctx context.Context, id, class string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Edge
	for _, e := range g.bySrc[id] {
		if e.Class == class {
			out = append(out, *e)
		}
	}
	return out, nil
}

// EdgesByClass returns all edges in the graph with the given edge_class.
func (g *Graph) EdgesByClass(ctx context.Context, class string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	src := g.byClass[class]
	out := make([]Edge, len(src))
	for i, e := range src {
		out[i] = *e
	}
	return out, nil
}

// AllEdges returns every edge in the graph (used by cycle detection).
func (g *Graph) AllEdges(ctx context.Context) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Edge, len(g.edges))
	for i, e := range g.edges {
		out[i] = *e
	}
	return out, nil
}

// OutgoingEdges returns all edges where src == nodeID.
func (g *Graph) OutgoingEdges(ctx context.Context, nodeID string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	src := g.bySrc[nodeID]
	out := make([]Edge, len(src))
	for i, e := range src {
		out[i] = *e
	}
	return out, nil
}

// EdgesByKind returns all edges of the given kind.
func (g *Graph) EdgesByKind(ctx context.Context, kind string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	src := g.byKind[kind]
	out := make([]Edge, len(src))
	for i, e := range src {
		out[i] = *e
	}
	return out, nil
}

// TraverseDecision performs BFS from startID following only decision-class edges.
func (g *Graph) TraverseDecision(ctx context.Context, startID string, maxDepth int) (*TraversalResult, error) {
	visited := make(map[string]bool)
	result := &TraversalResult{}

	type item struct {
		id    string
		depth int
	}
	queue := []item{{startID, 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if visited[cur.id] {
			continue
		}
		visited[cur.id] = true

		node, err := g.FindNode(ctx, cur.id)
		if err != nil {
			return nil, err
		}
		if node != nil {
			result.Nodes = append(result.Nodes, node)
		}

		if cur.depth >= maxDepth {
			continue
		}

		edges, err := g.NeighborsByClass(ctx, cur.id, EdgeClassDecision)
		if err != nil {
			return nil, fmt.Errorf("TraverseDecision neighbors %s: %w", cur.id, err)
		}

		for _, e := range edges {
			result.Edges = append(result.Edges, e)
			if !visited[e.Dst] {
				queue = append(queue, item{e.Dst, cur.depth + 1})
			}
		}
	}

	return result, nil
}
