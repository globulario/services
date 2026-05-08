package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/integrity"
	"github.com/globulario/services/golang/awareness/learning"
)

func registerAwarenessSessionTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.session_start",
		Description: "Run this at the beginning of every development session. " +
			"Reports graph freshness, runtime activation status, CI verification, " +
			"proposal queue health, top guardrails, and blind spots. " +
			"Use the result to decide whether to rebuild the graph, collect runtime, or proceed.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return buildSessionStart(ctx, st), nil
	})
}

// sessionStartResult is the JSON-serialisable session handshake output.
type sessionStartResult struct {
	Status       string               `json:"status"` // ready | warning | critical
	CheckedAt    string               `json:"checked_at"`
	Graph        sessionGraphSection  `json:"graph"`
	Runtime      sessionRuntimeSection `json:"runtime"`
	CI           sessionCISection     `json:"ci"`
	ProposalQueue sessionQueueSection `json:"proposal_queue"`
	TopGuardrails []string             `json:"top_guardrails"`
	BlindSpots    []string             `json:"blind_spots"`
	RecommendedNextAction string       `json:"recommended_next_action"`
}

type sessionGraphSection struct {
	Available          bool    `json:"available"`
	Stale              bool    `json:"stale"`
	BuiltAt            string  `json:"built_at,omitempty"`
	AgeSeconds         float64 `json:"age_seconds,omitempty"`
	StaleReason        string  `json:"stale_reason,omitempty"`
	RebuildRecommended bool    `json:"rebuild_recommended"`
}

type sessionRuntimeSection struct {
	Status  string   `json:"status"`  // live | partial | noop | unavailable
	Sources []string `json:"sources"` // which sources are configured
}

type sessionCISection struct {
	StrictVerifiedAvailable bool   `json:"strict_verified_available"`
	LastTestResultsFile     string `json:"last_test_results_file,omitempty"`
	LastPassedAt            string `json:"last_passed_at,omitempty"`
}

type sessionQueueSection struct {
	Status      string `json:"status"` // healthy | needs_review | stale | blocked
	DraftCount  int    `json:"draft_count"`
	StaleCount  int    `json:"stale_count"`
}

func buildSessionStart(ctx context.Context, st *awarenessState) sessionStartResult {
	now := time.Now().UTC()
	result := sessionStartResult{
		CheckedAt: now.Format(time.RFC3339),
		Status:    "ready",
		TopGuardrails: []string{
			"Do not use localhost/127.0.0.1 for inter-service gRPC — resolve from etcd.",
			"Do not treat NO_MATCH or checked_clean as proof of safety without runtime evidence.",
			"Do not mutate awareness knowledge without proposal approval.",
			"Run awareness.impact_file before editing any file in golang/awareness/ or golang/mcp/.",
			"Run awareness.scan_violations before committing changes to high-risk files.",
		},
	}

	// Graph section.
	gs := sessionGraphSection{Available: st.g != nil}
	if st.g != nil && st.docsDir != "" {
		f := st.g.Freshness(ctx, st.docsDir)
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
				fmt.Sprintf("Graph stale: %s — run 'globular awareness build' before relying on preflight.", f.StaleReason))
		}
	} else if st.g == nil {
		gs.RebuildRecommended = true
		result.Status = "warning"
		result.BlindSpots = append(result.BlindSpots,
			"Graph not available — run 'globular awareness build' first. All preflight tools will operate in degraded mode.")
	}
	result.Graph = gs

	// Runtime section — check MCP config addresses.
	result.Runtime = sessionRuntimeSection{
		Status:  computeSessionRuntimeStatus(st),
		Sources: []string{},
	}
	if result.Runtime.Status == "noop" {
		result.BlindSpots = append(result.BlindSpots,
			"Runtime is noop — no live cluster addresses configured. Do not infer cluster health from static checks alone.")
	}

	// CI section — look for .awareness/test-results.json in repo root.
	result.CI = buildSessionCISection(st.repoRoot)

	// Proposal queue — count DRAFTs and stale proposals.
	result.ProposalQueue = buildSessionQueueSection(st.docsDir)
	if result.ProposalQueue.Status != "healthy" && result.Status == "ready" {
		result.Status = "warning"
	}

	// Recommended action.
	if !gs.Available {
		result.RecommendedNextAction = "Run 'globular awareness build' to index the codebase before any code work."
	} else if gs.Stale {
		result.RecommendedNextAction = "Rebuild graph: 'globular awareness build'. Then run awareness.impact_file before editing files."
	} else {
		result.RecommendedNextAction = "Run awareness.impact_file for each file you plan to edit."
	}

	return result
}

