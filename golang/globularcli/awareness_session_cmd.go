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
	CI                   cliSessionCI           `json:"ci"`
	ProposalQueue        cliSessionQueue        `json:"proposal_queue"`
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

type cliSessionCI struct {
	StrictVerifiedAvailable bool   `json:"strict_verified_available"`
	LastTestResultsFile     string `json:"last_test_results_file,omitempty"`
	LastPassedAt            string `json:"last_passed_at,omitempty"`
}

type cliSessionQueue struct {
	Status     string `json:"status"` // healthy | needs_review | stale | blocked
	DraftCount int    `json:"draft_count"`
	StaleCount int    `json:"stale_count"`
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
	} else {
		gs.Available = false
		gs.RebuildRecommended = true
		result.Status = "warning"
		result.BlindSpots = append(result.BlindSpots,
			"Graph not available — run 'globular awareness build'. All preflight tools operate in degraded mode.")
	}
	result.Graph = gs

	// Runtime section — heuristic: check etcd config to guess reachability.
	runtimeStatus := "noop"
	if _, err := os.Stat("/var/lib/globular/config/etcd.yaml"); err == nil {
		runtimeStatus = "partial" // config exists → cluster may be reachable
	}
	result.Runtime = cliSessionRuntime{Status: runtimeStatus}
	if runtimeStatus == "noop" {
		result.BlindSpots = append(result.BlindSpots,
			"Runtime is noop — no live cluster config detected. Static checks only.")
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
	} else {
		result.RecommendedNextAction = "Run 'globular awareness impact --file <path>' for each file you plan to edit."
	}

	return result
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

	return cliSessionQueue{Status: status, DraftCount: total, StaleCount: stale}
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

	// CI.
	if r.CI.StrictVerifiedAvailable {
		fmt.Printf("  ci:       strict_verified available (last: %s)\n", r.CI.LastPassedAt)
	} else {
		fmt.Println("  ci:       no test-results.json found")
	}

	// Queue.
	fmt.Printf("  proposals: %s (%d total, %d stale)\n", r.ProposalQueue.Status, r.ProposalQueue.DraftCount, r.ProposalQueue.StaleCount)

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
	awarenessSessionStartCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessSessionStartCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")

	awarenessCmd.AddCommand(awarenessSessionStartCmd)
}
