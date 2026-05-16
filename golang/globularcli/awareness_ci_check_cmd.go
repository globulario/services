package main

// awareness_ci_check_cmd.go: CI check commands for the awareness graph.
//
// Usage:
//
//	globular awareness ci-check [--playbooks <path>] [--test-results <path>] [--repo-root <path>] [--json]
//	globular awareness graph-integrity [--docs-dir <path>] [--repo-root <path>] [--test-results <path>] [--strict] [--json]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/globulario/awareness/integrity"
)

// ── Schema types ──────────────────────────────────────────────────────────────

type agentPlaybooks struct {
	CapabilityGapPatterns []capabilityGapPattern `yaml:"capability_gap_patterns"`
}

type capabilityGapPattern struct {
	ID            string   `yaml:"id"`
	Status        string   `yaml:"status"`
	TestsRequired []string `yaml:"tests_required"`
}

type testResultsFile struct {
	Command      string      `json:"command"`
	Passed       bool        `json:"passed"`
	Tests        []testEntry `json:"tests"`
	FailedTests  []string    `json:"failed_tests"`
	SkippedTests []string    `json:"skipped_tests"`
}

type testEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type gapReport struct {
	GapID              string `json:"gap_id"`
	Status             string `json:"gap_status"`
	VerificationStatus string `json:"verification_status"`
	Note               string `json:"note"`
}

// ── ci-check command ──────────────────────────────────────────────────────────

var ciCheckCfg = struct {
	playbooksPath   string
	testResultsPath string
	repoRoot        string
	jsonOutput      bool
}{}

var awarenessCiCheckCmd = &cobra.Command{
	Use:   "ci-check",
	Short: "Verify awareness gap tests against CI test evidence",
	Long: `Reads agent_playbooks.yaml to find capability gaps with required_tests,
scans the golang/awareness/ test files to verify those tests exist, and
optionally upgrades verification status using a test-results.json file
produced by 'globular awareness test-results'.

Exit codes:
  0 — all gaps are at tests_found or above
  1 — at least one gap has tests_not_found or invalid_metadata`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		pb, err := loadPlaybooks(ciCheckCfg.playbooksPath)
		if err != nil {
			return fmt.Errorf("cannot read playbooks: %w", err)
		}

		var tr *testResultsFile
		if ciCheckCfg.testResultsPath != "" {
			tr, err = loadTestResults(ciCheckCfg.testResultsPath)
			if err != nil {
				return fmt.Errorf("cannot read test-results: %w", err)
			}
		}

		// Awareness tests live in both golang/awareness/ and golang/mcp/ (consolidated).
		scanDirs := []string{
			filepath.Join(ciCheckCfg.repoRoot, "golang", "awareness"),
			filepath.Join(ciCheckCfg.repoRoot, "golang", "mcp"),
		}

		var reports []gapReport
		notFoundCount, invalidMetaCount, strictVerifiedCount, testsFoundCount := 0, 0, 0, 0

		for _, gap := range pb.CapabilityGapPatterns {
			if gap.Status != "implemented" && gap.Status != "closed" {
				continue
			}
			status, note := verifyGapTestsDirs(scanDirs, gap.TestsRequired)
			if tr != nil && (status == "tests_found" || status == "tests_partial") {
				status, note = upgradeGapStatus(status, note, gap.TestsRequired, tr)
			}
			switch status {
			case "tests_not_found":
				notFoundCount++
			case "invalid_metadata":
				invalidMetaCount++
			case "strict_verified":
				strictVerifiedCount++
			case "tests_found", "tests_partial":
				testsFoundCount++
			}
			reports = append(reports, gapReport{
				GapID:              gap.ID,
				Status:             gap.Status,
				VerificationStatus: status,
				Note:               note,
			})
		}

		if ciCheckCfg.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]interface{}{
				"gaps":                   reports,
				"strict_verified_count":  strictVerifiedCount,
				"tests_found_count":      testsFoundCount,
				"tests_not_found_count":  notFoundCount,
				"invalid_metadata_count": invalidMetaCount,
			})
		} else {
			for _, r := range reports {
				marker := "  "
				if r.VerificationStatus == "tests_not_found" || r.VerificationStatus == "invalid_metadata" {
					marker = "✗ "
				} else if r.VerificationStatus == "strict_verified" {
					marker = "✓ "
				}
				fmt.Printf("%s%-60s %s\n", marker, r.GapID, r.VerificationStatus)
				if r.Note != "" {
					fmt.Printf("     %s\n", r.Note)
				}
			}
			fmt.Printf("\nstrict_verified: %d\ntests_found:     %d\ntests_not_found: %d\ninvalid_metadata:%d\n",
				strictVerifiedCount, testsFoundCount, notFoundCount, invalidMetaCount)
		}

		if notFoundCount > 0 || invalidMetaCount > 0 {
			if !ciCheckCfg.jsonOutput {
				fmt.Fprintf(os.Stderr, "\nFAIL: %d gap(s) have missing or invalid test evidence\n", notFoundCount+invalidMetaCount)
			}
			os.Exit(1)
		}
		return nil
	},
}

