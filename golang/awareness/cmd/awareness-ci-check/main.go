// awareness-ci-check verifies awareness gap tests against CI test evidence,
// and optionally runs a full graph integrity check.
//
// It reads agent_playbooks.yaml to find gaps with required_tests, scans the
// awareness test files to verify those tests exist, and optionally upgrades
// the verification status using a test-results.json file produced by
// go-test-to-awareness.
//
// Usage:
//
//	awareness-ci-check \
//	  --playbooks docs/awareness/knowledge/agent_playbooks.yaml \
//	  --test-results .awareness/test-results.json \
//	  --repo-root .
//
//	awareness-ci-check --graph-integrity \
//	  --docs-dir docs/awareness \
//	  --repo-root . \
//	  --test-results .awareness/test-results.json
//
// Exit codes:
//
//	0 — all gaps are at tests_found or above, no integrity failures
//	1 — at least one gap has tests_not_found or invalid_metadata (CI should fail)
//	2 — configuration error (missing required flags, unreadable files)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/integrity"
)

// ── Schema types ─────────────────────────────────────────────────────────────

type agentPlaybooks struct {
	CapabilityGapPatterns []capabilityGapPattern `yaml:"capability_gap_patterns"`
}

type capabilityGapPattern struct {
	ID             string   `yaml:"id"`
	Status         string   `yaml:"status"`
	TestsRequired  []string `yaml:"tests_required"`
}

type testResultsFile struct {
	Command      string   `json:"command"`
	Passed       bool     `json:"passed"`
	Tests        []testEntry `json:"tests"`
	FailedTests  []string `json:"failed_tests"`
	SkippedTests []string `json:"skipped_tests"`
}

type testEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ── Gap report ───────────────────────────────────────────────────────────────

type gapReport struct {
	GapID              string `json:"gap_id"`
	Status             string `json:"gap_status"`
	VerificationStatus string `json:"verification_status"`
	Note               string `json:"note"`
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	playbooksPath := flag.String("playbooks", "docs/awareness/knowledge/agent_playbooks.yaml", "Path to agent_playbooks.yaml")
	testResultsPath := flag.String("test-results", "", "Path to .awareness/test-results.json (optional)")
	repoRoot := flag.String("repo-root", ".", "Repo root — golang/awareness/ test files are scanned from here")
	jsonOutput := flag.Bool("json", false, "Output JSON report instead of human-readable text")
	// Graph integrity flags.
	graphIntegrity := flag.Bool("graph-integrity", false, "Run graph integrity check (shape validation, contradictions, test refs)")
	docsDir := flag.String("docs-dir", "docs/awareness", "Path to docs/awareness for graph integrity check")
	strictIntegrity := flag.Bool("strict", false, "If true, treat integrity warnings as failures (exit code 1)")
	flag.Parse()

	if *graphIntegrity {
		runGraphIntegrity(*docsDir, *repoRoot, *testResultsPath, *strictIntegrity, *jsonOutput)
		return
	}

	pb, err := loadPlaybooks(*playbooksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot read playbooks: %v\n", err)
		os.Exit(2)
	}

	var tr *testResultsFile
	if *testResultsPath != "" {
		tr, err = loadTestResults(*testResultsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot read test-results: %v\n", err)
			os.Exit(2)
		}
	}

	awareDir := filepath.Join(*repoRoot, "golang", "awareness")

	var reports []gapReport
	notFoundCount := 0
	invalidMetaCount := 0
	strictVerifiedCount := 0
	testsFoundCount := 0

	for _, gap := range pb.CapabilityGapPatterns {
		if gap.Status != "implemented" && gap.Status != "closed" {
			continue
		}
		status, note := verifyTests(awareDir, gap.TestsRequired)
		if tr != nil && (status == "tests_found" || status == "tests_partial") {
			status, note = upgradeStatus(status, note, gap.TestsRequired, tr)
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

	if *jsonOutput {
		out := map[string]interface{}{
			"gaps":                  reports,
			"strict_verified_count": strictVerifiedCount,
			"tests_found_count":     testsFoundCount,
			"tests_not_found_count": notFoundCount,
			"invalid_metadata_count": invalidMetaCount,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
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
		fmt.Printf("\n")
		fmt.Printf("strict_verified: %d\n", strictVerifiedCount)
		fmt.Printf("tests_found:     %d\n", testsFoundCount)
		fmt.Printf("tests_not_found: %d\n", notFoundCount)
		fmt.Printf("invalid_metadata:%d\n", invalidMetaCount)
	}

	if notFoundCount > 0 || invalidMetaCount > 0 {
		if !*jsonOutput {
			fmt.Fprintf(os.Stderr, "\nFAIL: %d gap(s) have missing or invalid test evidence\n", notFoundCount+invalidMetaCount)
		}
		os.Exit(1)
	}
}

// ── Graph integrity check ─────────────────────────────────────────────────────

// runGraphIntegrity runs the Phase 11 graph integrity check and exits with the
// appropriate code.
//
// CI fail conditions (exit 1):
//   - DONE fix case missing required tests
//   - Invalid YAML metadata in any shape
//   - Causal rule contradicts forbidden fix or ordering constraint
//   - Referenced forbidden fix doesn't exist
//
// CI warn conditions (exit 0 unless --strict):
//   - PARTIAL fix cases
//   - Missing safe_alternative in forbidden fix
//   - Pathless tests (function exists but no source path)
//   - Inferred edges without provenance
func runGraphIntegrity(docsDir, repoRoot, testResultsFile string, strict, jsonOut bool) {
	opts := integrity.Options{
		DocsDir:         docsDir,
		RepoRoot:        repoRoot,
		TestResultsFile: testResultsFile,
		Strict:          strict,
	}

	result, err := integrity.Check(context.Background(), opts, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: graph integrity check failed: %v\n", err)
		os.Exit(3)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	} else {
		printIntegrityResult(result)
	}

	os.Exit(result.ExitCode)
}

func printIntegrityResult(r *integrity.IntegrityResult) {
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

// ── File loading ──────────────────────────────────────────────────────────────

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

// ── Verification logic ────────────────────────────────────────────────────────

func verifyTests(awareDir string, testsRequired []string) (status, note string) {
	if len(testsRequired) == 0 {
		return "no_tests_required", ""
	}

	normalized := normalizeFuncNames(testsRequired)

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
	_ = filepath.Walk(awareDir, func(path string, info os.FileInfo, err error) error {
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
		return "tests_not_found", fmt.Sprintf("0/%d required tests found in %s", total, awareDir)
	}
}

func upgradeStatus(baseStatus, baseNote string, testsRequired []string, tr *testResultsFile) (string, string) {
	if tr == nil {
		return baseStatus, baseNote
	}
	normalized := normalizeFuncNames(testsRequired)

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

func normalizeFuncNames(entries []string) []string {
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
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) < 5 {
		return false
	}
	return unicode.IsUpper(rune(name[4]))
}
