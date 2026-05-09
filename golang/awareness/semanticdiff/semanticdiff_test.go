package semanticdiff_test

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/semanticdiff"
)

const fileHeader = `--- a/golang/cluster_controller/reconcile.go
+++ b/golang/cluster_controller/reconcile.go
`

func makeUnifiedDiff(removedLines, addedLines []string) string {
	var sb strings.Builder
	sb.WriteString(fileHeader)
	sb.WriteString("@@ -10,6 +10,8 @@ func Reconcile() {\n")
	for _, l := range removedLines {
		sb.WriteString("-")
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	for _, l := range addedLines {
		sb.WriteString("+")
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	return sb.String()
}

// Test 1: Desired → Installed assignment is forbidden.
func TestDesiredToInstalledForbidden(t *testing.T) {
	diff := makeUnifiedDiff(nil, []string{
		" installed.State = desired.State",
		" installed.Version = desired.Version",
	})
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictBlock {
		t.Errorf("expected block, got %s", report.Verdict)
	}
	if report.Severity != semanticdiff.SeverityForbidden {
		t.Errorf("expected forbidden severity, got %s", report.Severity)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "desired_state_promoted_to_installed_without_proof" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected finding desired_state_promoted_to_installed_without_proof, findings: %+v", report.Findings)
	}
}

// Test 2: Runtime → Desired assignment is forbidden.
func TestRuntimeToDesiredForbidden(t *testing.T) {
	diff := makeUnifiedDiff(nil, []string{
		" desired.State = runtimeObs.State",
	})
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictBlock {
		t.Errorf("expected block, got %s", report.Verdict)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "runtime_state_promoted_to_desired" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected finding runtime_state_promoted_to_desired, findings: %+v", report.Findings)
	}
}

// Test 3: Generation compare removed → block.
func TestGenerationCompareRemovedBlocks(t *testing.T) {
	diff := makeUnifiedDiff(
		[]string{
			" if current.Generation != desired.Generation {",
			"     return ErrGenerationMismatch",
			" }",
		},
		nil,
	)
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictBlock {
		t.Errorf("expected block, got %s (summary: %s)", report.Verdict, report.Summary)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "generation_compare_removed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected generation_compare_removed finding, findings: %+v", report.Findings)
	}
}

// Test 4: Health gate removed → block.
func TestHealthGateRemovedBlocks(t *testing.T) {
	diff := makeUnifiedDiff(
		[]string{
			" if !workflowHealthy {",
			"     return ErrBackendUnhealthy",
			" }",
		},
		nil,
	)
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictBlock {
		t.Errorf("expected block, got %s", report.Verdict)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "health_gate_removed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected health_gate_removed finding, findings: %+v", report.Findings)
	}
}

// Test 5: Atomicity added → allow.
func TestAtomicityAddedAllows(t *testing.T) {
	diff := makeUnifiedDiff(
		[]string{
			" putInstalled()",
			" promoteResult()",
			" deleteAction()",
		},
		[]string{
			" txn.Then(putInstalledOp, promoteResultOp, deleteActionOp)",
		},
	)
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictAllow {
		t.Errorf("expected allow, got %s (summary: %s)", report.Verdict, report.Summary)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "state_transition_atomicity_added" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected state_transition_atomicity_added finding, findings: %+v", report.Findings)
	}
}

// Test 6: Transaction split → block.
func TestTransactionSplitBlocks(t *testing.T) {
	diff := makeUnifiedDiff(
		[]string{
			" txn.Then(putInstalledOp, promoteResultOp)",
		},
		[]string{
			" putInstalled()",
			" promoteResult()",
		},
	)
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictBlock {
		t.Errorf("expected block, got %s (summary: %s)", report.Verdict, report.Summary)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "state_transition_atomicity_removed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected state_transition_atomicity_removed finding, findings: %+v", report.Findings)
	}
}

// Test 7: Fallback promoted to authority → block.
func TestFallbackPromotedToAuthorityBlocks(t *testing.T) {
	diff := makeUnifiedDiff(nil, []string{
		" setInstalled(installedStateFallback)",
	})
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictBlock {
		t.Errorf("expected block, got %s (summary: %s)", report.Verdict, report.Summary)
	}
	found := false
	for _, f := range report.Findings {
		if f.Kind == "fallback_promoted_to_authority" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected fallback_promoted_to_authority finding, findings: %+v", report.Findings)
	}
}

// Test 8: Stale report detection.
func TestStaleReportDetection(t *testing.T) {
	diffA := makeUnifiedDiff(nil, []string{" x := 1"})
	diffB := makeUnifiedDiff(nil, []string{" y := 2"})

	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diffA,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !semanticdiff.IsReportStale(report, diffB) {
		t.Error("expected report to be stale with different diff")
	}
	if semanticdiff.IsReportStale(report, diffA) {
		t.Error("expected report to not be stale with same diff")
	}
}

// Test 9: Safe refactor → allow with no findings.
func TestSafeRefactorAllows(t *testing.T) {
	diff := makeUnifiedDiff(
		[]string{" oldName := getState()"},
		[]string{" newName := getState()"},
	)
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != semanticdiff.VerdictAllow {
		t.Errorf("expected allow, got %s (findings: %+v)", report.Verdict, report.Findings)
	}
	if len(report.Findings) != 0 {
		t.Errorf("expected no findings, got %d: %+v", len(report.Findings), report.Findings)
	}
}

// Test 10: Storage round-trip — store and reload.
func TestStorageRoundtrip(t *testing.T) {
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("open memory graph: %v", err)
	}
	defer g.Close()

	diff := makeUnifiedDiff(nil, []string{
		" installed.State = desired.State",
	})
	report, err := semanticdiff.InterpretSemanticDiff(context.Background(), semanticdiff.SemanticDiffRequest{
		DiffText:   diff,
		DiffSource: "roundtrip-test",
	})
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}

	st := semanticdiff.NewStore(g)
	ctx := context.Background()
	if err := st.StoreReport(ctx, report); err != nil {
		t.Fatalf("store report: %v", err)
	}

	loaded, err := st.GetReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if loaded.ID != report.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, report.ID)
	}
	if loaded.Verdict != report.Verdict {
		t.Errorf("verdict mismatch: got %s, want %s", loaded.Verdict, report.Verdict)
	}
	if len(loaded.Findings) != len(report.Findings) {
		t.Errorf("findings count mismatch: got %d, want %d", len(loaded.Findings), len(report.Findings))
	}
}
