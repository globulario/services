package main

// awareness meta-check: meta-awareness summary that combines failure_mode
// coverage with graph + bundle staleness. The point is to keep the awareness
// system itself visible — adding subsystems without measuring coverage risks
// turning the system into a rubber stamp where NO_MATCH silently means
// "we did not check the right thing."
//
// (Distinct from `awareness self-check`, which exercises the broader audit /
// MCP-safety contract and lives in awareness_self_check_cmd.go.)
//
// Usage:
//
//	globular awareness meta-check [--json] [--orphans-fail]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/assurance"
	"github.com/globulario/services/golang/awareness/bundlesync"
)

var awarenessMetaCheckCfg = struct {
	dbPath              string
	repoPath            string
	bundleRoot          string
	manifestPath        string
	docsDir             string
	jsonOutput          bool
	orphansFail         bool
	maxOrphans          int
	orphanAudit         bool
	baselineOrphans     int
	criticalOrphansFail bool
	minWellCovered      int // ratchet: fail if well_covered_count drops below this
	minDetected         int // ratchet: fail if DETECTED+ENFORCED count drops below this
}{}

type orphanAuditRow struct {
	FailureMode       string `json:"failure_mode"`
	Severity          string `json:"severity"`
	HasYAMLRefs       bool   `json:"has_yaml_refs"`
	HasTestRefs       bool   `json:"has_test_refs"`
	HasDetectorRefs   bool   `json:"has_detector_refs"`
	GraphEdgesMissing string `json:"graph_edges_missing"`
	LikelyCause       string `json:"likely_cause"`
	RecommendedFix    string `json:"recommended_fix"`
}

var awarenessMetaCheckCmd = &cobra.Command{
	Use:   "meta-check",
	Short: "Meta-awareness: per-failure_mode coverage + graph/bundle staleness",
	Long: `Reports the awareness system's coverage of itself.

Computes per-failure_mode coverage (mitigations + tests + detectors), then
inspects graph build age, bundle manifest age, and YAML inventory drift.

Exits non-zero if --orphans-fail is set and any failure_mode has no
mitigation, no test, and no detector — i.e. the YAML names a problem the
awareness graph cannot enforce.

The bundle manifest is auto-discovered from <bundle-root>/current/manifest.json.
If no bundle is installed, the staleness check covers the graph only.`,
	RunE: runAwarenessMetaCheck,
}

func runAwarenessMetaCheck(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	g, err := openAwarenessGraph(awarenessMetaCheckCfg.dbPath, awarenessMetaCheckCfg.repoPath)
	if err != nil {
		return err
	}
	defer g.Close()

	repoRoot, _ := resolveRepoRoot(awarenessMetaCheckCfg.repoPath)
	docsDir := awarenessMetaCheckCfg.docsDir
	if docsDir == "" {
		docsDir = filepath.Join(repoRoot, "docs", "awareness")
	}

	manifest := loadBundleManifestForMetaCheck(
		awarenessMetaCheckCfg.manifestPath,
		awarenessMetaCheckCfg.bundleRoot,
	)

	coverage, err := assurance.ComputeCoverage(ctx, g)
	if err != nil {
		return fmt.Errorf("compute coverage: %w", err)
	}
	staleness, err := assurance.CheckStaleness(ctx, g, assurance.Options{
		DocsDir:  docsDir,
		Manifest: manifest,
	})
	if err != nil {
		return fmt.Errorf("check staleness: %w", err)
	}

	if awarenessMetaCheckCfg.jsonOutput {
		audit := buildOrphanAudit(coverage, docsDir)
		out := struct {
			Coverage    *assurance.CoverageReport `json:"coverage"`
			Staleness   *assurance.Staleness      `json:"staleness"`
			Trust       assurance.TrustEnvelope   `json:"trust"`
			Lifecycle   map[string]int            `json:"coverage_lifecycle"`
			OrphanAudit []orphanAuditRow          `json:"orphan_audit,omitempty"`
		}{coverage, staleness, composeMetaCheckTrust(coverage, staleness), lifecycleCounts(coverage), audit}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}
	} else {
		printMetaCheckHuman(coverage, staleness)
		if awarenessMetaCheckCfg.orphanAudit {
			printOrphanAuditTable(buildOrphanAudit(coverage, docsDir))
		}
		printTrustEnvelope(composeMetaCheckTrust(coverage, staleness))
	}

	if exitOnMetaCheckGate(coverage, staleness) {
		os.Exit(1)
	}
	return nil
}

