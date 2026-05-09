package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/graph"
)

func registerAwarenessInvariantTools(s *server, st *awarenessState) {
	registerExplainInvariantTool(s, st)
	registerFileInvariantContextTool(s, st)
}

// ── awareness.explain_invariant ───────────────────────────────────────────────

func registerExplainInvariantTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.explain_invariant",
		Description: `Return full implementation evidence for a named invariant.

Given an invariant ID, returns:
- All source files that implement or partially implement it
- All tests that verify or cover it (tested_by + verifies edges)
- Forbidden actions (forbidden_fix nodes)
- Authority sources the invariant reads
- Decision guidance sentences
- Known failure modes that violate it
- Implementation gaps (missing edges)

Use this tool when you want to understand what enforces an invariant,
how well it is tested, and what would break it.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"invariant_id": {
					Type:        "string",
					Description: "Canonical invariant ID (e.g. 'service.endpoint.etcd_address_reachability')",
				},
			},
			Required: []string{"invariant_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		invID := strArg(args, "invariant_id")
		if invID == "" {
			return map[string]interface{}{"error": "invariant_id is required"}, nil
		}
		if st.g == nil {
			return map[string]interface{}{
				"invariant_id": invID,
				"error":        "graph unavailable — run 'globular awareness build' first",
			}, nil
		}
		return buildExplainInvariant(ctx, st.g, invID)
	})
}

func buildExplainInvariant(ctx context.Context, g *graph.Graph, invID string) (map[string]interface{}, error) {
	nodeID := "invariant:" + invID

	n, err := g.FindNode(ctx, nodeID)
	if err != nil || n == nil {
		return map[string]interface{}{
			"invariant_id": invID,
			"error":        fmt.Sprintf("invariant %q not found in graph", invID),
		}, nil
	}

	inEdges, err := g.Neighbors(ctx, nodeID, "in")
	if err != nil {
		return nil, fmt.Errorf("explain_invariant %s: in-edges: %w", invID, err)
	}
	outEdges, err := g.Neighbors(ctx, nodeID, "out")
	if err != nil {
		return nil, fmt.Errorf("explain_invariant %s: out-edges: %w", invID, err)
	}

	var implementations []map[string]interface{}
	var tests []map[string]interface{}
	var forbiddenFixes []string
	var authoritySources []string
	var failureModes []string

	for _, e := range inEdges {
		switch e.Kind {
		case graph.EdgeImplements, graph.EdgePartiallyImplements, graph.EdgeEnforces, graph.EdgeConfigures, graph.EdgeObserves:
			trust := "declared"
			if e.Metadata != nil {
				if t, ok := e.Metadata["trust_level"].(string); ok && t != "" {
					trust = t
				}
			}
			implementations = append(implementations, map[string]interface{}{
				"file":       e.Src,
				"edge_kind":  e.Kind,
				"trust_level": trust,
			})
		case graph.EdgeVerifies:
			tests = append(tests, map[string]interface{}{
				"test":      e.Src,
				"edge_kind": "verifies",
			})
		case graph.EdgeViolates:
			failureModes = append(failureModes, e.Src)
		case graph.EdgeBlocksForbiddenAction:
			forbiddenFixes = append(forbiddenFixes, e.Src)
		}
	}
	for _, e := range outEdges {
		switch e.Kind {
		case graph.EdgeTestedBy:
			// Only add if not already in verifies.
			found := false
			for _, t := range tests {
				if t["test"] == e.Dst {
					found = true
				}
			}
			if !found {
				tests = append(tests, map[string]interface{}{
					"test":      e.Dst,
					"edge_kind": "tested_by",
				})
			}
		case graph.EdgeForbids:
			forbiddenFixes = append(forbiddenFixes, e.Dst)
		case graph.EdgeReadsAuthority:
			authoritySources = append(authoritySources, e.Dst)
		case graph.EdgeAffects:
			failureModes = append(failureModes, e.Dst)
		}
	}

	// Gaps.
	var gaps []string
	if len(implementations) == 0 {
		gaps = append(gaps, "no implementing source file found (implement/partially_implement/enforce)")
	}
	if len(tests) == 0 {
		gaps = append(gaps, "no test coverage (tested_by or verifies edge)")
	}
	hasVerifies := false
	for _, t := range tests {
		if t["edge_kind"] == "verifies" {
			hasVerifies = true
			break
		}
	}
	if len(tests) > 0 && !hasVerifies {
		gaps = append(gaps, "tests linked via tested_by only — add a verifies edge for stronger proof")
	}
	if len(authoritySources) == 0 && len(implementations) > 0 {
		gaps = append(gaps, "no authority source declared — add authority[] to the invariant YAML")
	}

	// Decision guidance from node metadata.
	var decisionGuidance []string
	if n.Metadata != nil {
		if dg, ok := n.Metadata["decision_guidance"].([]interface{}); ok {
			for _, g := range dg {
				if s, ok := g.(string); ok {
					decisionGuidance = append(decisionGuidance, s)
				}
			}
		}
	}

	return map[string]interface{}{
		"invariant_id":      invID,
		"summary":           n.Summary,
		"implementations":   implementations,
		"tests":             tests,
		"forbidden_actions": forbiddenFixes,
		"authority_sources": authoritySources,
		"failure_modes":     failureModes,
		"decision_guidance": decisionGuidance,
		"gaps":              gaps,
	}, nil
}

// ── awareness.file_invariant_context ─────────────────────────────────────────

func registerFileInvariantContextTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.file_invariant_context",
		Description: `Return all invariants linked to a source file, with edit warnings.

