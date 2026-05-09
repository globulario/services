package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
)

func openAgentUsageGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

// TestAgentUsage_PreflightRunRecorded verifies that calling RecordAgentUsage
// with tool="awareness.preflight" increases PreflightCalls in the summary.
func TestAgentUsage_PreflightRunRecorded(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	sessionHash := "test-session-abc123"
	if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
		ID:            "preflight-run-001",
		Agent:         "claude",
		SessionIDHash: sessionHash,
		Tool:          "awareness.preflight",
		Operation:     "called",
		TaskType:      "preflight",
	}); err != nil {
		t.Fatalf("RecordAgentUsage: %v", err)
	}

	summary, err := g.QueryAgentUsageSummary(ctx, 30)
	if err != nil {
		t.Fatalf("QueryAgentUsageSummary: %v", err)
	}
	if summary.PreflightCalls != 1 {
		t.Errorf("expected PreflightCalls=1, got %d", summary.PreflightCalls)
	}
	if summary.SessionsTotal != 1 {
		t.Errorf("expected SessionsTotal=1, got %d", summary.SessionsTotal)
	}
}

// TestAgentUsage_PreflightSkipRateCalculated verifies that the skip rate is
// correctly calculated when sessions outnumber preflight calls.
// With 3 sessions and 1 preflight call, skip rate = (1 - 1/3) * 100 ≈ 66.7%.
func TestAgentUsage_PreflightSkipRateCalculated(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	// Record 3 distinct sessions.
	for i := 0; i < 3; i++ {
		sessionHash := fmt.Sprintf("session-%d", i)
		if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
			ID:            fmt.Sprintf("session-start-%d", i),
			Agent:         "claude",
			SessionIDHash: sessionHash,
			Tool:          "awareness.session_start",
			Operation:     "called",
		}); err != nil {
			t.Fatalf("RecordAgentUsage session %d: %v", i, err)
		}
	}

	// Only 1 of the 3 sessions called preflight.
	if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
		ID:            "preflight-run-001",
		Agent:         "claude",
		SessionIDHash: "session-0",
		Tool:          "awareness.preflight",
		Operation:     "called",
	}); err != nil {
		t.Fatalf("RecordAgentUsage preflight: %v", err)
	}

	summary, err := g.QueryAgentUsageSummary(ctx, 30)
	if err != nil {
		t.Fatalf("QueryAgentUsageSummary: %v", err)
	}

	if summary.SessionsTotal != 3 {
		t.Errorf("expected SessionsTotal=3, got %d", summary.SessionsTotal)
	}
	if summary.PreflightCalls != 1 {
		t.Errorf("expected PreflightCalls=1, got %d", summary.PreflightCalls)
	}
	// Skip rate = (1 - 1/3) * 100 = 66.66...
	if summary.PreflightSkipRatePct < 60 || summary.PreflightSkipRatePct > 70 {
		t.Errorf("expected preflight skip rate ≈ 66.7%%, got %.1f%%", summary.PreflightSkipRatePct)
	}
	if summary.Status != "warning" {
		t.Errorf("expected status=warning when skip rate > 50%%, got %q", summary.Status)
	}
}

// TestPreEditContext_ReturnsInvariantContext verifies that the pre_edit_context
// handler records a usage event AND returns file invariant context.
func TestPreEditContext_ReturnsInvariantContext(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	// Pre-seed a source file node with an invariant edge.
	testFile := "golang/awareness/graph/db.go"
	fileID := "source_file:" + testFile
	invID := "invariant:test.db_schema"
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: testFile})
	_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: "test.db_schema",
		Summary: "DB schema must be backward compatible"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: invID})

	// Call buildFileInvariantContext directly (same logic used by pre_edit_context handler).
	result, err := buildFileInvariantContext(ctx, g, testFile)
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	// Verify the result contains the invariant.
	invariants, ok := result["invariants"].([]map[string]interface{})
	if !ok || len(invariants) == 0 {
		t.Fatalf("expected non-empty invariants list, got %v (type %T)", result["invariants"], result["invariants"])
	}
	found := false
	for _, inv := range invariants {
		if inv["invariant_id"] == "test.db_schema" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invariant test.db_schema in result, got %v", invariants)
	}

	// Record the pre_edit_context event manually (mirrors the handler behavior).
	if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
		ID:           fmt.Sprintf("pre_edit_%s_%d", sanitizeID(testFile), time.Now().UnixNano()),
		Agent:        "claude",
		SessionIDHash: "test-session-pre-edit",
		Tool:         "awareness.pre_edit_context",
		Operation:    "called",
		TaskType:     "pre_edit",
	}); err != nil {
		t.Fatalf("RecordAgentUsage: %v", err)
	}

	summary, err := g.QueryAgentUsageSummary(ctx, 30)
	if err != nil {
		t.Fatalf("QueryAgentUsageSummary: %v", err)
	}
	if summary.PreEditContextCalls != 1 {
		t.Errorf("expected PreEditContextCalls=1, got %d", summary.PreEditContextCalls)
	}
}

// TestAgentUsage_CommitsWithoutIntegrityWarn verifies that recording a commit
// with operation="skipped" for tool="commit.graph_integrity" increments
// CommitsWithoutIntegrityCheck and triggers a warning status.
func TestAgentUsage_CommitsWithoutIntegrityWarn(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	// Simulate a commit that bypassed graph integrity check.
	if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
		ID:            "commit-no-integrity-001",
		Agent:         "claude",
		SessionIDHash: "session-xyz",
		Tool:          "commit.graph_integrity",
		Operation:     "skipped",
		TaskType:      "commit",
	}); err != nil {
		t.Fatalf("RecordAgentUsage: %v", err)
	}

	// Also record a normal session so SessionsTotal > 0.
	if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
		ID:            "session-start-001",
		Agent:         "claude",
		SessionIDHash: "session-xyz",
		Tool:          "awareness.session_start",
		Operation:     "called",
	}); err != nil {
		t.Fatalf("RecordAgentUsage session: %v", err)
	}

	summary, err := g.QueryAgentUsageSummary(ctx, 30)
	if err != nil {
		t.Fatalf("QueryAgentUsageSummary: %v", err)
	}
	if summary.CommitsWithoutIntegrityCheck != 1 {
		t.Errorf("expected CommitsWithoutIntegrityCheck=1, got %d", summary.CommitsWithoutIntegrityCheck)
	}
	if summary.Status != "warning" {
		t.Errorf("expected status=warning when commits bypassed integrity, got %q", summary.Status)
	}
}