func composeMetaCheckTrust(c *assurance.CoverageReport, s *assurance.Staleness) assurance.TrustEnvelope {
	return assurance.Compose(assurance.ComposeInputs{
		MatchFound:     c != nil && c.FailureModesTotal > 0,
		PerFailureMode: representativeFailureMode(c),
		Staleness:      s,
	})
}

func representativeFailureMode(c *assurance.CoverageReport) *assurance.FailureModeCoverage {
	if c == nil || len(c.PerFailureMode) == 0 {
		return nil
	}
	priority := map[string]int{"ORPHAN": 0, "PARTIAL": 1, "DETECTED": 2, "TESTED": 3, "ENFORCED": 4, "DEPRECATED": 5, "INTENTIONAL_GAP": 6}
	best := c.PerFailureMode[0]
	bestRank := 99
	if r, ok := priority[best.State]; ok {
		bestRank = r
	}
	for i := 1; i < len(c.PerFailureMode); i++ {
		fm := c.PerFailureMode[i]
		rank := 99
		if r, ok := priority[fm.State]; ok {
			rank = r
		}
		if rank < bestRank {
			best = fm
			bestRank = rank
		}
	}
	return &best
}

func printTrustEnvelope(env assurance.TrustEnvelope) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Trust:")
	fmt.Fprintf(os.Stdout, "  verdict: %s\n", env.Verdict)
	fmt.Fprintf(os.Stdout, "  confidence: %s\n", env.Confidence)
	fmt.Fprintf(os.Stdout, "  freshness: %s\n", env.Freshness)
	fmt.Fprintf(os.Stdout, "  coverage: %s\n", env.Coverage)
	if env.Reason != "" {
		fmt.Fprintf(os.Stdout, "  reason: %s\n", env.Reason)
	}
	if len(env.Limitations) > 0 {
		fmt.Fprintf(os.Stdout, "  limitations: %v\n", env.Limitations)
	}
	if len(env.RequiredActions) > 0 {
		fmt.Fprintf(os.Stdout, "  required_action: %v\n", env.RequiredActions)
	}
}

// loadBundleManifestForMetaCheck resolves the bundle manifest from the most
// likely on-disk location. Returns nil when nothing is found — staleness still
// reports a warn-level "bundle missing" alarm.
func loadBundleManifestForMetaCheck(manifestPath, bundleRoot string) *bundlesync.Manifest {
	candidates := make([]string, 0, 3)
	if manifestPath != "" {
		candidates = append(candidates, manifestPath)
	}
	if bundleRoot != "" {
		candidates = append(candidates, filepath.Join(bundleRoot, "current", "manifest.json"))
	}
	candidates = append(candidates, "/var/lib/globular/awareness/current/manifest.json")

	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		m, err := bundlesync.LoadManifest(p)
		if err == nil {
			return m
		}
	}
	return nil
}

