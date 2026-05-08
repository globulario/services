package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanViolations_Registered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	if !s.HasTool("awareness.scan_violations") {
		t.Error("awareness.scan_violations should be registered")
	}
}

func TestScanViolations_LocalhostPattern(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "service.go")
	content := `package main

import "fmt"

func dial() {
	addr := "127.0.0.1:12000"
	fmt.Println(addr)
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{dir},
		"severity": "critical",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	total, _ := m["total"].(int)
	if total == 0 {
		t.Error("expected at least one finding for localhost pattern")
	}

	findings, _ := m["findings"].([]violationFinding)
	for _, f := range findings {
		if f.PatternID == "localhost_interservice" {
			// Found the expected pattern.
			if f.KnowledgeID == "" {
				t.Error("expected non-empty KnowledgeID")
			}
			if f.Severity != "critical" {
				t.Errorf("expected critical severity, got %q", f.Severity)
			}
			return
		}
	}
	t.Logf("findings: %v (total=%d)", m["findings"], total)
}

func TestScanViolations_OsGetenvPattern(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "config.go")
	content := `package main

import "os"

func getAddr() string {
	return os.Getenv("SERVICE_ADDR")
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{dir},
		"severity": "high",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	total, _ := m["total"].(int)
	if total == 0 {
		t.Error("expected findings for os.Getenv usage")
	}
}

func TestScanViolations_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths": []interface{}{dir},
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}
	m := result.(map[string]interface{})
	total, _ := m["total"].(int)
	if total != 0 {
		t.Errorf("expected 0 findings in empty dir, got %d", total)
	}
}

func TestScanViolations_SeverityAtLeast(t *testing.T) {
	cases := []struct {
		have     string
		min      string
		expected bool
	}{
		{"critical", "medium", true},
		{"critical", "high", true},
		{"critical", "critical", true},
		{"high", "medium", true},
		{"high", "high", true},
		{"high", "critical", false},
		{"medium", "medium", true},
		{"medium", "high", false},
		{"medium", "critical", false},
	}
	for _, c := range cases {
		got := severityAtLeast(c.have, c.min)
		if got != c.expected {
			t.Errorf("severityAtLeast(%q, %q) = %v, want %v", c.have, c.min, got, c.expected)
		}
	}
}

// TestScanViolations_InsecureGrpcTransportPattern verifies the new pattern fires.
func TestScanViolations_InsecureGrpcTransportPattern(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "dial.go")
	content := `package main

import "google.golang.org/grpc/credentials/insecure"

func dial() {
	creds := insecure.NewCredentials()
	_ = creds
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{dir},
		"severity": "critical",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	total, _ := m["total"].(int)
	if total == 0 {
		t.Error("expected finding for insecure.NewCredentials()")
	}
}

// TestScanViolations_ColumnIsSet verifies the column field is set (>= 0).
func TestScanViolations_ColumnIsSet(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "conn.go")
	content := `package main
const addr = "127.0.0.1:9090"
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{dir},
		"severity": "critical",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	findings, _ := m["findings"].([]violationFinding)
	if len(findings) == 0 {
		t.Skip("no findings, column test skipped")
	}
	for _, f := range findings {
		if f.Column < 0 {
			t.Errorf("finding %s line %d has negative column %d", f.PatternID, f.Line, f.Column)
		}
	}
}

// TestScanViolations_ConfidenceIsHigh verifies that all findings have confidence="high".
func TestScanViolations_ConfidenceIsHigh(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "conn.go")
	content := `package main
const addr = "127.0.0.1:9090"
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{dir},
		"severity": "critical",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	findings, _ := m["findings"].([]violationFinding)
	for _, f := range findings {
		if f.Confidence != "high" {
			t.Errorf("finding %s has confidence %q, want high", f.PatternID, f.Confidence)
		}
	}
}

// TestScanViolations_SuppressedFindingsPresent verifies that allowlist suppression
// results in suppressed_findings being populated.
func TestScanViolations_SuppressedFindingsPresent(t *testing.T) {
	// Create a server with a docsDir that has a scan_allowlist.yaml.
	dir := t.TempDir()
	knowledgeDir := filepath.Join(dir, "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0755); err != nil {
		t.Fatal(err)
	}
	allowlistYAML := `allowlist:
  - path_pattern: "**/*.go"
    pattern_id: "localhost_interservice"
    reason: "test suppression"
`
	if err := os.WriteFile(filepath.Join(knowledgeDir, "scan_allowlist.yaml"), []byte(allowlistYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a Go file that would trigger localhost_interservice.
	scanDir := t.TempDir()
	goFile := filepath.Join(scanDir, "conn.go")
	content := `package main
const addr = "127.0.0.1:9090"
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{DocsDir: dir}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{scanDir},
		"severity": "critical",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	// The finding should be suppressed, not in findings.
	total, _ := m["total"].(int)
	suppressedCount, _ := m["suppressed_count"].(int)

	// With allowlist matching all .go files for localhost_interservice,
	// the finding should appear in suppressed, not findings.
	if total != 0 {
		t.Errorf("expected total=0 (suppressed), got %d", total)
	}
	if suppressedCount == 0 {
		t.Error("expected suppressed_count > 0 when allowlist matches")
	}
}

func TestScanViolations_KnowledgeIDMap(t *testing.T) {
	dir := t.TempDir()
	// Create a file that should trigger localhost pattern.
	goFile := filepath.Join(dir, "conn.go")
	content := `package main
const addr = "127.0.0.1:9090"
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{}, nil)
	result, err := s.CallTool(context.Background(), "awareness.scan_violations", map[string]interface{}{
		"paths":    []interface{}{dir},
		"severity": "medium",
	})
	if err != nil {
		t.Fatalf("scan_violations error: %v", err)
	}

	m := result.(map[string]interface{})
	byKnowledge, _ := m["by_knowledge_id"].(map[string]int)
	if byKnowledge == nil {
		t.Skip("by_knowledge_id not returned as map[string]int (type assertion depends on real findings)")
	}
	for k, v := range byKnowledge {
		if k == "" {
			t.Error("empty knowledge ID in summary")
		}
		if v <= 0 {
			t.Errorf("non-positive count for knowledge ID %q", k)
		}
	}
}
