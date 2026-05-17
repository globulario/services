package main

// awareness_session_cmd.go: globular awareness session-start
//
// Reports graph freshness, runtime activation status, CI verification,
// proposal queue health, top guardrails, and recommended first action.
// Run this at the start of every development session before editing Globular code.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/awareness/preflight"
	"github.com/spf13/cobra"
)

var sessionStartCfg = struct {
	format string
}{}

var awarenessSessionStartCmd = &cobra.Command{
	Use:   "session-start",
	Short: "Run awareness handshake before code work — graph freshness, runtime, CI, proposals",
	Long: `Run at the beginning of every development session before editing Globular code.

Reports:
  - Graph freshness (stale → rebuild recommended)
  - Runtime activation status (noop → no live cluster evidence)
  - CI verification status (strict_verified available?)
  - Proposal queue health (stale DRAFTs?)
  - Top guardrails to keep in mind
  - Blind spots and recommended first action

The output is designed to fit in a Claude context window.

Example:

  globular awareness session-start --output json`,

	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = resolveAwarenessDBPath(repoRoot)
		}

		result := buildCLISessionStart(repoRoot, docsDir, dbPath)

		if sessionStartCfg.format == "json" {
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		}

		// Table output.
		printSessionStartTable(result)
		return nil
	},
}

// cliSessionResult is the JSON-serialisable session handshake output.
type cliSessionResult struct {
	Status               string                 `json:"status"` // ready | warning | critical
	CheckedAt            string                 `json:"checked_at"`
	Graph                cliSessionGraph        `json:"graph"`
	Runtime              cliSessionRuntime      `json:"runtime"`
	WorkflowHealth       cliWorkflowHealth      `json:"workflow_health"`
	CI                   cliSessionCI           `json:"ci"`
	ProposalQueue        cliSessionQueue        `json:"proposal_queue"`
	TopFindings          []string               `json:"top_findings"`
	TopGuardrails        []string               `json:"top_guardrails"`
	BlindSpots           []string               `json:"blind_spots"`
	RecommendedNextAction string                `json:"recommended_next_action"`
}

type cliSessionGraph struct {
	Available          bool    `json:"available"`
	Stale              bool    `json:"stale"`
	BuiltAt            string  `json:"built_at,omitempty"`
	AgeSeconds         float64 `json:"age_seconds,omitempty"`
	StaleReason        string  `json:"stale_reason,omitempty"`
	RebuildRecommended bool    `json:"rebuild_recommended"`
}

type cliSessionRuntime struct {
	Status string `json:"status"` // live | partial | noop | unavailable
}

type cliWorkflowHealth struct {
	Verdict    string `json:"verdict"` // healthy | degraded | unknown
	Reason     string `json:"reason"`
	NextAction string `json:"next_action"`
}

type cliSessionCI struct {
	StrictVerifiedAvailable bool   `json:"strict_verified_available"`
	LastTestResultsFile     string `json:"last_test_results_file,omitempty"`
	LastPassedAt            string `json:"last_passed_at,omitempty"`
}

type cliSessionQueue struct {
	Status     string `json:"status"` // healthy | needs_review | stale | blocked
	DraftCount int    `json:"draft_count"`
	StaleCount int    `json:"stale_count"`
	NextAction string `json:"next_action,omitempty"`
}