func printMetaCheckHuman(c *assurance.CoverageReport, s *assurance.Staleness) {
	fmt.Fprintf(os.Stdout, "Awareness meta-check\n")
	fmt.Fprintf(os.Stdout, "====================\n\n")

	fmt.Fprintf(os.Stdout, "Coverage (per failure_mode):\n")
	fmt.Fprintf(os.Stdout, "  total          : %d\n", c.FailureModesTotal)
	fmt.Fprintf(os.Stdout, "  well_covered   : %d (%.1f%%)\n", c.WellCoveredCount, c.WellCoveredPercent)
	fmt.Fprintf(os.Stdout, "  partial        : %d\n", c.PartialCount)
	fmt.Fprintf(os.Stdout, "  theoretical    : %d\n", c.TheoreticalCount)
	fmt.Fprintf(os.Stdout, "  orphan         : %d\n", c.OrphanCount)
	fmt.Fprintf(os.Stdout, "  any-coverage%%  : %.1f%%\n\n", c.CoveragePercent)
	lc := lifecycleCounts(c)
	if len(lc) > 0 {
		fmt.Fprintf(os.Stdout, "Coverage lifecycle:\n")
		for _, k := range []string{"ORPHAN", "PARTIAL", "TESTED", "DETECTED", "ENFORCED", "DEPRECATED", "INTENTIONAL_GAP"} {
			if v, ok := lc[k]; ok {
				fmt.Fprintf(os.Stdout, "  %-15s: %d\n", strings.ToLower(k), v)
			}
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(c.OrphanIDs) > 0 {
		fmt.Fprintf(os.Stdout, "Orphan failure modes (no mitigation, no test, no detector):\n")
		for _, id := range c.OrphanIDs {
			fmt.Fprintf(os.Stdout, "  - %s\n", id)
		}
		fmt.Fprintln(os.Stdout)
	}
	if len(c.TheoreticalIDs) > 0 {
		fmt.Fprintf(os.Stdout, "Theoretical failure modes (decision/learning only, no enforcement):\n")
		for _, id := range c.TheoreticalIDs {
			fmt.Fprintf(os.Stdout, "  - %s\n", id)
		}
		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintf(os.Stdout, "Staleness:\n")
	if s.GraphBuiltAtUnix > 0 {
		fmt.Fprintf(os.Stdout, "  graph_age      : %.1f hours\n", s.GraphAgeSeconds/3600.0)
	}
	fmt.Fprintf(os.Stdout, "  graph_stale    : %v\n", s.GraphStale)
	if s.BundlePresent {
		fmt.Fprintf(os.Stdout, "  bundle_age     : %.1f hours (v%s build_id=%s)\n",
			s.BundleAgeSeconds/3600.0, s.BundleVersion, s.BundleBuildID)
		fmt.Fprintf(os.Stdout, "  bundle<graph?  : %v\n", s.BundleOlderThanGraph)
	} else {
		fmt.Fprintf(os.Stdout, "  bundle         : (not installed)\n")
	}
	fmt.Fprintf(os.Stdout, "  untracked_yamls: %d\n", s.UntrackedYAMLCount)
	if len(s.NewerThanGraph) > 0 {
		fmt.Fprintf(os.Stdout, "  yaml_newer_than_graph: %d files\n", len(s.NewerThanGraph))
	}
	fmt.Fprintln(os.Stdout)

	if len(s.Alarms) > 0 {
		// Sort by severity (critical first) so operators see the worst first.
		alarms := append([]assurance.Alarm(nil), s.Alarms...)
		sevOrder := map[assurance.AlarmSeverity]int{
			assurance.AlarmCritical: 0,
			assurance.AlarmWarn:     1,
			assurance.AlarmInfo:     2,
		}
		sort.SliceStable(alarms, func(i, j int) bool {
			return sevOrder[alarms[i].Severity] < sevOrder[alarms[j].Severity]
		})
		fmt.Fprintf(os.Stdout, "Alarms (%d):\n", len(alarms))
		for _, a := range alarms {
			fmt.Fprintf(os.Stdout, "  [%s] %s — %s\n", a.Severity, a.ID, a.Message)
		}
	} else {
		fmt.Fprintf(os.Stdout, "No alarms.\n")
	}
}

func printOrphanAuditTable(rows []orphanAuditRow) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Orphan audit")
	fmt.Fprintln(os.Stdout, "===========")
	fmt.Fprintln(os.Stdout, "failure_mode | severity | has_yaml_refs | has_test_refs | has_detector_refs | graph_edges_missing | likely_cause | recommended_fix")
	for _, row := range rows {
		fmt.Fprintf(os.Stdout, "%s | %s | %t | %t | %t | %s | %s | %s\n",
			row.FailureMode, row.Severity, row.HasYAMLRefs, row.HasTestRefs, row.HasDetectorRefs, row.GraphEdgesMissing, row.LikelyCause, row.RecommendedFix)
	}
}

func buildOrphanAudit(c *assurance.CoverageReport, docsDir string) []orphanAuditRow {
	if c == nil {
		return nil
	}
	yamlRefByID := loadFailureModeYAMLRefs(filepath.Join(docsDir, "failure_modes.yaml"))
	titleCount := map[string]int{}
	for _, fm := range c.PerFailureMode {
		if fm.State != "ORPHAN" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(fm.Title))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(fm.ID))
		}
		titleCount[key]++
	}
	var rows []orphanAuditRow
	for _, fm := range c.PerFailureMode {
		if fm.State != "ORPHAN" {
			continue
		}
		rows = append(rows, classifyOrphanRow(fm, titleCount, yamlRefByID[fm.ID]))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].FailureMode < rows[j].FailureMode })
	return rows
}

