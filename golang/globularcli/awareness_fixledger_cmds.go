package main

// awareness_fixledger_cmds.go: Fix ledger CLI commands (Task 4).
//
// Commands:
//
//	globular awareness fix-status --id <fix-case-id>
//	globular awareness pattern-status --pattern "<pattern>"
//	globular awareness did-we-fix --task "<task>"
//	globular awareness partials
//	globular awareness regressions
//	globular awareness coverage
//	globular awareness remaining-gaps
//	globular awareness guardrail-status --id <guardrail-id>

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/awareness/fixledger"
	"github.com/globulario/awareness/learning"
)

var fixledgerCfg = struct {
	fixCaseID   string
	guardrailID string
	pattern     string
}{} // task is in awareCfg.task

// fixCasesYAMLPath returns the path to docs/awareness/fix_cases.yaml.
func fixCasesYAMLPath(repoRoot string) string {
	return filepath.Join(repoRoot, "docs", "awareness", "fix_cases.yaml")
}

// guardrailsYAMLPath returns the path to docs/awareness/guardrails.yaml.
func guardrailsYAMLPath(repoRoot string) string {
	return filepath.Join(repoRoot, "docs", "awareness", "guardrails.yaml")
}

// statusTrackerYAMLPath returns the path to docs/awareness/status_tracker.yaml.
func statusTrackerYAMLPath(repoRoot string) string {
	return filepath.Join(repoRoot, "docs", "awareness", "status_tracker.yaml")
}

// loadFixLedger loads fix cases and aliases from the repo, returning them silently on error.
func loadFixLedger(repoRoot string) ([]fixledger.FixCase, learning.ContextAliasMap) {
	cases, _ := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
	aliases := loadAliasesQuiet(repoRoot)
	return cases, aliases
}

// ---- fix-status command ----

var awarenessFixStatusCmd = &cobra.Command{
	Use:   "fix-status",
	Short: "Show the status of a specific fix case by ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if fixledgerCfg.fixCaseID == "" {
			return fmt.Errorf("--id is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, err := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load fix cases: %w", err)
		}

		for _, fc := range cases {
			if fc.ID == fixledgerCfg.fixCaseID {
				printFixCase(fc)
				return nil
			}
		}
		return fmt.Errorf("fix case %q not found in %s", fixledgerCfg.fixCaseID, fixCasesYAMLPath(repoRoot))
	},
}

// ---- pattern-status command ----

var awarenessPatternStatusCmd = &cobra.Command{
	Use:   "pattern-status",
	Short: "List fix cases matching a pattern",
	RunE: func(cmd *cobra.Command, args []string) error {
		if fixledgerCfg.pattern == "" {
			return fmt.Errorf("--pattern is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, err := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load fix cases: %w", err)
		}

		matched := fixledger.PatternStatus(fixledgerCfg.pattern, cases)
		if len(matched) == 0 {
			fmt.Fprintf(os.Stdout, "No fix cases match pattern %q\n", fixledgerCfg.pattern)
			return nil
		}

		fmt.Fprintf(os.Stdout, "# Fix Cases Matching %q\n\n", fixledgerCfg.pattern)
		for _, fc := range matched {
			fmt.Fprintf(os.Stdout, "- **%s** [%s]: %s\n", fc.ID, fc.Status, fc.Title)
		}
		return nil
	},
}

// ---- did-we-fix command ----

