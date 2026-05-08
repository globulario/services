package main

// awareness_enforce_cmds.go: CLI commands for annotation enforcement (Task 9).
//
// Commands:
//
//	globular awareness audit [--repo <path>] [--db <path>] [--json]
//	globular awareness validate-annotations [--repo <path>]
//	globular awareness validate-required-tests [--db <path>]
//	globular awareness validate-contracts [--db <path>]
//	globular awareness graph-drift [--repo <path>] [--db <path>]
//	globular awareness pr-report [--files <f1,f2>]
//	globular awareness hook --file <file> [--task <task>] [--db <path>]

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/enforce"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/preflight"
)

var enforceCfg = struct {
	jsonOutput            bool
	files                 []string
	fromGitDiff           bool
	strict                bool
	watchlist             string
	auditStrict           bool
	summary               bool
	failOnWarning         bool
	warningThreshold      int
	suppressionsFile      string
	showSuppressed        bool
	maxRequiredTestNoPath int
	trendFile             string
	trendRecord           bool
	trendLast             int
	scaffoldLimit         int
}{
	warningThreshold:      -1, // disabled by default
	maxRequiredTestNoPath: -1,
	trendLast:             20,
	scaffoldLimit:         20,
}

type requiredTestBacklogEntry struct {
	Test        string   `json:"test"`
	Refs        []string `json:"refs"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// ---- audit command ----

var awarenessAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run all awareness enforcement checks (annotations, contracts, tests, drift)",
	Long: `Validates annotation well-formedness, hash-schema contracts, required-test
existence, and graph drift. Exits with code 1 if any ERROR findings are found.

Suppressions file (--suppressions) moves known warning backlogs out of the main
output without hiding them — they appear in the "Suppressed" section with counts.
ERRORs are never suppressible.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		g, _ := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if g != nil {
			defer g.Close()
		}

		golangDir := filepath.Join(repoRoot, "golang")
		result := enforce.Audit(ctx, g, enforce.AuditOptions{
			RepoRoot: repoRoot,
			SrcDir:   golangDir,
		})
		if enforceCfg.auditStrict {
			watchlist := enforceCfg.watchlist
			if watchlist == "" {
				watchlist = filepath.Join(repoRoot, "docs", "awareness", "high_risk_files.yaml")
			}
			cov := enforce.AnnotationCoverage(ctx, g, enforce.AnnotationCoverageOptions{
				RepoRoot:      repoRoot,
				SrcDir:        golangDir,
				WatchlistPath: watchlist,
				DocsDir:       filepath.Join(repoRoot, "docs", "awareness"),
			})
			result.Findings = append(result.Findings, cov.Findings...)
			for _, f := range cov.Findings {
				switch f.Severity {
				case enforce.SeverityError:
					result.ErrorCount++
				case enforce.SeverityWarning:
					result.WarningCount++
				default:
					result.InfoCount++
				}
			}
			result.Pass = result.ErrorCount == 0
		}

		// Load suppressions (default: docs/awareness/audit_suppressions.yaml).
		suppressionsPath := enforceCfg.suppressionsFile
		if suppressionsPath == "" {
			suppressionsPath = filepath.Join(repoRoot, "docs", "awareness", "audit_suppressions.yaml")
		}
		sf, err := enforce.LoadSuppressions(suppressionsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load suppressions from %s: %v\n", suppressionsPath, err)
			sf = &enforce.SuppressionFile{}
		}

		// Apply suppressions and group findings.
		triaged := enforce.Triage(result, sf, timeNow())

		// Render output.
		if enforceCfg.jsonOutput {
			fmt.Fprint(os.Stdout, enforce.RenderTriagedJSON(triaged))
			fmt.Fprintln(os.Stdout)
		} else {
			fmt.Fprint(os.Stdout, enforce.RenderTriagedMarkdown(triaged, enforce.RenderOptions{
				Summary:        enforceCfg.summary,
				ShowSuppressed: enforceCfg.showSuppressed,
			}))
		}

		// Exit codes:
		//   1 — errors present (always)
		//   1 — --fail-on-warning and unsuppressed warnings > 0
		//   1 — --warning-threshold exceeded
		//   1 — strict mode and suppression problems (expired/max_count/invalid)
		if !triaged.Pass {
			os.Exit(1)
		}
		if enforceCfg.failOnWarning && triaged.WarningCount > 0 {
			os.Exit(1)
		}
		if enforce.FailsWarningThreshold(triaged, enforceCfg.warningThreshold) {
			os.Exit(1)
		}
		if enforceCfg.maxRequiredTestNoPath >= 0 {
			if count := warningGroupCount(triaged.Groups, "REQUIRED_TEST_NO_PATH"); count > enforceCfg.maxRequiredTestNoPath {
				fmt.Fprintf(os.Stderr, "REQUIRED_TEST_NO_PATH threshold exceeded: %d > %d\n", count, enforceCfg.maxRequiredTestNoPath)
				os.Exit(1)
			}
		}
		if enforceCfg.auditStrict && enforce.HasSuppressionProblems(triaged) {
			os.Exit(1)
		}
		return nil
	},
}

