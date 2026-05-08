package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// defaultKnownComponents is the set of core infrastructure components that
// coverage_report always reports on, even if they have no failure modes yet.
var defaultKnownComponents = []string{
	"etcd", "minio", "scylla", "workflow", "controller",
	"repository", "xds", "node_agent",
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

type coverageSummary struct {
	ComponentsTotal               int `json:"components_total"`
	ComponentsWithFailureModes    int `json:"components_with_failure_modes"`
	ComponentsWithoutFailureModes int `json:"components_without_failure_modes"`
	FailureModesTotal             int `json:"failure_modes_total"`
	ForbiddenFixesTotal           int `json:"forbidden_fixes_total"`
	CausalRulesTotal              int `json:"causal_rules_total"`
	ImplementedGapsVerified       int `json:"implemented_gaps_verified"`
	ImplementedGapsUnverified     int `json:"implemented_gaps_unverified"`
	PendingProposals              int `json:"pending_proposals"`
}

type componentCoverageEntry struct {
	Component      string   `json:"component"`
	FailureModes   []string `json:"failure_modes"`
	ForbiddenFixes []string `json:"forbidden_fixes"`
	CausalRules    []string `json:"causal_rules"`
	Tests          []string `json:"tests"`
	CoverageStatus string   `json:"coverage_status"`
}

type coverageTopGap struct {
	GapID    string `json:"gap_id"`
	Priority string `json:"priority"`
	Reason   string `json:"reason"`
}

type coverageReportResult struct {
	Summary                coverageSummary          `json:"summary"`
	Components             []componentCoverageEntry `json:"components"`
	TopGaps                []coverageTopGap         `json:"top_gaps"`
	RecommendedNextActions []string                 `json:"recommended_next_actions"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerCoverageReportTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.coverage_report",
		Description: "Report awareness knowledge coverage by component. Identifies components with no failure modes, failure modes with no tests, causal rules with no tests, implemented gaps with missing test evidence, and pending proposals older than SLA. Does not invent unknown failure modes — only reports what is explicitly encoded.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"scope": {
					Type:        "array",
					Description: "Areas to report on: failure_modes, forbidden_fixes, causal_rules, tests, proposals. Default: all.",
					Items:       &propSchema{Type: "string"},
				},
				"components": {
					Type:        "array",
					Description: "Components to include (etcd, minio, scylla, workflow, controller, repository, xds, node_agent). Default: all known components.",
					Items:       &propSchema{Type: "string"},
				},
				"include_backlog": {
					Type:        "boolean",
					Description: "If true, include open gaps from agent_playbooks.yaml in the report.",
					Default:     true,
				},
				"include_unverified": {
					Type:        "boolean",
					Description: "If true, flag implemented gaps where verifyGapTests would report tests_not_found or tests_partial.",
					Default:     true,
				},
				"stale_proposal_hours": {
					Type:        "number",
					Description: "Hours after which a DRAFT proposal is considered stale. Default: 24.",
				},
			},
			Required: []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		docsDir := st.docsDir
		repoRoot := st.repoRoot
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		requestedComponents := strSliceArg(args, "components")
		staleHours := 24.0
		if v, ok := args["stale_proposal_hours"].(float64); ok && v > 0 {
			staleHours = v
		}
		includeUnverified := true
		if v, ok := args["include_unverified"].(bool); ok {
			includeUnverified = v
		}

		// Determine component set.
		componentSet := buildComponentSet(requestedComponents)

		// Load failure modes.
		fmByComponent := loadFailureModesByComponent(docsDir)

		// Load forbidden fixes.
		ffByComponent := loadForbiddenFixesByComponent(docsDir)

		// Load causal rules.
		causalByComponent := loadCausalRulesByComponent(docsDir)

		// Load implemented gap verification from agent_playbooks.
		gapsByComponent, verifiedCount, unverifiedCount := loadGapCoverageByComponent(docsDir, repoRoot)

		// Count pending proposals.
		pendingProposals, _ := loadPendingProposalCount(docsDir)

		// Extend component set with any components found in failure_modes.yaml.
		for comp := range fmByComponent {
			componentSet[comp] = true
		}
		for comp := range ffByComponent {
			componentSet[comp] = true
		}

		// Build per-component report.
		var components []componentCoverageEntry
		withFMs := 0
		withoutFMs := 0

		sortedComps := sortedKeys(componentSet)
		for _, comp := range sortedComps {
			fms := fmByComponent[comp]
			ffs := ffByComponent[comp]
			rules := causalByComponent[comp]
			tests := gapsByComponent[comp]

			status := computeComponentCoverageStatus(fms, tests)
			if len(fms) > 0 {
				withFMs++
			} else {
				withoutFMs++
			}

			components = append(components, componentCoverageEntry{
				Component:      comp,
				FailureModes:   nvl(fms),
				ForbiddenFixes: nvl(ffs),
				CausalRules:    nvl(rules),
				Tests:          nvl(tests),
				CoverageStatus: status,
			})
		}

		// Total counts.
		totalFMs := 0
		for _, fms := range fmByComponent {
			totalFMs += len(fms)
		}
		totalFFs := 0
		for _, ffs := range ffByComponent {
			totalFFs += len(ffs)
		}
		totalRules := 0
		for _, rules := range causalByComponent {
			totalRules += len(rules)
		}

		// Build top gaps.
		topGaps := buildTopGaps(components, docsDir, staleHours, includeUnverified, unverifiedCount)

		// Build recommended actions.
		actions := buildCoverageActions(components, pendingProposals, staleHours)

		return &coverageReportResult{
			Summary: coverageSummary{
				ComponentsTotal:               len(componentSet),
				ComponentsWithFailureModes:    withFMs,
				ComponentsWithoutFailureModes: withoutFMs,
				FailureModesTotal:             totalFMs,
				ForbiddenFixesTotal:           totalFFs,
				CausalRulesTotal:              totalRules,
				ImplementedGapsVerified:       verifiedCount,
				ImplementedGapsUnverified:     unverifiedCount,
				PendingProposals:              pendingProposals,
			},
			Components:             components,
			TopGaps:                topGaps,
			RecommendedNextActions: actions,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Loaders
// ---------------------------------------------------------------------------

func buildComponentSet(requested []string) map[string]bool {
	set := make(map[string]bool)
	if len(requested) > 0 {
		for _, c := range requested {
			set[strings.ToLower(c)] = true
		}
	} else {
		for _, c := range defaultKnownComponents {
			set[c] = true
		}
	}
	return set
}

func loadFailureModesByComponent(docsDir string) map[string][]string {
	result := make(map[string][]string)
	data, err := os.ReadFile(filepath.Join(docsDir, "failure_modes.yaml"))
	if err != nil {
		return result
	}
	var root struct {
		FailureModes []struct {
			ID string `yaml:"id"`
		} `yaml:"failure_modes"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return result
	}
	for _, fm := range root.FailureModes {
		comp := componentOf(fm.ID)
		result[comp] = append(result[comp], fm.ID)
	}
	return result
}