var awarenessDidWeFixCmd = &cobra.Command{
	Use:   "did-we-fix",
	Short: "Check whether a task or incident type has known fix cases",
	Long: `Queries the fix ledger to find fix cases that match the given task description.

Output includes matched fix cases, overall status, remaining gaps, and required tests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, aliases := loadFixLedger(repoRoot)
		result := fixledger.DidWeFix(awareCfg.task, cases, fixledger.ContextAliasMap(aliases))

		fmt.Fprintf(os.Stdout, "# Did We Fix?\n\n")
		fmt.Fprintf(os.Stdout, "**Task**: %s\n", awareCfg.task)
		fmt.Fprintf(os.Stdout, "**Matched fix cases**: %d\n", len(result.MatchedFixCases))
		fmt.Fprintf(os.Stdout, "**Overall status**: %s\n\n", result.OverallStatus)

		if len(result.MatchedFixCases) > 0 {
			fmt.Fprintf(os.Stdout, "## Matched fix cases\n")
			for _, fc := range result.MatchedFixCases {
				fmt.Fprintf(os.Stdout, "- %s [%s]\n", fc.ID, fc.Status)
			}
			fmt.Fprintln(os.Stdout)
		}

		if len(result.RemainingFiles) > 0 {
			fmt.Fprintf(os.Stdout, "## Remaining gaps\n")
			for _, f := range result.RemainingFiles {
				fmt.Fprintf(os.Stdout, "- %s\n", f)
			}
			fmt.Fprintln(os.Stdout)
		}

		if len(result.RequiredTests) > 0 {
			fmt.Fprintf(os.Stdout, "## Required tests\n")
			for _, t := range result.RequiredTests {
				fmt.Fprintf(os.Stdout, "- %s\n", t)
			}
			fmt.Fprintln(os.Stdout)
		}

		fmt.Fprintf(os.Stdout, "## Next action\n%s\n", result.NextAction)

		return nil
	},
}

// ---- partials command ----

var awarenessPartialsCmd = &cobra.Command{
	Use:   "partials",
	Short: "List fix cases with PARTIAL status",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, err := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load fix cases: %w", err)
		}

		partials := fixledger.ListPartials(cases)
		if len(partials) == 0 {
			fmt.Fprintf(os.Stdout, "No partial fix cases found.\n")
			return nil
		}

		fmt.Fprintf(os.Stdout, "# Partial Fix Cases (%d)\n\n", len(partials))
		for _, fc := range partials {
			fmt.Fprintf(os.Stdout, "## %s\n", fc.ID)
			fmt.Fprintf(os.Stdout, "**Title**: %s\n", fc.Title)
			if len(fc.RemainingFiles) > 0 {
				fmt.Fprintf(os.Stdout, "**Remaining files**:\n")
				for _, f := range fc.RemainingFiles {
					fmt.Fprintf(os.Stdout, "  - %s\n", f)
				}
			}
			if len(fc.RequiredTests) > 0 {
				fmt.Fprintf(os.Stdout, "**Required tests**:\n")
				for _, t := range fc.RequiredTests {
					fmt.Fprintf(os.Stdout, "  - %s\n", t)
				}
			}
			fmt.Fprintln(os.Stdout)
		}
		return nil
	},
}

// ---- regressions command ----

var awarenessRegressionsCmd = &cobra.Command{
	Use:   "regressions",
	Short: "List fix cases with REGRESSED status",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, err := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load fix cases: %w", err)
		}

		regressions := fixledger.ListRegressions(cases)
		if len(regressions) == 0 {
			fmt.Fprintf(os.Stdout, "No regressions found.\n")
			return nil
		}

		fmt.Fprintf(os.Stdout, "# REGRESSIONS DETECTED (%d)\n\n", len(regressions))
		for _, fc := range regressions {
			fmt.Fprintf(os.Stdout, "- **%s**: %s\n", fc.ID, fc.Title)
		}
		return nil
	},
}

// ---- coverage command ----

var awarenessCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Show invariant coverage by fix cases",
	Long: `Lists all known invariants with the fix cases that target each one.
Highlights invariants with no fix cases and fix cases with no required tests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, err := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load fix cases: %w", err)
		}

		// Collect all invariant IDs mentioned in fix cases.
		invSeen := make(map[string]bool)
		var invariants []string
		for _, fc := range cases {
			for _, invID := range fc.TargetInvariants {
				if !invSeen[invID] {
					invSeen[invID] = true
					invariants = append(invariants, invID)
				}
			}
		}

		report := fixledger.CoverageReport(cases, invariants)

		fmt.Fprintf(os.Stdout, "# Fix Case Coverage Report\n\n")
		fmt.Fprintf(os.Stdout, "**Fix cases**: %d\n", len(cases))
		fmt.Fprintf(os.Stdout, "**Invariants covered**: %d\n\n", len(invariants))

		for invID, fcs := range report {
			if len(fcs) == 0 {
				fmt.Fprintf(os.Stdout, "- **%s** — NO FIX CASES\n", invID)
			} else {
				testCount := 0
				for _, fc := range fcs {
					testCount += len(fc.RequiredTests)
				}
				testNote := ""
				if testCount == 0 {
					testNote = " ⚠ no required tests"
				}
				fmt.Fprintf(os.Stdout, "- **%s** — %d fix case(s), %d test(s)%s\n",
					invID, len(fcs), testCount, testNote)
				for _, fc := range fcs {
					fmt.Fprintf(os.Stdout, "    - %s [%s]\n", fc.ID, fc.Status)
				}
			}
		}

		return nil
	},
}

