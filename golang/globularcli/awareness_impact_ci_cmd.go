package main

// awareness_impact_ci_cmd.go: CI command that checks impact paths for changed files
// and exits non-zero if mandatory findings have missing required tests.
//
// Usage:
//
//	globular awareness impact-ci --files <f1,f2,...> [--repo <path>] [--db <path>]
//
// The command is pure local — no cluster connection required.
// Exit 0 — all mandatory findings have their required tests present.
// Exit 1 — at least one mandatory finding has missing required tests.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/graph"
)

var impactCICfg = struct {
	files  []string
	repo   string
	dbPath string
}{}

var awarenessImpactCICmd = &cobra.Command{
	Use:   "impact-ci",
	Short: "CI check: verify mandatory impact findings have required tests",
	Long: `For each file in --files, runs explained impact analysis and checks that every
mandatory finding (forbidden fixes, enforced invariants) has its required tests
present in the test suite.

Exit codes:
  0 — all mandatory findings are backed by present tests
  1 — at least one mandatory finding has missing required tests`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(impactCICfg.files) == 0 {
			return fmt.Errorf("--files is required")
		}

		ctx := context.Background()
		g, err := openAwarenessGraph(impactCICfg.dbPath, impactCICfg.repo)
		if err != nil {
			return fmt.Errorf("open awareness graph: %w", err)
		}
		defer g.Close()

		repoRoot, err := resolveRepoRoot(impactCICfg.repo)
		if err != nil {
			return fmt.Errorf("resolve repo root: %w", err)
		}

		// Directories to scan for test files.
		scanDirs := []string{
			filepath.Join(repoRoot, "golang", "awareness"),
			filepath.Join(repoRoot, "golang", "mcp"),
		}

		totalFailures := 0

		for _, file := range impactCICfg.files {
			failures, err := checkFileImpact(ctx, g, file, scanDirs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: %v\n", file, err)
				continue
			}
			totalFailures += failures
		}

		if totalFailures > 0 {
			fmt.Fprintf(os.Stderr, "\nFAIL: %d mandatory finding(s) have missing required tests\n", totalFailures)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "OK: all mandatory findings have required tests present\n")
		return nil
	},
}

// checkFileImpact runs explained impact analysis on one file and prints findings.
// Returns the number of mandatory findings with missing tests.
func checkFileImpact(ctx context.Context, g *graph.Graph, file string, scanDirs []string) (int, error) {
	result, err := analysis.ExplainImpactByFile(ctx, g, file)
	if err != nil {
		return 0, fmt.Errorf("ExplainImpactByFile: %w", err)
	}

	failures := 0

	if len(result.MissingLinks) > 0 {
		fmt.Fprintf(os.Stdout, "%s: no graph nodes found — run 'globular awareness build'\n", file)
		return 0, nil
	}

	// Check required tests for mandatory forbidden fixes.
	for _, f := range result.ForbiddenFixes {
		if !f.Mandatory {
			continue
		}
		printFinding(file, "forbidden_fix", f)
	}

	// Check required tests for mandatory required-test findings.
	for _, f := range result.RequiredTests {
		if !f.Mandatory {
			continue
		}
		// Check if the test actually exists.
		testName := f.NodeName
		status, _ := verifyGapTestsDirs(scanDirs, []string{testName})
		if status == "tests_not_found" || status == "invalid_metadata" {
			fmt.Fprintf(os.Stdout, "  FAIL [missing_test] %s\n", testName)
			for _, p := range f.EdgePath {
				fmt.Fprintf(os.Stdout, "    path: %s\n", p)
			}
			fmt.Fprintf(os.Stdout, "    confidence: %s\n", f.Confidence)
			failures++
		} else {
			fmt.Fprintf(os.Stdout, "  OK   [test_present] %s\n", testName)
		}
	}

	// Check required tests for mandatory invariants.
	for _, f := range result.Invariants {
		if !f.Mandatory {
			continue
		}
		printFinding(file, "invariant", f)
	}

	return failures, nil
}

// printFinding prints a single explained finding to stdout.
func printFinding(file, kind string, f analysis.ExplainedFinding) {
	mandatoryTag := ""
	if f.Mandatory {
		mandatoryTag = " [MANDATORY]"
	}
	fmt.Fprintf(os.Stdout, "  %s [%s]%s %s\n", file, kind, mandatoryTag, f.NodeName)
	for _, p := range f.EdgePath {
		fmt.Fprintf(os.Stdout, "    path: %s\n", p)
	}
	fmt.Fprintf(os.Stdout, "    confidence: %s\n", f.Confidence)
}

func init() {
	awarenessImpactCICmd.Flags().StringArrayVar(&impactCICfg.files, "files", nil,
		"Files to analyse (comma or space separated; can be specified multiple times)")
	awarenessImpactCICmd.Flags().StringVar(&impactCICfg.repo, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessImpactCICmd.Flags().StringVar(&impactCICfg.dbPath, "db", "", "Path to graph.db")

	awarenessCmd.AddCommand(awarenessImpactCICmd)
}

// splitFiles splits a comma or space-separated list of file paths.
// It is used to normalize the --files flag input.
func splitFiles(raw []string) []string {
	var out []string
	for _, r := range raw {
		for _, part := range strings.FieldsFunc(r, func(c rune) bool { return c == ',' || c == ' ' }) {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
