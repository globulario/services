package intentaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func writeYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDir_BasicNode(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "test.yaml", `
id: test.basic
title: Basic test intent
intent: Something must be true.
status: seed
`)
	nodes, errs := LoadDir(dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes["test.basic"]
	if n == nil {
		t.Fatal("missing test.basic node")
	}
	if n.Title != "Basic test intent" {
		t.Errorf("title = %q, want %q", n.Title, "Basic test intent")
	}
}

func TestLoadDir_WithNewMetadata(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "test.yaml", `
id: test.meta
title: Node with audit metadata
intent: Must have tests and patterns.
required_tests:
  - pkg:TestFoo
  - pkg:TestBar
change_risk:
  - security_sensitive
violation_patterns:
  - "InsecureSkipVerify"
exceptions:
  - name: bootstrap_exception
    description: TLS bypass during bootstrap
    bounded: true
    permanent: false
status: seed
`)
	nodes, errs := LoadDir(dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	n := nodes["test.meta"]
	if n == nil {
		t.Fatal("missing test.meta")
	}
	if len(n.RequiredTests) != 2 {
		t.Errorf("required_tests: got %d, want 2", len(n.RequiredTests))
	}
	if len(n.ChangeRisk) != 1 || n.ChangeRisk[0] != "security_sensitive" {
		t.Errorf("change_risk: got %v", n.ChangeRisk)
	}
	if len(n.ViolationPatterns) != 1 {
		t.Errorf("violation_patterns: got %d, want 1", len(n.ViolationPatterns))
	}
	if len(n.Exceptions) != 1 || n.Exceptions[0].Name != "bootstrap_exception" {
		t.Errorf("exceptions: got %v", n.Exceptions)
	}
}

func TestLoadDir_UnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "test.yaml", `
id: test.unknown
title: Node with unknown fields
intent: Should not fail.
some_future_field: value
another_new_thing:
  - item
status: seed
`)
	nodes, errs := LoadDir(dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if nodes["test.unknown"] == nil {
		t.Fatal("missing test.unknown — unknown fields should be ignored")
	}
}

