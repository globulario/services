package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Invariant is the specialized invariant record.
type Invariant struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Severity string `json:"severity,omitempty"`
	Status   string `json:"status,omitempty"`
}

// FailureMode is the specialized failure mode record.
type FailureMode struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary,omitempty"`
	Symptoms        []string `json:"symptoms,omitempty"`
	RootCause       string   `json:"root_cause,omitempty"`
	ArchitectureFix string   `json:"architecture_fix,omitempty"`
}

// DesignContext is the set of design patterns, anti-patterns, and code smells
// linked to a given set of invariant node IDs.
type DesignContext struct {
	DesignPatterns []string
	AntiPatterns   []string
	CodeSmells     []string
}

// AllInvariants returns all invariant records ordered by ID.
func (g *Graph) AllInvariants(ctx context.Context) ([]*Invariant, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*Invariant, 0, len(g.invariants))
	for _, inv := range g.invariants {
		cp := *inv
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// AllFailureModes returns all failure mode records ordered by ID.
func (g *Graph) AllFailureModes(ctx context.Context) ([]*FailureMode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*FailureMode, 0, len(g.failureModes))
	for _, fm := range g.failureModes {
		cp := *fm
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// UpsertInvariant upserts an invariant record.
func (g *Graph) UpsertInvariant(ctx context.Context, inv Invariant) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertInvariant %s: graph is read-only", inv.ID)
	}
	g.mu.Lock()
	cp := inv
	g.invariants[inv.ID] = &cp
	g.mu.Unlock()
	return nil
}

// UpsertFailureMode upserts a failure mode record.
func (g *Graph) UpsertFailureMode(ctx context.Context, fm FailureMode) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertFailureMode %s: graph is read-only", fm.ID)
	}
	g.mu.Lock()
	cp := fm
	g.failureModes[fm.ID] = &cp
	g.mu.Unlock()
	return nil
}

// FindInvariant returns an invariant record by ID, or (nil, nil) if not found.
func (g *Graph) FindInvariant(ctx context.Context, id string) (*Invariant, error) {
	g.mu.RLock()
	inv := g.invariants[id]
	g.mu.RUnlock()
	if inv == nil {
		return nil, nil
	}
	cp := *inv
	return &cp, nil
}

