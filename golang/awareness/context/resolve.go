package awarectx

import (
	"context"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// RefKind describes how a node reference was resolved.
type RefKind string

const (
	RefKindExact     RefKind = "exact"
	RefKindSymbol    RefKind = "symbol"
	RefKindFile      RefKind = "file"
	RefKindPackage   RefKind = "package"
	RefKindService   RefKind = "service"
	RefKindInvariant RefKind = "invariant"
	RefKindFailure   RefKind = "failure_mode"
	RefKindTest      RefKind = "test"
	RefKindProto     RefKind = "proto"
	RefKindWorkflow  RefKind = "workflow"
	RefKindName      RefKind = "name"
)

// ResolveResult holds the outcome of a node reference lookup.
type ResolveResult struct {
	Exact      *graph.Node
	Candidates []*graph.Node
	Kind       RefKind
	Ref        string
}

// ResolveNode resolves a free-form reference to a graph node.
// Resolution order: exact ID → invariant table → file path → typed name →
// name-like search. Returns (result with empty Exact and Candidates) if nothing found.
func ResolveNode(ctx context.Context, g *graph.Graph, ref string) (*ResolveResult, error) {
	if ref == "" {
		return &ResolveResult{Ref: ref, Kind: RefKindName}, nil
	}

	// 1. Exact node ID lookup.
	n, err := g.FindNode(ctx, ref)
	if err != nil {
		return nil, err
	}
	if n != nil {
		return &ResolveResult{Exact: n, Kind: RefKindExact, Ref: ref}, nil
	}

	// 2. Invariant table lookup (IDs like "service.endpoint.reachability").
	inv, err := g.FindInvariant(ctx, ref)
	if err == nil && inv != nil {
		node, _ := g.FindNodeByTypeAndName(ctx, graph.NodeTypeInvariant, ref)
		if node == nil {
			node = &graph.Node{ID: inv.ID, Type: graph.NodeTypeInvariant, Name: inv.Title, Summary: inv.Summary}
		}
		return &ResolveResult{Exact: node, Kind: RefKindInvariant, Ref: ref}, nil
	}

	// 3. File path heuristic.
	if isFilePath(ref) {
		nodes, err := g.FindNodesByPath(ctx, ref)
		if err != nil {
			return nil, err
		}
		if len(nodes) == 1 {
			return &ResolveResult{Exact: nodes[0], Kind: RefKindFile, Ref: ref}, nil
		}
		if len(nodes) > 1 {
			// Multiple nodes can share a path (e.g. source_file + symbols defined in it).
			// Prefer the source_file node.
			for _, n := range nodes {
				if n.Type == graph.NodeTypeSourceFile {
					return &ResolveResult{Exact: n, Kind: RefKindFile, Ref: ref}, nil
				}
			}
			return &ResolveResult{Candidates: nodes, Kind: RefKindFile, Ref: ref}, nil
		}
	}

	// 4. Typed name lookups — most specific first.
	for _, entry := range []struct {
		nodeType string
		kind     RefKind
	}{
		{graph.NodeTypeGlobularService, RefKindService},
		{graph.NodeTypeSymbol, RefKindSymbol},
		{graph.NodeTypeGoPackage, RefKindPackage},
		{graph.NodeTypeTest, RefKindTest},
		{graph.NodeTypeProtoService, RefKindProto},
		{graph.NodeTypeRPCMethod, RefKindProto},
		{graph.NodeTypeWorkflow, RefKindWorkflow},
		{graph.NodeTypeSourceFile, RefKindFile},
		{graph.NodeTypeFailureMode, RefKindFailure},
	} {
		node, err := g.FindNodeByTypeAndName(ctx, entry.nodeType, ref)
		if err != nil {
			continue
		}
		if node != nil {
			return &ResolveResult{Exact: node, Kind: entry.kind, Ref: ref}, nil
		}
	}

	// 5. Name-like search as fallback.
	candidates, err := g.FindNodesByNameLike(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 1 {
		return &ResolveResult{Exact: candidates[0], Kind: RefKindName, Ref: ref}, nil
	}
	return &ResolveResult{Candidates: candidates, Kind: RefKindName, Ref: ref}, nil
}

// isFilePath returns true if ref looks like a file path.
func isFilePath(ref string) bool {
	return strings.Contains(ref, "/") ||
		strings.HasSuffix(ref, ".go") ||
		strings.HasSuffix(ref, ".proto") ||
		strings.HasSuffix(ref, ".yaml") ||
		strings.HasSuffix(ref, ".yml")
}