var awarenessRequiredTestsBacklogCmd = &cobra.Command{
	Use:   "required-tests-backlog",
	Short: "List unique missing required test targets and referencing symbols",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		findings := enforce.ValidateRequiredTestsWithRepo(ctx, g, repoRoot)
		entries, err := buildRequiredTestBacklogEntries(ctx, g, findings)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Fprintln(os.Stdout, "No REQUIRED_TEST_NO_PATH backlog entries.")
			return nil
		}
		outputMode := strings.ToLower(strings.TrimSpace(rootCfg.output))
		if outputMode == "json" {
			out := struct {
				Total   int                        `json:"total"`
				Entries []requiredTestBacklogEntry `json:"entries"`
			}{
				Total:   len(entries),
				Entries: entries,
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(os.Stdout, string(b))
			return nil
		}
		for _, entry := range entries {
			fmt.Fprintf(os.Stdout, "- %s", entry.Test)
			if len(entry.Refs) > 0 {
				fmt.Fprintf(os.Stdout, " | refs: %s", strings.Join(entry.Refs, ", "))
			}
			if len(entry.Suggestions) > 0 {
				fmt.Fprintf(os.Stdout, " | suggested-sources: %s", strings.Join(entry.Suggestions, ", "))
			}
			fmt.Fprintln(os.Stdout)
		}
		fmt.Fprintf(os.Stdout, "\nTotal missing test targets: %d\n", len(entries))
		return nil
	},
}

var awarenessRequiredTestsScaffoldCmd = &cobra.Command{
	Use:   "required-tests-scaffold",
	Short: "Generate test stubs for missing required tests",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		findings := enforce.ValidateRequiredTestsWithRepo(ctx, g, repoRoot)
		entries, err := buildRequiredTestBacklogEntries(ctx, g, findings)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Fprintln(os.Stdout, "No REQUIRED_TEST_NO_PATH backlog entries.")
			return nil
		}
		limit := enforceCfg.scaffoldLimit
		if limit > 0 && len(entries) > limit {
			entries = entries[:limit]
		}

		outputMode := strings.ToLower(strings.TrimSpace(rootCfg.output))
		if outputMode == "json" {
			out := struct {
				Generated int                        `json:"generated"`
				Entries   []requiredTestBacklogEntry `json:"entries"`
			}{
				Generated: len(entries),
				Entries:   entries,
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(os.Stdout, string(b))
			return nil
		}

		for _, entry := range entries {
			primary := ""
			if len(entry.Suggestions) > 0 {
				primary = entry.Suggestions[0]
			}
			if primary == "" {
				primary = "golang/<area>/TODO_test.go"
			}
			fmt.Fprintf(os.Stdout, "// target: %s\n", primary)
			fmt.Fprintf(os.Stdout, "// refs: %s\n", strings.Join(entry.Refs, ", "))
			fmt.Fprintf(os.Stdout, "func %s(t *testing.T) {\n", entry.Test)
			fmt.Fprintf(os.Stdout, "\tt.Skip(\"TODO: implement required awareness test\")\n")
			fmt.Fprintf(os.Stdout, "}\n\n")
		}
		fmt.Fprintf(os.Stdout, "Generated %d test stubs (from %d backlog entries).\n", len(entries), len(entries))
		return nil
	},
}

