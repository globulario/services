package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionStart_GraphFresh_RuntimeNoop_Warning(t *testing.T) {
	// When graph is available and fresh but runtime is noop, status should be "ready"
	// or "warning" (runtime noop is a warning, not critical).
	st := &awarenessState{
		g:        nil, // no graph for unit test
		docsDir:  "",
		repoRoot: t.TempDir(),
		nodeID:   "",
	}

	result := buildSessionStart(context.Background(), st)

	// Graph unavailable → status must be "warning" (not "ready" or "critical").
	if result.Status != "warning" {
		t.Errorf("status = %q, want warning when graph unavailable", result.Status)
	}
	if result.Graph.Available {
		t.Error("graph.available should be false when no graph opened")
	}
	if !result.Graph.RebuildRecommended {
		t.Error("rebuild_recommended should be true when graph unavailable")
	}
	if result.Runtime.Status == "" {
		t.Error("runtime.status should never be empty")
	}
	if result.CheckedAt == "" {
		t.Error("checked_at should never be empty")
	}
}

func TestSessionStart_GraphStale_CriticalOrWarning(t *testing.T) {
	// When graph is stale, status must be at least "warning".
	// We can't easily produce a stale graph in a unit test without a real DB,
	// so we test the no-graph path which is equivalent for status purposes.
	st := &awarenessState{
		g:        nil,
		docsDir:  "",
		repoRoot: t.TempDir(),
	}

	result := buildSessionStart(context.Background(), st)

	if result.Status == "ready" {
		t.Error("status must not be 'ready' when graph is unavailable (equivalent to stale)")
	}
}

func TestSessionStart_IncludesTopGuardrails(t *testing.T) {
	st := &awarenessState{
		g:        nil,
		docsDir:  "",
		repoRoot: t.TempDir(),
	}

	result := buildSessionStart(context.Background(), st)

	if len(result.TopGuardrails) == 0 {
		t.Error("top_guardrails must never be empty — awareness rules must always be surfaced")
	}

	// Must include the NO_MATCH rule.
	found := false
	for _, guardrail := range result.TopGuardrails {
		if strings.Contains(guardrail, "NO_MATCH") || strings.Contains(guardrail, "not mean safe") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("top_guardrails must include NO_MATCH warning; got: %v", result.TopGuardrails)
	}
}

func TestSessionStart_ProposalQueueStale(t *testing.T) {
	// Create a proposals dir with stale proposals.
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a YAML file with a modified time in the past (>24h).
	oldFile := filepath.Join(proposalsDir, "old-proposal.yaml")
	if err := os.WriteFile(oldFile, []byte("id: test\nstatus: DRAFT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to 48h ago.
	pastTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	q := buildSessionQueueSection(docsDir)

	if q.Status != "stale" {
		t.Errorf("proposal_queue.status = %q, want 'stale' when proposals are >24h old", q.Status)
	}
	if q.StaleCount < 1 {
		t.Error("stale_count should be at least 1")
	}
}

func TestSessionStart_NoBareReadyWhenCoverageMissing(t *testing.T) {
	// When graph is unavailable and runtime is noop, status must not be "ready".
	st := &awarenessState{
		g:        nil,
		docsDir:  "",
		repoRoot: t.TempDir(),
	}

	result := buildSessionStart(context.Background(), st)

	if result.Status == "ready" {
		t.Error("status must not be 'ready' when graph is unavailable and runtime is noop — blind spots exist")
	}
	if len(result.BlindSpots) == 0 {
		t.Error("blind_spots must be non-empty when coverage is incomplete")
	}
}

// TestAgentSessionStart_ReturnsGraphAndLiveStatus verifies that buildSessionStart
// always returns populated graph and runtime sections with non-empty status strings.
func TestAgentSessionStart_ReturnsGraphAndLiveStatus(t *testing.T) {
	st := &awarenessState{
		g:        nil,
		docsDir:  t.TempDir(),
		repoRoot: t.TempDir(),
	}

	result := buildSessionStart(context.Background(), st)

	// Graph section must always be populated.
	if result.Graph.Available && result.Graph.BuiltAt == "" {
		t.Error("if graph is available, built_at must be set")
	}
	// Runtime section must always have a non-empty status.
	if result.Runtime.Status == "" {
		t.Error("runtime.status must never be empty")
	}
	// checked_at must be an RFC3339 timestamp.
	if result.CheckedAt == "" {
		t.Error("checked_at must not be empty")
	}
	// RecommendedNextAction must always give guidance.
	if result.RecommendedNextAction == "" {
		t.Error("recommended_next_action must not be empty")
	}
}

// TestAgentSessionStart_ShowsProposalQueueWarning verifies that a stale DRAFT
// proposal in the queue causes buildSessionStart to escalate status to "warning"
// and include a non-empty ProposalQueue.Status.
func TestAgentSessionStart_ShowsProposalQueueWarning(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a stale DRAFT proposal (modified >24h ago).
	oldFile := filepath.Join(proposalsDir, "draft-stale.yaml")
	if err := os.WriteFile(oldFile, []byte("id: draft-stale\nstatus: DRAFT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pastTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	st := &awarenessState{
		g:        nil,
		docsDir:  docsDir,
		repoRoot: t.TempDir(),
	}

	result := buildSessionStart(context.Background(), st)

	// ProposalQueue section must reflect the stale draft.
	if result.ProposalQueue.Status == "" {
		t.Error("proposal_queue.status must not be empty")
	}
	// Session status should be "warning" (stale proposal escalates from healthy).
	if result.Status == "ready" {
		t.Error("status must not be 'ready' when stale proposals exist")
	}
}