// ── graph-integrity command ───────────────────────────────────────────────────

var graphIntegrityCfg = struct {
	docsDir         string
	repoRoot        string
	testResultsPath string
	strict          bool
	jsonOutput      bool
}{}

var awarenessGraphIntegrityCmd = &cobra.Command{
	Use:   "graph-integrity",
	Short: "Run graph integrity check (shape validation, contradictions, test refs)",
	Long: `Validates the awareness graph for shape violations, causal rule contradictions,
missing test references, and inferred edges without provenance.

Exit codes:
  0 — graph is clean
  1 — integrity failures found
  3 — check could not run (internal error)`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := integrity.Options{
			DocsDir:         graphIntegrityCfg.docsDir,
			RepoRoot:        graphIntegrityCfg.repoRoot,
			TestResultsFile: graphIntegrityCfg.testResultsPath,
			Strict:          graphIntegrityCfg.strict,
		}

		result, err := integrity.Check(context.Background(), opts, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: graph integrity check failed: %v\n", err)
			os.Exit(3)
		}

		if graphIntegrityCfg.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		} else {
			printGraphIntegrityResult(result)
		}

		os.Exit(result.ExitCode)
		return nil
	},
}

func printGraphIntegrityResult(r *integrity.IntegrityResult) {
	fmt.Printf("graph integrity: %s\n\n", r.Status)
	fmt.Printf("nodes: %d  edges: %d\n", r.Summary.Nodes, r.Summary.Edges)
	fmt.Printf("shape violations: %d  contradictions: %d  missing tests: %d\n\n",
		r.Summary.InvalidShapes, r.Summary.Contradictions, r.Summary.MissingTests)

	for _, v := range r.InvalidShapes {
		marker := "⚠ "
		if v.Severity == "critical" {
			marker = "✗ "
		}
		fmt.Printf("%s[%s] %s.%s: %s\n", marker, v.Severity, v.NodeID, v.Field, v.Message)
	}
	for _, c := range r.Contradictions {
		fmt.Printf("✗ [critical] contradiction in causal_rule:%s — %s\n", c.CausalRuleID, c.Reason)
	}
	for _, ti := range r.MissingTests {
		marker := "⚠ "
		if ti.Severity == "critical" {
			marker = "✗ "
		}
		fmt.Printf("%s[%s] %s → %s: %s\n", marker, ti.Severity, ti.FixCaseID, ti.TestName, ti.Issue)
	}
	if len(r.RecommendedActions) > 0 {
		fmt.Printf("\nrecommended actions:\n")
		for _, a := range r.RecommendedActions {
			fmt.Printf("  • %s\n", a)
		}
	}
	if r.ExitCode == 0 {
		fmt.Println("\ngraph integrity: OK")
	}
}

// ── Verification helpers ──────────────────────────────────────────────────────

func loadPlaybooks(path string) (*agentPlaybooks, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pb agentPlaybooks
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, err
	}
	return &pb, nil
}

