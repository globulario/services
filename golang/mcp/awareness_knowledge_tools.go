package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/globulario/awareness/knowledge"
)

// registerAwarenessKnowledgeTools registers lean knowledge tools backed by the
// standalone github.com/globulario/awareness module: assurance (coverage
// report) and selfcheck (orphan/stale/unproven knowledge audit).
func registerAwarenessKnowledgeTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness_assurance",
		Description: "Return a coverage report over the hand-authored awareness knowledge base (invariants, decisions, failure modes, forbidden assumptions, required tests, authority rules, subsystem boundaries, remediation contracts). Counts each knowledge type and flags missing subsystem boundaries, unproven critical invariants, and untested failure modes.",
		InputSchema: inputSchema{Type: "object"},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			msg := "failed to load knowledge base"
			if err != nil {
				msg += ": " + err.Error()
			}
			return map[string]interface{}{"error": msg}, nil
		}
		rep := knowledge.Assurance(base)
		return map[string]interface{}{
			"counts": map[string]int{
				"invariants":            rep.Counts.Invariants,
				"failure_modes":         rep.Counts.FailureModes,
				"forbidden_fixes":       rep.Counts.ForbiddenFixes,
				"incident_patterns":     rep.Counts.IncidentPatterns,
				"decisions":             rep.Counts.Decisions,
				"forbidden_assumptions": rep.Counts.ForbiddenAssumptions,
				"required_tests":        rep.Counts.RequiredTests,
				"subsystem_boundaries":  rep.Counts.SubsystemBoundaries,
				"authority_rules":       rep.Counts.AuthorityRules,
				"preflight_questions":   rep.Counts.PreflightQuestions,
				"remediation_contracts": rep.Counts.RemediationContracts,
			},
			"subsystems_named": rep.SubsystemsNamed,
			"warnings":         rep.Warnings,
			"lines":            rep.Lines(),
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness_selfcheck",
		Description: "Audit the hand-authored awareness knowledge base for orphan records, unproven critical invariants, stale entries, missing cross-references between decisions/invariants/failure modes/forbidden assumptions. Returns ok=true when no errors are found (warnings are still reported).",
		InputSchema: inputSchema{Type: "object"},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			msg := "failed to load knowledge base"
			if err != nil {
				msg += ": " + err.Error()
			}
			return map[string]interface{}{"error": msg}, nil
		}
		rep := knowledge.Selfcheck(base)
		return map[string]interface{}{
			"ok":       rep.OK,
			"errors":   rep.Errors,
			"warnings": rep.Warnings,
			"summary":  rep.String(),
		}, nil
	})

	// ------------------------------------------------------------------ //
	// awareness_decision_lookup
	// ------------------------------------------------------------------ //
	s.register(toolDef{
		Name:        "awareness_decision_lookup",
		Description: "Look up architectural decisions from the hand-authored knowledge base. Pass a keyword or decision ID to filter; omit query to return all decisions. Matches against ID, Title, and Because entries (case-insensitive).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {Type: "string", Description: "Keyword or decision ID to search for (optional — omit to list all)"},
			},
		},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			return map[string]interface{}{"error": "knowledge base unavailable"}, nil
		}
		query := knowledgeQueryArg(args)
		var results []map[string]interface{}
		for _, d := range base.Decisions {
			if query == "" || knowledgeDecisionMatches(d, query) {
				results = append(results, map[string]interface{}{
					"id":                    d.ID,
					"title":                 d.Title,
					"status":                d.Status,
					"because":               d.Because,
					"protects_invariants":   d.ProtectsInvariants,
					"related_failure_modes": d.RelatedFailureModes,
					"forbidden_fixes":       d.ForbiddenFixes,
				})
			}
		}
		if results == nil {
			results = []map[string]interface{}{}
		}
		return map[string]interface{}{"results": results, "count": len(results)}, nil
	})

	// ------------------------------------------------------------------ //
	// awareness_forbidden_assumption_lookup
	// ------------------------------------------------------------------ //
	s.register(toolDef{
		Name:        "awareness_forbidden_assumption_lookup",
		Description: "Look up forbidden assumptions from the knowledge base — beliefs that have historically led to incidents. Pass a keyword or ID; omit to list all. Matches against ID and Statement (case-insensitive).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {Type: "string", Description: "Keyword or assumption ID to search for (optional)"},
			},
		},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			return map[string]interface{}{"error": "knowledge base unavailable"}, nil
		}
		query := knowledgeQueryArg(args)
		var results []map[string]interface{}
		for _, fa := range base.ForbiddenAssumptions {
			if query == "" || knowledgeForbiddenAssumptionMatches(fa, query) {
				results = append(results, map[string]interface{}{
					"id":                 fa.ID,
					"statement":          fa.Statement,
					"why_wrong":          fa.WhyWrong,
					"safer_checks":       fa.SaferChecks,
					"caused_failures":    fa.CausedFailures,
					"related_invariants": fa.RelatedInvariants,
				})
			}
		}
		if results == nil {
			results = []map[string]interface{}{}
		}
		return map[string]interface{}{"results": results, "count": len(results)}, nil
	})

	// ------------------------------------------------------------------ //
	// awareness_authority_lookup
	// ------------------------------------------------------------------ //
	s.register(toolDef{
		Name:        "awareness_authority_lookup",
		Description: "Look up data-authority rules — which layer (Repository/Desired/Installed/Runtime) owns a given piece of state. Useful before reading or writing cluster state to avoid layer-authority violations. Pass a layer name, question keyword, or rule ID.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {Type: "string", Description: "Layer name (e.g. 'desired'), keyword from the question, or rule ID (optional)"},
			},
		},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			return map[string]interface{}{"error": "knowledge base unavailable"}, nil
		}
		query := knowledgeQueryArg(args)
		var results []map[string]interface{}
		for _, ar := range base.AuthorityRules {
			if query == "" || knowledgeAuthorityRuleMatches(ar, query) {
				results = append(results, map[string]interface{}{
					"id":                 ar.ID,
					"title":              ar.Title,
					"layer":              ar.Layer,
					"question":           ar.Question,
					"rule":               ar.Rule,
					"wrong_authority":    ar.WrongAuthority,
					"correct_authority":  ar.CorrectAuthority,
					"related_invariants": ar.RelatedInvariants,
				})
			}
		}
		if results == nil {
			results = []map[string]interface{}{}
		}
		return map[string]interface{}{"results": results, "count": len(results)}, nil
	})

	// ------------------------------------------------------------------ //
	// awareness_required_tests
	// ------------------------------------------------------------------ //
	s.register(toolDef{
		Name:        "awareness_required_tests",
		Description: "Return tests that must pass before committing a change. Filter by task description (keyword matching against task_terms) and/or changed file paths (glob matching against required_for_changes paths). If neither is provided, returns all required tests.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task":          {Type: "string", Description: "Human-readable task description — words are matched against required_for_changes.task_terms (optional)"},
				"changed_files": {Type: "array", Description: "List of file paths that were changed — matched against required_for_changes.paths globs (optional)"},
			},
		},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			return map[string]interface{}{"error": "knowledge base unavailable"}, nil
		}
		task := ""
		if v, ok := args["task"].(string); ok {
			task = v
		}
		var changedFiles []string
		if v, ok := args["changed_files"].([]interface{}); ok {
			for _, f := range v {
				if s, ok := f.(string); ok {
					changedFiles = append(changedFiles, s)
				}
			}
		}
		filterActive := task != "" || len(changedFiles) > 0
		var results []map[string]interface{}
		for _, rt := range base.RequiredTests {
			if !filterActive || knowledgeRequiredTestMatches(rt, task, changedFiles) {
				results = append(results, map[string]interface{}{
					"id":    rt.ID,
					"title": rt.Title,
					"protects": map[string]interface{}{
						"invariants":    rt.Protects.Invariants,
						"failure_modes": rt.Protects.FailureModes,
					},
					"required_for_changes": map[string]interface{}{
						"paths":      rt.RequiredForChanges.Paths,
						"task_terms": rt.RequiredForChanges.TaskTerms,
					},
					"commands": rt.Commands,
					"evidence": rt.Evidence,
				})
			}
		}
		if results == nil {
			results = []map[string]interface{}{}
		}
		return map[string]interface{}{"results": results, "count": len(results)}, nil
	})

	// ------------------------------------------------------------------ //
	// awareness_remediation_lookup
	// ------------------------------------------------------------------ //
	s.register(toolDef{
		Name:        "awareness_remediation_lookup",
		Description: "Look up remediation contracts — pre-approved and forbidden actions for known failure modes. Pass a failure mode ID or keyword to filter; omit to list all contracts.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {Type: "string", Description: "Failure mode ID, contract ID, or keyword (optional)"},
			},
		},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		base, err := loadServicesKnowledgeBase(st)
		if err != nil || base == nil {
			return map[string]interface{}{"error": "knowledge base unavailable"}, nil
		}
		query := knowledgeQueryArg(args)
		var results []map[string]interface{}
		for _, rc := range base.RemediationContracts {
			if query == "" || knowledgeRemediationContractMatches(rc, query) {
				results = append(results, map[string]interface{}{
					"id":                      rc.ID,
					"title":                   rc.Title,
					"when_failure_modes":       rc.When.FailureModes,
					"allowed_actions":         rc.AllowedActions,
					"forbidden_actions":       rc.ForbiddenActions,
					"requires_human_approval": rc.RequiresHumanApproval,
				})
			}
		}
		if results == nil {
			results = []map[string]interface{}{}
		}
		return map[string]interface{}{"results": results, "count": len(results)}, nil
	})
}