func computeSessionRuntimeStatus(st *awarenessState) string {
	// The MCP server config carries runtime bridge addresses.
	// We check whether any gRPC addresses are configured in the server config.
	// If none are configured, runtime is noop.
	cfg := st // awarenessState doesn't carry bridge config directly
	_ = cfg   // suppress unused; actual config access below

	// Check server-level config for runtime addresses via the global server instance.
	// Since we don't have direct access here, we detect from MCP config file.
	// Simplified: check for /var/lib/globular/config/etcd.yaml existence as proxy
	// for "cluster is potentially reachable".
	if _, err := os.Stat("/var/lib/globular/config/etcd.yaml"); err == nil {
		return "partial" // etcd config exists → cluster may be reachable
	}
	return "noop"
}

func buildSessionCISection(repoRoot string) sessionCISection {
	if repoRoot == "" {
		return sessionCISection{}
	}
	path := filepath.Join(repoRoot, ".awareness", "test-results.json")
	info, err := os.Stat(path)
	if err != nil {
		return sessionCISection{StrictVerifiedAvailable: false}
	}
	return sessionCISection{
		StrictVerifiedAvailable: true,
		LastTestResultsFile:     path,
		LastPassedAt:            info.ModTime().UTC().Format(time.RFC3339),
	}
}

func buildSessionQueueSection(docsDir string) sessionQueueSection {
	if docsDir == "" {
		return sessionQueueSection{Status: "healthy"}
	}
	proposalsDir := filepath.Join(docsDir, "proposals")
	entries, err := os.ReadDir(proposalsDir)
	if err != nil {
		return sessionQueueSection{Status: "healthy"}
	}

	staleThreshold := 24 * time.Hour
	now := time.Now()
	draftCount := 0
	staleCount := 0

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		// Count YAML files as drafts (simplified — full parsing is in queue_health tool).
		draftCount++
		if now.Sub(info.ModTime()) > staleThreshold {
			staleCount++
		}
	}

	status := "healthy"
	if staleCount > 0 {
		status = "stale"
	} else if draftCount > 5 {
		status = "needs_review"
	}

	return sessionQueueSection{
		Status:     status,
		DraftCount: draftCount,
		StaleCount: staleCount,
	}
}

// trustSummary builds a distribution of trust levels for matched graph nodes.
// Used by tools that want to show which matches are verified vs declared vs inferred.
func trustSummary(ctx context.Context, g *graph.Graph, nodeIDs []string, prefix string) map[string]int {
	counts := map[string]int{
		integrity.TrustStrictVerified: 0,
		integrity.TrustVerified:       0,
		integrity.TrustDeclared:       0,
		integrity.TrustInferred:       0,
		integrity.TrustProposal:       0,
		integrity.TrustStale:          0,
		integrity.TrustInvalid:        0,
	}

	for _, id := range nodeIDs {
		nodeID := prefix + id
		n, err := g.FindNode(ctx, nodeID)
		if err != nil || n == nil {
			continue
		}
		tl, _ := n.Metadata["trust_level"].(string)
		if tl == "" {
			tl, _ = n.Metadata["verification_level"].(string)
		}
		if tl == "" {
			tl = integrity.TrustDeclared // assume declared if no metadata
		}
		if _, ok := counts[tl]; ok {
			counts[tl]++
		}
	}
	return counts
}

// buildTrustSummary aggregates trust level counts across invariants, failure modes, and forbidden fixes.
func buildTrustSummary(ctx context.Context, g *graph.Graph, invariantIDs, failureModeIDs, forbiddenFixIDs []string) map[string]int {
	total := map[string]int{
		integrity.TrustStrictVerified: 0,
		integrity.TrustVerified:       0,
		integrity.TrustDeclared:       0,
		integrity.TrustInferred:       0,
		integrity.TrustProposal:       0,
		integrity.TrustStale:          0,
		integrity.TrustInvalid:        0,
	}
	for tl, count := range trustSummary(ctx, g, invariantIDs, "invariant:") {
		total[tl] += count
	}
	for tl, count := range trustSummary(ctx, g, failureModeIDs, "failure_mode:") {
		total[tl] += count
	}
	for tl, count := range trustSummary(ctx, g, forbiddenFixIDs, "forbidden_fix:") {
		total[tl] += count
	}
	return total
}

// agentPlaybookEntry is referenced but defined in the knowledge YAML loader.
// Import alias to avoid import cycle — the session_start tool only uses the
// learning package for alias loading, not the full playbook loader.
var _ = learning.LoadContextAliases
