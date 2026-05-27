package intentaudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendHistory_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-history.jsonl")

	report := &AuditReport{
		RunID:     "audit-test-001",
		Timestamp: time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "security.rbac", Title: "RBAC", Status: StatusPass},
			{IntentID: "state.etcd_only", Title: "etcd only", Status: StatusCandidateViolation},
		},
		Summary: AuditSummary{Total: 2, Pass: 1, CandidateViolation: 1},
	}

	err := AppendHistory(path, report, nil)
	if err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var rec HistoryRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if rec.RunID != "audit-test-001" {
		t.Errorf("run_id = %q, want audit-test-001", rec.RunID)
	}
	if rec.IntentsAudited != 2 {
		t.Errorf("intents_audited = %d, want 2", rec.IntentsAudited)
	}
	if rec.Summary.CandidateViolation != 1 {
		t.Errorf("summary.candidate_violation = %d, want 1", rec.Summary.CandidateViolation)
	}
	if len(rec.TopViolations) != 1 || rec.TopViolations[0] != "state.etcd_only" {
		t.Errorf("top_violations = %v, want [state.etcd_only]", rec.TopViolations)
	}
}

func TestAppendHistory_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-history.jsonl")

	report1 := &AuditReport{
		RunID:     "audit-test-001",
		Timestamp: time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "security.rbac", Title: "RBAC", Status: StatusPass},
		},
		Summary: AuditSummary{Total: 1, Pass: 1},
	}
	report2 := &AuditReport{
		RunID:     "audit-test-002",
		Timestamp: time.Date(2026, 5, 26, 13, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "security.rbac", Title: "RBAC", Status: StatusCandidateViolation},
		},
		Summary: AuditSummary{Total: 1, CandidateViolation: 1},
	}

	if err := AppendHistory(path, report1, nil); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := AppendHistory(path, report2, nil); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var rec1, rec2 HistoryRecord
	json.Unmarshal([]byte(lines[0]), &rec1)
	json.Unmarshal([]byte(lines[1]), &rec2)

	if rec1.RunID != "audit-test-001" {
		t.Errorf("line 1 run_id = %q, want audit-test-001", rec1.RunID)
	}
	if rec2.RunID != "audit-test-002" {
		t.Errorf("line 2 run_id = %q, want audit-test-002", rec2.RunID)
	}

	// Second append should detect regression (new violation).
	if rec2.Regressions == nil {
		t.Fatal("expected regressions on second record")
	}
	if len(rec2.Regressions.NewViolations) != 1 || rec2.Regressions.NewViolations[0] != "security.rbac" {
		t.Errorf("new_violations = %v, want [security.rbac]", rec2.Regressions.NewViolations)
	}
}

func TestReadLastHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-history.jsonl")

	// Empty file returns nil, nil.
	os.WriteFile(path, []byte(""), 0o644)
	rec, err := ReadLastHistory(path)
	if err != nil {
		t.Fatalf("empty file: %v", err)
	}
	if rec != nil {
		t.Fatal("expected nil for empty file")
	}

	// Write two lines, should return second.
	line1, _ := json.Marshal(HistoryRecord{RunID: "r1", IntentsAudited: 1})
	line2, _ := json.Marshal(HistoryRecord{RunID: "r2", IntentsAudited: 2})
	os.WriteFile(path, append(append(line1, '\n'), append(line2, '\n')...), 0o644)

	rec, err = ReadLastHistory(path)
	if err != nil {
		t.Fatalf("read last: %v", err)
	}
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.RunID != "r2" {
		t.Errorf("run_id = %q, want r2", rec.RunID)
	}
	if rec.IntentsAudited != 2 {
		t.Errorf("intents_audited = %d, want 2", rec.IntentsAudited)
	}

	// Non-existent file returns error.
	_, err = ReadLastHistory(filepath.Join(dir, "nope.jsonl"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestComputeRegressions(t *testing.T) {
	prev := &HistoryRecord{
		TopViolations: []string{"state.etcd_only", "security.no_localhost"},
	}

	current := &AuditReport{
		Results: []IntentResult{
			{IntentID: "state.etcd_only", Status: StatusCandidateViolation},
			{IntentID: "pki.mtls_required", Status: StatusCandidateViolation},
			{IntentID: "security.rbac", Status: StatusPass},
		},
	}

	reg := ComputeRegressions(prev, current)

	// pki.mtls_required is new.
	if !containsStr(reg.NewViolations, "pki.mtls_required") {
		t.Errorf("expected pki.mtls_required in new_violations, got %v", reg.NewViolations)
	}
	// state.etcd_only should NOT be new (was in prev).
	if containsStr(reg.NewViolations, "state.etcd_only") {
		t.Errorf("state.etcd_only should not be in new_violations")
	}

	// security.no_localhost was resolved.
	if !containsStr(reg.ResolvedViolations, "security.no_localhost") {
		t.Errorf("expected security.no_localhost in resolved_violations, got %v", reg.ResolvedViolations)
	}
	// state.etcd_only should NOT be resolved (still violating).
	if containsStr(reg.ResolvedViolations, "state.etcd_only") {
		t.Errorf("state.etcd_only should not be in resolved_violations")
	}
}

func TestRegressionDetection_SameScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-history.jsonl")

	scope := []string{"security.rbac", "state.etcd_only"}

	// First scoped run — no violations.
	report1 := &AuditReport{
		RunID:     "audit-scope-001",
		Timestamp: time.Date(2026, 5, 26, 14, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "security.rbac", Title: "RBAC", Status: StatusPass},
			{IntentID: "state.etcd_only", Title: "etcd", Status: StatusPass},
		},
		Summary: AuditSummary{Total: 2, Pass: 2},
	}
	if err := AppendHistory(path, report1, scope); err != nil {
		t.Fatalf("first append: %v", err)
	}

	// Second scoped run — same scope, new violation.
	report2 := &AuditReport{
		RunID:     "audit-scope-002",
		Timestamp: time.Date(2026, 5, 26, 15, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "security.rbac", Title: "RBAC", Status: StatusCandidateViolation},
			{IntentID: "state.etcd_only", Title: "etcd", Status: StatusPass},
		},
		Summary: AuditSummary{Total: 2, Pass: 1, CandidateViolation: 1},
	}
	if err := AppendHistory(path, report2, scope); err != nil {
		t.Fatalf("second append: %v", err)
	}

	// Read back the second record and verify regression was detected.
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var rec2 HistoryRecord
	if err := json.Unmarshal([]byte(lines[1]), &rec2); err != nil {
		t.Fatalf("unmarshal second record: %v", err)
	}

	if rec2.Regressions == nil {
		t.Fatal("expected regressions on second scoped record with same scope")
	}
	if !containsStr(rec2.Regressions.NewViolations, "security.rbac") {
		t.Errorf("expected security.rbac in new_violations, got %v", rec2.Regressions.NewViolations)
	}
}

func TestRegressionDetection_DifferentScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-history.jsonl")

	// First run — full audit (no scope), with a violation.
	report1 := &AuditReport{
		RunID:     "audit-full-001",
		Timestamp: time.Date(2026, 5, 26, 14, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "security.rbac", Title: "RBAC", Status: StatusCandidateViolation},
			{IntentID: "state.etcd_only", Title: "etcd", Status: StatusPass},
			{IntentID: "pki.mtls_required", Title: "mTLS", Status: StatusPass},
		},
		Summary: AuditSummary{Total: 3, Pass: 2, CandidateViolation: 1},
	}
	if err := AppendHistory(path, report1, nil); err != nil {
		t.Fatalf("full audit append: %v", err)
	}

	// Second run — scoped audit (different scope), no violation.
	scope := []string{"pki.mtls_required"}
	report2 := &AuditReport{
		RunID:     "audit-scoped-002",
		Timestamp: time.Date(2026, 5, 26, 15, 0, 0, 0, time.UTC),
		Results: []IntentResult{
			{IntentID: "pki.mtls_required", Title: "mTLS", Status: StatusPass},
		},
		Summary: AuditSummary{Total: 1, Pass: 1},
	}
	if err := AppendHistory(path, report2, scope); err != nil {
		t.Fatalf("scoped audit append: %v", err)
	}

	// Read back the second record — should have NO regressions because
	// the scoped audit has no prior run with the same scope to compare against.
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var rec2 HistoryRecord
	if err := json.Unmarshal([]byte(lines[1]), &rec2); err != nil {
		t.Fatalf("unmarshal second record: %v", err)
	}

	if rec2.Regressions != nil {
		t.Errorf("expected no regressions for different-scope run, got new=%v resolved=%v",
			rec2.Regressions.NewViolations, rec2.Regressions.ResolvedViolations)
	}
}

// containsStr is defined in changerisk_test.go
