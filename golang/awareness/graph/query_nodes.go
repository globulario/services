package graph

import (
	"context"
	"sort"
	"strings"
)

// FindNode returns a node by ID, or (nil, nil) if not found.
func (g *Graph) FindNode(ctx context.Context, id string) (*Node, error) {
	g.mu.RLock()
	n := g.nodes[id]
	g.mu.RUnlock()
	if n == nil {
		return nil, nil
	}
	cp := *n
	return &cp, nil
}

// FindNodesByType returns all nodes of the given type ordered by name.
func (g *Graph) FindNodesByType(ctx context.Context, nodeType string) ([]*Node, error) {
	g.mu.RLock()
	src := g.byType[nodeType]
	out := make([]*Node, len(src))
	for i, n := range src {
		cp := *n
		out[i] = &cp
	}
	g.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// FindNodesByPath returns nodes whose path exactly matches the given value.
func (g *Graph) FindNodesByPath(ctx context.Context, path string) ([]*Node, error) {
	g.mu.RLock()
	src := g.byPath[path]
	out := make([]*Node, len(src))
	for i, n := range src {
		cp := *n
		out[i] = &cp
	}
	g.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// FindNodeByTypeAndName returns the first node matching type + exact name.
func (g *Graph) FindNodeByTypeAndName(ctx context.Context, nodeType, name string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, n := range g.byType[nodeType] {
		if n.Name == name {
			cp := *n
			return &cp, nil
		}
	}
	return nil, nil
}

// FindNodesByNameLike returns nodes whose name contains the query string (case-insensitive).
func (g *Graph) FindNodesByNameLike(ctx context.Context, query string) ([]*Node, error) {
	q := strings.ToLower(query)
	g.mu.RLock()
	var out []*Node
	for _, n := range g.nodes {
		if strings.Contains(strings.ToLower(n.Name), q) {
			cp := *n
			out = append(out, &cp)
		}
	}
	g.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Stats returns current node/edge/invariant/failure-mode counts.
func (g *Graph) Stats(ctx context.Context) (BuildStats, error) {
	g.mu.RLock()
	s := BuildStats{
		Nodes:        len(g.nodes),
		Edges:        len(g.edges),
		Invariants:   len(g.invariants),
		FailureModes: len(g.failureModes),
	}
	g.mu.RUnlock()
	return s, nil
}