func buildCLISessionStart(repoRoot, docsDir, dbPath string) cliSessionResult {
	now := time.Now().UTC()
	result := cliSessionResult{
		CheckedAt: now.Format(time.RFC3339),
		Status:    "ready",
		TopGuardrails: []string{
			"NO_MATCH does not mean safe — check coverage and blind_spots.",
			"Do not use localhost/127.0.0.1 for inter-service gRPC — resolve from etcd.",
			"Do not treat runtime noop as cluster healthy — noop means no evidence collected.",
			"Run awareness.impact_file before editing files in golang/awareness/ or golang/mcp/.",
			"Do not mutate awareness knowledge without proposal approval.",
		},
	}

	// Graph section.
	gs := cliSessionGraph{}
	var grefreshed *preflight.LiveOverlayFreshness
	g, err := openAwarenessGraph(dbPath, awareCfg.repoPath)
	if err == nil && g != nil {
		defer g.Close()
		gs.Available = true
		if docsDir != "" {
			f := g.Freshness(context.Background(), docsDir)
			gs.Stale = f.Stale
			gs.StaleReason = f.StaleReason
			gs.RebuildRecommended = f.RebuildRecommended
			gs.AgeSeconds = f.AgeSeconds
			if !f.BuiltAt.IsZero() {
				gs.BuiltAt = f.BuiltAt.UTC().Format(time.RFC3339)
			}
			if f.Stale {
				result.Status = "warning"
				result.BlindSpots = append(result.BlindSpots,
					"Graph stale: "+f.StaleReason+" — rebuild before relying on preflight.")
			}
		}
		grefreshed = preflight.ComputeLiveOverlayFreshness(context.Background(), g, now)
	} else {
		gs.Available = false
		gs.RebuildRecommended = true
		result.Status = "warning"
		result.BlindSpots = append(result.BlindSpots,
			"Graph not available — run 'globular awareness build'. All preflight tools operate in degraded mode.")
	}
	result.Graph = gs

	// Runtime section — prefer live overlay evidence when available.
	runtimeStatus := "noop"
	if grefreshed != nil {
		switch grefreshed.Status {
		case "fresh":
			runtimeStatus = "live"
		case "partial", "failed", "stale":
			runtimeStatus = "partial"
		}
	}
	if runtimeStatus == "noop" {
		if _, err := os.Stat("/var/lib/globular/config/etcd.yaml"); err == nil {
			runtimeStatus = "partial" // config exists → cluster may be reachable
		}
	}
	result.WorkflowHealth = deriveCLIWorkflowHealth(runtimeStatus, grefreshed)
	result.Runtime = cliSessionRuntime{Status: runtimeStatus}
	if runtimeStatus == "noop" {
		if result.Status == "ready" {
			result.Status = "warning"
		}
		result.BlindSpots = append(result.BlindSpots,
			"Runtime is noop — no live cluster config detected. Static checks only.")
	} else if grefreshed != nil && grefreshed.Status == "stale" {
		if result.Status == "ready" {
			result.Status = "warning"
		}
		result.BlindSpots = append(result.BlindSpots,
			"Runtime live overlay is stale — refresh runtime evidence before high-risk changes.")
	}

	// CI section.
	ciPath := filepath.Join(repoRoot, ".awareness", "test-results.json")
	if info, err := os.Stat(ciPath); err == nil {
		result.CI = cliSessionCI{
			StrictVerifiedAvailable: true,
			LastTestResultsFile:     ciPath,
			LastPassedAt:            info.ModTime().UTC().Format(time.RFC3339),
		}
	} else {
		result.CI = cliSessionCI{StrictVerifiedAvailable: false}
	}

	// Proposal queue section.
	result.ProposalQueue = buildCLIQueueSection(docsDir)
	if result.ProposalQueue.Status != "healthy" && result.Status == "ready" {
		result.Status = "warning"
	}

	// Recommended action.
	if !gs.Available {
		result.RecommendedNextAction = "Run 'globular awareness build' to index the codebase."
	} else if gs.Stale {
		result.RecommendedNextAction = "Rebuild graph ('globular awareness build'), then run 'globular awareness impact --file <path>' before editing."
	} else if runtimeStatus == "noop" || (grefreshed != nil && grefreshed.Status == "stale") {
		result.RecommendedNextAction = "Run 'globular awareness live-snapshot' (or 'globular awareness runtime-snapshot --write-graph') to collect live evidence, then continue."
	} else if result.ProposalQueue.Status == "stale" {
		result.RecommendedNextAction = "Run 'globular awareness list-proposals' to triage stale drafts, then approve/promote or close each stale item."
	} else {
		result.RecommendedNextAction = "Run 'globular awareness impact --file <path>' for each file you plan to edit."
	}
	result.TopFindings = deriveSessionTopFindings(result)

	return result
}

func deriveSessionTopFindings(r cliSessionResult) []string {
	findings := []string{}
	if !r.Graph.Available {
		findings = append(findings, "Graph unavailable: awareness tools run degraded; rebuild required.")
	} else if r.Graph.Stale {
		findings = append(findings, "Graph stale: preflight/impact confidence reduced until rebuild.")
	}

	if r.Runtime.Status == "noop" {
		findings = append(findings, "Runtime evidence missing: cluster health cannot be inferred from static graph.")
	}
	if r.Runtime.Status == "partial" {
		for _, b := range r.BlindSpots {
			if b == "Runtime live overlay is stale — refresh runtime evidence before high-risk changes." {
				findings = append(findings, "Runtime evidence stale/partial: refresh live snapshot before risky edits.")
				break
			}
		}
	}
	if r.WorkflowHealth.Verdict == "degraded" {
		findings = append(findings, "Workflow degraded: backend/workflow runtime collector reported failures.")
	} else if r.WorkflowHealth.Verdict == "unknown" && r.Runtime.Status != "noop" {
		findings = append(findings, "Workflow health unknown: no workflow runtime collector evidence yet.")
	}

	if !r.CI.StrictVerifiedAvailable {
		findings = append(findings, "Strict CI evidence missing: no .awareness/test-results.json found.")
	}
	if r.ProposalQueue.Status == "stale" {
		findings = append(findings, "Knowledge queue stale: run 'globular awareness list-proposals' and resolve stale drafts.")
	}
	if r.ProposalQueue.Status == "needs_review" {
		findings = append(findings, "Knowledge queue large: review pending DRAFT proposals.")
	}

	if len(findings) > 3 {
		findings = findings[:3]
	}
	return findings
}

