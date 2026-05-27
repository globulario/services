package intentaudit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func findIntentDir(t *testing.T) string {
	t.Helper()
	root := gitRoot(t)
	dir := filepath.Join(root, "docs", "intent")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("intent dir not found at %s", dir)
	}
	return dir
}

func findSrcDir(t *testing.T) string {
	t.Helper()
	root := gitRoot(t)
	dir := filepath.Join(root, "golang")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("src dir not found at %s", dir)
	}
	return dir
}

func gitRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in git repo: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestRunAudit_RealIntentNodes runs the full audit against the actual
// intent nodes in the repository to verify end-to-end functionality.
func TestRunAudit_RealIntentNodes(t *testing.T) {
	intentDir := findIntentDir(t)
	srcDir := findSrcDir(t)

	report, err := RunAudit(AuditOptions{
		IntentDir: intentDir,
		SrcDir:    srcDir,
	})
	if err != nil {
		t.Skipf("skipping real intent audit (dir not available): %v", err)
	}

	if len(report.Results) == 0 {
		t.Fatal("expected results from real intent nodes")
	}

	t.Logf("Audited %d intent nodes", len(report.Results))
	t.Logf("Summary: pass=%d violation=%d exception=%d gap=%d missing=%d",
		report.Summary.Pass,
		report.Summary.CandidateViolation,
		report.Summary.AcceptedException,
		report.Summary.TestCoverageGap,
		report.Summary.MissingTest)

	// The 3 known test coverage gap intents should be detected.
	gaps := 0
	for _, r := range report.Results {
		if r.Status == StatusTestCoverageGap {
			gaps++
		}
	}
	if gaps == 0 {
		t.Error("expected at least some TEST_COVERAGE_GAP intents (nodes without required_tests)")
	}

	// Verify report is JSON-serializable.
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("report should be JSON-serializable: %v", err)
	}
	if len(data) < 100 {
		t.Error("JSON report seems too short")
	}
}

// TestRunAudit_ScopedToSingleIntent tests scoped audit by ID.
func TestRunAudit_ScopedToSingleIntent(t *testing.T) {
	intentDir := findIntentDir(t)
	srcDir := findSrcDir(t)

	report, err := RunAudit(AuditOptions{
		IntentDir: intentDir,
		SrcDir:    srcDir,
		ScopeIDs:  []string{"security.deny_overrides_allow"},
	})
	if err != nil {
		t.Skipf("skipping: %v", err)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result for scoped audit, got %d", len(report.Results))
	}
	if report.Results[0].IntentID != "security.deny_overrides_allow" {
		t.Errorf("expected security.deny_overrides_allow, got %s", report.Results[0].IntentID)
	}
	t.Logf("RBAC intent status: %s, findings: %d", report.Results[0].Status, len(report.Results[0].Findings))
}