func buildRequiredTestBacklogEntries(ctx context.Context, g *graph.Graph, findings []enforce.Finding) ([]requiredTestBacklogEntry, error) {
	byTest := make(map[string]map[string]bool)
	re := regexp.MustCompile(`tested_by target '([^']+)'`)
	for _, f := range findings {
		if f.Code != "REQUIRED_TEST_NO_PATH" {
			continue
		}
		m := re.FindStringSubmatch(f.Message)
		if len(m) != 2 {
			continue
		}
		testName := m[1]
		if byTest[testName] == nil {
			byTest[testName] = make(map[string]bool)
		}
		if strings.TrimSpace(f.Symbol) != "" {
			byTest[testName][f.Symbol] = true
		}
	}

	var tests []string
	for t := range byTest {
		tests = append(tests, t)
	}
	sort.Strings(tests)
	entries := make([]requiredTestBacklogEntry, 0, len(tests))
	for _, tname := range tests {
		var symbols []string
		for s := range byTest[tname] {
			symbols = append(symbols, s)
		}
		sort.Strings(symbols)
		entry := requiredTestBacklogEntry{
			Test: tname,
			Refs: symbols,
		}
		for _, sym := range symbols {
			n, err := g.FindNode(ctx, sym)
			if err != nil || n == nil {
				continue
			}
			if p := strings.TrimSpace(n.Path); p != "" {
				entry.Suggestions = append(entry.Suggestions, p)
			}
		}
		entry.Suggestions = dedupeSorted(entry.Suggestions)
		entries = append(entries, entry)
	}
	return entries, nil
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

type warningTrendEntry struct {
	Timestamp string         `json:"timestamp"`
	Codes     map[string]int `json:"codes"`
}

var awarenessAuditTrendCmd = &cobra.Command{
	Use:   "audit-trend",
	Short: "Show or record audit warning trends by code",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		g, _ := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if g != nil {
			defer g.Close()
		}
		result := enforce.Audit(ctx, g, enforce.AuditOptions{
			RepoRoot: repoRoot,
			SrcDir:   filepath.Join(repoRoot, "golang"),
		})
		var warnings []enforce.Finding
		for _, f := range result.Findings {
			if f.Severity == enforce.SeverityWarning {
				warnings = append(warnings, f)
			}
		}
		groups := enforce.GroupFindings(warnings)
		current := make(map[string]int)
		for _, gr := range groups {
			current[gr.Code] = gr.Count
		}

		trendPath := enforceCfg.trendFile
		if trendPath == "" {
			trendPath = resolveAwarenessTrendPath(repoRoot)
		}
		entries, _ := readTrendEntries(trendPath)
		if enforceCfg.trendRecord {
			if err := os.MkdirAll(filepath.Dir(trendPath), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(trendPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			entry := warningTrendEntry{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Codes:     current,
			}
			b, _ := json.Marshal(entry)
			if _, err := f.Write(append(b, '\n')); err != nil {
				return err
			}
			entries = append(entries, entry)
		}

		start := 0
		if enforceCfg.trendLast > 0 && len(entries) > enforceCfg.trendLast {
			start = len(entries) - enforceCfg.trendLast
		}
		fmt.Fprintf(os.Stdout, "Current warning groups: %d\n", len(current))
		printTopWarningCodes(os.Stdout, current, 10)
		if len(entries) >= 2 {
			prev := entries[len(entries)-2].Codes
			fmt.Fprintln(os.Stdout, "\nDelta vs previous snapshot:")
			printWarningDelta(os.Stdout, prev, current)
		}
		if len(entries) > 0 {
			fmt.Fprintf(os.Stdout, "\nTrend snapshots (%d shown):\n", len(entries[start:]))
			for _, e := range entries[start:] {
				total := 0
				for _, c := range e.Codes {
					total += c
				}
				fmt.Fprintf(os.Stdout, "- %s total=%d codes=%d\n", e.Timestamp, total, len(e.Codes))
			}
		}
		return nil
	},
}

// ---- validate-annotations command ----

var awarenessValidateAnnotationsCmd = &cobra.Command{
	Use:   "validate-annotations",
	Short: "Validate //globular: annotation syntax in all Go source files",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		golangDir := filepath.Join(repoRoot, "golang")
		findings := enforce.ValidateAnnotations(golangDir)
		result := &enforce.AuditResult{}
		for _, f := range findings {
			result.Findings = append(result.Findings, f)
			switch f.Severity {
			case enforce.SeverityError:
				result.ErrorCount++
			case enforce.SeverityWarning:
				result.WarningCount++
			default:
				result.InfoCount++
			}
		}
		result.Pass = result.ErrorCount == 0

		if enforceCfg.jsonOutput {
			fmt.Fprint(os.Stdout, enforce.RenderAuditJSON(result))
			fmt.Fprintln(os.Stdout)
		} else {
			fmt.Fprint(os.Stdout, enforce.RenderAuditMarkdown(result))
		}

		if !result.Pass {
			os.Exit(1)
		}
		return nil
	},
}

// ---- validate-required-tests command ----

var awarenessValidateRequiredTestsCmd = &cobra.Command{
	Use:   "validate-required-tests",
	Short: "Check that all //globular:tested_by tests exist in the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		findings := enforce.ValidateRequiredTests(ctx, g)
		result := buildResultFromFindings(findings)

		if enforceCfg.jsonOutput {
			fmt.Fprint(os.Stdout, enforce.RenderAuditJSON(result))
			fmt.Fprintln(os.Stdout)
		} else {
			fmt.Fprint(os.Stdout, enforce.RenderAuditMarkdown(result))
		}

		if !result.Pass {
			os.Exit(1)
		}
		return nil
	},
}

