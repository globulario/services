// Package analysis provides graph-based impact analysis, cycle detection,
// and agent context generation. No LLM calls — all matching is graph traversal
// and keyword matching against manually declared truth.
package analysis

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/graph"
)

// ImpactResult collects nodes reachable from a source file, partitioned by type.
type ImpactResult struct {
	SourceFile    *graph.Node
	Symbols       []*graph.Node
	Services      []*graph.Node
	Invariants    []*graph.Node
	FailureModes  []*graph.Node
	ForbiddenFixes []*graph.Node
	Tests         []*graph.Node
	Other         []*graph.Node
}

// ImpactByFile finds all nodes impacted by changes to the file at filePath,
// then partitions them by type into an ImpactResult.
func ImpactByFile(ctx context.Context, g *graph.Graph, filePath string) (*ImpactResult, error) {
	res, err := g.ImpactByFile(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ImpactByFile %s: %w", filePath, err)
	}

	result := &ImpactResult{}
	seen := make(map[string]bool)

	for _, n := range res.Nodes {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true

		switch n.Type {
		case graph.NodeTypeSourceFile:
			if n.Path == filePath {
				result.SourceFile = n
			}
		case graph.NodeTypeSymbol:
			result.Symbols = append(result.Symbols, n)
		case graph.NodeTypeGlobularService:
			result.Services = append(result.Services, n)
		case graph.NodeTypeInvariant:
			result.Invariants = append(result.Invariants, n)
		case graph.NodeTypeFailureMode:
			result.FailureModes = append(result.FailureModes, n)
		case graph.NodeTypeForbiddenFix:
			result.ForbiddenFixes = append(result.ForbiddenFixes, n)
		case graph.NodeTypeTest:
			result.Tests = append(result.Tests, n)
		default:
			result.Other = append(result.Other, n)
		}
	}

	return result, nil
}
