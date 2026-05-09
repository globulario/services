package semanticdiff

import (
	"fmt"
	"strings"
)

// FormatReport returns a human-readable semantic diff report.
func FormatReport(r *SemanticDiffReport) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder

	fmt.Fprintf(&sb, "SEMANTIC DIFF VERDICT: %s\n\n", strings.ToUpper(r.Verdict))
	fmt.Fprintf(&sb, "Severity: %s\n\n", strings.ToUpper(r.Severity))
	fmt.Fprintf(&sb, "Summary: %s\n\n", r.Summary)

	blockers := filterBySeverity(r.Findings, SeverityForbidden, SeverityCritical)
	if len(blockers) > 0 {
		fmt.Fprintf(&sb, "Violations:\n")
		for _, f := range blockers {
			fmt.Fprintf(&sb, "  [%s] %s\n", strings.ToUpper(f.Severity), f.Message)
			if f.FilePath != "" {
				fmt.Fprintf(&sb, "    File: %s\n", f.FilePath)
			}
			if f.Symbol != "" {
				fmt.Fprintf(&sb, "    Symbol: %s\n", f.Symbol)
			}
			if f.LayerFrom != "" && f.LayerTo != "" {
				fmt.Fprintf(&sb, "    Layer: %s → %s\n", f.LayerFrom, f.LayerTo)
			}
			if f.Evidence != "" {
				fmt.Fprintf(&sb, "    Evidence: %s\n", f.Evidence)
			}
			if f.Recommendation != "" {
				fmt.Fprintf(&sb, "    Recommendation: %s\n", f.Recommendation)
			}
			fmt.Fprintln(&sb)
		}
	}

	warnings := filterBySeverity(r.Findings, SeverityWarning)
	if len(warnings) > 0 {
		fmt.Fprintf(&sb, "Warnings:\n")
		for _, f := range warnings {
			fmt.Fprintf(&sb, "  [WARNING] %s", f.Message)
			if f.FilePath != "" {
				fmt.Fprintf(&sb, " (%s)", f.FilePath)
			}
			fmt.Fprintln(&sb)
		}
		fmt.Fprintln(&sb)
	}

	strengthenings := filterBySeverity(r.Findings, SeverityInfo)
	if len(strengthenings) > 0 {
		fmt.Fprintf(&sb, "Strengthenings:\n")
		for _, f := range strengthenings {
			fmt.Fprintf(&sb, "  [OK] %s\n", f.Message)
		}
		fmt.Fprintln(&sb)
	}

	fmt.Fprintf(&sb, "Fingerprint: %s\n", r.Fingerprint)
	return sb.String()
}

func filterBySeverity(findings []SemanticDiffFinding, severities ...string) []SemanticDiffFinding {
	sevSet := map[string]bool{}
	for _, s := range severities {
		sevSet[s] = true
	}
	var out []SemanticDiffFinding
	for _, f := range findings {
		if sevSet[f.Severity] {
			out = append(out, f)
		}
	}
	return out
}