// ---- validate-contracts command ----

var awarenessValidateContractsCmd = &cobra.Command{
	Use:   "validate-contracts",
	Short: "Check that all hash_schema contracts have both a producer and a consumer",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		findings := enforce.ValidateContracts(ctx, g)
		result := buildResultFromFindings(findings)

		if enforceCfg.jsonOutput {
			fmt.Fprint(os.Stdout, enforce.RenderAuditJSON(result))
			fmt.Fprintln(os.Stdout)
		} else {
			fmt.Fprint(os.Stdout, enforce.RenderAuditMarkdown(result))
		}

		if !result.Pass {
			os.Exit(1)
		}
		return nil
	},
}

// ---- graph-drift command ----

var awarenessGraphDriftCmd = &cobra.Command{
	Use:   "graph-drift",
	Short: "Report graph nodes that no longer correspond to files on disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		findings := enforce.AuditDrift(ctx, g, filepath.Join(repoRoot, "golang"))
		result := buildResultFromFindings(findings)

		if enforceCfg.jsonOutput {
			fmt.Fprint(os.Stdout, enforce.RenderAuditJSON(result))
			fmt.Fprintln(os.Stdout)
		} else {
			fmt.Fprint(os.Stdout, enforce.RenderAuditMarkdown(result))
		}

		// drift is informational — do not exit 1 on WARNING/INFO only
		if result.ErrorCount > 0 {
			os.Exit(1)
		}
		return nil
	},
}

