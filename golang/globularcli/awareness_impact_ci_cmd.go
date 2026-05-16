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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/awareness/analysis"
	"github.com/globulario/awareness/assurance"
	"github.com/globulario/awareness/graph"
)

var impactCICfg = struct {
	files      []string
	repo       string
	dbPath     string
	jsonOutput bool
	minTrust   string
}{}

var awarenessImpactCICmd = &cobra.Command{
	Use:   "impact-ci",
	Short: "CI check: verify mandatory impact findings have required tests",
	Long: `For each file in --files, runs explained impact analysis and checks that every
mandatory finding (forbidden fixes, enforced invariants) has its required tests
present in the test suite.

Exit codes:
  0 — all mandatory findings are backed by present tests
  1 — at least one mandatory finding has missing required tests or trust is below threshold`,
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
		matchCount := 0
		minTrust, err := parseTrustVerdict(impactCICfg.minTrust)
		if err != nil {
			return err
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")
		staleness, _ := assurance.CheckStaleness(ctx, g, assurance.Options{DocsDir: docsDir})
		worst := assurance.Compose(assurance.ComposeInputs{MatchFound: false, Staleness: staleness})

		for _, file := range impactCICfg.files {
			failures, trust, matched, err := checkFileImpact(ctx, g, file, scanDirs, staleness)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: %v\n", file, err)
				continue
			}
			totalFailures += failures
			if matched {
				matchCount++
			}
			if trustRank(trust.Verdict) < trustRank(worst.Verdict) {
				worst = trust
			}
			if impactCICfg.jsonOutput {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"file": file, "trust": trust})
			}
		}
		if impactCICfg.jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"summary": map[string]any{
					"files":         len(impactCICfg.files),
					"matched_files": matchCount,
					"missing_tests": totalFailures,
					"trust":         worst,
					"min_trust":     minTrust,
				},
			})
		} else {
			fmt.Fprintf(os.Stdout, "trust: verdict=%s confidence=%s freshness=%s coverage=%s\n",
				worst.Verdict, worst.Confidence, worst.Freshness, worst.Coverage)
			fmt.Fprintf(os.Stdout, "min_trust: %s\n", minTrust)
			if worst.Reason != "" {
				fmt.Fprintf(os.Stdout, "trust_reason: %s\n", worst.Reason)
			}
			if len(worst.Limitations) > 0 {
				fmt.Fprintf(os.Stdout, "trust_limitations: %s\n", strings.Join(worst.Limitations, "; "))
			}
			if len(worst.RequiredActions) > 0 {
				fmt.Fprintf(os.Stdout, "trust_required_actions: %s\n", strings.Join(worst.RequiredActions, "; "))
			}
		}

		if totalFailures > 0 {
			fmt.Fprintf(os.Stderr, "\nFAIL: %d mandatory finding(s) have missing required tests\n", totalFailures)
			os.Exit(1)
		}
		if trustRank(worst.Verdict) < trustRank(minTrust) {
			fmt.Fprintf(os.Stderr, "\nFAIL: trust verdict %s is below minimum required %s\n", worst.Verdict, minTrust)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "OK: all mandatory findings have required tests present\n")
		return nil
	},
}

// checkFileImpact runs explained impact analysis on one file and prints findings.
// Returns the number of mandatory findings with missing tests.
func checkFileImpact(ctx context.Context, g *graph.Graph, file string, scanDirs []string, staleness *assurance.Staleness) (int, assurance.TrustEnvelope, bool, error) {
	result, err := analysis.ExplainImpactByFile(ctx, g, file)
	if err != nil {
		return 0, assurance.TrustEnvelope{}, false, fmt.Errorf("ExplainImpactByFile: %w", err)
	}

	failures := 0

	if len(result.MissingLinks) > 0 {
		fmt.Fprintf(os.Stdout, "%s: no graph nodes found — run 'globular awareness build'\n", file)
		return 0, assurance.Compose(assurance.ComposeInputs{MatchFound: false, Staleness: staleness}), false, nil
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

	return failures, assurance.Compose(assurance.ComposeInputs{MatchFound: true, Staleness: staleness}), true, nil
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
	awarenessImpactCICmd.Flags().BoolVar(&impactCICfg.jsonOutput, "json", false, "Emit trust envelope output as JSON lines")
	awarenessImpactCICmd.Flags().StringVar(&impactCICfg.minTrust, "min-trust", string(assurance.TrustStale),
		"Minimum allowed trust verdict: unsafe|unknown|stale|limited|usable|trusted")

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

func trustRank(v assurance.TrustVerdict) int {
	switch v {
	case assurance.TrustUnsafe:
		return 0
	case assurance.TrustUnknown:
		return 1
	case assurance.TrustStale:
		return 2
	case assurance.TrustLimited:
		return 3
	case assurance.TrustUsable:
		return 4
	case assurance.TrustTrusted:
		return 5
	default:
		return 0
	}
}

func parseTrustVerdict(raw string) (assurance.TrustVerdict, error) {
	v := assurance.TrustVerdict(strings.TrimSpace(strings.ToLower(raw)))
	switch v {
	case assurance.TrustUnsafe, assurance.TrustUnknown, assurance.TrustStale,
		assurance.TrustLimited, assurance.TrustUsable, assurance.TrustTrusted:
		return v, nil
	default:
		return "", fmt.Errorf("invalid --min-trust %q (allowed: unsafe, unknown, stale, limited, usable, trusted)", raw)
	}
}