Given a file path (relative to repo root), returns:
- All invariants the file implements, partially implements, enforces, configures, or observes
- The edge kind and trust level for each link
- Edit warnings derived from forbidden_actions on each invariant
- Required tests that must pass after editing this file

Use this tool BEFORE editing any file to understand what invariants you might break.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "Relative file path (e.g. 'golang/cluster_controller/cluster_controller_server/release_reconciler.go')",
				},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file := strArg(args, "file")
		if file == "" {
			return map[string]interface{}{"error": "file is required"}, nil
		}
		if st.g == nil {
			return map[string]interface{}{
				"file":  file,
				"error": "graph unavailable — run 'globular awareness build' first",
			}, nil
		}
		return buildFileInvariantContext(ctx, st.g, file)
	})
}

func buildFileInvariantContext(ctx context.Context, g *graph.Graph, file string) (map[string]interface{}, error) {
	fileID := "source_file:" + file

	n, err := g.FindNode(ctx, fileID)
	if err != nil || n == nil {
		return map[string]interface{}{
			"file":    file,
			"warning": "file not indexed in graph — run 'globular awareness build' to index it",
			"invariants": []interface{}{},
		}, nil
	}

	outEdges, err := g.Neighbors(ctx, fileID, "out")
	if err != nil {
		return nil, fmt.Errorf("file_invariant_context %s: out-edges: %w", file, err)
	}

	var invariantLinks []map[string]interface{}
	var allEditWarnings []string
	var allRequiredTests []string
	seenTests := map[string]bool{}
	seenWarnings := map[string]bool{}

	implKinds := map[string]bool{
		graph.EdgeImplements:         true,
		graph.EdgePartiallyImplements: true,
		graph.EdgeEnforces:           true,
		graph.EdgeConfigures:         true,
		graph.EdgeObserves:           true,
		graph.EdgeMayAffect:          true,
	}

	for _, e := range outEdges {
		if !implKinds[e.Kind] {
			continue
		}
		if len(e.Dst) < len("invariant:") || e.Dst[:len("invariant:")] != "invariant:" {
			continue
		}

		invNode, err := g.FindNode(ctx, e.Dst)
		if err != nil || invNode == nil {
			continue
		}
		invID := invNode.Name

		trust := "declared"
		if e.Metadata != nil {
			if t, ok := e.Metadata["trust_level"].(string); ok && t != "" {
				trust = t
			}
		}

		// Gather forbidden fixes and tests from the invariant.
		invOut, err := g.Neighbors(ctx, e.Dst, "out")
		if err != nil {
			continue
		}
		var invForbiddenFixes []string
		var invTests []string
		for _, ie := range invOut {
			switch ie.Kind {
			case graph.EdgeForbids:
				invForbiddenFixes = append(invForbiddenFixes, ie.Dst)
				warn := fmt.Sprintf("editing %s may violate invariant %s — forbidden: %s", file, invID, ie.Dst)
				if !seenWarnings[warn] {
					allEditWarnings = append(allEditWarnings, warn)
					seenWarnings[warn] = true
				}
			case graph.EdgeTestedBy:
				invTests = append(invTests, ie.Dst)
				if !seenTests[ie.Dst] {
					allRequiredTests = append(allRequiredTests, ie.Dst)
					seenTests[ie.Dst] = true
				}
			}
		}

		invariantLinks = append(invariantLinks, map[string]interface{}{
			"invariant_id":     invID,
			"summary":          invNode.Summary,
			"edge_kind":        e.Kind,
			"trust_level":      trust,
			"forbidden_fixes":  invForbiddenFixes,
			"required_tests":   invTests,
		})
	}

	warning := ""
	if len(invariantLinks) > 0 {
		warning = fmt.Sprintf("editing %s affects %d invariant(s) — review forbidden_fixes and run required_tests", file, len(invariantLinks))
	}

	return map[string]interface{}{
		"file":           file,
		"invariants":     invariantLinks,
		"edit_warnings":  allEditWarnings,
		"required_tests": allRequiredTests,
		"warning":        warning,
	}, nil
}