func TestLoadDir_MissingID(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yaml", `
title: No ID node
intent: Missing id field.
status: seed
`)
	nodes, errs := LoadDir(dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing id, got %d", len(errs))
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestCheckRequiredTests_EmptyList(t *testing.T) {
	node := &Node{ID: "test.empty", RequiredTests: nil}
	findings := checkRequiredTests(node, "")
	if len(findings) != 1 || findings[0].Status != StatusTestCoverageGap {
		t.Errorf("expected TEST_COVERAGE_GAP, got %v", findings)
	}
}

func TestCheckRequiredTests_TestExists(t *testing.T) {
	// Create a temp dir with a test file containing a test function.
	dir := t.TempDir()
	testFile := filepath.Join(dir, "example_test.go")
	os.WriteFile(testFile, []byte("package foo\nfunc TestExampleCheck(t *testing.T) {}\n"), 0644)

	node := &Node{
		ID:            "test.exists",
		RequiredTests: []string{"foo:TestExampleCheck"},
	}
	findings := checkRequiredTests(node, dir)
	found := false
	for _, f := range findings {
		if f.Status == StatusPass {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PASS for existing test, got %v", findings)
	}
}

func TestCheckRequiredTests_TestMissing(t *testing.T) {
	dir := t.TempDir()
	// No test files — test should be missing.
	node := &Node{
		ID:            "test.missing",
		RequiredTests: []string{"foo:TestNonexistent"},
	}
	findings := checkRequiredTests(node, dir)
	found := false
	for _, f := range findings {
		if f.Status == StatusMissingTest {
			found = true
		}
	}
	if !found {
		t.Errorf("expected MISSING_REQUIRED_TEST, got %v", findings)
	}
}

func TestWorstStatus(t *testing.T) {
	tests := []struct {
		statuses []string
		want     string
	}{
		{[]string{StatusPass, StatusPass}, StatusPass},
		{[]string{StatusPass, StatusTestCoverageGap}, StatusTestCoverageGap},
		{[]string{StatusPass, StatusAcceptedException}, StatusAcceptedException},
		{[]string{StatusAcceptedException, StatusCandidateViolation}, StatusCandidateViolation},
		{[]string{StatusMissingTest, StatusCandidateViolation}, StatusCandidateViolation},
	}
	for _, tc := range tests {
		var findings []Finding
		for _, s := range tc.statuses {
			findings = append(findings, Finding{Status: s})
		}
		got := worstStatus(findings)
		if got != tc.want {
			t.Errorf("worstStatus(%v) = %q, want %q", tc.statuses, got, tc.want)
		}
	}
}

func TestExceptionMatching_FileList(t *testing.T) {
	exceptions := []Exception{
		{
			Name:        "bootstrap_tls_bypass",
			ID:          "exc-bootstrap",
			Description: "Bootstrap TLS bypass for initial CA fetch",
			Files:       []string{"security/tls.go", "security/bootstrap.go"},
		},
		{
			Name:        "reachability_probe",
			ID:          "exc-reachability",
			Description: "Reachability probe for connectivity test",
			Files:       []string{"globular_client/clients.go"},
		},
		{
			Name:        "loopback_minio",
			ID:          "exc-loopback",
			Description: "Loopback MinIO connections",
			Files:       []string{"repository/repository_server", "media/media_server", "file/file_server"},
		},
	}

	tests := []struct {
		file string
		want string
	}{
		{"security/tls.go", "exc-bootstrap"},
		{"security/bootstrap.go", "exc-bootstrap"},
		{"globular_client/clients.go", "exc-reachability"},
		{"repository/repository_server/server.go", "exc-loopback"},
		{"media/media_server/upload.go", "exc-loopback"},
		{"file/file_server/handler.go", "exc-loopback"},
		{"cluster_controller/controller.go", ""},  // no match
		{"dns/dns_server/server.go", ""},           // no match
	}

	for _, tc := range tests {
		got := matchesException(tc.file, 0, exceptions)
		if got != tc.want {
			t.Errorf("matchesException(%q) = %q, want %q", tc.file, got, tc.want)
		}
	}
}

func TestExceptionMatching_FileListCaseInsensitive(t *testing.T) {
	exceptions := []Exception{
		{
			Name:  "test_exc",
			ID:    "exc-case",
			Files: []string{"Security/TLS.go"},
		},
	}
	got := matchesException("security/tls.go", 0, exceptions)
	if got != "exc-case" {
		t.Errorf("case-insensitive file match failed: got %q, want %q", got, "exc-case")
	}
}

func TestExceptionMatching_EmptyFilesFallsBackToDescription(t *testing.T) {
	exceptions := []Exception{
		{
			Name:        "bootstrap_fallback",
			ID:          "exc-fallback",
			Description: "Bootstrap TLS bypass",
			Files:       nil, // no explicit files
		},
	}
	// Should match via description keyword fallback.
	got := matchesException("security/bootstrap.go", 0, exceptions)
	if got != "exc-fallback" {
		t.Errorf("description fallback failed: got %q, want %q", got, "exc-fallback")
	}
	// Should NOT match a non-bootstrap file.
	got = matchesException("cluster_controller/server.go", 0, exceptions)
	if got != "" {
		t.Errorf("non-bootstrap file should not match: got %q", got)
	}
}

func TestExtractTestName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"rbac/rbac_server:TestDenyOverridesSAAllow", "TestDenyOverridesSAAllow"},
		{"TestSimple", "TestSimple"},
		{"pkg/sub:TestDeep", "TestDeep"},
	}
	for _, tc := range tests {
		got := extractTestName(tc.ref)
		if got != tc.want {
			t.Errorf("extractTestName(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}
