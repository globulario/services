package main

import (
	"testing"
)

// TestSelfReview_TestResultsPassed_StrictVerified verifies that when the test
// suite passes and no failures/skips are reported, status upgrades to strict_verified.
func TestSelfReview_TestResultsPassed_StrictVerified(t *testing.T) {
	tr := &testResultsInput{
		Command: "go test ./awareness/...",
		Passed:  true,
	}
	status, note := upgradeWithTestResults("tests_found", "", []string{"TestFoo_Bar", "TestFoo_Baz"}, tr)
	if status != "strict_verified" {
		t.Errorf("expected strict_verified, got %q (note: %s)", status, note)
	}
}

// TestSelfReview_TestResultsFailed_NotClosed verifies that a failing test
// (listed in FailedTests) produces tests_failed.
func TestSelfReview_TestResultsFailed_NotClosed(t *testing.T) {
	tr := &testResultsInput{
		Command:     "go test ./awareness/...",
		Passed:      false,
		FailedTests: []string{"TestFoo_Bar"},
	}
	status, _ := upgradeWithTestResults("tests_found", "", []string{"TestFoo_Bar", "TestFoo_Baz"}, tr)
	if status != "tests_failed" {
		t.Errorf("expected tests_failed, got %q", status)
	}
}

// TestSelfReview_TestContainsSkip_NotStrictVerified verifies that a skipped
// required test produces tests_found_but_skipped, not strict_verified.
func TestSelfReview_TestContainsSkip_NotStrictVerified(t *testing.T) {
	tr := &testResultsInput{
		Command:      "go test ./awareness/...",
		Passed:       true,
		SkippedTests: []string{"TestFoo_Bar"},
	}
	status, _ := upgradeWithTestResults("tests_found", "", []string{"TestFoo_Bar", "TestFoo_Baz"}, tr)
	if status != "tests_found_but_skipped" {
		t.Errorf("expected tests_found_but_skipped, got %q", status)
	}
}

// TestSelfReview_NoTestResults_MetadataOnly verifies that when no test_results
// are provided (nil), the base verification status is returned unchanged.
func TestSelfReview_NoTestResults_MetadataOnly(t *testing.T) {
	status, _ := upgradeWithTestResults("tests_found", "found 3 functions", []string{"TestFoo_Bar"}, nil)
	if status != "tests_found" {
		t.Errorf("expected tests_found unchanged, got %q", status)
	}
}
