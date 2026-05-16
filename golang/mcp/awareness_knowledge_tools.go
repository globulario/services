package main

import (
	"context"
	"encoding/json"
	"path/filepath"

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