func classifyOrphanRow(fm assurance.FailureModeCoverage, titleCount map[string]int, sourceHasRefs bool) orphanAuditRow {
	hasYAMLRefs := (fm.DecisionPaths+fm.LearningEntries) > 0 || sourceHasRefs
	hasTestRefs := fm.Tests > 0
	hasDetectorRefs := fm.Detectors > 0
	missing := []string{}
	if fm.Mitigations == 0 {
		missing = append(missing, "mitigates")
	}
	if fm.Tests == 0 {
		missing = append(missing, "tests")
	}
	if fm.Detectors == 0 {
		missing = append(missing, "detectors")
	}
	likelyCause := "conceptual_orphan"
	recommendedFix := "add mitigates/tests/detectors links in failure_modes.yaml or contributing docs"
	if hasYAMLRefs {
		likelyCause = "extraction_orphan"
		recommendedFix = "verify extractor wiring for yaml-provided links and add regression tests"
	}
	if fm.State == "DEPRECATED" {
		likelyCause = "deprecated"
		recommendedFix = "mark as DEPRECATED and remove from active orphan queue"
	}
	if fm.State == "INTENTIONAL_GAP" {
		likelyCause = "intentional_gap"
		recommendedFix = "document waiver rationale and review cadence"
	}
	key := strings.ToLower(strings.TrimSpace(fm.Title))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(fm.ID))
	}
	if titleCount[key] > 1 {
		likelyCause = "possible_duplicate"
		recommendedFix = "merge duplicate failure modes or add alias/supersede relation"
	}
	sev := fm.Severity
	if sev == "" {
		sev = "unknown"
	}
	return orphanAuditRow{
		FailureMode:       fm.ID,
		Severity:          sev,
		HasYAMLRefs:       hasYAMLRefs,
		HasTestRefs:       hasTestRefs,
		HasDetectorRefs:   hasDetectorRefs,
		GraphEdgesMissing: strings.Join(missing, ","),
		LikelyCause:       likelyCause,
		RecommendedFix:    recommendedFix,
	}
}

type failureModesFile struct {
	FailureModes []failureModeYAML `yaml:"failure_modes"`
}

type failureModeYAML struct {
	ID                string   `yaml:"id"`
	ForbiddenFixes    []string `yaml:"forbidden_fixes"`
	RelatedInvariants []string `yaml:"related_invariants"`
	RelatedServices   []string `yaml:"related_services"`
	RequiredTests     []string `yaml:"required_tests"`
	Mitigates         []string `yaml:"mitigates"`
	Detectors         []string `yaml:"detectors"`
	RelatedIncidents  []string `yaml:"related_incidents"`
}

func loadFailureModeYAMLRefs(path string) map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var f failureModesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return out
	}
	for _, fm := range f.FailureModes {
		hasRefs := len(fm.ForbiddenFixes) > 0 ||
			len(fm.RelatedInvariants) > 0 ||
			len(fm.RelatedServices) > 0 ||
			len(fm.RequiredTests) > 0 ||
			len(fm.Mitigates) > 0 ||
			len(fm.Detectors) > 0 ||
			len(fm.RelatedIncidents) > 0
		out[fm.ID] = hasRefs
	}
	return out
}

// exitOnMetaCheckGate returns true if the run should exit non-zero. Critical
// alarms always gate. Orphans only gate when --orphans-fail is set; the
// default is "report, don't block" so this command can run in CI without
// breaking on YAML that legitimately has no enforcement yet.
func exitOnMetaCheckGate(c *assurance.CoverageReport, s *assurance.Staleness) bool {
	if len(s.CriticalAlarms()) > 0 {
		return true
	}
	if awarenessMetaCheckCfg.baselineOrphans >= 0 && c.OrphanCount > awarenessMetaCheckCfg.baselineOrphans {
		return true
	}
	if awarenessMetaCheckCfg.orphansFail && c.OrphanCount > awarenessMetaCheckCfg.maxOrphans {
		return true
	}
	if awarenessMetaCheckCfg.criticalOrphansFail && criticalOrphanCount(c) > 0 {
		return true
	}
	if awarenessMetaCheckCfg.minWellCovered > 0 && c.WellCoveredCount < awarenessMetaCheckCfg.minWellCovered {
		return true
	}
	if awarenessMetaCheckCfg.minDetected > 0 && detectedCount(c) < awarenessMetaCheckCfg.minDetected {
		return true
	}
	return false
}

