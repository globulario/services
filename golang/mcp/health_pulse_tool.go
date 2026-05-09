package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/awareness/extractors/manual"
)

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

type healthPulseAlert struct {
	Severity          string `json:"severity"` // warning | critical
	ID                string `json:"id"`
	Message           string `json:"message"`
	RecommendedAction string `json:"recommended_action"`
}

type healthPulseCoverageSection struct {
	ComponentsTotal               int    `json:"components_total"`
	ComponentsWithoutFailureModes int    `json:"components_without_failure_modes"`
	UnverifiedImplementedGaps     int    `json:"unverified_implemented_gaps"`
	Status                        string `json:"status"` // ok | warn | critical
}

type healthPulseRuntimeSection struct {
	RuntimeAwarenessStatus string `json:"runtime_awareness_status"` // live | partial | noop | misconfigured
	ConfiguredSources      int    `json:"configured_sources"`
	TotalSources           int    `json:"total_sources"`
	Status                 string `json:"status"` // ok | warn | critical
}

type healthPulseQueueSection struct {
	PendingProposals int    `json:"pending_proposals"`
	StaleProposals   int    `json:"stale_proposals"`
	QueueStatus      string `json:"queue_status"` // healthy | stale | needs_review | blocked
	Status           string `json:"status"`       // ok | warn | critical
}

type healthPulseGraphSection struct {
	Available            bool    `json:"available"`
	Stale                bool    `json:"stale"`
	StaleReason          string  `json:"stale_reason,omitempty"`
	AgeHours             float64 `json:"age_hours,omitempty"`
	RebuildRecommended   bool    `json:"rebuild_recommended"`
	LastBuildDurationMs  int64   `json:"last_build_duration_ms,omitempty"`
	Status               string  `json:"status"` // ok | warn | critical | no_graph
}

type healthPulseSelfReviewSection struct {
	TotalImplemented int    `json:"total_implemented"`
	TestsFound       int    `json:"tests_found"`
	TestsNotFound    int    `json:"tests_not_found"`
	InvalidMetadata  int    `json:"invalid_metadata"`
	Status           string `json:"status"` // ok | warn | critical
}

type healthPulseUnindexedEntry struct {
	Path   string `json:"path"`
	TopKey string `json:"top_key"`
}

type healthPulseUnindexedSection struct {
	Count          int                         `json:"count"`
	UnindexedFiles []healthPulseUnindexedEntry `json:"unindexed_files,omitempty"`
	Status         string                      `json:"status"` // ok | warn
}

type healthPulseAgentUsageSection struct {
	WindowDays           int     `json:"window_days"`
	SessionsTotal        int     `json:"sessions_total"`
	PreflightCalls       int     `json:"preflight_calls"`
	PreflightSkipRatePct float64 `json:"preflight_skip_rate_pct"`
	Status               string  `json:"status"` // ok | warning | no_data
	RecommendedAction    string  `json:"recommended_action,omitempty"`
}

type healthPulseSections struct {
	Coverage         healthPulseCoverageSection   `json:"coverage"`
	RuntimeSources   healthPulseRuntimeSection    `json:"runtime_sources"`
	ProposalQueue    healthPulseQueueSection      `json:"proposal_queue"`
	GraphFreshness   healthPulseGraphSection      `json:"graph_freshness"`
	SelfReview       healthPulseSelfReviewSection `json:"self_review_verification"`
	UnindexedYAML    healthPulseUnindexedSection  `json:"unindexed_yaml"`
	AgentUsage       healthPulseAgentUsageSection `json:"agent_usage"`
}