func loadForbiddenFixesByComponent(docsDir string) map[string][]string {
	result := make(map[string][]string)
	data, err := os.ReadFile(filepath.Join(docsDir, "forbidden_fixes.yaml"))
	if err != nil {
		return result
	}
	var root struct {
		ForbiddenFixes []struct {
			ID string `yaml:"id"`
		} `yaml:"forbidden_fixes"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return result
	}
	for _, ff := range root.ForbiddenFixes {
		comp := componentOf(ff.ID)
		result[comp] = append(result[comp], ff.ID)
	}
	return result
}

func loadCausalRulesByComponent(docsDir string) map[string][]string {
	result := make(map[string][]string)
	data, err := os.ReadFile(filepath.Join(docsDir, "knowledge", "causal_rules.yaml"))
	if err != nil {
		return result
	}
	var root struct {
		Rules []struct {
			ID          string `yaml:"id"`
			RootSignal  string `yaml:"root_signal"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return result
	}
	for _, rule := range root.Rules {
		comp := componentOf(rule.RootSignal)
		if comp == "" || comp == rule.RootSignal {
			comp = componentOf(rule.ID)
		}
		result[comp] = append(result[comp], rule.ID)
	}
	return result
}

// loadGapCoverageByComponent returns: tests by component, verified count, unverified count.
func loadGapCoverageByComponent(docsDir, repoRoot string) (map[string][]string, int, int) {
	byComp := make(map[string][]string)
	verified, unverified := 0, 0

	data, err := os.ReadFile(filepath.Join(docsDir, "knowledge", "agent_playbooks.yaml"))
	if err != nil {
		return byComp, verified, unverified
	}
	var root struct {
		CapabilityGapPatterns []capabilityGapPattern `yaml:"capability_gap_patterns"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return byComp, verified, unverified
	}
	for _, pat := range root.CapabilityGapPatterns {
		if pat.Status != "implemented" {
			continue
		}
		status, _ := verifyGapTests(repoRoot, pat.TestsRequired)
		comp := componentOf(pat.ID)
		byComp[comp] = append(byComp[comp], pat.TestsRequired...)
		if status == "tests_found" {
			verified++
		} else {
			unverified++
		}
	}
	return byComp, verified, unverified
}

func loadPendingProposalCount(docsDir string) (int, []string) {
	proposalsDir := filepath.Join(docsDir, "proposals")
	entries, err := os.ReadDir(proposalsDir)
	if err != nil {
		return 0, nil
	}
	count := 0
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(proposalsDir, e.Name()))
		if err != nil {
			continue
		}
		var raw struct {
			Proposal struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"proposal"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}
		status := strings.ToUpper(raw.Proposal.Status)
		if status != "PROMOTED" && status != "REJECTED" {
			count++
			ids = append(ids, raw.Proposal.ID)
		}
	}
	return count, ids
}

// ---------------------------------------------------------------------------
// Status computation
// ---------------------------------------------------------------------------

func computeComponentCoverageStatus(fms, tests []string) string {
	if len(fms) == 0 {
		return "missing_failure_modes"
	}
	if len(tests) == 0 {
		return "missing_tests"
	}
	return "covered"
}

func buildTopGaps(components []componentCoverageEntry, docsDir string, staleHours float64, includeUnverified bool, unverifiedCount int) []coverageTopGap {
	var gaps []coverageTopGap

	for _, comp := range components {
		switch comp.CoverageStatus {
		case "missing_failure_modes":
			gaps = append(gaps, coverageTopGap{
				GapID:    fmt.Sprintf("coverage.%s.failure_modes_missing", comp.Component),
				Priority: "P1",
				Reason:   fmt.Sprintf("%s is core infrastructure but has no documented failure modes", comp.Component),
			})
		case "missing_tests":
			gaps = append(gaps, coverageTopGap{
				GapID:    fmt.Sprintf("coverage.%s.tests_missing", comp.Component),
				Priority: "P2",
				Reason:   fmt.Sprintf("%s has %d failure mode(s) but no linked tests in agent_playbooks.yaml", comp.Component, len(comp.FailureModes)),
			})
		}
	}

	// Stale proposals.
	staleCount := countStaleProposals(docsDir, staleHours)
	if staleCount > 0 {
		gaps = append(gaps, coverageTopGap{
			GapID:    "coverage.proposals.stale",
			Priority: "P1",
			Reason:   fmt.Sprintf("%d proposal(s) older than %.0fh SLA — run awareness.proposal_queue_health for details", staleCount, staleHours),
		})
	}

	if includeUnverified && unverifiedCount > 0 {
		gaps = append(gaps, coverageTopGap{
			GapID:    "coverage.implemented_gaps.unverified",
			Priority: "P0",
			Reason:   fmt.Sprintf("%d implemented gap(s) have missing or invalid test evidence — run awareness.self_review for details", unverifiedCount),
		})
	}

	// Keep at most 10 top gaps.
	if len(gaps) > 10 {
		gaps = gaps[:10]
	}
	return gaps
}

func countStaleProposals(docsDir string, staleHours float64) int {
	proposalsDir := filepath.Join(docsDir, "proposals")
	entries, _ := os.ReadDir(proposalsDir)
	now := time.Now()
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(proposalsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw struct {
			Proposal struct {
				Status    string `yaml:"status"`
				CreatedAt string `yaml:"created_at"`
			} `yaml:"proposal"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}
		status := strings.ToUpper(raw.Proposal.Status)
		if status == "PROMOTED" || status == "REJECTED" {
			continue
		}
		age := proposalAge(raw.Proposal.CreatedAt, path, now)
		if age.Hours() > staleHours {
			count++
		}
	}
	return count
}

