package main

// awareness_enforce_cmds.go — stubs after the enforce package was removed from
// standalone awareness module. The enforcement commands (audit, validate-annotations,
// validate-required-tests, validate-contracts, graph-drift, pr-report, hook) are
// not available in this build.
// Use the MCP tools 'awareness_scan_violations', 'awareness_pre_commit_check',
// 'awareness_semantic_diff_from_git' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var enforceCfg = struct {
	jsonOutput       bool
	files            []string
	fromGitDiff      bool
	strict           bool
	watchlist        string
	auditStrict      bool
	summary          bool
	failOnWarning    bool
	warningThreshold int
	suppressionsFile string
	showSuppressed   bool
	hookFile         string
	hookTask         string
}{}

func makeEnforceStub(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short + " (not available — enforce package removed from standalone)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s is not available: enforce package removed — use MCP tools awareness_scan_violations, awareness_pre_commit_check, or awareness_semantic_diff_from_git instead", use)
		},
	}
}

var awarenessAuditCmd = makeEnforceStub("audit", "Run the enforcement audit on the awareness graph")
var awarenessValidateAnnotationsCmd = makeEnforceStub("validate-annotations", "Validate source file annotations")
var awarenessValidateRequiredTestsCmd = makeEnforceStub("validate-required-tests", "Validate required test coverage")
var awarenessValidateContractsCmd = makeEnforceStub("validate-contracts", "Validate package contracts")
var awarenessGraphDriftCmd = makeEnforceStub("graph-drift", "Check for stale graph references")
var awarenessPRReportCmd = makeEnforceStub("pr-report", "Generate PR awareness report")
var awarenessHookCmd = makeEnforceStub("hook", "Run awareness checks as a git hook")

func init() {
	// Minimal flag definitions to avoid "unknown flag" errors if called by scripts.
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.strict, "strict", false, "Fail on warnings")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.summary, "summary", false, "Summary only")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.failOnWarning, "fail-on-warning", false, "Exit non-zero on warning")
	awarenessAuditCmd.Flags().IntVar(&enforceCfg.warningThreshold, "max-warnings", 0, "Max warnings before failing")
	awarenessAuditCmd.Flags().StringVar(&enforceCfg.suppressionsFile, "suppressions", "", "Suppressions YAML file")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.showSuppressed, "show-suppressed", false, "Show suppressed findings")

	awarenessValidateAnnotationsCmd.Flags().StringVar(&enforceCfg.watchlist, "watchlist", "", "High-risk files watchlist")
	awarenessValidateAnnotationsCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")

	awarenessHookCmd.Flags().StringVar(&enforceCfg.hookFile, "file", "", "File to check (required)")
	awarenessHookCmd.Flags().StringVar(&enforceCfg.hookTask, "task", "", "Optional task description")

	awarenessPRReportCmd.Flags().StringSliceVar(&enforceCfg.files, "files", nil, "Files changed in PR")
	awarenessPRReportCmd.Flags().BoolVar(&enforceCfg.fromGitDiff, "from-git-diff", false, "Detect changed files from git diff")

	awarenessCmd.AddCommand(awarenessAuditCmd)
	awarenessCmd.AddCommand(awarenessValidateAnnotationsCmd)
	awarenessCmd.AddCommand(awarenessValidateRequiredTestsCmd)
	awarenessCmd.AddCommand(awarenessValidateContractsCmd)
	awarenessCmd.AddCommand(awarenessGraphDriftCmd)
	awarenessCmd.AddCommand(awarenessPRReportCmd)
	awarenessCmd.AddCommand(awarenessHookCmd)
}