// detectedCount returns the number of failure_modes whose lifecycle state is
// DETECTED or ENFORCED — i.e. has at least one runtime detector wired in. This
// is the ratchet axis that matters AFTER the orphan crisis is resolved: it
// answers "are we losing observable coverage?" rather than the now-trivial
// "are we still wiring edges?".
func detectedCount(c *assurance.CoverageReport) int {
	if c == nil {
		return 0
	}
	n := 0
	for _, fm := range c.PerFailureMode {
		switch strings.TrimSpace(fm.State) {
		case "DETECTED", "ENFORCED":
			n++
		}
	}
	return n
}

func criticalOrphanCount(c *assurance.CoverageReport) int {
	n := 0
	for _, fm := range c.PerFailureMode {
		if fm.State != "ORPHAN" {
			continue
		}
		if strings.EqualFold(fm.Severity, "critical") {
			n++
		}
	}
	return n
}

func lifecycleCounts(c *assurance.CoverageReport) map[string]int {
	out := map[string]int{}
	if c == nil {
		return out
	}
	for _, fm := range c.PerFailureMode {
		s := strings.TrimSpace(fm.State)
		if s == "" {
			continue
		}
		out[s]++
	}
	return out
}

func init() {
	awarenessMetaCheckCmd.Flags().StringVar(&awarenessMetaCheckCfg.dbPath, "db", "", "Path to graph.db")
	awarenessMetaCheckCmd.Flags().StringVar(&awarenessMetaCheckCfg.repoPath, "repo", "", "Repo root")
	awarenessMetaCheckCmd.Flags().StringVar(&awarenessMetaCheckCfg.bundleRoot, "bundle-root", "/var/lib/globular/awareness",
		"Bundle root containing current/manifest.json")
	awarenessMetaCheckCmd.Flags().StringVar(&awarenessMetaCheckCfg.manifestPath, "manifest", "",
		"Explicit path to manifest.json (overrides --bundle-root)")
	awarenessMetaCheckCmd.Flags().StringVar(&awarenessMetaCheckCfg.docsDir, "docs", "",
		"Path to docs/awareness (default: <repo>/docs/awareness)")
	awarenessMetaCheckCmd.Flags().BoolVar(&awarenessMetaCheckCfg.jsonOutput, "json", false, "Emit JSON instead of human-readable text")
	awarenessMetaCheckCmd.Flags().BoolVar(&awarenessMetaCheckCfg.orphansFail, "orphans-fail", false,
		"Exit non-zero if any failure_mode is orphan (above --max-orphans)")
	awarenessMetaCheckCmd.Flags().IntVar(&awarenessMetaCheckCfg.maxOrphans, "max-orphans", 0,
		"Allowed orphan count when --orphans-fail is set")
	awarenessMetaCheckCmd.Flags().BoolVar(&awarenessMetaCheckCfg.orphanAudit, "orphan-audit", false,
		"Print orphan audit table with likely cause and fix guidance")
	awarenessMetaCheckCmd.Flags().IntVar(&awarenessMetaCheckCfg.baselineOrphans, "baseline-orphans", -1,
		"CI ratchet baseline; fail if current orphan count exceeds this value")
	awarenessMetaCheckCmd.Flags().BoolVar(&awarenessMetaCheckCfg.criticalOrphansFail, "critical-orphans-fail", false,
		"Fail when any severity=critical failure_mode remains ORPHAN")
	awarenessMetaCheckCmd.Flags().IntVar(&awarenessMetaCheckCfg.minWellCovered, "min-well-covered", 0,
		"CI ratchet (positive direction): fail if well_covered_count is below this floor. Set to current count to prevent regression.")
	awarenessMetaCheckCmd.Flags().IntVar(&awarenessMetaCheckCfg.minDetected, "min-detected", 0,
		"CI ratchet: fail if count of DETECTED+ENFORCED failure_modes is below this floor. Tracks runtime-observable coverage.")

	awarenessCmd.AddCommand(awarenessMetaCheckCmd)
}
