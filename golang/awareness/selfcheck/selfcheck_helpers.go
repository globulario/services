package selfcheck

import "strings"

func normalizeSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, it := range items {
		out[normalizeToken(it)] = true
	}
	return out
}

func normalizeToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func strSet(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, s := range in {
		out[s] = true
	}
	return out
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no path") ||
		strings.Contains(err.Error(), "no route")
}

func appendIfKind(dst []string, cr CheckResult, kind CheckKind) []string {
	if cr.Kind != kind {
		return dst
	}
	if cr.Status == StatusFail && cr.Detail != "" {
		return append(dst, cr.Detail)
	}
	return dst
}

func filterMissingAliases(cr CheckResult) []string {
	var out []string
	for _, m := range cr.Missing {
		if strings.Contains(m, "alias") {
			out = append(out, m)
		}
	}
	return out
}

func filterMissingTests(cr CheckResult) []string {
	var out []string
	for _, m := range cr.Missing {
		if strings.Contains(m, "test") || strings.Contains(m, "Test") {
			out = append(out, m)
		}
	}
	return out
}

func buildRecommendedFixes(checks []CheckResult) []string {
	var out []string
	seen := map[string]bool{}

	addOnce := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	for _, cr := range checks {
		if cr.Status != StatusFail && cr.Status != StatusWeak {
			continue
		}
		switch cr.Kind {
		case KindBuild:
			addOnce("Run 'globular awareness build' to rebuild the graph")
		case KindAudit:
			addOnce("Fix enforcement audit errors (run 'globular awareness audit')")
		case KindCoverage:
			addOnce("Add +globular: annotations to uncovered high-risk files")
		case KindDrift:
			addOnce("Remove stale graph references (run 'globular awareness graph-drift')")
		case KindSmoke:
			addOnce("Add context aliases for tasks that produce false silences (docs/awareness/context_aliases.yaml)")
		case KindNodeContext:
			addOnce("Verify node-context aliases cover architectural task patterns")
		case KindSemanticPath:
			addOnce("Check graph edge provenance for semantic path disconnections")
		case KindDebugSession:
			addOnce("Verify debugsession package compiles and runs against the current graph schema")
		case KindCheckEdit:
			addOnce("Add +globular: annotations to high-risk files to enable check-edit signals")
		case KindMCPDiscovery:
			addOnce("Remove promote_proposal from MCP tool registration (promotion must be CLI-only)")
		}
	}

	return out
}
