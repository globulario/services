package main

// awareness_audit_cmd.go — `globular awareness audit`
//
// Self-audit: runs all mechanical validators against the awareness graph
// and reports health. Catches drift, staleness, coverage gaps, stale refs,
// and missing tests — in one command.
//
// Usage:
//
//	globular awareness audit
//	globular awareness audit --verbose
//	globular awareness audit --check   # exit 1 on any FAIL (CI mode)

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/awareness-graph/golang/extractor"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── flags ────────────────────────────────────────────────────────────────

var (
	auditVerbose bool
	auditCIMode  bool // CI mode: exit 1 on any FAIL
)

var awarenessAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Self-audit the awareness graph for drift, gaps, and inconsistencies",
	Long: `Runs all mechanical validators against the awareness graph and reports
health. Each check produces PASS, WARN, or FAIL.

Checks:
  embeddata-freshness    Is the committed awareness.nt current with YAML sources?
  yaml-validity          Do all YAML files parse and import cleanly (strict mode)?
  ntriples-validity      Is the generated N-Triples output well-formed?
  coverage-gaps          High-risk files with zero awareness anchors?
  stale-file-refs        Entries referencing files that no longer exist on disk?
  test-coverage          Critical/high invariants missing required_tests?

Use --check for CI: exits 1 if any check is FAIL.
Use --verbose for per-finding details.`,
	RunE: runAwarenessAudit,
}

// ── result types ─────────────────────────────────────────────────────────

type checkLevel int

const (
	checkPASS checkLevel = iota
	checkWARN
	checkFAIL
)

func (r checkLevel) String() string {
	switch r {
	case checkPASS:
		return "PASS"
	case checkWARN:
		return "WARN"
	case checkFAIL:
		return "FAIL"
	}
	return "?"
}

type auditCheck struct {
	name    string
	result  checkLevel
	summary string
	details []string
}

// ── entry point ──────────────────────────────────────────────────────────

func runAwarenessAudit(cmd *cobra.Command, args []string) error {
	svcRepo, err := resolveServicesRepo()
	if err != nil {
		return err
	}
	agRepo, err := resolveAGRepo(svcRepo)
	if err != nil {
		return err
	}

	fmt.Println("Awareness graph self-audit")
	fmt.Println()

	inputDirs, intentDir, err := collectInputDirs(svcRepo, agRepo)
	if err != nil {
		return err
	}

	var checks []auditCheck

	// Generate NT once, reuse across checks.
	fmt.Println("  generating N-Triples...")
	ntBytes, totalTriples, yamlCount, genErr := generateNTriples(inputDirs, intentDir, svcRepo, agRepo)

	// 1. Embeddata freshness
	if genErr != nil {
		checks = append(checks, auditCheck{name: "embeddata-freshness", result: checkFAIL, summary: genErr.Error()})
	} else {
		checks = append(checks, auditEmbeddataFreshness(ntBytes, agRepo))
	}

	// 2. YAML validity (strict)
	if genErr != nil {
		checks = append(checks, auditCheck{name: "yaml-validity", result: checkFAIL, summary: genErr.Error()})
	} else {
		checks = append(checks, auditYAMLValidity(inputDirs, intentDir, svcRepo, agRepo, yamlCount))
	}

	// 3. N-Triples well-formedness
	if genErr != nil {
		checks = append(checks, auditCheck{name: "ntriples-validity", result: checkFAIL, summary: genErr.Error()})
	} else {
		checks = append(checks, auditNTriplesValidity(ntBytes, totalTriples))
	}

	// 4. Coverage gaps
	if genErr == nil {
		checks = append(checks, auditCoverageGaps(svcRepo, ntBytes))
	}

	// 5. Stale file references
	if genErr == nil {
		checks = append(checks, auditStaleFileRefs(svcRepo, ntBytes))
	}

	// 6. Test coverage for critical invariants
	checks = append(checks, auditTestCoverage(svcRepo))

	// ── report ───────────────────────────────────────────────────────────
	fmt.Println()
	fails := 0
	warns := 0
	for _, c := range checks {
		marker := " "
		switch c.result {
		case checkFAIL:
			marker = "x"
			fails++
		case checkWARN:
			marker = "!"
			warns++
		}
		fmt.Printf("  %s %-24s %s  %s\n", marker, c.name, c.result, c.summary)
		if auditVerbose && len(c.details) > 0 {
			for _, d := range c.details {
				fmt.Printf("      %s\n", d)
			}
		}
	}

	fmt.Println()
	fmt.Printf("  %d checks: %d pass, %d warn, %d fail\n",
		len(checks), len(checks)-fails-warns, warns, fails)

	if auditCIMode && fails > 0 {
		os.Exit(1)
	}
	return nil
}