type healthPulseResult struct {
	Status    string              `json:"status"` // healthy | warning | critical
	CheckedAt string              `json:"checked_at"`
	Sections  healthPulseSections `json:"sections"`
	Alerts    []healthPulseAlert  `json:"alerts"`
	ExitCode  int                 `json:"exit_code"` // 0=healthy 1=warning 2=critical 3=check_failed
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerHealthPulseTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.health_pulse",
		Description: "Aggregate awareness self-health check. Runs coverage_report, runtime_activation_check, proposal_queue_health, graph_freshness, and self_review verification in one call. Returns a machine-readable report with severity-tagged alerts and an exit code (0=healthy, 1=warning, 2=critical, 3=check_failed). Designed to be scheduled by cron, systemd timer, or CI pipeline.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"stale_proposal_hours": {
					Type:        "number",
					Description: "Hours after which a proposal is considered stale. Default: 24.",
				},
				"include_graph_check": {
					Type:        "boolean",
					Description: "If true, check graph freshness. Default: true.",
					Default:     true,
				},
			},
			Required: []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		docsDir := st.docsDir
		repoRoot := st.repoRoot

		staleHours := 24.0
		if v, ok := args["stale_proposal_hours"].(float64); ok && v > 0 {
			staleHours = v
		}
		includeGraph := true
		if v, ok := args["include_graph_check"].(bool); ok {
			includeGraph = v
		}

		var alerts []healthPulseAlert
		checkedAt := time.Now().UTC().Format(time.RFC3339)

		coverageSection, coverageAlerts := buildCoverageSection(docsDir, repoRoot, staleHours)
		alerts = append(alerts, coverageAlerts...)

		runtimeSection, runtimeAlerts := buildRuntimeSection(repoRoot)
		alerts = append(alerts, runtimeAlerts...)

		queueSection, queueAlerts := buildQueueSection(docsDir, staleHours)
		alerts = append(alerts, queueAlerts...)

		graphSection, graphAlerts := buildGraphSection(ctx, st, docsDir, includeGraph)
		alerts = append(alerts, graphAlerts...)

		srSection, srAlerts := buildSelfReviewSection(docsDir, repoRoot)
		alerts = append(alerts, srAlerts...)

		unindexedSection, unindexedAlerts := buildUnindexedYAMLSection(docsDir)
		alerts = append(alerts, unindexedAlerts...)

		agentUsageSection, agentUsageAlerts := buildAgentUsageSection(ctx, st)
		alerts = append(alerts, agentUsageAlerts...)

		overallStatus, exitCode := computePulseStatus(
			coverageSection.Status,
			runtimeSection.Status,
			queueSection.Status,
			graphSection.Status,
			srSection.Status,
			unindexedSection.Status,
		)

		return &healthPulseResult{
			Status:    overallStatus,
			CheckedAt: checkedAt,
			Sections: healthPulseSections{
				Coverage:       coverageSection,
				RuntimeSources: runtimeSection,
				ProposalQueue:  queueSection,
				GraphFreshness: graphSection,
				SelfReview:     srSection,
				UnindexedYAML:  unindexedSection,
				AgentUsage:     agentUsageSection,
			},
			Alerts:   alerts,
			ExitCode: exitCode,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Section builders
// ---------------------------------------------------------------------------

func buildCoverageSection(docsDir, repoRoot string, staleHours float64) (healthPulseCoverageSection, []healthPulseAlert) {
	var alerts []healthPulseAlert
	if docsDir == "" {
		return healthPulseCoverageSection{Status: "warn"}, []healthPulseAlert{{
			Severity:          "warning",
			ID:                "coverage.docs_dir_missing",
			Message:           "docs dir not configured — coverage check skipped",
			RecommendedAction: "Set DocsDir in MCP config",
		}}
	}

	fmByComp := loadFailureModesByComponent(docsDir)
	compSet := buildComponentSet(nil)
	for c := range fmByComp {
		compSet[c] = true
	}

	withoutFMs := 0
	for comp := range compSet {
		if len(fmByComp[comp]) == 0 {
			withoutFMs++
		}
	}

	_, _, unverified := loadGapCoverageByComponent(docsDir, repoRoot)

	status := "ok"
	if withoutFMs > 0 {
		status = "warn"
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "coverage.missing_failure_modes",
			Message:           fmt.Sprintf("%d component(s) have no documented failure modes", withoutFMs),
			RecommendedAction: "Run awareness.coverage_report for details, then use learn_from_fix after next incident.",
		})
	}
	if unverified > 0 {
		if status != "warn" {
			status = "warn"
		}
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "coverage.unverified_gaps",
			Message:           fmt.Sprintf("%d implemented gap(s) have unverified test evidence", unverified),
			RecommendedAction: "Run awareness.self_review to identify gaps with tests_not_found or invalid_metadata.",
		})
	}

	return healthPulseCoverageSection{
		ComponentsTotal:               len(compSet),
		ComponentsWithoutFailureModes: withoutFMs,
		UnverifiedImplementedGaps:     unverified,
		Status:                        status,
	}, alerts
}

