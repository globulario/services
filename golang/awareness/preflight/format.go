package preflight

import "strings"

// Format is the output format for a preflight report.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatJSON     Format = "json"
	FormatAgent    Format = "agent"
)

const (
	agentTopRootCauseLimit     = 8
	agentTopForbiddenLimit     = 8
	agentTopCodeSmellsLimit    = 8
	agentTopInspectLimit       = 8
	agentTopRequiredTestsLimit = 12
)

type Verbosity string

const (
	VerbosityCompact  Verbosity = "compact"
	VerbosityStandard Verbosity = "standard"
	VerbosityFull     Verbosity = "full"
)

// Budget is a high-level control that selects which sections appear in agent
// format output and implies a default verbosity. It takes precedence over
// the Verbosity field when set. Safety fields (safety_status, risk_tier,
// confidence, coverage, trust, forbidden_fixes, required_tests) are always
// present regardless of budget.
type Budget string

const (
	// BudgetCompact emits only the essential safety fields: classification
	// header, safety_status, risk_tier, confidence, trust, warnings, top-3
	// findings, top-3 forbidden fixes, top-5 required tests, agent
	// instruction. All other sections (decision traces, design patterns,
	// anti-patterns, code smells, did-we-fix, experience hints, required
	// searches, investigation order, package admission, cycles) are omitted.
	BudgetCompact Budget = "compact"

	// BudgetStandard is the current default: all sections with existing top-N
	// limits. Equivalent to no --budget flag.
	BudgetStandard Budget = "standard"

	// BudgetDeep is standard plus full decision traces (no truncation) with
	// all pivots expanded. Use for architecture-sensitive changes.
	BudgetDeep Budget = "deep"

	// BudgetForensic is full verbosity across all sections. Use only when the
	// cluster is actively broken or the root cause is unknown.
	BudgetForensic Budget = "forensic"
)

type RenderOptions struct {
	Verbosity Verbosity
	Budget    Budget
}

// effectiveVerbosity returns the verbosity to use for rendering, applying
// the budget's implied verbosity when a budget is set.
func effectiveVerbosity(opts RenderOptions) Verbosity {
	switch opts.Budget {
	case BudgetCompact:
		return VerbosityCompact
	case BudgetForensic:
		return VerbosityFull
	}
	if opts.Verbosity != "" {
		return opts.Verbosity
	}
	return VerbosityStandard
}

// Render formats a Report for the given output format.
func Render(r *Report, format Format) (string, error) {
	return RenderWithOptions(r, format, RenderOptions{Verbosity: VerbosityStandard})
}

// RenderWithOptions formats a Report for the given output format and render options.
func RenderWithOptions(r *Report, format Format, opts RenderOptions) (string, error) {
	switch format {
	case FormatJSON:
		return renderJSON(r)
	case FormatAgent:
		return renderAgent(r, opts), nil
	default:
		return renderMarkdown(r), nil
	}
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