// ── check 1: embeddata freshness ─────────────────────────────────────────

func auditEmbeddataFreshness(ntBytes []byte, agRepo string) auditCheck {
	seedPath := filepath.Join(agRepo, "golang", "server", "embeddata", "awareness.nt")

	committed, err := os.ReadFile(seedPath)
	if err != nil {
		return auditCheck{name: "embeddata-freshness", result: checkFAIL, summary: "cannot read embeddata: " + err.Error()}
	}

	newHash := sha256.Sum256(ntBytes)
	oldHash := sha256.Sum256(committed)
	if newHash == oldHash {
		return auditCheck{name: "embeddata-freshness", result: checkPASS, summary: "current"}
	}

	newLines := bytes.Count(ntBytes, []byte("\n"))
	oldLines := bytes.Count(committed, []byte("\n"))
	return auditCheck{
		name: "embeddata-freshness", result: checkFAIL,
		summary: fmt.Sprintf("STALE (committed: %d lines, generated: %d lines)", oldLines, newLines),
		details: []string{"run: globular awareness rebuild"},
	}
}

// ── check 2: YAML validity ───────────────────────────────────────────────

func auditYAMLValidity(inputDirs []string, intentDir, svcRepo, agRepo string, totalFiles int) auditCheck {
	opts := extractor.ImportDirOptions{
		StripPathPrefixes: []string{agRepo, svcRepo},
	}
	var skipped int
	var details []string

	scanDir := func(dir string) {
		_, report, err := extractor.ImportAwarenessDirWithOpts(dir, &bytes.Buffer{}, opts)
		if err != nil {
			return
		}
		for _, f := range report.Skipped() {
			if f.Status == extractor.StatusUnknownSchema || f.Status == extractor.StatusInvalid {
				skipped++
				details = append(details, fmt.Sprintf("%s: %s (%s)", f.Status, f.Path, f.Reason))
			}
		}
	}

	for _, dir := range inputDirs {
		scanDir(dir)
	}
	if intentDir != "" {
		scanDir(intentDir)
	}

	if skipped > 0 {
		return auditCheck{
			name: "yaml-validity", result: checkFAIL,
			summary: fmt.Sprintf("%d/%d files unknown or invalid", skipped, totalFiles),
			details: details,
		}
	}
	return auditCheck{name: "yaml-validity", result: checkPASS, summary: fmt.Sprintf("%d files clean", totalFiles)}
}

// ── check 3: N-Triples well-formedness ───────────────────────────────────

func auditNTriplesValidity(ntBytes []byte, totalTriples int) auditCheck {
	errs := extractor.ValidateNTriples(bytes.NewReader(ntBytes))
	if len(errs) == 0 {
		return auditCheck{
			name: "ntriples-validity", result: checkPASS,
			summary: fmt.Sprintf("%d triples, all valid", totalTriples),
		}
	}
	var details []string
	for i, e := range errs {
		if i >= 10 {
			details = append(details, fmt.Sprintf("... %d more", len(errs)-i))
			break
		}
		details = append(details, e.Error())
	}
	return auditCheck{
		name: "ntriples-validity", result: checkFAIL,
		summary: fmt.Sprintf("%d validation errors in %d triples", len(errs), totalTriples),
		details: details,
	}
}

// ── check 4: coverage gaps ───────────────────────────────────────────────