// buildRuntimeSection reports runtime bridge status by loading the real runtime
// sources config from repoRoot/.awareness/runtime_sources.yaml. Uses the same
// evaluateRuntimeActivation logic as the runtime_activation_check tool so the
// two never diverge.
func buildRuntimeSection(repoRoot string) (healthPulseRuntimeSection, []healthPulseAlert) {
	cfg := loadRuntimeSourcesConfig(repoRoot)
	result := evaluateRuntimeActivation(cfg, false, false)

	configured := 0
	total := 0
	for _, src := range result.Sources {
		total++
		if src.Configured {
			configured++
		}
	}

	runtimeStatus := result.RuntimeAwarenessStatus
	sectionStatus := "ok"
	var alerts []healthPulseAlert
	switch runtimeStatus {
	case "noop":
		sectionStatus = "warn"
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "runtime.noop",
			Message:           "Runtime awareness is noop — no cluster addresses configured",
			RecommendedAction: "Run awareness.runtime_config_bootstrap to generate a config, or awareness.runtime_activation_check for details.",
		})
	case "misconfigured":
		sectionStatus = "warn"
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "runtime.misconfigured",
			Message:           "Runtime awareness is misconfigured — addresses set but TLS credentials missing",
			RecommendedAction: "Run awareness.runtime_activation_check to see which credentials are missing.",
		})
	case "partial":
		sectionStatus = "warn"
		missing := 0
		for _, src := range result.Sources {
			if !src.Configured && src.Transport != "etcd_resolved" {
				missing++
			}
		}
		msg := fmt.Sprintf("Runtime awareness partial — %d source(s) missing static address and not etcd-resolvable", missing)
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "runtime.partial",
			Message:           msg,
			RecommendedAction: "Run awareness.runtime_activation_check for missing source details.",
		})
	}

	return healthPulseRuntimeSection{
		RuntimeAwarenessStatus: runtimeStatus,
		ConfiguredSources:      configured,
		TotalSources:           total,
		Status:                 sectionStatus,
	}, alerts
}

func buildQueueSection(docsDir string, staleHours float64) (healthPulseQueueSection, []healthPulseAlert) {
	var alerts []healthPulseAlert

	pending, _ := loadPendingProposalCount(docsDir)
	stale := countStaleProposals(docsDir, staleHours)

	counts := proposalCounts{Stale: stale}
	qStatus := computeQueueStatus(counts, stale, 0)

	sectionStatus := "ok"
	if stale > 0 {
		sectionStatus = "warn"
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "proposal_queue.stale",
			Message:           fmt.Sprintf("%d proposal(s) older than %.0fh SLA", stale, staleHours),
			RecommendedAction: "Run awareness.proposal_queue_health for details, then awareness.proposal_review_plan to prioritize.",
		})
	}
	if qStatus == "needs_review" {
		sectionStatus = "warn"
	}

	return healthPulseQueueSection{
		PendingProposals: pending,
		StaleProposals:   stale,
		QueueStatus:      qStatus,
		Status:           sectionStatus,
	}, alerts
}

func buildGraphSection(ctx context.Context, st *awarenessState, docsDir string, includeGraph bool) (healthPulseGraphSection, []healthPulseAlert) {
	if !includeGraph {
		return healthPulseGraphSection{Status: "ok", Available: false}, nil
	}
	if st.g == nil {
		return healthPulseGraphSection{
			Available:          false,
			Stale:              true,
			StaleReason:        "no graph.db found — run 'globular awareness build'",
			RebuildRecommended: true,
			Status:             "no_graph",
		}, []healthPulseAlert{{
			Severity:          "warning",
			ID:                "graph.no_db",
			Message:           "Awareness graph database not found",
			RecommendedAction: "Run 'globular awareness build' to initialize the graph.",
		}}
	}

	f := st.g.Freshness(ctx, docsDir)
	sectionStatus := "ok"
	var alerts []healthPulseAlert
	if f.Stale {
		sectionStatus = "critical"
		alerts = append(alerts, healthPulseAlert{
			Severity:          "critical",
			ID:                "graph.stale",
			Message:           f.StaleReason,
			RecommendedAction: "Run 'globular awareness build' to rebuild the graph.",
		})
	}

	sec := healthPulseGraphSection{
		Available:          true,
		Stale:              f.Stale,
		StaleReason:        f.StaleReason,
		AgeHours:           f.AgeSeconds / 3600,
		RebuildRecommended: f.RebuildRecommended,
		Status:             sectionStatus,
	}
	if rec, err := st.g.LatestBuildRecord(ctx); err == nil && rec != nil {
		sec.LastBuildDurationMs = rec.Stats.DurationMs
	}
	return sec, alerts
}

