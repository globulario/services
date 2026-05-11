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
	if r.AuthorityChange != nil {
		fmt.Fprintf(&sb, "Authority Change:\n")
		fmt.Fprintf(&sb, "  detected: %t\n", r.AuthorityChange.Detected)
		if r.AuthorityChange.FromLayer != "" || r.AuthorityChange.ToLayer != "" {
			fmt.Fprintf(&sb, "  from_layer: %s\n", r.AuthorityChange.FromLayer)
			fmt.Fprintf(&sb, "  to_layer: %s\n", r.AuthorityChange.ToLayer)
		}
		fmt.Fprintf(&sb, "  risk: %s\n", r.AuthorityChange.Risk)
		fmt.Fprintf(&sb, "  requires_review: %t\n\n", r.AuthorityChange.RequiresReview)
	}
	if r.AuthorityBudget != nil {
		fmt.Fprintf(&sb, "Authority Budget:\n")
		fmt.Fprintf(&sb, "  layer_changed: %t\n", r.AuthorityBudget.LayerChanged)
		if r.AuthorityBudget.SourceLayer != "" || r.AuthorityBudget.TargetLayer != "" {
			fmt.Fprintf(&sb, "  source_layer: %s\n", r.AuthorityBudget.SourceLayer)
			fmt.Fprintf(&sb, "  target_layer: %s\n", r.AuthorityBudget.TargetLayer)
		}
		fmt.Fprintf(&sb, "  allowed_without_review: %t\n", r.AuthorityBudget.AllowedWithoutReview)
		fmt.Fprintf(&sb, "  required_awareness_coverage: %s\n\n", r.AuthorityBudget.RequiredAwarenessCoverage)
	}
	if r.Trust != nil {
		fmt.Fprintf(&sb, "Trust:\n")
		fmt.Fprintf(&sb, "  verdict: %s\n", r.Trust.Verdict)
		fmt.Fprintf(&sb, "  confidence: %s\n", r.Trust.Confidence)
		fmt.Fprintf(&sb, "  freshness: %s\n", r.Trust.Freshness)
		fmt.Fprintf(&sb, "  coverage: %s\n", r.Trust.Coverage)
		if len(r.Trust.Limitations) > 0 {
			fmt.Fprintf(&sb, "  limitations: %s\n", strings.Join(r.Trust.Limitations, "; "))
		}
		if len(r.Trust.RequiredActions) > 0 {
			fmt.Fprintf(&sb, "  required_action: %s\n", strings.Join(r.Trust.RequiredActions, "; "))
		}
		fmt.Fprintln(&sb)
	}

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