var awarenessAnnotationCoverageCmd = &cobra.Command{
	Use:   "annotation-coverage",
	Short: "Report annotation coverage gaps on high-risk files and critical contracts",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		g, _ := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if g != nil {
			defer g.Close()
		}
		watchlist := enforceCfg.watchlist
		if watchlist == "" {
			watchlist = filepath.Join(repoRoot, "docs", "awareness", "high_risk_files.yaml")
		}
		result := enforce.AnnotationCoverage(ctx, g, enforce.AnnotationCoverageOptions{
			RepoRoot:      repoRoot,
			SrcDir:        filepath.Join(repoRoot, "golang"),
			WatchlistPath: watchlist,
			DocsDir:       filepath.Join(repoRoot, "docs", "awareness"),
		})
		if enforceCfg.jsonOutput {
			fmt.Fprint(os.Stdout, enforce.RenderAuditJSON(result))
			fmt.Fprintln(os.Stdout)
		} else {
			fmt.Fprint(os.Stdout, enforce.RenderAuditMarkdown(result))
		}
		if !result.Pass {
			os.Exit(1)
		}
		return nil
	},
}

// ---- pr-report command ----

var awarenessPRReportCmd = &cobra.Command{
	Use:   "pr-report",
	Short: "Report annotation findings for changed files (CI use)",
	Long: `Runs annotation validation only on changed files. Use --from-git-diff to
auto-detect changed files from the current git diff, or --files to supply them
explicitly. Intended for CI annotation workflows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		files := enforceCfg.files

		if enforceCfg.fromGitDiff {
			changed, err := changedGoFiles()
			if err != nil {
				return fmt.Errorf("git diff: %w", err)
			}
			files = append(files, changed...)
		}

		if len(files) == 0 {
			fmt.Fprintf(os.Stdout, "No changed files — nothing to report.\n")
			return nil
		}

		result := enforce.AuditFiles(files)

		pr := &enforce.PRReport{
			ChangedFiles: files,
			Findings:     result.Findings,
			ErrorCount:   result.ErrorCount,
			WarningCount: result.WarningCount,
			Pass:         result.Pass,
		}

		fmt.Fprint(os.Stdout, enforce.RenderPRReport(pr))

		if !result.Pass {
			os.Exit(1)
		}
		return nil
	},
}

// ---- hook command ----

var awarenessHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Pre-edit awareness hook for Claude Code — surfaces constraints for edited files",
	Long: `Runs annotation validation and surfaces graph-registered invariants, forbidden
fixes, and risks for the specified files. Designed to be called from a Claude Code
PreToolUse hook. Outputs human-readable markdown to stdout.

Exits 0 even when findings exist (informational hook, not a gate).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		files := enforceCfg.files
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		g, _ := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if g != nil {
			defer g.Close()
		}

		result, err := enforce.RunHook(ctx, g, files, awareCfg.task)
		if err != nil {
			return err
		}

		fmt.Fprint(os.Stdout, enforce.RenderHookText(result))

		// Default mode remains warning-only. Strict mode blocks only for high-risk files.
		if !enforceCfg.strict {
			return nil
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")
		watchlistPath := enforceCfg.watchlist
		if watchlistPath == "" {
			watchlistPath = filepath.Join(docsDir, "high_risk_files.yaml")
		}
		patterns, err := enforce.LoadHighRiskWatchlist(watchlistPath)
		if err != nil {
			return nil
		}
		highRisk := false
		for _, f := range files {
			rel := filepath.ToSlash(f)
			if filepath.IsAbs(f) {
				if r, err := filepath.Rel(repoRoot, f); err == nil {
					rel = filepath.ToSlash(r)
				}
			}
			if enforce.IsHighRiskFile(rel, patterns) {
				highRisk = true
				break
			}
		}
		if !highRisk {
			return nil
		}

		// Preflight for strict checks.
		pfr, _ := preflight.Run(ctx, preflight.Options{
			Task:    awareCfg.task,
			Files:   files,
			DocsDir: docsDir,
		}, g)

		fileAudit := enforce.AuditFiles(files)
		refFindings := enforce.ValidateAnnotationReferences(ctx, g, files)
		auditResult := enforce.Audit(ctx, g, enforce.AuditOptions{
			RepoRoot: repoRoot,
			SrcDir:   filepath.Join(repoRoot, "golang"),
		})
		var perFileAudit []enforce.Finding
		fileSet := map[string]bool{}
		for _, f := range files {
			fileSet[filepath.ToSlash(f)] = true
			fileSet[f] = true
		}
		for _, af := range auditResult.Findings {
			if fileSet[filepath.ToSlash(af.File)] {
				perFileAudit = append(perFileAudit, af)
			}
		}

		decision := enforce.EvaluateStrictGate(enforce.StrictGateInput{
			Strict:                true,
			HighRisk:              true,
			Files:                 files,
			Preflight:             pfr,
			FileAudit:             fileAudit,
			AnnotationRefFindings: refFindings,
			PerFileAuditFindings:  perFileAudit,
		})
		if decision.ShouldBlock {
			fmt.Fprintln(os.Stdout, "\n## Awareness hook: STRICT BLOCK")
			for _, reason := range decision.Reasons {
				fmt.Fprintln(os.Stdout, "- "+reason)
			}
			fmt.Fprintf(os.Stdout, "\nRun: globular awareness preflight --task %q --format agent\n", awareCfg.task)
			os.Exit(2)
		}
		return nil
	},
}

