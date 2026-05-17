package preflight

import (
	"fmt"
	"sort"
	"strings"
)

// renderAgent formats the report as a directive agent instruction block.
//
// Section inclusion by budget:
//
//	compact   — safety header, root-cause(3), forbidden(3), tests(5), warnings, safety/risk/confidence/trust, agent instruction
//	standard  — all sections with top-N limits (current default)
//	deep      — standard + full decision traces (no truncation)
//	forensic  — all sections at full verbosity
func renderAgent(r *Report, opts RenderOptions) string {
	var sb strings.Builder
	budget := opts.Budget
	verbosity := effectiveVerbosity(opts)

	sb.WriteString("AGENT PREFLIGHT RESULT\n\n")

	// Classification summary — always shown.
	if hasClass(r.Classification, ClassArchitectureSensitive) || hasClass(r.Classification, ClassConvergenceRisk) {
		sb.WriteString("This is architecture-sensitive.\n")
	}
	if hasClass(r.Classification, ClassRestartStorm) {
		sb.WriteString("Restart storm detected — do not patch local restart behavior first.\n")
	}
	if hasClass(r.Classification, ClassStateMismatch) {
		sb.WriteString("State mismatch detected — resolve at the correct layer (Desired → Installed → Runtime).\n")
	}
	if hasClass(r.Classification, ClassDependencyCycle) {
		sb.WriteString("Dependency cycle detected — resolve the cycle before any code change.\n")
	}
	sb.WriteString("\n")
	if r.Coverage.Runtime == CoverageNoop || r.Coverage.IncidentStore == CoverageNotChecked {
		sb.WriteString("Static-only confidence: runtime/incident evidence not fully checked in this run.\n\n")
	}

	// Decision traces: skipped in compact; full expansion in deep/forensic.
	if budget != BudgetCompact {
		traceVerbosity := verbosity
		if budget == BudgetDeep || budget == BudgetForensic {
			traceVerbosity = VerbosityFull
		}
		writeDecisionTraces(&sb, r.DecisionTraces, traceVerbosity)
	}

	// Root-cause area — always shown.
	rootCause := append([]string{}, r.Invariants...)
	rootCause = append(rootCause, r.FailureModes...)
	if len(rootCause) > 0 {
		sb.WriteString("Likely root-cause area:\n")
		rootCause = rankForTask(rootCause, r.Task, r.Files, r.Packages)
		writeAgentTopList(&sb, rootCause, agentLimit(verbosity, agentTopRootCauseLimit, 3))
		sb.WriteString("\n")
	}

	// Forbidden fixes — always shown.
	if len(r.ForbiddenFixes) > 0 {
		sb.WriteString("Forbidden fixes:\n")
		forbidden := rankForTask(r.ForbiddenFixes, r.Task, r.Files, r.Packages)
		writeAgentTopList(&sb, forbidden, agentLimit(verbosity, agentTopForbiddenLimit, 3))
		sb.WriteString("\n")
	}

	// Required tests — always shown.
	if len(r.RequiredTests) > 0 {
		sb.WriteString("Required tests:\n")
		writeAgentTopList(&sb, r.RequiredTests, agentLimit(verbosity, agentTopRequiredTestsLimit, 5))
		sb.WriteString("\n")
	}

	// Warnings — always shown.
	if len(r.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString("- " + w + "\n")
		}
		sb.WriteString("\n")
	}

	// Safety/risk/confidence summary — always shown.
	sb.WriteString(fmt.Sprintf("safety_status: %s\n", r.SafetyStatus))
	sb.WriteString(fmt.Sprintf("risk_tier: %s\n", r.RiskTier))
	sb.WriteString(fmt.Sprintf("confidence: %s", r.Confidence))
	if r.ConfidenceReason != "" {
		sb.WriteString(" — " + r.ConfidenceReason)
	}
	sb.WriteString("\n")
	if r.Trust != nil {
		sb.WriteString(fmt.Sprintf("trust: verdict=%s confidence=%s freshness=%s coverage=%s",
			r.Trust.Verdict, r.Trust.Confidence, r.Trust.Freshness, r.Trust.Coverage))
		if r.Trust.Reason != "" {
			sb.WriteString(" — " + r.Trust.Reason)
		}
		sb.WriteString("\n")
		if len(r.Trust.Limitations) > 0 {
			sb.WriteString("trust_limitations: " + strings.Join(r.Trust.Limitations, "; ") + "\n")
		}
		if len(r.Trust.RequiredActions) > 0 {
			sb.WriteString("trust_required_action: " + strings.Join(r.Trust.RequiredActions, "; ") + "\n")
		}
	}
	sb.WriteString("\n")

	// Compact stops here.
	if budget == BudgetCompact {
		sb.WriteString(r.AgentInstruction + "\n")
		return sb.String()
	}

	// Did we already fix this? (standard+)
	if r.DidWeFix != nil && r.DidWeFix.Status != "" {
		sb.WriteString(fmt.Sprintf("did_we_fix: %s\n", r.DidWeFix.Status))
		if len(r.DidWeFix.MatchedPatterns) > 0 {
			sb.WriteString("  matched: " + strings.Join(r.DidWeFix.MatchedPatterns, ", ") + "\n")
		}
		if len(r.DidWeFix.RemainingGaps) > 0 {
			sb.WriteString("  gaps: " + strings.Join(r.DidWeFix.RemainingGaps, "; ") + "\n")
		}
		if r.DidWeFix.NextAction != "" {
			sb.WriteString("  next: " + r.DidWeFix.NextAction + "\n")
		}
		sb.WriteString("\n")
	}

	// Code smells (standard+).
	if len(r.CodeSmells) > 0 {
		sb.WriteString("Code smells:\n")
		writeAgentTopList(&sb, r.CodeSmells, agentLimit(verbosity, agentTopCodeSmellsLimit, 5))
		sb.WriteString("\n")
	}

	// Design patterns / anti-patterns (standard+).
	if len(r.DesignPatterns) > 0 {
		sb.WriteString("Design patterns:\n")
		writeAgentTopList(&sb, r.DesignPatterns, agentLimit(verbosity, 5, 3))
		sb.WriteString("\n")
	}
	if len(r.AntiPatterns) > 0 {
		sb.WriteString("Anti-patterns to avoid:\n")
		writeAgentTopList(&sb, r.AntiPatterns, agentLimit(verbosity, 5, 3))
		sb.WriteString("\n")
	}

	// Experience hints (standard+).
	if len(r.ExperienceHints) > 0 {
		sb.WriteString("Similar experiences:\n")
		limit := agentLimit(verbosity, 3, 2)
		shown := r.ExperienceHints
		if limit > 0 && len(shown) > limit {
			shown = shown[:limit]
		}
		for _, h := range shown {
			sb.WriteString(fmt.Sprintf("- %s (score %.2f)", h.ExperienceID, h.Score))
			if h.Hint != "" {
				sb.WriteString(": " + h.Hint)
			}
			sb.WriteString("\n")
			if len(h.WorkedPaths) > 0 {
				sb.WriteString("  worked: " + strings.Join(h.WorkedPaths, " | ") + "\n")
			}
			if len(h.FailedPaths) > 0 {
				sb.WriteString("  failed: " + strings.Join(h.FailedPaths, " | ") + "\n")
			}
		}
		sb.WriteString("\n")
	}

	// Required searches (standard+).
	if len(r.RequiredSearches) > 0 {
		sb.WriteString("Required searches:\n")
		writeAgentTopList(&sb, r.RequiredSearches, agentLimit(verbosity, 5, 3))
		sb.WriteString("\n")
	}

	// Investigation order (standard+).
	if len(r.RecommendedOrder) > 0 {
		sb.WriteString("Investigation order:\n")
		items := r.RecommendedOrder
		limit := agentLimit(verbosity, agentTopInspectLimit, 5)
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		for i, step := range items {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// Package admission (standard+).
	if r.PackageAdmission != nil {
		sb.WriteString(fmt.Sprintf("package_admission: %s\n", r.PackageAdmission.Status))
		for _, reason := range r.PackageAdmission.Reasons {
			sb.WriteString("  - " + reason + "\n")
		}
		sb.WriteString("\n")
	}

	// Cycles (standard+).
	if len(r.Cycles) > 0 {
		sb.WriteString("Dependency cycles:\n")
		for _, c := range r.Cycles {
			sb.WriteString(fmt.Sprintf("- [%s] phase=%s: %s\n", c.Classification, c.Phase, strings.Join(c.Path, " → ")))
		}
		sb.WriteString("\n")
	}

	// Degraded-mode playbook (standard+).
	if r.DegradedMode.Enabled {
		sb.WriteString("degraded_mode: " + r.DegradedMode.Reason + "\n")
		if len(r.DegradedMode.AllowedNextSteps) > 0 {
			sb.WriteString("  allowed: " + strings.Join(r.DegradedMode.AllowedNextSteps, "; ") + "\n")
		}
		if len(r.DegradedMode.BlockedActions) > 0 {
			sb.WriteString("  blocked: " + strings.Join(r.DegradedMode.BlockedActions, "; ") + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(r.AgentInstruction + "\n")
	return sb.String()
}

// writeDecisionTraces writes the decision trace block in agent format, sorted by
// order (FailureMode first) — agents reading the text get the
// most-actionable findings up top.
func writeDecisionTraces(sb *strings.Builder, traces []DecisionTrace, verbosity Verbosity) {
	if len(traces) == 0 {
		return
	}

	// Cap traces by verbosity. Full verbosity shows everything; standard
	// and compact trim to the top 5.
	maxTraces := agentLimit(verbosity, 5, 3)
	maxPivots := agentLimit(verbosity, 3, 2)
	maxActions := agentLimit(verbosity, 3, 2)
	maxFalsifiers := agentLimit(verbosity, 2, 2)
	maxEvidence := agentLimit(verbosity, 3, 2)

	ranked := make([]DecisionTrace, len(traces))
	copy(ranked, traces)
	sort.SliceStable(ranked, func(i, j int) bool {
		ri, rj := agentTraceRank(ranked[i].FindingType), agentTraceRank(ranked[j].FindingType)
		if ri != rj {
			return ri < rj
		}
		return ranked[i].FindingID < ranked[j].FindingID
	})
	if verbosity != VerbosityFull && len(ranked) > maxTraces {
		ranked = ranked[:maxTraces]
	}

	sb.WriteString("Decision traces:\n")
	for i, tr := range ranked {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("- finding: %s.%s\n", tr.FindingType, tr.FindingID))
		sb.WriteString(fmt.Sprintf("  confidence: %s\n", tr.Confidence))

		if owner := renderOwnerOneLine(tr.Owner); owner != "" {
			sb.WriteString("  owner: " + owner + "\n")
		}

		if len(tr.MatchedBy) > 0 {
			sb.WriteString("  why:\n")
			n := len(tr.MatchedBy)
			if n > maxEvidence {
				n = maxEvidence
			}
			for _, ev := range tr.MatchedBy[:n] {
				sb.WriteString("    - " + renderEvidenceOneLine(ev) + "\n")
			}
		}

		// Pivots: split forbidden_fix entries into their own line group so
		// the agent sees "do not do X" without having to read every pivot.
		var forbiddenPivots, otherPivots []ContextPivot
		for _, p := range tr.Pivots {
			if p.Kind == "forbidden_fix" {
				forbiddenPivots = append(forbiddenPivots, p)
			} else {
				otherPivots = append(otherPivots, p)
			}
		}
		if len(otherPivots) > 0 {
			sb.WriteString("  pivots:\n")
			n := len(otherPivots)
			if n > maxPivots {
				n = maxPivots
			}
			for _, p := range otherPivots[:n] {
				sb.WriteString(fmt.Sprintf("    - %s: %s\n", p.Kind, p.ID))
			}
		}
		if len(forbiddenPivots) > 0 {
			sb.WriteString("  forbidden:\n")
			n := len(forbiddenPivots)
			if n > maxPivots {
				n = maxPivots
			}
			for _, p := range forbiddenPivots[:n] {
				sb.WriteString("    - " + p.ID + "\n")
			}
		}

		if len(tr.NextActions) > 0 {
			sb.WriteString("  next:\n")
			n := len(tr.NextActions)
			if n > maxActions {
				n = maxActions
			}
			for _, a := range tr.NextActions[:n] {
				if a.Command != "" {
					sb.WriteString("    - " + a.Command + "\n")
				} else {
					sb.WriteString("    - " + a.Kind + ": " + a.Reason + "\n")
				}
			}
		}

		if len(tr.Falsifiers) > 0 {
			sb.WriteString("  falsify:\n")
			n := len(tr.Falsifiers)
			if n > maxFalsifiers {
				n = maxFalsifiers
			}
			for _, f := range tr.Falsifiers[:n] {
				sb.WriteString("    - " + f.Claim + "\n")
			}
		}
	}
	if verbosity != VerbosityFull && len(traces) > maxTraces {
		sb.WriteString(fmt.Sprintf("\n  ... %d more trace(s) — see JSON output for full detail\n",
			len(traces)-maxTraces))
	}
	sb.WriteString("\n")
}

// agentTraceRank orders DecisionTrace findings for agent-format display.
// Per the design doc: forbidden_fix > invariant > runtime failure_mode >
// raw_knowledge > experience. The JSON ordering (FailureMode first) is
// preserved for that channel; this is render-only.
func agentTraceRank(t FindingType) int {
	switch t {
	case FindingForbiddenFix:
		return 0
	case FindingInvariant:
		return 1
	case FindingFailureMode:
		return 2
	case FindingRuntime:
		return 3
	case FindingRawKnowledge:
		return 4
	case FindingExperience:
		return 5
	}
	return 99
}

// renderOwnerOneLine collapses an OwnerContext to a single "layer /
// service / package" line, skipping empty fields. Returns "" when every
// owner field is empty.
func renderOwnerOneLine(o OwnerContext) string {
	parts := make([]string, 0, 3)
	if o.Layer != "" && o.Layer != "unknown" {
		parts = append(parts, o.Layer)
	}
	if o.Service != "" {
		parts = append(parts, o.Service)
	}
	if o.Package != "" {
		parts = append(parts, o.Package)
	}
	return strings.Join(parts, " / ")
}

// renderEvidenceOneLine summarises an EvidenceRef into one short line.
// Format: "<source>: <reason>" or "<source> <path_summary>" — whichever
// is more informative. Confidence + freshness appear in parentheses when
// non-default.
func renderEvidenceOneLine(ev EvidenceRef) string {
	main := ev.Source
	switch {
	case ev.PathSummary != "":
		main = ev.Source + ": " + ev.PathSummary
	case ev.Reason != "":
		main = ev.Source + ": " + ev.Reason
	}
	suffix := ""
	if ev.Freshness != "" && ev.Freshness != "unknown" {
		suffix = " [" + ev.Freshness + "]"
	}
	return main + suffix
}

func writeAgentTopList(sb *strings.Builder, items []string, limit int) {
	if len(items) == 0 {
		return
	}
	if limit <= 0 || len(items) <= limit {
		for _, item := range items {
			sb.WriteString("- " + item + "\n")
		}
		return
	}
	for _, item := range items[:limit] {
		sb.WriteString("- " + item + "\n")
	}
	sb.WriteString(fmt.Sprintf("- ... %d more (use --format json for full list)\n", len(items)-limit))
}

func agentLimit(v Verbosity, standard, compact int) int {
	switch v {
	case VerbosityFull:
		return 0
	case VerbosityCompact:
		return compact
	default:
		return standard
	}
}

func rankForTask(items []string, task string, files, packages []string) []string {
	if len(items) <= 1 {
		return items
	}
	tokens := make(map[string]struct{})
	for _, tok := range splitTokens(task) {
		tokens[tok] = struct{}{}
	}
	for _, f := range files {
		for _, tok := range splitTokens(f) {
			tokens[tok] = struct{}{}
		}
	}
	for _, p := range packages {
		for _, tok := range splitTokens(p) {
			tokens[tok] = struct{}{}
		}
	}
	type scored struct {
		item  string
		score int
	}
	out := make([]scored, 0, len(items))
	for _, item := range items {
		score := 0
		for _, tok := range splitTokens(item) {
			if _, ok := tokens[tok]; ok {
				score++
			}
		}
		out = append(out, scored{item: item, score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].score > out[j].score
	})
	ordered := make([]string, 0, len(out))
	for _, s := range out {
		ordered = append(ordered, s.item)
	}
	return ordered
}

func splitTokens(s string) []string {
	raw := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		if len(tok) < 3 {
			continue
		}
		out = append(out, tok)
	}
	return out
}
