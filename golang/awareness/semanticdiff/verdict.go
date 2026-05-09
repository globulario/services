package semanticdiff

import "fmt"

// EvaluateSemanticVerdict computes the final verdict from a report's findings.
func EvaluateSemanticVerdict(report *SemanticDiffReport) (verdict, severity, summary string) {
	forbidden := 0
	critical := 0
	warnings := 0
	strengthenings := 0

	for _, f := range report.Findings {
		switch f.Severity {
		case SeverityForbidden:
			forbidden++
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warnings++
		case SeverityInfo:
			strengthenings++
		}
	}

	if forbidden > 0 {
		return VerdictBlock, SeverityForbidden, fmt.Sprintf(
			"BLOCKED (FORBIDDEN): %d forbidden violation(s) detected. This diff violates core state authority rules.", forbidden)
	}
	if critical > 0 {
		return VerdictBlock, SeverityCritical, fmt.Sprintf(
			"BLOCKED: %d critical finding(s). Guards, proofs, or atomicity weakened.", critical)
	}
	if warnings > 0 {
		if strengthenings > 0 {
			return VerdictAllowWithWarnings, SeverityWarning, fmt.Sprintf(
				"ALLOW WITH WARNINGS: %d warning(s), %d strengthening(s).", warnings, strengthenings)
		}
		return VerdictAllowWithWarnings, SeverityWarning, fmt.Sprintf(
			"ALLOW WITH WARNINGS: %d warning(s) detected.", warnings)
	}
	if strengthenings > 0 {
		return VerdictAllow, SeverityInfo, fmt.Sprintf(
			"ALLOW: %d safety strengthening(s) detected, no violations.", strengthenings)
	}
	if len(report.Atoms) == 0 {
		return VerdictAllow, SeverityInfo, "ALLOW: No semantic changes detected — safe refactor."
	}
	return VerdictAllow, SeverityInfo, "ALLOW: No architectural violations detected."
}
