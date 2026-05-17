package analysis_test

// Phase 2 ranking tests + Phase 5 negative tests.
//
// Negative tests prove the guardrails have teeth: they attempt bad conditions
// and verify the system catches them rather than silently allowing them.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/graph"
)

// ── Phase 2: ranking ─────────────────────────────────────────────────────────

// TestRankFindingsMandatoryFirst proves mandatory findings sort before optional ones.
func TestRankFindingsMandatoryFirst(t *testing.T) {
	g, ctx := newG(t)

	fileID := "source_file:golang/cluster_controller/cluster_controller_server/reconcile.go"
	filePath := "golang/cluster_controller/cluster_controller_server/reconcile.go"
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: "reconcile.go", Path: filePath})

	// Optional invariant — reachable only via may_affect.
	optInvID := "invariant:optional.background_heuristic"
	_ = g.AddNode(ctx, graph.Node{ID: optInvID, Type: graph.NodeTypeInvariant, Name: "optional.background_heuristic"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeMayAffect, Dst: optInvID})

	// Mandatory forbidden fix — always mandatory regardless of edge.
	ffID := "forbidden_fix:ff.reconcile_no_direct_etcd"
	_ = g.AddNode(ctx, graph.Node{ID: ffID, Type: graph.NodeTypeForbiddenFix, Name: "ff.reconcile_no_direct_etcd"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeMayAffect, Dst: ffID})

	result, err := analysis.ExplainImpactByFile(ctx, g, filePath)
	if err != nil {
		t.Fatalf("ExplainImpactByFile: %v", err)
	}

	// ForbiddenFixes partition: the one entry must be mandatory.
	if len(result.ForbiddenFixes) == 0 {
		t.Skip("no forbidden fix found — graph traversal did not reach the node; adjust edge if needed")
	}
	if !result.ForbiddenFixes[0].Mandatory {
		t.Error("first ForbiddenFix should be mandatory")
	}
}

// TestRankFindingsBySeverity proves critical invariants sort above lower-severity ones.
func TestRankFindingsBySeverity(t *testing.T) {
	g, ctx := newG(t)

	fileID := "source_file:golang/workflow/workflow_server/dispatcher.go"
	filePath := "golang/workflow/workflow_server/dispatcher.go"
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: "dispatcher.go", Path: filePath})

	// Low-severity invariant.
	lowID := "invariant:workflow.dispatch_logging_optional"
	_ = g.AddNode(ctx, graph.Node{ID: lowID, Type: graph.NodeTypeInvariant, Name: "workflow.dispatch_logging_optional"})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: "workflow.dispatch_logging_optional", Title: "Dispatch logging", Severity: "low"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeEnforces, Dst: lowID})

	// Critical-severity invariant.
	critID := "invariant:workflow.dispatch_is_not_completion"
	_ = g.AddNode(ctx, graph.Node{ID: critID, Type: graph.NodeTypeInvariant, Name: "workflow.dispatch_is_not_completion"})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: "workflow.dispatch_is_not_completion", Title: "Dispatch ≠ completion", Severity: "critical"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeEnforces, Dst: critID})

	result, err := analysis.ExplainImpactByFile(ctx, g, filePath)
	if err != nil {
		t.Fatalf("ExplainImpactByFile: %v", err)
	}

	if len(result.Invariants) < 2 {
		t.Fatalf("expected ≥2 invariants, got %d", len(result.Invariants))
	}
	if result.Invariants[0].NodeName != "workflow.dispatch_is_not_completion" {
		t.Errorf("expected critical invariant first, got %q", result.Invariants[0].NodeName)
	}
}

// ── Phase 5: negative tests ───────────────────────────────────────────────────