// ---- remaining-gaps command ----

var awarenessRemainingGapsCmd = &cobra.Command{
	Use:   "remaining-gaps",
	Short: "Show all files with remaining work across partial fix cases",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		cases, err := fixledger.LoadFixCases(fixCasesYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load fix cases: %w", err)
		}

		partials := fixledger.ListPartials(cases)
		if len(partials) == 0 {
			fmt.Fprintf(os.Stdout, "No remaining gaps found — all tracked fixes are DONE or better.\n")
			return nil
		}

		fmt.Fprintf(os.Stdout, "# Remaining Gaps\n\n")
		for _, fc := range partials {
			if len(fc.RemainingFiles) == 0 {
				continue
			}
			fmt.Fprintf(os.Stdout, "## %s\n", fc.ID)
			for _, f := range fc.RemainingFiles {
				fmt.Fprintf(os.Stdout, "- %s\n", f)
			}
			fmt.Fprintln(os.Stdout)
		}
		return nil
	},
}

// ---- guardrail-status command ----

var awarenessGuardrailStatusCmd = &cobra.Command{
	Use:   "guardrail-status",
	Short: "Show the status of a specific guardrail by ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if fixledgerCfg.guardrailID == "" {
			return fmt.Errorf("--id is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		guardrails, err := fixledger.LoadGuardrails(guardrailsYAMLPath(repoRoot))
		if err != nil {
			return fmt.Errorf("load guardrails: %w", err)
		}

		for _, g := range guardrails {
			if g.ID == fixledgerCfg.guardrailID {
				fmt.Fprintf(os.Stdout, "# Guardrail: %s\n\n", g.ID)
				fmt.Fprintf(os.Stdout, "**Title**: %s\n", g.Title)
				fmt.Fprintf(os.Stdout, "**Priority**: %s\n", g.Priority)
				fmt.Fprintf(os.Stdout, "**Status**: %s\n", g.Status)
				fmt.Fprintf(os.Stdout, "**Category**: %s\n", g.Category)
				if g.Summary != "" {
					fmt.Fprintf(os.Stdout, "**Summary**: %s\n", g.Summary)
				}
				if len(g.TargetInvariants) > 0 {
					fmt.Fprintf(os.Stdout, "\n**Target invariants**:\n")
					for _, inv := range g.TargetInvariants {
						fmt.Fprintf(os.Stdout, "  - %s\n", inv)
					}
				}
				if len(g.RequiredFixes) > 0 {
					fmt.Fprintf(os.Stdout, "\n**Required fixes**:\n")
					for _, fixID := range g.RequiredFixes {
						fmt.Fprintf(os.Stdout, "  - %s\n", fixID)
					}
				}
				return nil
			}
		}
		return fmt.Errorf("guardrail %q not found in %s", fixledgerCfg.guardrailID, guardrailsYAMLPath(repoRoot))
	},
}

