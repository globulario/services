package preflight

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"os"

	"gopkg.in/yaml.v3"
)

var rawTokenRE = regexp.MustCompile(`[a-zA-Z0-9_./:-]+`)

var rawStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "that": true,
	"this": true, "into": true, "when": true, "where": true, "what": true, "will": true,
	"make": true, "fix": true, "code": true, "file": true, "tool": true, "safe": true,
	"module": true, "awareness": true, "globular": true, "service": true, "services": true,
}

// RawKnowledgeFallback is the exported version of rawKnowledgeFallback.
// It scans hand-authored awareness YAML files directly for MCP tools and external callers.
func RawKnowledgeFallback(task string, files []string, docsDir string) []RawKnowledgeMatch {
	return rawKnowledgeFallback(task, files, docsDir)
}

// rawKnowledgeFallback scans hand-authored awareness YAML files directly.
// It is intentionally simple and deterministic: if the graph query misses,
// this gives the agent a second lantern before it walks into the cave.
func rawKnowledgeFallback(task string, files []string, docsDir string) []RawKnowledgeMatch {
	if strings.TrimSpace(docsDir) == "" {
		return nil
	}
	terms := rawSearchTerms(task, files)
	if len(terms) == 0 {
		return nil
	}

	candidates := []struct{ file, kind, listKey string }{
		{"failure_modes.yaml", "failure_mode", "failure_modes"},
		{"invariants.yaml", "invariant", "invariants"},
		{"convergence_rules.yaml", "invariant", "invariants"},
		{"forbidden_fixes.yaml", "forbidden_fix", "forbidden_fixes"},
		{"incident_patterns.yaml", "incident_pattern", "incident_patterns"},
		{"design_patterns.yaml", "design_pattern", "design_patterns"},
		{"patterns.yaml", "pattern", "patterns"},
		// Extended knowledge types (missing pieces).
		{"decisions.yaml", "decision", "decisions"},
		{"forbidden_assumptions.yaml", "forbidden_assumption", "forbidden_assumptions"},
		{"required_tests.yaml", "required_test", "required_tests"},
		{"authority_rules.yaml", "authority_rule", "authority_rules"},
		{"subsystem_boundaries.yaml", "subsystem_boundary", "subsystem_boundaries"},
		{"preflight_questions.yaml", "preflight_question", "preflight_questions"},
		{"remediation_contracts.yaml", "remediation_contract", "remediation_contracts"},
	}

	var out []RawKnowledgeMatch
	for _, c := range candidates {
		path := filepath.Join(docsDir, c.file)
		data, err := os.ReadFile(path)
		if err != nil || len(bytes.TrimSpace(data)) == 0 {
			continue
		}
		var root map[string]interface{}
		if err := yaml.Unmarshal(data, &root); err != nil {
			continue
		}
		items, _ := root[c.listKey].([]interface{})
		for _, item := range items {
			m, _ := item.(map[string]interface{})
			if len(m) == 0 {
				continue
			}
			id, _ := m["id"].(string)
			if id == "" {
				// subsystem_boundaries use "subsystem" as the identifier.
				id, _ = m["subsystem"].(string)
			}
			blobBytes, _ := yaml.Marshal(item)
			blob := strings.ToLower(string(blobBytes))
			matched := make([]string, 0)
			for _, term := range terms {
				if strings.Contains(blob, strings.ToLower(term)) {
					matched = append(matched, term)
				}
			}
			if len(matched) == 0 {
				continue
			}
			score := len(matched)
			// Strong bump when a file path or dotted invariant/failure-mode ID matched.
			for _, mt := range matched {
				if strings.Contains(mt, "/") || strings.Contains(mt, ".") || strings.Contains(mt, "_") {
					score++
				}
			}
			out = append(out, RawKnowledgeMatch{
				Source:       c.file,
				Kind:         c.kind,
				ID:           id,
				Score:        score,
				MatchedTerms: uniqueStrings(matched),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func rawSearchTerms(task string, files []string) []string {
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.Trim(strings.ToLower(s), " \t\n\r,.;()[]{}'\"")
		if len(s) < 3 || rawStopWords[s] || seen[s] {
			return
		}
		seen[s] = true
	}
	for _, t := range rawTokenRE.FindAllString(task, -1) {
		add(t)
		if strings.Contains(t, ".") || strings.Contains(t, "_") || strings.Contains(t, ":") {
			for _, part := range regexp.MustCompile(`[._:/-]+`).Split(t, -1) {
				add(part)
			}
		}
	}
	for _, f := range files {
		f = filepath.ToSlash(f)
		add(f)
		add(filepath.Base(f))
		parts := strings.FieldsFunc(f, func(r rune) bool { return r == '/' || r == '-' || r == '_' || r == '.' })
		for _, part := range parts {
			add(part)
		}
	}
	terms := make([]string, 0, len(seen))
	for t := range seen {
		terms = append(terms, t)
	}
	sort.Strings(terms)
	return terms
}

func mergeRawKnowledgeMatches(r *Report, matches []RawKnowledgeMatch) *Report {
	for _, m := range matches {
		if m.ID == "" || m.Score < 2 {
			continue
		}
		switch m.Kind {
		case "invariant":
			r.Invariants = append(r.Invariants, m.ID)
		case "failure_mode":
			r.FailureModes = append(r.FailureModes, m.ID)
		case "forbidden_fix":
			r.ForbiddenFixes = append(r.ForbiddenFixes, m.ID)
		case "decision":
			r.MatchedDecisions = append(r.MatchedDecisions, m.ID)
		case "forbidden_assumption":
			r.MatchedForbiddenAssumptions = append(r.MatchedForbiddenAssumptions, m.ID)
		case "required_test":
			r.RequiredTests = append(r.RequiredTests, m.ID)
		case "authority_rule":
			r.MatchedAuthorityRules = append(r.MatchedAuthorityRules, m.ID)
		case "preflight_question":
			r.MatchedPreflightQuestions = append(r.MatchedPreflightQuestions, m.ID)
		case "remediation_contract":
			r.MatchedRemediationContracts = append(r.MatchedRemediationContracts, m.ID)
		}
	}
	r.Invariants = unique(r.Invariants)
	r.FailureModes = unique(r.FailureModes)
	r.ForbiddenFixes = unique(r.ForbiddenFixes)
	r.MatchedDecisions = unique(r.MatchedDecisions)
	r.MatchedForbiddenAssumptions = unique(r.MatchedForbiddenAssumptions)
	r.RequiredTests = unique(r.RequiredTests)
	r.MatchedAuthorityRules = unique(r.MatchedAuthorityRules)
	r.MatchedPreflightQuestions = unique(r.MatchedPreflightQuestions)
	r.MatchedRemediationContracts = unique(r.MatchedRemediationContracts)
	return r
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
