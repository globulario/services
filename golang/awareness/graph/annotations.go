package graph

import (
	"context"
	"fmt"
)

// FileAnnotationSummary contains protocol annotations explicitly declared on
// symbols (or the file itself) defined in a specific source file.
type FileAnnotationSummary struct {
	Invariants       []string
	ForbiddenFixes   []string
	HashSchemas      []string
	StateTransitions []string
	RequiredTests    []string
	Risks            []string
	HasCritical      bool
}

// AnnotationsForFile returns explicit protocol annotations attached to symbols
// defined in the file at relPath.
func (g *Graph) AnnotationsForFile(ctx context.Context, relPath string) (*FileAnnotationSummary, error) {
	summary := &FileAnnotationSummary{}

	fileNodeID := "source_file:" + relPath

	// Collect symbol IDs defined by this file plus the file node itself.
	ownerIDs := []string{fileNodeID}

	g.mu.RLock()
	for _, e := range g.bySrc[fileNodeID] {
		if e.Kind == EdgeDefines {
			ownerIDs = append(ownerIDs, e.Dst)
		}
	}
	g.mu.RUnlock()

	for _, ownerID := range ownerIDs {
		edges, err := g.Neighbors(ctx, ownerID, "out")
		if err != nil {
			continue
		}
		for _, e := range edges {
			switch e.Kind {
			case EdgeEnforces, EdgeProtects:
				name, critical := g.resolveInvariantName(ctx, e.Dst)
				if name != "" {
					summary.Invariants = annotAppendUnique(summary.Invariants, name)
					if critical {
						summary.HasCritical = true
					}
				}

			case EdgeForbids:
				if n, _ := g.FindNode(ctx, e.Dst); n != nil {
					summary.ForbiddenFixes = annotAppendUnique(summary.ForbiddenFixes, n.Name)
				}

			case EdgeProduces, EdgeRequires:
				if n, _ := g.FindNode(ctx, e.Dst); n != nil && n.Type == NodeTypeHashSchema {
					summary.HashSchemas = annotAppendUnique(summary.HashSchemas, n.Name)
				}

			case EdgeAffects:
				if n, _ := g.FindNode(ctx, e.Dst); n != nil {
					switch n.Type {
					case NodeTypeStateTransition:
						summary.StateTransitions = annotAppendUnique(summary.StateTransitions, n.Name)
					case NodeTypeRiskSurface:
						summary.Risks = annotAppendUnique(summary.Risks, n.Name)
					}
				}

			case EdgeTestedBy:
				if n, _ := g.FindNode(ctx, e.Dst); n != nil {
					summary.RequiredTests = annotAppendUnique(summary.RequiredTests, n.Name)
				}
			}
		}
	}

	return summary, nil
}

// resolveInvariantName returns the invariant's human-readable ID and whether
// it has severity="critical".
func (g *Graph) resolveInvariantName(ctx context.Context, nodeID string) (name string, critical bool) {
	const prefix = "invariant:"
	tableID := nodeID
	if len(nodeID) > len(prefix) && nodeID[:len(prefix)] == prefix {
		tableID = nodeID[len(prefix):]
	}

	if inv, _ := g.FindInvariant(ctx, tableID); inv != nil {
		return inv.ID, inv.Severity == "critical"
	}

	if n, _ := g.FindNode(ctx, nodeID); n != nil {
		return n.Name, false
	}
	return "", false
}

// annotAppendUnique appends s to slice if not already present.
func annotAppendUnique(slice []string, s string) []string {
	if s == "" {
		return slice
	}
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// SetEdgeConfidence updates the confidence of an existing edge.
func (g *Graph) SetEdgeConfidence(ctx context.Context, src, kind, dst, phase string, confidence float64) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("SetEdgeConfidence: graph is read-only")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	k := edgeKey{Src: src, Kind: kind, Dst: dst, Phase: phase}
	if idx, ok := g.edgeKeys[k]; ok {
		g.edges[idx].Confidence = confidence
	}
	return nil
}