func deriveCLIWorkflowHealth(runtimeStatus string, liveOverlay *preflight.LiveOverlayFreshness) cliWorkflowHealth {
	if runtimeStatus == "noop" {
		return cliWorkflowHealth{
			Verdict:    "unknown",
			Reason:     "No runtime evidence is available.",
			NextAction: "Run 'globular awareness live-snapshot --collect-workflow --workflow-addr <host:port>' and re-check.",
		}
	}
	if liveOverlay == nil {
		return cliWorkflowHealth{
			Verdict:    "unknown",
			Reason:     "No live overlay record found.",
			NextAction: "Run 'globular awareness live-snapshot' and re-check workflow health.",
		}
	}
	for _, c := range liveOverlay.Collectors {
		if c.CollectorID != "workflow_execution" {
			continue
		}
		switch c.Status {
		case "ok", "checked_clean", "checked_with_matches":
			return cliWorkflowHealth{
				Verdict:    "healthy",
				Reason:     "Workflow runtime collector reported healthy evidence.",
				NextAction: "Continue with workflow-aware preflight checks before edits.",
			}
		default:
			reason := "Workflow runtime collector reported degraded status."
			if c.Error != "" {
				reason = "Workflow runtime collector error: " + c.Error
			}
			return cliWorkflowHealth{
				Verdict:    "degraded",
				Reason:     reason,
				NextAction: "Stabilize workflow backend/control-plane first, then re-run live snapshot.",
			}
		}
	}
	return cliWorkflowHealth{
		Verdict:    "unknown",
		Reason:     "Live overlay exists but has no workflow collector evidence.",
		NextAction: "Run 'globular awareness live-snapshot --collect-workflow --workflow-addr <host:port>' and re-check.",
	}
}

func buildCLIQueueSection(docsDir string) cliSessionQueue {
	if docsDir == "" {
		return cliSessionQueue{Status: "healthy"}
	}
	proposalsDir := filepath.Join(docsDir, "proposals")
	entries, err := os.ReadDir(proposalsDir)
	if err != nil {
		return cliSessionQueue{Status: "healthy"}
	}

	staleThreshold := 24 * time.Hour
	now := time.Now()
	total := 0
	stale := 0

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		total++
		if now.Sub(info.ModTime()) > staleThreshold {
			stale++
		}
	}

	status := "healthy"
	if stale > 0 {
		status = "stale"
	} else if total > 5 {
		status = "needs_review"
	}

	nextAction := ""
	switch status {
	case "stale":
		nextAction = "Run 'globular awareness list-proposals' to triage stale drafts."
	case "needs_review":
		nextAction = "Run 'globular awareness list-proposals' to review and prune proposal backlog."
	}

	return cliSessionQueue{Status: status, DraftCount: total, StaleCount: stale, NextAction: nextAction}
}

func printSessionStartTable(r cliSessionResult) {
	statusIcon := "✓"
	if r.Status == "warning" {
		statusIcon = "⚠"
	} else if r.Status == "critical" {
		statusIcon = "✗"
	}
	fmt.Printf("AWARENESS SESSION START  [%s %s]  %s\n\n", statusIcon, r.Status, r.CheckedAt)

	// Graph.
	if r.Graph.Available {
		freshLabel := "fresh"
		if r.Graph.Stale {
			freshLabel = "STALE — rebuild recommended"
		}
		fmt.Printf("  graph:    available (%s", freshLabel)
		if r.Graph.BuiltAt != "" {
			fmt.Printf(", built %s", r.Graph.BuiltAt)
		}
		fmt.Println(")")
	} else {
		fmt.Println("  graph:    NOT available — run 'globular awareness build'")
	}

	// Runtime.
	fmt.Printf("  runtime:  %s\n", r.Runtime.Status)
	fmt.Printf("  workflow: %s (%s)\n", r.WorkflowHealth.Verdict, r.WorkflowHealth.Reason)

	// CI.
	if r.CI.StrictVerifiedAvailable {
		fmt.Printf("  ci:       strict_verified available (last: %s)\n", r.CI.LastPassedAt)
	} else {
		fmt.Println("  ci:       no test-results.json found")
	}

	// Queue.
	fmt.Printf("  proposals: %s (%d total, %d stale)\n", r.ProposalQueue.Status, r.ProposalQueue.DraftCount, r.ProposalQueue.StaleCount)
	if r.ProposalQueue.NextAction != "" {
		fmt.Printf("             %s\n", r.ProposalQueue.NextAction)
	}

	// Guardrails.
	if len(r.TopFindings) > 0 {
		fmt.Println("\n  Top findings:")
		for _, f := range r.TopFindings {
			fmt.Printf("    ! %s\n", f)
		}
	}

	// Guardrails.
	fmt.Println("\n  Top guardrails:")
	for _, g := range r.TopGuardrails {
		fmt.Printf("    • %s\n", g)
	}

	// Blind spots.
	if len(r.BlindSpots) > 0 {
		fmt.Println("\n  Blind spots:")
		for _, b := range r.BlindSpots {
			fmt.Printf("    ⚠ %s\n", b)
		}
	}

	// Recommended.
	fmt.Printf("\n  Next: %s\n", r.RecommendedNextAction)
}

func init() {
	awarenessSessionStartCmd.Flags().StringVar(&sessionStartCfg.format, "output", "table", "Output format: table | json")
	awarenessSessionStartCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json (default: .globular/awareness/graph.json)")
	awarenessSessionStartCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")

	awarenessCmd.AddCommand(awarenessSessionStartCmd)
}