// ---- helpers ----

func buildResultFromFindings(findings []enforce.Finding) *enforce.AuditResult {
	r := &enforce.AuditResult{Findings: findings}
	for _, f := range findings {
		switch f.Severity {
		case enforce.SeverityError:
			r.ErrorCount++
		case enforce.SeverityWarning:
			r.WarningCount++
		default:
			r.InfoCount++
		}
	}
	r.Pass = r.ErrorCount == 0
	return r
}

func warningGroupCount(groups []enforce.FindingGroup, code string) int {
	for _, g := range groups {
		if g.Code == code {
			return g.Count
		}
	}
	return 0
}

func readTrendEntries(path string) ([]warningTrendEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var out []warningTrendEntry
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var e warningTrendEntry
		if err := json.Unmarshal([]byte(ln), &e); err == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

func printTopWarningCodes(w *os.File, codes map[string]int, n int) {
	type pair struct {
		code  string
		count int
	}
	var pairs []pair
	for c, ct := range codes {
		pairs = append(pairs, pair{c, ct})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].code < pairs[j].code
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	for _, p := range pairs {
		fmt.Fprintf(w, "- %s: %d\n", p.code, p.count)
	}
}

func printWarningDelta(w *os.File, prev, curr map[string]int) {
	seen := make(map[string]bool)
	var codes []string
	for c := range prev {
		seen[c] = true
		codes = append(codes, c)
	}
	for c := range curr {
		if !seen[c] {
			codes = append(codes, c)
		}
	}
	sort.Strings(codes)
	for _, c := range codes {
		d := curr[c] - prev[c]
		if d == 0 {
			continue
		}
		fmt.Fprintf(w, "- %s: %s%d (now %d)\n", c, map[bool]string{true: "+", false: ""}[d >= 0], d, curr[c])
	}
}

// timeNow is a variable so tests can stub it. Production callers leave it as-is.
var timeNow = func() time.Time { return time.Now() }

// changedGoFiles returns .go files changed in the current git working tree diff.
func changedGoFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasSuffix(line, ".go") {
			files = append(files, line)
		}
	}
	return files, nil
}