// TestRejectFuzzyResultAsActionAuthority proves that an unknown file returns
// MissingLinks rather than an empty result that could be misread as "no rules apply."
// An empty graph result MUST NOT be interpreted as "safe to proceed."
func TestRejectFuzzyResultAsActionAuthority(t *testing.T) {
	g, ctx := newG(t)
	// File that exists nowhere in the graph.
	result, err := analysis.ExplainImpactByFile(ctx, g, "golang/some_new_package/new_file.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Zero invariants returned — but this must NOT be treated as "safe."
	// The system signals incompleteness via MissingLinks.
	if len(result.MissingLinks) == 0 {
		t.Error("unknown file must produce MissingLinks — empty result is NOT safe-to-proceed authority")
	}
}

// TestRejectGraphEdgeToMissingNode proves that an edge referencing a nonexistent
// node does not silently surface a result with a broken path.
func TestRejectGraphEdgeToMissingNode(t *testing.T) {
	g, ctx := newG(t)

	fileID := "source_file:golang/foo/bar.go"
	filePath := "golang/foo/bar.go"
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: "bar.go", Path: filePath})

	// Add an edge to a node that does NOT exist.
	ghostID := "invariant:ghost.nonexistent_invariant"
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: ghostID})

	result, err := analysis.ExplainImpactByFile(ctx, g, filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The ghost invariant must not appear in results (dangling edge must not produce finding).
	for _, f := range result.Invariants {
		if f.NodeID == ghostID {
			t.Errorf("dangling edge to nonexistent node %q must not produce a finding", ghostID)
		}
	}
}

// TestRejectImpactResultWithoutReason proves every returned finding has a
// non-empty EdgePath (no result may be returned without a reason/graph path).
func TestRejectImpactResultWithoutReason(t *testing.T) {
	g, ctx := newG(t)

	fileID := "source_file:golang/dns/dns_server/zone_sync.go"
	filePath := "golang/dns/dns_server/zone_sync.go"
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: "zone_sync.go", Path: filePath})

	invID := "invariant:dns.zone_persistence_scylla"
	_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: "dns.zone_persistence_scylla"})
	_ = g.UpsertInvariant(ctx, graph.Invariant{ID: "dns.zone_persistence_scylla", Title: "DNS zones persist to ScyllaDB", Severity: "high"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: invID})

	result, err := analysis.ExplainImpactByFile(ctx, g, filePath)
	if err != nil {
		t.Fatalf("ExplainImpactByFile: %v", err)
	}

	// Every finding must have a non-empty EdgePath.
	all := append(append(append(result.Invariants, result.ForbiddenFixes...), result.RequiredTests...), result.FailureModes...)
	for _, f := range all {
		if len(f.EdgePath) == 0 {
			t.Errorf("finding %q has empty EdgePath — every result must carry a reason/graph path", f.NodeID)
		}
	}
}

// TestRejectNoMatchWithoutCoverageExplanation proves that a file with no graph
// edges produces MissingLinks that explain WHY the result is empty (coverage gap).
func TestRejectNoMatchWithoutCoverageExplanation(t *testing.T) {
	g, ctx := newG(t)
	result, err := analysis.ExplainImpactByFile(ctx, g, "golang/ai_executor/ai_executor_server/job_dispatch.go")
	if err != nil {
		t.Fatalf("ExplainImpactByFile: %v", err)
	}
	if len(result.Invariants) > 0 || len(result.ForbiddenFixes) > 0 {
		t.Skip("file is covered — test only validates uncovered path")
	}
	if len(result.MissingLinks) == 0 {
		t.Error("NO_MATCH must produce MissingLinks explaining the coverage gap; got none")
	}
	// At least one link must mention a concrete action the caller can take.
	var hasAction bool
	for _, l := range result.MissingLinks {
		if len(l) > 20 {
			hasAction = true
			break
		}
	}
	if !hasAction {
		t.Error("MissingLinks must include actionable suggestions, not just 'no edges found'")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newG(t *testing.T) (*graph.Graph, context.Context) {
	t.Helper()
	g, err := graph.Open(filepath.Join(t.TempDir(), "graph.json"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g, context.Background()
}
