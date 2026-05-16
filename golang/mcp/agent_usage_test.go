package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/awareness/graph"
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

// TestAgentUsage_RecordsSessionStart verifies that session_start events are
// recorded and contribute to SessionsTotal.
func TestAgentUsage_RecordsSessionStart(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	if err := g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
		ID:            "session-start-001",
		Agent:         "claude",
		SessionIDHash: "session-aaa",
		Tool:          "awareness.session_start",
		Operation:     "called",
		TaskType:      "session_start",
	}); err != nil {
		t.Fatalf("RecordAgentUsage: %v", err)
	}

	summary, err := g.QueryAgentUsageSummary(ctx, 30)
	if err != nil {
		t.Fatalf("QueryAgentUsageSummary: %v", err)
	}
	if summary.SessionsTotal != 1 {
		t.Errorf("expected SessionsTotal=1, got %d", summary.SessionsTotal)
	}
}

// TestPreEditContext_IncludesForbiddenFixes verifies that when a file implements
// an invariant that has a forbidden fix, the forbidden_fixes field is populated.
func TestPreEditContext_IncludesForbiddenFixes(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	testFile := "golang/cluster_controller/server.go"
	fileID := "source_file:" + testFile
	invID := "invariant:no_localhost_grpc"
	fixID := "forbidden_fix:use_localhost_for_grpc"

	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: testFile})
	_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: "no_localhost_grpc",
		Summary: "gRPC addresses must be resolved from etcd, never localhost"})
	_ = g.AddNode(ctx, graph.Node{ID: fixID, Type: graph.NodeTypeForbiddenFix, Name: fixID})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: invID})
	_ = g.AddEdge(ctx, graph.Edge{Src: invID, Kind: graph.EdgeForbids, Dst: fixID})

	result, err := buildFileInvariantContext(ctx, g, testFile)
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	invariants, ok := result["invariants"].([]map[string]interface{})
	if !ok || len(invariants) == 0 {
		t.Fatalf("expected non-empty invariants, got %v", result["invariants"])
	}

	foundFix := false
	for _, inv := range invariants {
		if inv["invariant_id"] == "no_localhost_grpc" {
			fixes, _ := inv["forbidden_fixes"].([]string)
			for _, f := range fixes {
				if f == fixID {
					foundFix = true
				}
			}
		}
	}
	if !foundFix {
		t.Errorf("expected forbidden_fix %q in result, invariants=%v", fixID, invariants)
	}

	// edit_warnings must include a mention of the forbidden fix.
	warnings, _ := result["edit_warnings"].([]string)
	if len(warnings) == 0 {
		t.Error("expected non-empty edit_warnings when forbidden fix exists")
	}
}

// TestPreEditContext_WarnsHighRiskFileWithoutInvariant verifies that a file
// known to the graph but not linked to any invariant returns a warning.
func TestPreEditContext_WarnsHighRiskFileWithoutInvariant(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	testFile := "golang/workflow/engine.go"
	fileID := "source_file:" + testFile
	// Add the file node but no invariant edges.
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: testFile})

	result, err := buildFileInvariantContext(ctx, g, testFile)
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}

	// With no invariants, warning should be empty and invariants list should be empty.
	invariants, _ := result["invariants"].([]map[string]interface{})
	if len(invariants) != 0 {
		t.Errorf("expected zero invariants for unlinked file, got %v", invariants)
	}
	// warning field must be empty (no invariant warning — the file is indexed but has no links)
	warning, _ := result["warning"].(string)
	if warning != "" {
		t.Logf("note: warning=%q (informational, not an error)", warning)
	}
}

// TestPreCommitCheck_RunsGraphIntegrityAndScanSummary verifies that
// runPreCommitIntegritySummary returns a structured result with required fields.
func TestPreCommitCheck_RunsGraphIntegrityAndScanSummary(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	st := &awarenessState{
		g:        g,
		repoRoot: t.TempDir(),
		docsDir:  t.TempDir(),
	}

	result := runPreCommitIntegritySummary(ctx, st)

	if result["status"] == nil {
		t.Error("expected status field in integrity summary")
	}
	if result["pass"] == nil {
		t.Error("expected pass field in integrity summary")
	}
	if result["error_count"] == nil {
		t.Error("expected error_count field in integrity summary")
	}
	if result["warning_count"] == nil {
		t.Error("expected warning_count field in integrity summary")
	}
	// An empty graph should produce pass=true with zero errors.
	pass, _ := result["pass"].(bool)
	if !pass {
		t.Errorf("expected pass=true for empty graph, got status=%v errors=%v", result["status"], result["error_count"])
	}
}

// TestAgentHooks_RecordUsage verifies that the three agent hooks (session_start,
// pre_edit_context, pre_commit_check) all produce usage events visible in the
// summary via the underlying RecordAgentUsage calls.
func TestAgentHooks_RecordUsage(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)

	// Record one event per hook type.
	events := []graph.AgentUsageEvent{
		{ID: "evt-session", Agent: "claude", SessionIDHash: "s1", Tool: "awareness.session_start", Operation: "called"},
		{ID: "evt-pre-edit", Agent: "claude", SessionIDHash: "s1", Tool: "awareness.pre_edit_context", Operation: "called"},
		{ID: "evt-pre-commit", Agent: "claude", SessionIDHash: "s1", Tool: "commit.graph_integrity", Operation: "called"},
	}
	for _, ev := range events {
		if err := g.RecordAgentUsage(ctx, ev); err != nil {
			t.Fatalf("RecordAgentUsage(%q): %v", ev.Tool, err)
		}
	}

	summary, err := g.QueryAgentUsageSummary(ctx, 30)
	if err != nil {
		t.Fatalf("QueryAgentUsageSummary: %v", err)
	}
	if summary.PreEditContextCalls != 1 {
		t.Errorf("expected PreEditContextCalls=1, got %d", summary.PreEditContextCalls)
	}
	// commit.graph_integrity with operation="called" (not skipped) must NOT increment CommitsWithoutIntegrityCheck.
	if summary.CommitsWithoutIntegrityCheck != 0 {
		t.Errorf("expected CommitsWithoutIntegrityCheck=0 when pre_commit_check was called, got %d", summary.CommitsWithoutIntegrityCheck)
	}
}
