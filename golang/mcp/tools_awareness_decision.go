package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/integrity"
	"github.com/globulario/services/golang/awareness/semantic"
)

func registerAwarenessDecisionTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.decision_context",
		Description: `Return ranked decision paths before editing Globular code.

Given a goal, changed files, symptoms, or services, returns:
- Top decision paths (what to obey before editing)
- Forbidden actions from the decision graph
- Required tests that close the loop
- Coverage and blind spots

Use this tool BEFORE editing any file in:
  awareness, cluster_controller, workflow, node_agent, repository, xds,
  runtime, MCP tools, graph, integrity, or scan packages.

A high score means "best known path", not "guaranteed safe".
NO_MATCH does not mean safe — always check coverage and blind_spots.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"goal": {
					Type:        "string",
					Description: "What you are about to do (required)",
				},
				"changed_files": {
					Type:        "array",
					Description: "Files you plan to edit",
					Items:       &propSchema{Type: "string"},
				},
				"symptoms": {
					Type:        "array",
					Description: "Error messages, log lines, or observable symptoms",
					Items:       &propSchema{Type: "string"},
				},
				"services": {
					Type:        "array",
					Description: "Service names involved",
					Items:       &propSchema{Type: "string"},
				},
				"include_information_paths": {
					Type:        "boolean",
					Description: "Include information-domain paths in output (default: false — decision paths only)",
					Default:     false,
				},
				"max_paths": {
					Type:        "number",
					Description: "Maximum paths to return (default: 10)",
					Default:     10,
				},
				"min_score": {
					Type:        "number",
					Description: "Minimum score threshold; paths below this are excluded (default: -50)",
					Default:     -50,
				},
			},
			Required: []string{"goal"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		goal := strArg(args, "goal")
		changedFiles := strSliceArg(args, "changed_files")
		symptoms := strSliceArg(args, "symptoms")
		services := strSliceArg(args, "services")
		includeInfo := boolArg(args, "include_information_paths")
		maxPaths := intArgDefault(args, "max_paths", 10)
		minScore := floatArgDefault(args, "min_score", -50)

		if st.g == nil {
			return decisionContextNoGraph(goal, changedFiles), nil
		}

		return buildDecisionContext(ctx, st, goal, changedFiles, symptoms, services, includeInfo, maxPaths, minScore)
	})
}

// decisionContextNoGraph returns a degraded but coverage-rich result when the
// graph DB is unavailable. Never returns bare NO_MATCH.
func decisionContextNoGraph(goal string, files []string) map[string]interface{} {
	return map[string]interface{}{
		"goal":       goal,
		"summary":    "Graph unavailable — decision context degraded. Run 'globular awareness build' before editing.",
		"confidence": "unknown",
		"coverage": map[string]string{
			"graph":          "not_checked",
			"raw_yaml":       "not_checked",
			"runtime":        "noop",
			"code_scan":      "not_checked",
		},
		"top_decision_paths": []interface{}{},
		"information_paths":  []interface{}{},
		"forbidden_actions":  []string{"Cannot determine — graph not available"},
		"required_tests":     []string{},
		"blind_spots": []string{
			"graph unavailable — run 'globular awareness build' first",
			fmt.Sprintf("cannot check %d file(s) for decision paths", len(files)),
			"runtime not collected",
		},
		"recommended_next_action": "Run 'globular awareness build' to index the codebase, then retry awareness.decision_context.",
		"warning":                 "NO_MATCH does not mean safe — the graph must be available before editing high-risk files",
		"trust": map[string]interface{}{
			"verdict":         "unknown",
			"confidence":      "none",
			"freshness":       "unknown",
			"coverage":        "none",
			"limitations":     []string{"graph unavailable — decision context cannot be verified"},
			"required_action": []string{"run 'globular awareness build' and retry awareness.decision_context"},
		},
	}
}

// buildDecisionContext queries the graph and returns ranked decision paths.
func buildDecisionContext(
	ctx context.Context,
	st *awarenessState,
	goal string,
	changedFiles []string,
	symptoms []string,
	services []string,
	includeInfo bool,
	maxPaths int,
	minScore float64,
) (map[string]interface{}, error) {
	g := st.g

	// 1. Traverse impact paths for changed files.
	var allPaths []integrity.ImpactPath

	if len(changedFiles) > 0 {
		q := integrity.ImpactPathQuery{ChangedFiles: changedFiles, MaxDepth: 6}
		filePaths, err := integrity.TraverseImpactPaths(ctx, g, q)
		if err != nil {
			return nil, fmt.Errorf("impact path traversal: %w", err)
		}
		allPaths = append(allPaths, filePaths...)
	}

	// 2. Also query by symptoms and services if provided.
	if len(symptoms) > 0 || len(services) > 0 {
		symptomPaths := traverseBySymptoms(ctx, g, symptoms, services)
		allPaths = append(allPaths, symptomPaths...)
	}

	// 3. Build scoring context.
	changedFileSet := make(map[string]bool)
	for _, f := range changedFiles {
		changedFileSet[f] = true
	}
	fresh := g.Freshness(ctx, st.docsDir)
	scoringCtx := semantic.ScoringContext{
		ChangedFilePaths: changedFileSet,
		GraphIsStale:     fresh.Stale,
		RuntimeIsNoop:    true, // runtime not collected in this tool
	}

	// 4. Score and rank.
	scored := semantic.RankPaths(allPaths, scoringCtx)

	// 5. Stratify by trust.
	strata := semantic.StratifyByTrust(scored)

	// 6. Filter and classify.
	var decisionPaths []interface{}
	var infoPaths []interface{}
	var allForbidden []string
	var allRequiredTests []string
	seenForbidden := map[string]bool{}
	seenTests := map[string]bool{}

	for _, sp := range scored {
		if sp.Score.Total < minScore {
			continue
		}
		if len(decisionPaths)+len(infoPaths) >= maxPaths {
			break
		}

		entry := buildPathEntry(sp)

		// Collect forbidden actions and required tests.
		for _, fa := range semantic.ForbiddenActionNames(sp.Steps) {
			if !seenForbidden[fa] {
				allForbidden = append(allForbidden, fa)
				seenForbidden[fa] = true
			}
		}
		for _, test := range semantic.RequiredTestNames(sp.Steps) {
			if !seenTests[test] {
				allRequiredTests = append(allRequiredTests, test)
				seenTests[test] = true
			}
		}

		if sp.Score.DecisionRelevant || sp.Score.RiskRelevant {
			decisionPaths = append(decisionPaths, entry)
		} else if includeInfo {
			infoPaths = append(infoPaths, entry)
		}
	}

	// 7. Coverage.
	graphCoverage := "not_checked"
	if len(allPaths) > 0 {
		if len(decisionPaths) > 0 || len(infoPaths) > 0 {
			graphCoverage = "checked_with_matches"
		} else {
			graphCoverage = "checked_clean"
		}
	}

	// 8. Confidence.
	confidence := deriveConfidence(scored, scoringCtx)

	// 9. Blind spots.
	blindSpots := buildDecisionBlindSpots(changedFiles, symptoms, services, scoringCtx, len(allPaths))

	// 10. Summary.
	summary := buildDecisionSummary(goal, len(decisionPaths), len(allForbidden), confidence)

	// 11. Trust breakdown for transparency.
	trustBreakdown := map[string]int{}
	for trust, bucket := range strata {
		trustBreakdown[trust] = len(bucket)
	}

	// 12. Decision-only causal chain traversal.
	// Traverse from the first matched file's graph node following only
	// decision-class edges (blocks, requires, forbids, violates, enforces, etc.).
	// This gives a tight causal chain independent of the scoring machinery above.
	decisionTraversalNodes := buildDecisionTraversalNodes(ctx, g, changedFiles, symptoms, services, 4)

	return map[string]interface{}{
		"goal":       goal,
		"summary":    summary,
		"confidence": confidence,
		"coverage": map[string]string{
			"graph":        graphCoverage,
			"raw_yaml":     "not_checked",
			"runtime":      "noop",
			"code_scan":    "not_checked",
			"class_filter": "decision",
		},
		"top_decision_paths":       decisionPaths,
		"information_paths":        infoPaths,
		"forbidden_actions":        allForbidden,
		"required_tests":           allRequiredTests,
		"trust_breakdown":          trustBreakdown,
		"blind_spots":              blindSpots,
		"decision_causal_chain":    decisionTraversalNodes,
		"warning":                  "NO_MATCH does not mean safe — check coverage.graph and blind_spots",
		"trust":                    trustFromConfidenceCoverage(st, confidence, graphCoverage, len(decisionPaths)+len(infoPaths) > 0, blindSpots),
	}, nil
}

// buildDecisionTraversalNodes uses g.TraverseDecision to walk decision-class
// edges from the provided starting points and returns a compact node list.
// This is additive — it does not replace the scored path output but supplements
// it with a pure decision-graph view.
func buildDecisionTraversalNodes(
	ctx context.Context,
	g *graph.Graph,
	changedFiles, symptoms, services []string,
	maxDepth int,
) []map[string]interface{} {
	// Build a list of starting node IDs to traverse from.
	var startIDs []string

	// Resolved file → node ID for changed files.
	for _, f := range changedFiles {
		nodes, err := g.FindNodesByPath(ctx, f)
		if err == nil && len(nodes) > 0 {
			startIDs = append(startIDs, nodes[0].ID)
		}
	}

	// Service name → service node IDs.
	for _, svc := range services {
		if svcNodes, err := g.FindNodesByNameLike(ctx, svc); err == nil {
			for _, n := range svcNodes {
				if n.Type == graph.NodeTypeGlobularService || n.Type == "package" {
					startIDs = append(startIDs, n.ID)
					break
				}
			}
		}
	}

	if len(startIDs) == 0 {
		return nil
	}

	// Traverse from the first start ID (most relevant anchor point).
	result, err := g.TraverseDecision(ctx, startIDs[0], maxDepth)
	if err != nil || result == nil {
		return nil
	}

	if len(result.Nodes) == 0 {
		return nil
	}

	out := make([]map[string]interface{}, 0, len(result.Nodes))
	for _, n := range result.Nodes {
		out = append(out, map[string]interface{}{
			"id":      n.ID,
			"type":    n.Type,
			"name":    n.Name,
			"summary": n.Summary,
		})
	}
	return out
}

// buildPathEntry converts a ScoredPath into the canonical output schema.
func buildPathEntry(sp semantic.ScoredPath) map[string]interface{} {
	stepStrings := semantic.FormatPathSteps(sp.Steps)
	required := semantic.RequiredBehavior(sp.Steps)
	forbidden := semantic.ForbiddenActionNames(sp.Steps)
	tests := semantic.RequiredTestNames(sp.Steps)

	return map[string]interface{}{
		"rank":               sp.Score.Rank,
		"score":              sp.Score.Total,
		"path_type":          sp.Score.PathType,
		"trust_level":        sp.Score.TrustLevel,
		"confidence":         semantic.BestTrustLabel(sp.Score.TrustLevel),
		"domains":            sp.Score.Domains,
		"decision_relevant":  sp.Score.DecisionRelevant,
		"risk_relevant":      sp.Score.RiskRelevant,
		"proof_available":    sp.Score.ProofAvailable,
		"path":               stepStrings,
		"required_behavior":  required,
		"forbidden_actions":  forbidden,
		"required_tests":     tests,
		"score_explanation":  sp.Score.Explanation,
		"changed_file":       sp.ChangedFile,
		"trust_description":  semantic.DescribePathTrust(sp.Score.TrustLevel),
	}
}

// traverseBySymptoms finds impact paths by matching symptoms against failure mode
// symptoms in the graph and services against globular_service nodes.
func traverseBySymptoms(ctx context.Context, g *graph.Graph, symptoms []string, services []string) []integrity.ImpactPath {
	var paths []integrity.ImpactPath

	// Find failure modes whose symptoms match the provided text.
	fmNodes, err := g.FindNodesByType(ctx, graph.NodeTypeFailureMode)
	if err != nil {
		return paths
	}
	var matchedFiles []string
	for _, fm := range fmNodes {
		fmSummary := strings.ToLower(fm.Summary)
		for _, sym := range symptoms {
			if strings.Contains(fmSummary, strings.ToLower(sym)) {
				matchedFiles = append(matchedFiles, fm.ID)
				break
			}
		}
	}

	// Find service nodes matching provided service names.
	for _, svc := range services {
		svcNodes, _ := g.FindNodesByNameLike(ctx, svc)
		for _, n := range svcNodes {
			if n.Type == graph.NodeTypeGlobularService {
				matchedFiles = append(matchedFiles, n.ID)
			}
		}
	}

	if len(matchedFiles) == 0 {
		return paths
	}

	q := integrity.ImpactPathQuery{ChangedFiles: matchedFiles, MaxDepth: 4}
	result, _ := integrity.TraverseImpactPaths(ctx, g, q)
	return result
}

// deriveConfidence returns a confidence string based on the best scored path.
func deriveConfidence(scored []semantic.ScoredPath, ctx semantic.ScoringContext) string {
	if len(scored) == 0 {
		return "unknown"
	}
	if ctx.GraphIsStale {
		return "low"
	}
	best := scored[0]
	return semantic.BestTrustLabel(best.Score.TrustLevel)
}

// buildDecisionBlindSpots returns known blind spots given the query inputs.
func buildDecisionBlindSpots(files, symptoms, services []string, ctx semantic.ScoringContext, matchCount int) []string {
	var bs []string
	if ctx.RuntimeIsNoop {
		bs = append(bs, "runtime not collected — live cluster state not verified")
	}
	if ctx.GraphIsStale {
		bs = append(bs, "graph is stale — rebuild before relying on this decision")
	}
	if matchCount == 0 && len(files) > 0 {
		bs = append(bs, fmt.Sprintf("no graph nodes found for %d file(s) — run 'globular awareness build' to index them", len(files)))
	}
	if len(symptoms) > 0 && matchCount == 0 {
		bs = append(bs, "symptoms did not match any failure mode in the graph — consider running awareness.explain_symptom")
	}
	if len(services) > 0 && matchCount == 0 {
		bs = append(bs, "services did not match any graph nodes — verify service names against the graph")
	}
	if len(bs) == 0 && matchCount == 0 {
		bs = append(bs, "no matches found — this may be a gap in the knowledge graph, not confirmation of safety")
	}
	return bs
}

// buildDecisionSummary returns a one-line summary of the decision context result.
func buildDecisionSummary(goal string, decisionCount, forbiddenCount int, confidence string) string {
	if decisionCount == 0 {
		return fmt.Sprintf("Goal %q: no decision paths found — confidence %s. Check blind_spots before proceeding.", goal, confidence)
	}
	s := fmt.Sprintf("Goal %q: %d decision path(s) found", goal, decisionCount)
	if forbiddenCount > 0 {
		s += fmt.Sprintf(", %d forbidden action(s)", forbiddenCount)
	}
	s += fmt.Sprintf(" — confidence %s", confidence)
	return s
}

// ── Argument helpers (local to this file) ────────────────────────────────────

func intArgDefault(args map[string]interface{}, key string, def int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func floatArgDefault(args map[string]interface{}, key string, def float64) float64 {
	if v, ok := args[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return def
}