// ---- helpers ----

func printFixCase(fc fixledger.FixCase) {
	fmt.Fprintf(os.Stdout, "# Fix Case: %s\n\n", fc.ID)
	fmt.Fprintf(os.Stdout, "**Title**: %s\n", fc.Title)
	fmt.Fprintf(os.Stdout, "**Status**: %s\n", fc.Status)
	if fc.Category != "" {
		fmt.Fprintf(os.Stdout, "**Category**: %s\n", fc.Category)
	}
	if fc.Pattern != "" {
		fmt.Fprintf(os.Stdout, "**Pattern**: %s\n", fc.Pattern)
	}
	if fc.DoD != "" {
		fmt.Fprintf(os.Stdout, "**DoD**: %s\n", fc.DoD)
	}
	if fc.Notes != "" {
		fmt.Fprintf(os.Stdout, "**Notes**: %s\n", fc.Notes)
	}
	if len(fc.TargetInvariants) > 0 {
		fmt.Fprintf(os.Stdout, "\n**Target invariants**:\n")
		for _, inv := range fc.TargetInvariants {
			fmt.Fprintf(os.Stdout, "  - %s\n", inv)
		}
	}
	if len(fc.FixedFiles) > 0 {
		fmt.Fprintf(os.Stdout, "\n**Fixed files**:\n")
		for _, f := range fc.FixedFiles {
			fmt.Fprintf(os.Stdout, "  - %s\n", f)
		}
	}
	if len(fc.RemainingFiles) > 0 {
		fmt.Fprintf(os.Stdout, "\n**Remaining files**:\n")
		for _, f := range fc.RemainingFiles {
			// Strip inline comments.
			f = strings.SplitN(f, "#", 2)[0]
			f = strings.TrimSpace(f)
			if f != "" {
				fmt.Fprintf(os.Stdout, "  - %s\n", f)
			}
		}
	}
	if len(fc.RequiredTests) > 0 {
		fmt.Fprintf(os.Stdout, "\n**Required tests**:\n")
		for _, t := range fc.RequiredTests {
			fmt.Fprintf(os.Stdout, "  - %s\n", t)
		}
	}
}

func init() {
	// fix-status flags.
	awarenessFixStatusCmd.Flags().StringVar(&fixledgerCfg.fixCaseID, "id", "", "Fix case ID")
	awarenessFixStatusCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// pattern-status flags.
	awarenessPatternStatusCmd.Flags().StringVar(&fixledgerCfg.pattern, "pattern", "", "Pattern to search for")
	awarenessPatternStatusCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// did-we-fix flags.
	awarenessDidWeFixCmd.Flags().StringVar(&awareCfg.task, "task", "", "Task description to match against fix cases")
	awarenessDidWeFixCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// partials flags.
	awarenessPartialsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// regressions flags.
	awarenessRegressionsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// coverage flags.
	awarenessCoverageCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// remaining-gaps flags.
	awarenessRemainingGapsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// guardrail-status flags.
	awarenessGuardrailStatusCmd.Flags().StringVar(&fixledgerCfg.guardrailID, "id", "", "Guardrail ID")
	awarenessGuardrailStatusCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Register all fix ledger commands under awarenessCmd.
	awarenessCmd.AddCommand(awarenessFixStatusCmd)
	awarenessCmd.AddCommand(awarenessPatternStatusCmd)
	awarenessCmd.AddCommand(awarenessDidWeFixCmd)
	awarenessCmd.AddCommand(awarenessPartialsCmd)
	awarenessCmd.AddCommand(awarenessRegressionsCmd)
	awarenessCmd.AddCommand(awarenessCoverageCmd)
	awarenessCmd.AddCommand(awarenessRemainingGapsCmd)
	awarenessCmd.AddCommand(awarenessGuardrailStatusCmd)
}