func buildSelfReviewSection(docsDir, repoRoot string) (healthPulseSelfReviewSection, []healthPulseAlert) {
	var alerts []healthPulseAlert

	if docsDir == "" {
		return healthPulseSelfReviewSection{Status: "warn"}, nil
	}

	pb, err := loadAgentPlaybooks(docsDir)
	if err != nil {
		return healthPulseSelfReviewSection{Status: "warn"}, []healthPulseAlert{{
			Severity:          "warning",
			ID:                "self_review.playbooks_unreadable",
			Message:           "Could not read agent_playbooks.yaml",
			RecommendedAction: "Check docs/awareness/knowledge/agent_playbooks.yaml for syntax errors.",
		}}
	}

	total, found, notFound, invalid := 0, 0, 0, 0
	for _, pat := range pb.CapabilityGapPatterns {
		if pat.Status != "implemented" {
			continue
		}
		total++
		status, _ := verifyGapTests(repoRoot, pat.TestsRequired)
		switch status {
		case "tests_found", "tests_partial":
			found++
		case "invalid_metadata":
			invalid++
		default:
			notFound++
		}
	}

	sectionStatus := "ok"
	if invalid > 0 {
		sectionStatus = "warn"
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "self_review.invalid_metadata",
			Message:           fmt.Sprintf("%d implemented gap(s) have description-style tests_required entries", invalid),
			RecommendedAction: "Update tests_required with exact Go test function names (TestXxx format).",
		})
	}
	if notFound > 0 {
		if sectionStatus != "warn" {
			sectionStatus = "warn"
		}
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "self_review.tests_not_found",
			Message:           fmt.Sprintf("%d implemented gap(s) have missing tests", notFound),
			RecommendedAction: "Write the missing tests or update tests_required to match actual test function names.",
		})
	}

	return healthPulseSelfReviewSection{
		TotalImplemented: total,
		TestsFound:       found,
		TestsNotFound:    notFound,
		InvalidMetadata:  invalid,
		Status:           sectionStatus,
	}, alerts
}

func buildUnindexedYAMLSection(docsDir string) (healthPulseUnindexedSection, []healthPulseAlert) {
	if docsDir == "" {
		return healthPulseUnindexedSection{Status: "ok"}, nil
	}

	files, err := manual.WalkUnindexed(docsDir)
	if err != nil {
		return healthPulseUnindexedSection{Status: "ok"}, nil
	}
	if len(files) == 0 {
		return healthPulseUnindexedSection{Status: "ok"}, nil
	}

	entries := make([]healthPulseUnindexedEntry, len(files))
	for i, f := range files {
		entries[i] = healthPulseUnindexedEntry{Path: f.Path, TopKey: f.TopKey}
	}

	// Build a compact summary for the alert message (first 5 files).
	summary := ""
	for i, f := range files {
		if i >= 5 {
			summary += fmt.Sprintf(" (+%d more)", len(files)-5)
			break
		}
		if i > 0 {
			summary += ", "
		}
		summary += filepath.Base(f.Path) + " (" + f.TopKey + ":)"
	}

	return healthPulseUnindexedSection{
			Count:          len(files),
			UnindexedFiles: entries,
			Status:         "warn",
		}, []healthPulseAlert{{
			Severity:          "warning",
			ID:                "knowledge.unindexed_yaml",
			Message:           fmt.Sprintf("%d YAML file(s) in docs/awareness have unrecognized top-level keys and are not loaded into the graph: %s", len(files), summary),
			RecommendedAction: "Review unindexed_yaml section. Add a loader for types that should be indexed; add a comment in the file for intentional config-only files.",
		}}
}

// ---------------------------------------------------------------------------
// Status aggregation
// ---------------------------------------------------------------------------

func computePulseStatus(statuses ...string) (string, int) {
	hasCritical := false
	hasWarn := false
	for _, s := range statuses {
		switch s {
		case "critical":
			hasCritical = true
		case "warn", "warning", "no_graph":
			hasWarn = true
		}
	}
	switch {
	case hasCritical:
		return "critical", 2
	case hasWarn:
		return "warning", 1
	default:
		return "healthy", 0
	}
}

func buildAgentUsageSection(ctx context.Context, st *awarenessState) (healthPulseAgentUsageSection, []healthPulseAlert) {
	if st.g == nil {
		return healthPulseAgentUsageSection{Status: "no_data"}, nil
	}
	summary, err := st.g.QueryAgentUsageSummary(ctx, 7)
	if err != nil {
		return healthPulseAgentUsageSection{Status: "no_data"}, nil
	}
	sec := healthPulseAgentUsageSection{
		WindowDays:           summary.WindowDays,
		SessionsTotal:        summary.SessionsTotal,
		PreflightCalls:       summary.PreflightCalls,
		PreflightSkipRatePct: summary.PreflightSkipRatePct,
		Status:               summary.Status,
		RecommendedAction:    summary.RecommendedAction,
	}
	var alerts []healthPulseAlert
	if summary.Status == "warning" {
		alerts = append(alerts, healthPulseAlert{
			Severity:          "warning",
			ID:                "agent_usage.high_skip_rate",
			Message:           fmt.Sprintf("preflight skip rate %.0f%% over last %d days — agents may be bypassing awareness", summary.PreflightSkipRatePct, summary.WindowDays),
			RecommendedAction: summary.RecommendedAction,
		})
	}
	return sec, alerts
}
