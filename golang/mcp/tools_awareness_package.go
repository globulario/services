package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/extractors/packages"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/learning"
)

func registerAwarenessPackageTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.validate_package",
		Description: "Validate a package's awareness.yaml contract against the admission rules and main graph. Returns ADMIT / WARN / BLOCK with full rule-by-rule reasoning.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Path to the package directory containing awareness.yaml",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		path := strArg(args, "path")
		if path == "" {
			return nil, fmt.Errorf("path is required")
		}

		contract, err := packages.LoadAwarenessContract(path)
		if err != nil {
			return nil, fmt.Errorf("load contract: %w", err)
		}

		packageKind := ""
		if contract != nil {
			packageKind = contract.PackageKind
		}

		if st.g == nil {
			return map[string]interface{}{
				"status":  "SKIPPED",
				"reasons": []string{"no graph DB — run 'globular awareness build' first"},
			}, nil
		}

		result, err := analysis.ValidatePackage(ctx, contract, packageKind, st.g)
		if err != nil {
			return nil, fmt.Errorf("validate: %w", err)
		}

		reasons := make([]string, 0, len(result.Reasons))
		for _, r := range result.Reasons {
			reasons = append(reasons, r.Message)
		}

		return map[string]interface{}{
			"status":               string(result.Status),
			"reasons":              reasons,
			"impacted_invariants":  result.ImpactedInvariants,
			"forbidden_fixes":      result.ForbiddenFixesFound,
			"required_tests":       result.RequiredTests,
			"missing_tests":        result.MissingTests,
			"required_workflows":   result.RequiredWorkflows,
			"missing_workflows":    result.MissingWorkflows,
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.package_context",
		Description: "Generate architectural context for a package from its awareness.yaml. Returns invariants, failure modes, and forbidden fixes related to the package's declared services and dependencies.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Path to the package directory containing awareness.yaml",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		path := strArg(args, "path")
		if path == "" {
			return nil, fmt.Errorf("path is required")
		}

		contract, err := packages.LoadAwarenessContract(path)
		if err != nil {
			return nil, fmt.Errorf("load contract: %w", err)
		}
		if contract == nil {
			return map[string]interface{}{
				"warnings": []string{"no awareness.yaml found in " + path},
			}, nil
		}

		if st.g == nil {
			return map[string]interface{}{
				"package": contract.Package, "service": contract.Service,
				"warnings": []string{"no graph DB — run 'globular awareness build' first"},
			}, nil
		}

		task := fmt.Sprintf("package %s service %s kind %s: %s",
			contract.Package, contract.Service, contract.PackageKind, contract.Summary)

		hints := analysis.AgentContextHints{Services: []string{contract.Service}}
		for _, dep := range contract.DependsOn {
			hints.Services = append(hints.Services, dep.Service)
		}

		var aliasMap learning.ContextAliasMap
		if st.docsDir != "" {
			aliasMap, _ = learning.LoadContextAliases(st.docsDir + "/context_aliases.yaml")
		}

		_, result, err := analysis.GenerateAgentContext(ctx, st.g, task, hints, analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"package":           contract.Package,
			"service":           contract.Service,
			"kind":              contract.PackageKind,
			"invariants":        result.InvariantIDs,
			"failure_modes":     result.FailureModeIDs,
			"forbidden_fixes":   result.ForbiddenFixes,
			"required_tests":    result.RequiredTests,
			"required_searches": result.RequiredSearches,
			"services":          result.ServiceNames,
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.impact_file",
		Description: "Show all graph nodes impacted by changes to a file — services, invariants, failure modes, forbidden fixes, and required tests. Returns coverage and blind spots when no path is found.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "File path (relative to repo root) to analyse",
				},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file := strArg(args, "file")
		if file == "" {
			return nil, fmt.Errorf("file is required")
		}

		if st.g == nil {
			return map[string]interface{}{
				"file":               file,
				"risk":               "unknown",
				"classification":     "UNKNOWN_IMPACT",
				"confidence":         "unknown",
				"confidence_reason":  "graph DB not available — no architecture facts can be matched; run 'globular awareness build' first",
				"coverage": map[string]string{
					"graph":    "not_checked",
					"raw_yaml": "not_checked",
					"runtime":  "noop",
				},
				"blind_spots": []string{
					"graph unavailable — run 'globular awareness build' first",
					"runtime not collected",
				},
				"recommended_next_action": "Run 'globular awareness build' to index the codebase, then retry.",
				"trust":                  awarenessTrustMap(st, false),
			}, nil
		}

		result, err := analysis.ImpactByFile(ctx, st.g, file)
		if err != nil {
			return nil, err
		}

		names := func(nodes []*graph.Node) []string {
			out := make([]string, 0, len(nodes))
			for _, n := range nodes {
				out = append(out, n.Name)
			}
			return out
		}

		found := result.SourceFile != nil
		invariants := names(result.Invariants)
		failureModes := names(result.FailureModes)
		forbiddenFixes := names(result.ForbiddenFixes)
		tests := names(result.Tests)
		services := names(result.Services)

		hasMatches := len(invariants)+len(failureModes)+len(forbiddenFixes)+len(tests) > 0

		var graphCov, risk, classification, confidence, confidenceReason string
		var blindSpots []string
		var recommendedAction string

		if !found {
			graphCov = "checked_clean"
			risk = "unknown"
			classification = "NO_KNOWN_STATIC_MATCH"
			confidence = "low"
			confidenceReason = "File not indexed in graph — no graph path from file to invariants or failure modes."
			blindSpots = []string{
				"No graph node for this file — run 'globular awareness build' after adding/renaming files.",
				"No implementation edge indexed for this file.",
				"Runtime not collected.",
			}
			recommendedAction = "Run 'globular awareness build', then retry. Also run awareness.scan_violations on this path."
		} else if !hasMatches {
			graphCov = "checked_clean"
			risk = "low"
			classification = "NO_KNOWN_STATIC_MATCH"
			confidence = "low"
			confidenceReason = "Graph found the file but no paths reach invariants, failure modes, or required tests. This does not prove the file is safe — graph traversal may be incomplete."
			blindSpots = []string{
				"No graph path from file to invariants.",
				"Runtime not collected.",
				"Code scan not run — use awareness.scan_violations.",
			}
			recommendedAction = "Run awareness.scan_violations and inspect package ownership manually."
		} else {
			graphCov = "checked_with_matches"
			risk = "high"
			if len(invariants)+len(forbiddenFixes) > 2 {
				risk = "high"
			} else {
				risk = "medium"
			}
			classification = "KNOWN_IMPACT"
			confidence = "medium"
			confidenceReason = "Graph found paths to invariants/failure modes. Runtime not collected — live cluster state not verified."
			blindSpots = []string{
				"Runtime not collected — no live cluster evidence.",
				"Code scan not run — use awareness.scan_violations.",
			}
			recommendedAction = "Run listed required tests before committing. Run awareness.scan_violations on changed paths."
		}

		return map[string]interface{}{
			"file":                    file,
			"found_in_graph":          found,
			"risk":                    risk,
			"classification":          classification,
			"confidence":              confidence,
			"confidence_reason":       confidenceReason,
			"services":                services,
			"invariants":              invariants,
			"failure_modes":           failureModes,
			"forbidden_fixes":         forbiddenFixes,
			"required_tests":          tests,
			"coverage": map[string]string{
				"graph":    graphCov,
				"raw_yaml": "not_checked",
				"runtime":  "noop",
			},
			"blind_spots":             blindSpots,
			"recommended_next_action": recommendedAction,
			"trust":                   trustFromConfidenceCoverage(st, confidence, graphCov, hasMatches, blindSpots),
		}, nil
	})
}
