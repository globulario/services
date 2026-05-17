package main

// awareness_graph_integrity_ci_test.go — CLI-layer tests for graph-integrity-check.
//
// Core correctness tests live in enforce/graph_integrity_ci_test.go.
// These tests verify that the CLI command is registered, flags are wired,
// and the strict-mode exit code logic is correct.

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/awareness/enforce"
	"github.com/globulario/services/golang/awareness/graph"
)

// openCITestGraph opens a temporary in-memory graph for CLI tests.
func openCITestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("open test graph: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

// TestGraphIntegrityCI_StrictModeBlocksMerge verifies that a graph with an
// active invariant missing implementation evidence causes GraphIntegrityCICheck
// to return Pass=false, confirming the CLI --strict path would exit 2.
func TestGraphIntegrityCI_StrictModeBlocksMerge(t *testing.T) {
	ctx := context.Background()
	g := openCITestGraph(t)

	// Invariant with no implementing source file → INVARIANT_NO_IMPLEMENTATION error in CI mode.
	_ = g.AddNode(ctx, graph.Node{ID: "invariant:ci.strict.no_impl", Type: graph.NodeTypeInvariant, Name: "ci.strict.no_impl"})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: "ci.strict.no_impl", Title: "ci.strict.no_impl", Severity: "critical", Status: "active"})

	res := enforce.GraphIntegrityCICheck(ctx, g, enforce.CICheckOptions{})

	if res.Pass {
		t.Error("expected CI check to fail for invariant without implementation — merge must be blocked")
	}
	if res.ErrorCount == 0 {
		t.Error("expected ErrorCount > 0 for unimplemented critical invariant")
	}
	// Verify the --strict flag path: Pass=false → exit 2 in RunE.
	// (os.Exit cannot be called in tests; we verify the condition that triggers it.)
	if len(res.FailureReasons) == 0 {
		t.Error("expected at least one failure reason for CI blocking")
	}
}

// TestGraphIntegrityCI_WarningsOnlyPassWithoutStrict verifies that a graph
// producing only warnings (no errors) passes non-strict CI — confirming
// the CLI would exit 0 without --strict.
func TestGraphIntegrityCI_WarningsOnlyPassWithoutStrict(t *testing.T) {
	ctx := context.Background()
	g := openCITestGraph(t)

	// Fully-implemented invariant missing only forbidden_fix (→ Warning, not Error).
	_ = g.AddNode(ctx, graph.Node{ID: "invariant:ci.strict.warn_only", Type: graph.NodeTypeInvariant, Name: "ci.strict.warn_only"})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: "ci.strict.warn_only", Title: "ci.strict.warn_only", Severity: "medium", Status: "active"})
	_ = g.AddNode(ctx, graph.Node{ID: "source_file:pkg/warn.go", Type: graph.NodeTypeSourceFile})
	_ = g.AddEdge(ctx, graph.Edge{Src: "source_file:pkg/warn.go", Kind: graph.EdgeImplements, Dst: "invariant:ci.strict.warn_only"})
	_ = g.AddNode(ctx, graph.Node{ID: "test:TestCIWarnOnly", Type: graph.NodeTypeTest})
	_ = g.AddEdge(ctx, graph.Edge{Src: "invariant:ci.strict.warn_only", Kind: graph.EdgeTestedBy, Dst: "test:TestCIWarnOnly"})
	_ = g.AddNode(ctx, graph.Node{ID: "failure_mode:ci_fm_warn", Type: graph.NodeTypeFailureMode})
	_ = g.AddEdge(ctx, graph.Edge{Src: "invariant:ci.strict.warn_only", Kind: graph.EdgeAffects, Dst: "failure_mode:ci_fm_warn"})

	res := enforce.GraphIntegrityCICheck(ctx, g, enforce.CICheckOptions{MaxRequiredTestNoPath: 100})

	if res.ErrorCount > 0 {
		t.Errorf("expected no errors for warning-only graph (strict=false), got %d: %v", res.ErrorCount, res.FailureReasons)
	}
	if !res.Pass {
		t.Error("expected Pass=true for warning-only graph without --strict")
	}
}

// TestGraphIntegrityCheckCmd_IsRegistered verifies that the graph-integrity-check
// command is registered on the awareness command and has --strict flag.
func TestGraphIntegrityCheckCmd_IsRegistered(t *testing.T) {
	cmd, _, err := awarenessCmd.Find([]string{"graph-integrity-check"})
	if err != nil || cmd == nil {
		t.Fatalf("graph-integrity-check command not found on awarenessCmd: %v", err)
	}
	strictFlag := cmd.Flags().Lookup("strict")
	if strictFlag == nil {
		t.Error("--strict flag not registered on graph-integrity-check command")
	}
}
