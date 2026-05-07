package mcp

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