func loadTestResults(path string) (*testResultsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tr testResultsFile
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func verifyGapTests(awareDir string, testsRequired []string) (status, note string) {
	return verifyGapTestsDirs([]string{awareDir}, testsRequired)
}

func verifyGapTestsDirs(dirs []string, testsRequired []string) (status, note string) {
	if len(testsRequired) == 0 {
		return "no_tests_required", ""
	}

	normalized := normalizeTestFuncNames(testsRequired)

	var invalid []string
	for i, n := range normalized {
		if !isValidTestFuncName(n) {
			invalid = append(invalid, fmt.Sprintf("%q (from %q)", n, testsRequired[i]))
		}
	}
	if len(invalid) > 0 {
		return "invalid_metadata", fmt.Sprintf(
			"tests_required contains %d non-function-name entry(ies): %s",
			len(invalid), strings.Join(invalid, "; "))
	}

	found := make(map[string]bool)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			content := string(data)
			for _, name := range normalized {
				if strings.Contains(content, "func "+name+"(") {
					found[name] = true
				}
			}
			return nil
		})
	}

	foundCount := len(found)
	total := len(normalized)
	switch {
	case foundCount == total:
		return "tests_found", fmt.Sprintf("%d/%d required tests found", foundCount, total)
	case foundCount > 0:
		var missing []string
		for _, name := range normalized {
			if !found[name] {
				missing = append(missing, name)
			}
		}
		return "tests_partial", fmt.Sprintf("%d/%d found; missing: %s", foundCount, total, strings.Join(missing, ", "))
	default:
		return "tests_not_found", fmt.Sprintf("0/%d required tests found in %s", total, strings.Join(dirs, " or "))
	}
}

func upgradeGapStatus(baseStatus, baseNote string, testsRequired []string, tr *testResultsFile) (string, string) {
	if tr == nil {
		return baseStatus, baseNote
	}
	normalized := normalizeTestFuncNames(testsRequired)

	failedSet := make(map[string]bool, len(tr.FailedTests))
	for _, t := range tr.FailedTests {
		failedSet[t] = true
	}
	skippedSet := make(map[string]bool, len(tr.SkippedTests))
	for _, t := range tr.SkippedTests {
		skippedSet[t] = true
	}

	var failed, skipped []string
	for _, name := range normalized {
		if failedSet[name] {
			failed = append(failed, name)
		}
		if skippedSet[name] {
			skipped = append(skipped, name)
		}
	}

	switch {
	case len(failed) > 0:
		return "tests_failed", fmt.Sprintf("%d required test(s) failed: %s", len(failed), strings.Join(failed, ", "))
	case len(skipped) > 0:
		return "tests_found_but_skipped", fmt.Sprintf("%d required test(s) skipped: %s", len(skipped), strings.Join(skipped, ", "))
	case tr.Passed:
		return "strict_verified", fmt.Sprintf("all %d required tests found and passed (%s)", len(normalized), tr.Command)
	default:
		return "tests_passed", fmt.Sprintf("required tests found; suite passed=%v", tr.Passed)
	}
}

func normalizeTestFuncNames(entries []string) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		if idx := strings.IndexByte(entry, ' '); idx >= 0 {
			out[i] = strings.TrimSpace(entry[:idx])
		} else {
			out[i] = strings.TrimSpace(entry)
		}
	}
	return out
}

func isValidTestFuncName(name string) bool {
	if !strings.HasPrefix(name, "Test") || len(name) < 5 {
		return false
	}
	return unicode.IsUpper(rune(name[4]))
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	awarenessCiCheckCmd.Flags().StringVar(&ciCheckCfg.playbooksPath, "playbooks", "docs/awareness/knowledge/agent_playbooks.yaml", "Path to agent_playbooks.yaml")
	awarenessCiCheckCmd.Flags().StringVar(&ciCheckCfg.testResultsPath, "test-results", "", "Path to .awareness/test-results.json (optional)")
	awarenessCiCheckCmd.Flags().StringVar(&ciCheckCfg.repoRoot, "repo-root", ".", "Repo root — golang/awareness/ test files are scanned from here")
	awarenessCiCheckCmd.Flags().BoolVar(&ciCheckCfg.jsonOutput, "json", false, "Output JSON report instead of human-readable text")

	awarenessGraphIntegrityCmd.Flags().StringVar(&graphIntegrityCfg.docsDir, "docs-dir", "docs/awareness", "Path to docs/awareness")
	awarenessGraphIntegrityCmd.Flags().StringVar(&graphIntegrityCfg.repoRoot, "repo-root", ".", "Repo root")
	awarenessGraphIntegrityCmd.Flags().StringVar(&graphIntegrityCfg.testResultsPath, "test-results", "", "Path to .awareness/test-results.json (optional)")
	awarenessGraphIntegrityCmd.Flags().BoolVar(&graphIntegrityCfg.strict, "strict", false, "Treat integrity warnings as failures")
	awarenessGraphIntegrityCmd.Flags().BoolVar(&graphIntegrityCfg.jsonOutput, "json", false, "Output JSON report instead of human-readable text")

	awarenessCmd.AddCommand(awarenessCiCheckCmd)
	awarenessCmd.AddCommand(awarenessGraphIntegrityCmd)
}