func init() {
	// audit
	awarenessAuditCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessAuditCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON instead of markdown")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.auditStrict, "strict", false, "Enable strict mode (includes annotation coverage + suppression problem checks)")
	awarenessAuditCmd.Flags().StringVar(&enforceCfg.watchlist, "watchlist", "", "Path to high-risk watchlist YAML")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.summary, "summary", false, "Print compact summary only")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.failOnWarning, "fail-on-warning", false, "Exit 1 if any unsuppressed warnings exist")
	awarenessAuditCmd.Flags().IntVar(&enforceCfg.warningThreshold, "warning-threshold", -1, "Exit 1 if unsuppressed warnings exceed N (-1 disables)")
	awarenessAuditCmd.Flags().StringVar(&enforceCfg.suppressionsFile, "suppressions", "", "Path to suppression YAML (default: docs/awareness/audit_suppressions.yaml)")
	awarenessAuditCmd.Flags().BoolVar(&enforceCfg.showSuppressed, "show-suppressed", false, "Include full detail of suppressed findings in output")
	awarenessAuditCmd.Flags().IntVar(&enforceCfg.maxRequiredTestNoPath, "max-required-test-no-path", -1, "Exit 1 if REQUIRED_TEST_NO_PATH warning count exceeds N (-1 disables)")

	// validate-annotations
	awarenessValidateAnnotationsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessValidateAnnotationsCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")

	// validate-required-tests
	awarenessValidateRequiredTestsCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessValidateRequiredTestsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessValidateRequiredTestsCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")

	// validate-contracts
	awarenessValidateContractsCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessValidateContractsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessValidateContractsCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")

	// graph-drift
	awarenessGraphDriftCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessGraphDriftCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessGraphDriftCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")

	// annotation-coverage
	awarenessAnnotationCoverageCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessAnnotationCoverageCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessAnnotationCoverageCmd.Flags().BoolVar(&enforceCfg.jsonOutput, "json", false, "Output JSON")
	awarenessAnnotationCoverageCmd.Flags().StringVar(&enforceCfg.watchlist, "watchlist", "", "Path to high-risk watchlist YAML")

	// pr-report
	awarenessPRReportCmd.Flags().StringSliceVar(&enforceCfg.files, "files", nil, "Comma-separated list of files to check")
	awarenessPRReportCmd.Flags().BoolVar(&enforceCfg.fromGitDiff, "from-git-diff", false, "Auto-detect changed files from git diff HEAD")

	// hook
	awarenessHookCmd.Flags().StringSliceVar(&enforceCfg.files, "file", nil, "File(s) being edited")
	awarenessHookCmd.Flags().StringVar(&awareCfg.task, "task", "", "Task description")
	awarenessHookCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessHookCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessHookCmd.Flags().BoolVar(&enforceCfg.strict, "strict", false, "Enable strict mode (blocks high-risk unsafe edits with exit code 2)")
	awarenessHookCmd.Flags().StringVar(&enforceCfg.watchlist, "watchlist", "", "Path to high-risk watchlist YAML")

	awarenessCmd.AddCommand(awarenessAuditCmd)
	awarenessCmd.AddCommand(awarenessValidateAnnotationsCmd)
	awarenessCmd.AddCommand(awarenessValidateRequiredTestsCmd)
	awarenessCmd.AddCommand(awarenessRequiredTestsBacklogCmd)
	awarenessRequiredTestsBacklogCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessRequiredTestsBacklogCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessCmd.AddCommand(awarenessRequiredTestsScaffoldCmd)
	awarenessRequiredTestsScaffoldCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessRequiredTestsScaffoldCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessRequiredTestsScaffoldCmd.Flags().IntVar(&enforceCfg.scaffoldLimit, "limit", 20, "Max number of test stubs to generate (0 = all)")
	awarenessCmd.AddCommand(awarenessValidateContractsCmd)
	awarenessCmd.AddCommand(awarenessGraphDriftCmd)
	awarenessCmd.AddCommand(awarenessAnnotationCoverageCmd)
	awarenessCmd.AddCommand(awarenessPRReportCmd)
	awarenessCmd.AddCommand(awarenessHookCmd)
	awarenessCmd.AddCommand(awarenessAuditTrendCmd)
	awarenessAuditTrendCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessAuditTrendCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessAuditTrendCmd.Flags().StringVar(&enforceCfg.trendFile, "trend-file", "", "Path to audit trend jsonl file")
	awarenessAuditTrendCmd.Flags().BoolVar(&enforceCfg.trendRecord, "record", false, "Append current warning-code snapshot to trend file")
	awarenessAuditTrendCmd.Flags().IntVar(&enforceCfg.trendLast, "last", 20, "Show last N trend snapshots")

	// Parse helper for max-required-test-no-path from env in CI wrappers if needed.
	if v := strings.TrimSpace(os.Getenv("AWARENESS_MAX_REQUIRED_TEST_NO_PATH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			enforceCfg.maxRequiredTestNoPath = n
		}
	}
}