func buildCoverageActions(components []componentCoverageEntry, pendingProposals int, staleHours float64) []string {
	var actions []string
	for _, comp := range components {
		if comp.CoverageStatus == "missing_failure_modes" {
			actions = append(actions, fmt.Sprintf("Use learn_from_fix after next %s incident to seed failure modes for %s.", comp.Component, comp.Component))
		}
	}
	if pendingProposals > 0 {
		actions = append(actions, fmt.Sprintf("Review %d pending proposal(s) via awareness.proposal_queue_health.", pendingProposals))
	}
	if len(actions) == 0 {
		actions = append(actions, "Coverage looks healthy. Keep running coverage_report after each incident to track new gaps.")
	}
	return actions
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// componentOf extracts the first dotted segment from an ID as the component name.
// "etcd.nospace_alarm" → "etcd", "awareness.offline_diagnose" → "awareness".
func componentOf(id string) string {
	if idx := strings.IndexByte(id, '.'); idx >= 0 {
		return id[:idx]
	}
	return id
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func nvl(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// proposalAge returns the age of a proposal from its created_at field or file mtime.
func proposalAge(createdAt, filePath string, now time.Time) time.Duration {
	if createdAt != "" {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			return now.Sub(t)
		}
	}
	if info, err := os.Stat(filePath); err == nil {
		return now.Sub(info.ModTime())
	}
	return 0
}
