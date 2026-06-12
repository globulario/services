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
	auditFix     bool // auto-fix mechanical issues
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
Use --verbose for per-finding details.
Use --fix to auto-repair mechanical issues:
  - Stale embeddata → rebuild
  - Stale file refs → remove entries pointing at deleted files
  - Oxigraph reload after any fix`,
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
	fixFn   func() error // nil = no auto-fix available
	fixDesc string       // human-readable description of what --fix will do
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

	seedPath := filepath.Join(agRepo, "golang", "server", "embeddata", "awareness.nt")

	// 1. Embeddata freshness
	if genErr != nil {
		checks = append(checks, auditCheck{name: "embeddata-freshness", result: checkFAIL, summary: genErr.Error()})
	} else {
		c := auditEmbeddataFreshness(ntBytes, agRepo)
		if c.result == checkFAIL {
			c.fixDesc = "rebuild embeddata + reload Oxigraph"
			c.fixFn = func() error {
				if err := updateEmbeddataAndReload(ntBytes, seedPath); err != nil {
					return err
				}
				return nil
			}
		}
		checks = append(checks, c)
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
		c := auditStaleFileRefs(svcRepo, ntBytes)
		if c.result != checkPASS && len(c.details) > 0 {
			staleFiles := append([]string{}, c.details...) // capture
			c.fixDesc = fmt.Sprintf("remove %d stale SourceFile entries from YAML + rebuild", len(staleFiles))
			c.fixFn = func() error {
				removed := removeStaleFileRefsFromYAML(svcRepo, agRepo, staleFiles)
				if removed > 0 {
					fmt.Printf("    removed %d stale file references from YAML\n", removed)
					fmt.Println("    rebuilding...")
					newNT, _, _, err := generateNTriples(inputDirs, intentDir, svcRepo, agRepo)
					if err != nil {
						return err
					}
					return updateEmbeddataAndReload(newNT, seedPath)
				}
				return nil
			}
		}
		checks = append(checks, c)
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

	// ── auto-fix ─────────────────────────────────────────────────────────
	if auditFix {
		var fixable []auditCheck
		for _, c := range checks {
			if c.result != checkPASS && c.fixFn != nil {
				fixable = append(fixable, c)
			}
		}
		if len(fixable) == 0 {
			fmt.Println("\n  --fix: nothing to auto-fix")
		} else {
			fmt.Printf("\n  --fix: %d fixable issue(s)\n", len(fixable))
			fixed := 0
			for _, c := range fixable {
				fmt.Printf("\n  fixing: %s — %s\n", c.name, c.fixDesc)
				if err := c.fixFn(); err != nil {
					fmt.Printf("    FAILED: %v\n", err)
				} else {
					fmt.Println("    done")
					fixed++
				}
			}
			fmt.Printf("\n  %d/%d fixes applied\n", fixed, len(fixable))
		}
	}

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
	var invalidCount, unknownCount int
	var invalidDetails, unknownDetails []string

	scanDir := func(dir string) {
		_, report, err := extractor.ImportAwarenessDirWithOpts(dir, &bytes.Buffer{}, opts)
		if err != nil {
			return
		}
		for _, f := range report.Skipped() {
			switch f.Status {
			case extractor.StatusInvalid:
				invalidCount++
				invalidDetails = append(invalidDetails, fmt.Sprintf("INVALID: %s (%s)", f.Path, f.Reason))
			case extractor.StatusUnknownSchema:
				unknownCount++
				unknownDetails = append(unknownDetails, fmt.Sprintf("unknown: %s (%s)", f.Path, f.Reason))
			}
		}
	}

	for _, dir := range inputDirs {
		scanDir(dir)
	}
	if intentDir != "" {
		scanDir(intentDir)
	}

	allDetails := append(invalidDetails, unknownDetails...)

	// Invalid files (parse errors) are FAIL. Unknown schemas (awaiting importers) are WARN.
	if invalidCount > 0 {
		return auditCheck{
			name: "yaml-validity", result: checkFAIL,
			summary: fmt.Sprintf("%d invalid, %d unknown schema (of %d files)", invalidCount, unknownCount, totalFiles),
			details: allDetails,
		}
	}
	if unknownCount > 0 {
		return auditCheck{
			name: "yaml-validity", result: checkWARN,
			summary: fmt.Sprintf("%d unknown schema, 0 invalid (of %d files)", unknownCount, totalFiles),
			details: allDetails,
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

// ── fix helpers ──────────────────────────────────────────────────────────

// updateEmbeddataAndReload writes the NT seed and reloads Oxigraph.
func updateEmbeddataAndReload(ntBytes []byte, seedPath string) error {
	updated, err := updateEmbeddata(ntBytes, seedPath)
	if err != nil {
		return err
	}
	if updated {
		fmt.Printf("    embeddata updated: %s\n", seedPath)
	}
	if err := reloadOxigraph(ntBytes); err != nil {
		fmt.Printf("    Oxigraph reload: skipped (%v)\n", err)
	} else {
		fmt.Println("    Oxigraph reload: ok")
	}
	return nil
}

// removeStaleFileRefsFromYAML scans awareness YAML files and removes
// entries whose protects.files list references only deleted files.
// Returns the number of entries cleaned.
func removeStaleFileRefsFromYAML(svcRepo, agRepo string, stalePaths []string) int {
	staleSet := make(map[string]bool, len(stalePaths))
	for _, p := range stalePaths {
		staleSet[p] = true
	}

	// The stale refs come from two sources:
	// 1. Services repo YAML (protects.files in invariants, failure_modes, etc.)
	// 2. Awareness-graph repo YAML (code_symbols, edges)
	//
	// For services repo: clean protects.files lists.
	// For awareness-graph repo refs (golang/server/*, golang/store/*, etc.):
	// these are awareness-graph-relative paths authored in awareness-graph YAML.
	// We can't fix those from here — they need fixing in the awareness-graph repo.

	cleaned := 0
	awarenessDir := filepath.Join(svcRepo, "docs", "awareness")

	for _, yamlFile := range []string{
		"invariants.yaml", "failure_modes.yaml", "incident_patterns.yaml",
		"forbidden_fixes.yaml", "required_tests.yaml",
	} {
		path := filepath.Join(awarenessDir, yamlFile)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var doc map[string]interface{}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			continue
		}

		modified := false
		for _, listKey := range []string{
			"invariants", "failure_modes", "incident_patterns",
			"forbidden_fixes", "required_tests",
		} {
			entries, ok := doc[listKey].([]interface{})
			if !ok {
				continue
			}
			for _, entry := range entries {
				m, ok := entry.(map[string]interface{})
				if !ok {
					continue
				}
				// Clean protects.files
				if protects, ok := m["protects"].(map[string]interface{}); ok {
					if files, ok := protects["files"].([]interface{}); ok {
						var kept []interface{}
						for _, f := range files {
							fs, ok := f.(string)
							if !ok || !staleSet[fs] {
								kept = append(kept, f)
							} else {
								cleaned++
								modified = true
							}
						}
						if len(kept) == 0 {
							delete(protects, "files")
						} else {
							protects["files"] = kept
						}
					}
				}
				// Clean top-level files (incident_patterns)
				if files, ok := m["files"].([]interface{}); ok {
					var kept []interface{}
					for _, f := range files {
						fs, ok := f.(string)
						if !ok || !staleSet[fs] {
							kept = append(kept, f)
						} else {
							cleaned++
							modified = true
						}
					}
					if len(kept) == 0 {
						delete(m, "files")
					} else {
						m["files"] = kept
					}
				}
			}
		}

		if modified {
			out, err := yaml.Marshal(doc)
			if err != nil {
				continue
			}
			_ = os.WriteFile(path, out, 0o644)
		}
	}

	return cleaned
}

// ── wiring ───────────────────────────────────────────────────────────────

func init() {
	awarenessAuditCmd.Flags().BoolVar(&auditVerbose, "verbose", false,
		"Show per-finding details for WARN and FAIL checks")
	awarenessAuditCmd.Flags().BoolVar(&auditCIMode, "check", false,
		"Exit 1 if any check is FAIL (CI mode)")
	awarenessAuditCmd.Flags().BoolVar(&auditFix, "fix", false,
		"Auto-repair mechanical issues (stale embeddata, stale file refs)")

	awarenessCmd.AddCommand(awarenessAuditCmd)
}