// CodeSmellsForInvariants returns all code_smells from pattern nodes that
// have a "requires" edge targeting any of the given invariant node IDs.
func (g *Graph) CodeSmellsForInvariants(ctx context.Context, invariantNodeIDs []string) ([]string, error) {
	if len(invariantNodeIDs) == 0 {
		return nil, nil
	}
	invSet := make(map[string]bool, len(invariantNodeIDs))
	for _, id := range invariantNodeIDs {
		invSet[id] = true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, e := range g.byKind[EdgeRequires] {
		if !invSet[e.Dst] {
			continue
		}
		n := g.nodes[e.Src]
		if n == nil || n.Type != NodeTypePattern {
			continue
		}
		for _, s := range extractCodeSmellsFromMeta(n.Metadata) {
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// PatternNamesForInvariants returns the names of NodeTypePattern nodes
// that have an EdgeRequires edge to any of the given invariant node IDs.
func (g *Graph) PatternNamesForInvariants(ctx context.Context, invariantNodeIDs []string) ([]string, error) {
	if len(invariantNodeIDs) == 0 {
		return nil, nil
	}
	invSet := make(map[string]bool, len(invariantNodeIDs))
	for _, id := range invariantNodeIDs {
		invSet[id] = true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, e := range g.byKind[EdgeRequires] {
		if !invSet[e.Dst] {
			continue
		}
		n := g.nodes[e.Src]
		if n == nil || n.Type != NodeTypePattern {
			continue
		}
		if !seen[n.Name] {
			seen[n.Name] = true
			out = append(out, n.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// DesignContextForInvariants returns design patterns and anti-patterns linked
// to the given invariant node IDs.
func (g *Graph) DesignContextForInvariants(ctx context.Context, invariantNodeIDs []string) (*DesignContext, error) {
	if len(invariantNodeIDs) == 0 {
		return &DesignContext{}, nil
	}
	invSet := make(map[string]bool, len(invariantNodeIDs))
	for _, id := range invariantNodeIDs {
		invSet[id] = true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	dc := &DesignContext{}
	seen := map[string]bool{}
	var antiPatternIDs []string

	for _, kind := range []string{EdgeRequires, EdgeMitigates} {
		for _, e := range g.byKind[kind] {
			if !invSet[e.Dst] {
				continue
			}
			n := g.nodes[e.Src]
			if n == nil || n.Type != NodeTypeDesignPattern {
				continue
			}
			if !seen[n.Name] {
				seen[n.Name] = true
				dc.DesignPatterns = append(dc.DesignPatterns, n.Name)
			}
		}
	}

	for _, e := range g.byKind[EdgeViolates] {
		if !invSet[e.Dst] {
			continue
		}
		n := g.nodes[e.Src]
		if n == nil || n.Type != NodeTypeAntiPattern {
			continue
		}
		if !seen[n.Name] {
			seen[n.Name] = true
			dc.AntiPatterns = append(dc.AntiPatterns, n.Name)
		}
		antiPatternIDs = append(antiPatternIDs, n.ID)
		for _, s := range extractCodeSmellsFromMeta(n.Metadata) {
			if s != "" && !seen["smell:"+s] {
				seen["smell:"+s] = true
				dc.CodeSmells = append(dc.CodeSmells, s)
			}
		}
	}

	antiSet := make(map[string]bool, len(antiPatternIDs))
	for _, id := range antiPatternIDs {
		antiSet[id] = true
	}
	for _, e := range g.byKind[EdgeSmellsLike] {
		if !antiSet[e.Src] {
			continue
		}
		n := g.nodes[e.Dst]
		if n == nil || n.Type != NodeTypeCodeSmell {
			continue
		}
		if n.Name != "" && !seen["smell:"+n.Name] {
			seen["smell:"+n.Name] = true
			dc.CodeSmells = append(dc.CodeSmells, n.Name)
		}
	}

	sort.Strings(dc.DesignPatterns)
	sort.Strings(dc.AntiPatterns)
	sort.Strings(dc.CodeSmells)
	return dc, nil
}

// DesignContextForNode returns design patterns and anti-patterns that are
// directly linked to the given node ID.
func (g *Graph) DesignContextForNode(ctx context.Context, nodeID string) (*DesignContext, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dc := &DesignContext{}
	seen := map[string]bool{}
	var antiPatternIDs []string

	for _, kind := range []string{EdgeImplements, EdgeExhibits, EdgeTouchesFile} {
		for _, e := range g.byKind[kind] {
			if e.Dst != nodeID {
				continue
			}
			n := g.nodes[e.Src]
			if n == nil {
				continue
			}
			switch n.Type {
			case NodeTypeDesignPattern:
				if !seen[n.Name] {
					seen[n.Name] = true
					dc.DesignPatterns = append(dc.DesignPatterns, n.Name)
				}
			case NodeTypeAntiPattern:
				if !seen[n.Name] {
					seen[n.Name] = true
					dc.AntiPatterns = append(dc.AntiPatterns, n.Name)
				}
				antiPatternIDs = append(antiPatternIDs, n.ID)
				for _, s := range extractCodeSmellsFromMeta(n.Metadata) {
					if s != "" && !seen["smell:"+s] {
						seen["smell:"+s] = true
						dc.CodeSmells = append(dc.CodeSmells, s)
					}
				}
			}
		}
	}

	antiSet := make(map[string]bool, len(antiPatternIDs))
	for _, id := range antiPatternIDs {
		antiSet[id] = true
	}
	for _, e := range g.byKind[EdgeSmellsLike] {
		if !antiSet[e.Src] {
			continue
		}
		n := g.nodes[e.Dst]
		if n == nil || n.Type != NodeTypeCodeSmell {
			continue
		}
		if n.Name != "" && !seen["smell:"+n.Name] {
			seen["smell:"+n.Name] = true
			dc.CodeSmells = append(dc.CodeSmells, n.Name)
		}
	}

	sort.Strings(dc.DesignPatterns)
	sort.Strings(dc.AntiPatterns)
	sort.Strings(dc.CodeSmells)
	return dc, nil
}

// extractCodeSmellsFromMeta unpacks the code_smells array from a node's Metadata map.
func extractCodeSmellsFromMeta(meta map[string]any) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta["code_smells"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, s := range v {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return v
	default:
		b, _ := json.Marshal(raw)
		var ss []string
		_ = json.Unmarshal(b, &ss)
		return ss
	}
}
