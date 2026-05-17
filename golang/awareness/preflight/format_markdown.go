package preflight

import (
	"fmt"
	"strings"
)

// renderMarkdown formats the report as GitHub-flavored Markdown.
func renderMarkdown(r *Report) string {
	var sb strings.Builder

	sb.WriteString("# Globular Awareness Preflight\n\n")

	// Task.
	sb.WriteString("## Task\n\n")
	sb.WriteString(r.Task + "\n\n")

	// Classification.
	sb.WriteString("## Classification\n\n")
	if len(r.Classification) == 0 {
		sb.WriteString("- LOCAL_CODE_CHANGE\n\n")
	} else {
		for _, c := range r.Classification {
			sb.WriteString("- " + string(c) + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Risk tier: `%s`\n\n", r.RiskTier))
	sb.WriteString(fmt.Sprintf("Fast path applied: `%t`\n\n", r.FastPathApplied))
	if r.Trust != nil {
		sb.WriteString("## Trust\n\n")
		sb.WriteString(fmt.Sprintf("- verdict: `%s`\n", r.Trust.Verdict))
		sb.WriteString(fmt.Sprintf("- confidence: `%s`\n", r.Trust.Confidence))
		sb.WriteString(fmt.Sprintf("- freshness: `%s`\n", r.Trust.Freshness))
		sb.WriteString(fmt.Sprintf("- coverage: `%s`\n", r.Trust.Coverage))
		if r.Trust.Reason != "" {
			sb.WriteString("- reason: " + r.Trust.Reason + "\n")
		}
		if len(r.Trust.Limitations) > 0 {
			sb.WriteString("- limitations: " + strings.Join(r.Trust.Limitations, "; ") + "\n")
		}
		if len(r.Trust.RequiredActions) > 0 {
			sb.WriteString("- required_action: " + strings.Join(r.Trust.RequiredActions, "; ") + "\n")
		}
		sb.WriteString("\n")
	}

	// Immediate warnings.
	if len(r.Warnings) > 0 {
		sb.WriteString("## Immediate warning\n\n")
		for _, w := range r.Warnings {
			sb.WriteString("> " + w + "\n")
		}
		sb.WriteString("\n")
	}

	// Matched awareness (aliases).
	sb.WriteString("## Matched awareness\n\n")
	if len(r.MatchedAliases) == 0 {
		sb.WriteString("No context aliases matched.\n\n")
	} else {
		for _, a := range r.MatchedAliases {
			sb.WriteString("- " + a + "\n")
		}
		sb.WriteString("\n")
	}

	// Did we already fix this?
	sb.WriteString("## Did we already fix this?\n\n")
	if r.DidWeFix != nil {
		sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", r.DidWeFix.Status))
		if len(r.DidWeFix.MatchedPatterns) > 0 {
			sb.WriteString("Matched patterns: " + strings.Join(r.DidWeFix.MatchedPatterns, ", ") + "\n\n")
		}
		if len(r.DidWeFix.FixCases) > 0 {
			sb.WriteString("Fix cases: " + strings.Join(r.DidWeFix.FixCases, ", ") + "\n\n")
		}
		if len(r.DidWeFix.RemainingGaps) > 0 {
			sb.WriteString("Remaining gaps:\n")
			for _, g := range r.DidWeFix.RemainingGaps {
				sb.WriteString("- " + g + "\n")
			}
			sb.WriteString("\n")
		}
		if r.DidWeFix.NextAction != "" {
			sb.WriteString("Next action: " + r.DidWeFix.NextAction + "\n\n")
		}
	} else {
		sb.WriteString("No fix-ledger data available.\n\n")
	}

	// Relevant invariants.
	writeListSection(&sb, "## Relevant invariants\n\n", r.Invariants, "No invariants matched.")

	// Known failure modes.
	writeListSection(&sb, "## Known failure modes\n\n", r.FailureModes, "No failure modes matched.")

	// Forbidden fixes.
	writeListSection(&sb, "## Forbidden fixes\n\n", r.ForbiddenFixes, "No forbidden fixes identified.")

	// Matched decisions.
	if len(r.MatchedDecisions) > 0 {
		writeListSection(&sb, "## Matched decisions\n\n", r.MatchedDecisions, "")
	}

	// Forbidden assumptions.
	if len(r.MatchedForbiddenAssumptions) > 0 {
		writeListSection(&sb, "## Forbidden assumptions\n\n", r.MatchedForbiddenAssumptions, "")
	}

	// Authority rules.
	if len(r.MatchedAuthorityRules) > 0 {
		writeListSection(&sb, "## Authority rules\n\n", r.MatchedAuthorityRules, "")
	}

	// Preflight questions.
	if len(r.MatchedPreflightQuestions) > 0 {
		writeListSection(&sb, "## Preflight questions\n\n", r.MatchedPreflightQuestions, "")
	}

	// Remediation contracts.
	if len(r.MatchedRemediationContracts) > 0 {
		writeListSection(&sb, "## Remediation contracts\n\n", r.MatchedRemediationContracts, "")
	}

	// Design pattern layer.
	if len(r.DesignPatterns) > 0 {
		writeListSection(&sb, "## Relevant design patterns\n\n", r.DesignPatterns, "")
	}
	if len(r.AntiPatterns) > 0 {
		writeListSection(&sb, "## Anti-patterns to avoid\n\n", r.AntiPatterns, "")
	}

	// Code smells from patterns.
	if len(r.CodeSmells) > 0 {
		writeListSection(&sb, "## Code smells to watch for\n\n", r.CodeSmells, "")
	}

	// Impacted files.
	writeListSection(&sb, "## Impacted files\n\n", r.Files, "No files provided.")

	if len(r.ExperienceHints) > 0 {
		sb.WriteString("## Similar experiences\n\n")
		for i, h := range r.ExperienceHints {
			sb.WriteString(fmt.Sprintf("%d. `%s` (score %.2f)\n", i+1, h.ExperienceID, h.Score))
			if h.Strategy != "" {
				sb.WriteString("   - strategy: " + h.Strategy + "\n")
			}
			if h.Hint != "" {
				sb.WriteString("   - hint: " + h.Hint + "\n")
			}
			if h.Summary != "" {
				sb.WriteString("   - summary: " + h.Summary + "\n")
			}
			if h.Verdict != "" {
				sb.WriteString("   - verdict: " + h.Verdict + "\n")
			}
			if h.FinalScore > 0 {
				sb.WriteString(fmt.Sprintf("   - final score: %.2f\n", h.FinalScore))
			}
			if len(h.Reasons) > 0 {
				sb.WriteString("   - reasons: " + strings.Join(h.Reasons, ", ") + "\n")
			}
			if len(h.WorkedPaths) > 0 {
				sb.WriteString("   - worked paths: " + strings.Join(h.WorkedPaths, " | ") + "\n")
			}
			if len(h.FailedPaths) > 0 {
				sb.WriteString("   - failed paths: " + strings.Join(h.FailedPaths, " | ") + "\n")
			}
			if len(h.EvidenceTypes) > 0 {
				sb.WriteString("   - expected evidence: " + strings.Join(h.EvidenceTypes, ", ") + "\n")
			}
		}
		sb.WriteString("\n")
	}

	// Package admission.
	sb.WriteString("## Package admission\n\n")
	if r.PackageAdmission == nil {
		sb.WriteString("No package provided.\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", r.PackageAdmission.Status))
		for _, reason := range r.PackageAdmission.Reasons {
			sb.WriteString("- " + reason + "\n")
		}
		sb.WriteString("\n")
	}

	// Dependency/cycle risks.
	sb.WriteString("## Dependency/cycle risks\n\n")
	if len(r.Cycles) == 0 {
		sb.WriteString("No dependency cycles detected.\n\n")
	} else {
		for i, c := range r.Cycles {
			sb.WriteString(fmt.Sprintf("**Cycle %d** [%s] phase=%s\n", i+1, c.Classification, c.Phase))
			sb.WriteString("Path: " + strings.Join(c.Path, " → ") + "\n")
			sb.WriteString("Reason: " + c.Reason + "\n\n")
		}
	}

	// Hash schemas (from protocol annotations).
	if len(r.HashSchemas) > 0 {
		writeListSection(&sb, "## Hash schemas\n\n", r.HashSchemas, "")
	}

	// State transitions (from protocol annotations).
	if len(r.StateTransitions) > 0 {
		writeListSection(&sb, "## State transitions\n\n", r.StateTransitions, "")
	}

	// Required tests.
	writeListSection(&sb, "## Required tests\n\n", r.RequiredTests, "No required tests identified.")

	// Required searches.
	writeListSection(&sb, "## Required searches\n\n", r.RequiredSearches, "No required searches identified.")

	if r.DegradedMode.Enabled {
		sb.WriteString("## Degraded-mode playbook\n\n")
		if r.DegradedMode.Reason != "" {
			sb.WriteString("Reason: " + r.DegradedMode.Reason + "\n\n")
		}
		writeListSection(&sb, "### Allowed next steps\n\n", r.DegradedMode.AllowedNextSteps, "None.")
		writeListSection(&sb, "### Blocked actions\n\n", r.DegradedMode.BlockedActions, "None.")
		writeListSection(&sb, "### Stop conditions\n\n", r.DegradedMode.StopConditions, "None.")
	}

	// Recommended investigation order.
	sb.WriteString("## Recommended investigation order\n\n")
	for i, step := range r.RecommendedOrder {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}
	sb.WriteString("\n")

	// Agent instruction.
	sb.WriteString("## Agent instruction\n\n")
	sb.WriteString(r.AgentInstruction + "\n\n")

	// Explicit do-not-do list.
	sb.WriteString("## Do not do\n\n")
	if len(r.ForbiddenFixes) > 0 {
		for _, ff := range r.ForbiddenFixes {
			sb.WriteString("- " + ff + "\n")
		}
	} else {
		sb.WriteString("No explicit prohibitions from current graph context.\n")
	}
	sb.WriteString("\n")

	return sb.String()
}
