package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSelfReview_StrictVerified_FromTestResultsFile verifies that when a CI
// test results file reports passed=true with no failures/skips, self_review
// upgrades verification to strict_verified.
func TestSelfReview_StrictVerified_FromTestResultsFile(t *testing.T) {
	dir := t.TempDir()
	resultsPath := filepath.Join(dir, "test-results.json")

	f := ciTestResultsFile{
		Command:      "go test ./awareness/...",
		Passed:       true,
		Packages:     5,
		Tests:        []ciTestResult{{Name: "TestFoo_Bar", Package: "mcp", Status: "passed", DurationMs: 10}},
		FailedTests:  []string{},
		SkippedTests: []string{},
	}
	data, _ := json.Marshal(f)
	if err := os.WriteFile(resultsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	tr, err := loadTestResultsFromFile(resultsPath)
	if err != nil {
		t.Fatalf("loadTestResultsFromFile: %v", err)
	}
	status, _ := upgradeWithTestResults("tests_found", "", []string{"TestFoo_Bar"}, tr)
	if status != "strict_verified" {
		t.Errorf("expected strict_verified, got %q", status)
	}
}

// TestSelfReview_FailedRequiredTest_NotStrictVerified verifies that a failing
// required test in the CI file results in tests_failed, not strict_verified.
func TestSelfReview_FailedRequiredTest_NotStrictVerified(t *testing.T) {
	dir := t.TempDir()
	resultsPath := filepath.Join(dir, "test-results.json")

	f := ciTestResultsFile{
		Command:     "go test ./awareness/...",
		Passed:      false,
		FailedTests: []string{"TestFoo_Bar"},
	}
	data, _ := json.Marshal(f)
	os.WriteFile(resultsPath, data, 0o644)

	tr, err := loadTestResultsFromFile(resultsPath)
	if err != nil {
		t.Fatal(err)
	}
	status, _ := upgradeWithTestResults("tests_found", "", []string{"TestFoo_Bar"}, tr)
	if status != "tests_failed" {
		t.Errorf("expected tests_failed, got %q", status)
	}
}

// TestSelfReview_SkippedRequiredTest_NotStrictVerified verifies that a skipped
// required test in the CI file results in tests_found_but_skipped.
func TestSelfReview_SkippedRequiredTest_NotStrictVerified(t *testing.T) {
	dir := t.TempDir()
	resultsPath := filepath.Join(dir, "test-results.json")

	// Use Tests[] entries so the loader derives skipped from status field.
	f := ciTestResultsFile{
		Command:      "go test ./awareness/...",
		Passed:       true,
		Tests:        []ciTestResult{{Name: "TestFoo_Bar", Package: "mcp", Status: "skipped"}},
		FailedTests:  []string{},
		SkippedTests: []string{},
	}
	data, _ := json.Marshal(f)
	os.WriteFile(resultsPath, data, 0o644)

	tr, err := loadTestResultsFromFile(resultsPath)
	if err != nil {
		t.Fatal(err)
	}
	status, _ := upgradeWithTestResults("tests_found", "", []string{"TestFoo_Bar"}, tr)
	if status != "tests_found_but_skipped" {
		t.Errorf("expected tests_found_but_skipped, got %q", status)
	}
}

// TestSelfReview_MissingTestResults_MetadataOnly verifies that when the test
// results file does not exist, loadTestResultsFromFile returns an error and
// self_review falls back to metadata-only verification.
func TestSelfReview_MissingTestResults_MetadataOnly(t *testing.T) {
	_, err := loadTestResultsFromFile("/nonexistent/test-results.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// TestTestResultsParser_GoTestJSON verifies that the ciTestResultsFile schema
// round-trips correctly through JSON marshal/unmarshal.
func TestTestResultsParser_GoTestJSON(t *testing.T) {
	f := ciTestResultsFile{
		Command:    "go test -json ./awareness/...",
		Passed:     true,
		Packages:   21,
		Tests: []ciTestResult{
			{Name: "TestFoo", Package: "mcp", Status: "passed", DurationMs: 5},
			{Name: "TestBar", Package: "mcp", Status: "skipped", DurationMs: 1},
		},
		FailedTests:  []string{},
		SkippedTests: []string{"TestBar"},
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var f2 ciTestResultsFile
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f2.Command != f.Command {
		t.Errorf("command mismatch: %q vs %q", f2.Command, f.Command)
	}
	if f2.Packages != 21 {
		t.Errorf("packages mismatch: %d", f2.Packages)
	}
	if len(f2.SkippedTests) != 1 || f2.SkippedTests[0] != "TestBar" {
		t.Errorf("skipped tests mismatch: %v", f2.SkippedTests)
	}
}
