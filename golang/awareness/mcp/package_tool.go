package mcp

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/extractors/packages"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/learning"
)

func registerPackageTools(s *Server) {
	registerValidatePackageTool(s)
	registerPackageContextTool(s)
	registerImpactFileTool(s)
}

func registerValidatePackageTool(s *Server) {
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

		if s.g == nil {
			return map[string]interface{}{
				"status":  "SKIPPED",
				"reasons": []string{"no graph DB — run 'globular awareness build' first"},
			}, nil
		}

		result, err := analysis.ValidatePackage(ctx, contract, packageKind, s.g)
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
}

func registerPackageContextTool(s *Server) {
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

		if s.g == nil {
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

		docsDir := s.resolvedDocsDir()
		var aliasMap learning.ContextAliasMap
		if docsDir != "" {
			aliasMap, _ = learning.LoadContextAliases(docsDir + "/context_aliases.yaml")
		}

		_, result, err := analysis.GenerateAgentContext(ctx, s.g, task, hints, analysis.AgentContextAliases(aliasMap))
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
}

func registerImpactFileTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.impact_file",
		Description: "Show all graph nodes impacted by changes to a file — services, invariants, failure modes, forbidden fixes, and required tests.",
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

		if s.g == nil {
			return map[string]interface{}{
				"file":     file,
				"warnings": []string{"no graph DB — run 'globular awareness build' first"},
			}, nil
		}

		result, err := analysis.ImpactByFile(ctx, s.g, file)
		if err != nil {
			return nil, err
		}

		found := result.SourceFile != nil

		names := func(nodes []*graph.Node) []string {
			out := make([]string, 0, len(nodes))
			for _, n := range nodes {
				out = append(out, n.Name)
			}
			return out
		}

		return map[string]interface{}{
			"file":            file,
			"found_in_graph":  found,
			"services":        names(result.Services),
			"invariants":      names(result.Invariants),
			"failure_modes":   names(result.FailureModes),
			"forbidden_fixes": names(result.ForbiddenFixes),
			"tests":           names(result.Tests),
		}, nil
	})
}
