package preflight

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Format is the output format for a preflight report.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatJSON     Format = "json"
	FormatAgent    Format = "agent"
)

// Render formats a Report for the given output format.
func Render(r *Report, format Format) (string, error) {
	switch format {
	case FormatJSON:
		return renderJSON(r)
	case FormatAgent:
		return renderAgent(r), nil
	default:
		return renderMarkdown(r), nil
	}
}

// renderJSON returns the report as a canonical JSON document.
func renderJSON(r *Report) (string, error) {
	// Build the JSON-friendly shape matching the spec schema.
	type didWeFixJSON struct {
		Status          string   `json:"status"`
		MatchedPatterns []string `json:"matched_patterns"`
		FixCases        []string `json:"fix_cases"`
		RemainingGaps   []string `json:"remaining_gaps"`
	}

	type jsonReport struct {
		Task                     string                   `json:"task"`
		Classification           []TaskClass              `json:"classification"`
		MatchedAliases           []string                 `json:"matched_aliases"`
		Services                 []string                 `json:"services"`
		Packages                 []string                 `json:"packages"`
		Files                    []string                 `json:"files"`
		Invariants               []string                 `json:"invariants"`
		FailureModes             []string                 `json:"failure_modes"`
		ForbiddenFixes           []string                 `json:"forbidden_fixes"`
		HashSchemas              []string                 `json:"hash_schemas,omitempty"`
		StateTransitions         []string                 `json:"state_transitions,omitempty"`
		DidWeFix                 *DidWeFixSection         `json:"did_we_fix"`
		PackageAdmission         *PackageAdmissionSection `json:"package_admission,omitempty"`
		Cycles                   []CycleWarning           `json:"cycles"`
		RequiredTests            []string                 `json:"required_tests"`
		RequiredSearches         []string                 `json:"required_searches"`
		RecommendedInvestigation []string                 `json:"recommended_investigation_order"`
		AgentInstruction         string                   `json:"agent_instruction"`
		Warnings                 []string                 `json:"warnings"`
		Runtime                  *RuntimeSection          `json:"runtime,omitempty"`
	}

	jr := jsonReport{
		Task:                     r.Task,
		Classification:           r.Classification,
		MatchedAliases:           orEmpty(r.MatchedAliases),
		Services:                 orEmpty(r.Services),
		Packages:                 orEmpty(r.Packages),
		Files:                    orEmpty(r.Files),
		Invariants:               orEmpty(r.Invariants),
		FailureModes:             orEmpty(r.FailureModes),
		ForbiddenFixes:           orEmpty(r.ForbiddenFixes),
		HashSchemas:              r.HashSchemas,
		StateTransitions:         r.StateTransitions,
		DidWeFix:                 r.DidWeFix,
		PackageAdmission:         r.PackageAdmission,
		Cycles:                   r.Cycles,
		RequiredTests:            orEmpty(r.RequiredTests),
		RequiredSearches:         orEmpty(r.RequiredSearches),
		RecommendedInvestigation: orEmpty(r.RecommendedOrder),
		AgentInstruction:         r.AgentInstruction,
		Warnings:                 orEmpty(r.Warnings),
		Runtime:                  r.Runtime,
	}

	b, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal preflight report: %w", err)
	}
	return string(b), nil
}

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

	// Impacted files.
	writeListSection(&sb, "## Impacted files\n\n", r.Files, "No files provided.")

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

// renderAgent formats the report as a directive agent instruction block.
func renderAgent(r *Report) string {
	var sb strings.Builder

	sb.WriteString("AGENT PREFLIGHT RESULT\n\n")

	// Classification summary.
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

	// Likely root-cause areas from invariants and failure modes.
	if len(r.Invariants) > 0 || len(r.FailureModes) > 0 {
		sb.WriteString("Likely root-cause area:\n")
		for _, inv := range r.Invariants {
			sb.WriteString("- " + inv + "\n")
		}
		for _, fm := range r.FailureModes {
			sb.WriteString("- " + fm + "\n")
		}
		sb.WriteString("\n")
	}

	// Forbidden fixes.
	if len(r.ForbiddenFixes) > 0 {
		sb.WriteString("Forbidden fixes:\n")
		for _, ff := range r.ForbiddenFixes {
			sb.WriteString("- " + ff + "\n")
		}
		sb.WriteString("\n")
	}

	// Did-we-fix.
	if r.DidWeFix != nil && r.DidWeFix.Status != "" && r.DidWeFix.Status != "UNKNOWN" {
		sb.WriteString(fmt.Sprintf("Did-we-fix status: %s\n", r.DidWeFix.Status))
		if r.DidWeFix.NextAction != "" {
			sb.WriteString("Next action: " + r.DidWeFix.NextAction + "\n")
		}
		sb.WriteString("\n")
	}

	// You must inspect.
	if len(r.RequiredSearches) > 0 {
		sb.WriteString("You must inspect:\n")
		for _, s := range r.RequiredSearches {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	// You must run.
	if len(r.RequiredTests) > 0 {
		sb.WriteString("You must run:\n")
		for _, t := range r.RequiredTests {
			sb.WriteString("- " + t + "\n")
		}
		sb.WriteString("\n")
	}

	// Investigation order.
	if len(r.RecommendedOrder) > 0 {
		sb.WriteString("Investigation order:\n")
		for i, step := range r.RecommendedOrder {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// Package admission.
	if r.PackageAdmission != nil {
		sb.WriteString(fmt.Sprintf("Package admission: %s\n", r.PackageAdmission.Status))
		for _, reason := range r.PackageAdmission.Reasons {
			sb.WriteString("  - " + reason + "\n")
		}
		sb.WriteString("\n")
	}

	// Cycles.
	if len(r.Cycles) > 0 {
		sb.WriteString("Cycle warnings:\n")
		for _, c := range r.Cycles {
			sb.WriteString(fmt.Sprintf("  [%s] %s → %s\n", c.Classification, c.Phase, strings.Join(c.Path, " → ")))
		}
		sb.WriteString("\n")
	}

	// Agent instruction summary.
	if len(r.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString("- " + w + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(r.AgentInstruction + "\n")

	return sb.String()
}

// writeListSection writes a markdown section with a bullet list or fallback message.
func writeListSection(sb *strings.Builder, header string, items []string, empty string) {
	sb.WriteString(header)
	if len(items) == 0 {
		sb.WriteString(empty + "\n\n")
		return
	}
	for _, item := range items {
		sb.WriteString("- " + item + "\n")
	}
	sb.WriteString("\n")
}

// orEmpty returns a non-nil empty slice when in is nil — keeps JSON output stable.
func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
