package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/awareness/enforce"
)

func TestChooseScaffoldTargetPathPrefersGolangPaths(t *testing.T) {
	entry := requiredTestBacklogEntry{
		Test: "TestExample",
		Suggestions: []string{
			"docs/awareness/some_test.go",
			"golang/node_agent/reconcile.go",
		},
	}
	got := chooseScaffoldTargetPath(entry)
	want := "golang/node_agent/awareness_required_tests_scaffold_test.go"
	if got != want {
		t.Fatalf("unexpected target path: got %q want %q", got, want)
	}
}

func TestChooseScaffoldTargetPathUsesFallbackWhenNoGolangSuggestion(t *testing.T) {
	entry := requiredTestBacklogEntry{
		Test:        "TestExample",
		Suggestions: []string{"docs/awareness/decision.md"},
	}
	got := chooseScaffoldTargetPath(entry)
	want := "golang/awareness/awareness_required_tests_scaffold_test.go"
	if got != want {
		t.Fatalf("unexpected fallback target: got %q want %q", got, want)
	}
}

func TestWriteRequiredTestStubIsIdempotent(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "golang", "pkg", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatalf("seed package file: %v", err)
	}

	entry := requiredTestBacklogEntry{
		Test: "TestSampleRequired",
		Refs: []string{"invariant:sample"},
	}
	rel := "golang/pkg/sample/awareness_required_tests_scaffold_test.go"

	status, err := writeRequiredTestStub(repoRoot, rel, entry)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if status != "written" {
		t.Fatalf("expected first write status=written, got %q", status)
	}

	status2, err := writeRequiredTestStub(repoRoot, rel, entry)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if status2 != "exists" {
		t.Fatalf("expected second write status=exists, got %q", status2)
	}

	b, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	body := string(b)
	if !strings.Contains(body, "package sample") {
		t.Fatalf("expected generated test file to use package sample, got:\n%s", body)
	}
	if strings.Count(body, "func TestSampleRequired(") != 1 {
		t.Fatalf("expected exactly one generated test function, got:\n%s", body)
	}
}

func TestExceedsRequiredTestNoPathThreshold(t *testing.T) {
	groups := []enforce.FindingGroup{
		{Code: "REQUIRED_TEST_NO_PATH", Count: 2},
		{Code: "OTHER_WARNING", Count: 5},
	}
	if count, exceeded := exceedsRequiredTestNoPathThreshold(groups, -1); exceeded || count != 0 {
		t.Fatalf("expected disabled threshold to never exceed, got count=%d exceeded=%v", count, exceeded)
	}
	if count, exceeded := exceedsRequiredTestNoPathThreshold(groups, 2); exceeded || count != 2 {
		t.Fatalf("expected equal threshold to pass, got count=%d exceeded=%v", count, exceeded)
	}
	if count, exceeded := exceedsRequiredTestNoPathThreshold(groups, 1); !exceeded || count != 2 {
		t.Fatalf("expected threshold exceed, got count=%d exceeded=%v", count, exceeded)
	}
}

func TestWarningGroupCountMissingCode(t *testing.T) {
	groups := []enforce.FindingGroup{
		{Code: "A", Count: 1},
		{Code: "B", Count: 3},
	}
	if got := warningGroupCount(groups, "REQUIRED_TEST_NO_PATH"); got != 0 {
		t.Fatalf("expected missing code count 0, got %d", got)
	}
}

// TestAwarenessAudit_MaxRequiredTestNoPathZero verifies threshold logic for
// both required-test-no-path and todo-scaffold-skips gates.
func TestAwarenessAudit_MaxRequiredTestNoPathZero(t *testing.T) {
	noPathGroups := []enforce.FindingGroup{
		{Code: "REQUIRED_TEST_NO_PATH", Count: 3},
	}

	// max=-1 → never exceeded.
	_, exceeded := exceedsRequiredTestNoPathThreshold(noPathGroups, -1)
	if exceeded {
		t.Error("expected not exceeded when max=-1")
	}

	// max=0 → exceeded when count > 0.
	count, exceeded := exceedsRequiredTestNoPathThreshold(noPathGroups, 0)
	if !exceeded {
		t.Errorf("expected exceeded when max=0 and count=%d", count)
	}

	// max=5 → not exceeded when count=3.
	_, exceeded = exceedsRequiredTestNoPathThreshold(noPathGroups, 5)
	if exceeded {
		t.Error("expected not exceeded when max=5 and count=3")
	}
}

// TestAwarenessAudit_MaxTodoScaffoldSkipsZero verifies the scaffold skip threshold.
func TestAwarenessAudit_MaxTodoScaffoldSkipsZero(t *testing.T) {
	scaffoldGroups := []enforce.FindingGroup{
		{Code: enforce.CodeScaffoldTodoSkip, Count: 5},
	}

	// max=-1 → never exceeded.
	_, exceeded := exceedsTodoScaffoldSkipsThreshold(scaffoldGroups, -1)
	if exceeded {
		t.Error("expected not exceeded when max=-1")
	}

	// max=0 → exceeded when count > 0.
	count, exceeded := exceedsTodoScaffoldSkipsThreshold(scaffoldGroups, 0)
	if !exceeded {
		t.Errorf("expected exceeded when max=0 and count=%d", count)
	}
}