func auditCoverageGaps(svcRepo string, ntBytes []byte) auditCheck {
	hrfPath := filepath.Join(svcRepo, "docs", "awareness", "high_risk_files.yaml")
	raw, err := os.ReadFile(hrfPath)
	if err != nil {
		return auditCheck{name: "coverage-gaps", result: checkWARN, summary: "cannot read high_risk_files.yaml: " + err.Error()}
	}
	var doc struct {
		Files []string `yaml:"files"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return auditCheck{name: "coverage-gaps", result: checkWARN, summary: "parse error: " + err.Error()}
	}

	// Build a set of all file paths referenced in the NT.
	fileRefs := collectFilePathsFromNT(ntBytes)

	var uncovered []string
	for _, f := range doc.Files {
		f = strings.TrimSuffix(f, "/")
		found := false
		for ref := range fileRefs {
			if strings.Contains(ref, f) {
				found = true
				break
			}
		}
		if !found {
			uncovered = append(uncovered, f)
		}
	}

	if len(uncovered) == 0 {
		return auditCheck{
			name: "coverage-gaps", result: checkPASS,
			summary: fmt.Sprintf("all %d high-risk files have anchors", len(doc.Files)),
		}
	}
	return auditCheck{
		name: "coverage-gaps", result: checkWARN,
		summary: fmt.Sprintf("%d/%d high-risk files have no anchors", len(uncovered), len(doc.Files)),
		details: uncovered,
	}
}

// ── check 5: stale file references ───────────────────────────────────────

func auditStaleFileRefs(svcRepo string, ntBytes []byte) auditCheck {
	fileRefs := collectFilePathsFromNT(ntBytes)

	var stale []string
	checked := 0
	for path := range fileRefs {
		// Only check paths that look like repo-relative Go/YAML files.
		if !strings.HasPrefix(path, "golang/") && !strings.HasPrefix(path, "docs/") &&
			!strings.HasPrefix(path, "proto/") && !strings.HasPrefix(path, "typescript/") {
			continue
		}
		checked++
		absPath := filepath.Join(svcRepo, path)
		if _, err := os.Stat(absPath); err != nil {
			stale = append(stale, path)
		}
	}

	if len(stale) == 0 {
		return auditCheck{
			name: "stale-file-refs", result: checkPASS,
			summary: fmt.Sprintf("all %d referenced files exist", checked),
		}
	}
	return auditCheck{
		name: "stale-file-refs", result: checkWARN,
		summary: fmt.Sprintf("%d/%d referenced files missing from disk", len(stale), checked),
		details: stale,
	}
}

// ── check 6: test coverage for critical invariants ───────────────────────

func auditTestCoverage(svcRepo string) auditCheck {
	invPath := filepath.Join(svcRepo, "docs", "awareness", "invariants.yaml")
	raw, err := os.ReadFile(invPath)
	if err != nil {
		return auditCheck{name: "test-coverage", result: checkWARN, summary: "cannot read invariants.yaml"}
	}
	var doc struct {
		Invariants []struct {
			ID            string   `yaml:"id"`
			Severity      string   `yaml:"severity"`
			RequiredTests []string `yaml:"required_tests"`
		} `yaml:"invariants"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return auditCheck{name: "test-coverage", result: checkWARN, summary: "parse error: " + err.Error()}
	}

	var critical, missing int
	var details []string
	for _, inv := range doc.Invariants {
		if inv.Severity != "critical" && inv.Severity != "high" {
			continue
		}
		critical++
		if len(inv.RequiredTests) == 0 {
			missing++
			details = append(details, fmt.Sprintf("[%s] %s", inv.Severity, inv.ID))
		}
	}

	if missing == 0 {
		return auditCheck{
			name: "test-coverage", result: checkPASS,
			summary: fmt.Sprintf("all %d critical/high invariants have required_tests", critical),
		}
	}
	return auditCheck{
		name: "test-coverage", result: checkWARN,
		summary: fmt.Sprintf("%d/%d critical/high invariants missing required_tests", missing, critical),
		details: details,
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

// collectFilePathsFromNT scans the NT for sourceFile IRIs and extracts
// repo-relative paths. Returns a set of path strings.
func collectFilePathsFromNT(ntBytes []byte) map[string]bool {
	paths := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(ntBytes))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		// Look for sourceFile IRIs: <https://globular.io/awareness#sourceFile/golang%2F...>
		idx := strings.Index(line, "#sourceFile/")
		if idx < 0 {
			continue
		}
		rest := line[idx+len("#sourceFile/"):]
		end := strings.IndexByte(rest, '>')
		if end < 0 {
			continue
		}
		path := rest[:end]
		path = strings.ReplaceAll(path, "%2F", "/")
		path = strings.ReplaceAll(path, "%2f", "/")
		paths[path] = true
	}
	return paths
}

// ── wiring ───────────────────────────────────────────────────────────────

func init() {
	awarenessAuditCmd.Flags().BoolVar(&auditVerbose, "verbose", false,
		"Show per-finding details for WARN and FAIL checks")
	awarenessAuditCmd.Flags().BoolVar(&auditCIMode, "check", false,
		"Exit 1 if any check is FAIL (CI mode)")

	awarenessCmd.AddCommand(awarenessAuditCmd)
}