// loadServicesKnowledgeBase loads the Globular knowledge base using the
// standalone knowledge.LoadFromPaths, picking up both core and extended YAML
// files from the docs/awareness directory.
func loadServicesKnowledgeBase(st *awarenessState) (*knowledge.Base, error) {
	dir := st.docsDir
	if dir == "" {
		return nil, nil
	}
	j := func(name string) string { return filepath.Join(dir, name) }

	invariants := existingPaths(j("invariants.yaml"), j("convergence_rules.yaml"))
	failureModes := existingPaths(j("failure_modes.yaml"))
	forbiddenFixes := existingPaths(j("forbidden_fixes.yaml"))

	return knowledge.LoadFromPaths(invariants, failureModes, forbiddenFixes, nil, dir)
}

// existingPaths returns only non-empty paths (the caller filters absent files
// via LoadFromPaths's graceful skip-on-error behaviour).
func existingPaths(paths ...string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// toolResultJSON is a helper for producing a JSON string from a value.
func toolResultJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// knowledgeQueryArg extracts the "query" string argument from an MCP args map.
func knowledgeQueryArg(args map[string]interface{}) string {
	if v, ok := args["query"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// knowledgeContains reports whether s contains substr, case-insensitively.
func knowledgeContains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// knowledgeDecisionMatches reports whether a Decision matches query.
// It checks ID, Title, and each Because entry.
func knowledgeDecisionMatches(d knowledge.Decision, query string) bool {
	if knowledgeContains(d.ID, query) || knowledgeContains(d.Title, query) {
		return true
	}
	for _, b := range d.Because {
		if knowledgeContains(b, query) {
			return true
		}
	}
	return false
}

// knowledgeForbiddenAssumptionMatches reports whether a ForbiddenAssumption
// matches query by ID or Statement.
func knowledgeForbiddenAssumptionMatches(fa knowledge.ForbiddenAssumption, query string) bool {
	return knowledgeContains(fa.ID, query) || knowledgeContains(fa.Statement, query)
}

// knowledgeAuthorityRuleMatches reports whether an AuthorityRule matches query
// by ID, Layer, or Question (case-insensitive).
func knowledgeAuthorityRuleMatches(ar knowledge.AuthorityRule, query string) bool {
	return knowledgeContains(ar.ID, query) ||
		knowledgeContains(ar.Layer, query) ||
		knowledgeContains(ar.Question, query)
}

// knowledgeRequiredTestMatches reports whether a RequiredTest matches the given
// task description or list of changed file paths.
//
//   - task words are checked against RequiredForChanges.TaskTerms (case-insensitive substring)
//   - changed files are checked against RequiredForChanges.Paths via filepath.Match
func knowledgeRequiredTestMatches(rt knowledge.RequiredTest, task string, changedFiles []string) bool {
	if task != "" {
		words := strings.Fields(strings.ToLower(task))
		for _, word := range words {
			for _, term := range rt.RequiredForChanges.TaskTerms {
				if knowledgeContains(term, word) {
					return true
				}
			}
		}
	}
	for _, file := range changedFiles {
		for _, pattern := range rt.RequiredForChanges.Paths {
			if matched, err := filepath.Match(pattern, file); err == nil && matched {
				return true
			}
		}
	}
	return false
}

// knowledgeRemediationContractMatches reports whether a RemediationContract
// matches query by ID, Title, or any failure mode in When.FailureModes.
func knowledgeRemediationContractMatches(rc knowledge.RemediationContract, query string) bool {
	if knowledgeContains(rc.ID, query) || knowledgeContains(rc.Title, query) {
		return true
	}
	for _, fm := range rc.When.FailureModes {
		if knowledgeContains(fm, query) {
			return true
		}
	}
	return false
}
