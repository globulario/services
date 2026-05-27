package intentaudit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

// HistoryRecord is one JSONL line appended after each audit run.
type HistoryRecord struct {
	Timestamp      time.Time          `json:"timestamp"`
	RunID          string             `json:"run_id"`
	GitCommit      string             `json:"git_commit,omitempty"`
	IntentsAudited int                `json:"intents_audited"`
	Summary        AuditSummary       `json:"summary"`
	ScopeIDs       []string           `json:"scope_ids,omitempty"`
	TopViolations  []string           `json:"top_violations,omitempty"`
	TopMissing     []string           `json:"top_missing_tests,omitempty"`
	Regressions    *RegressionSummary `json:"regressions,omitempty"`
}

// RegressionSummary captures changes between consecutive audit runs.
type RegressionSummary struct {
	NewViolations      []string `json:"new_violations,omitempty"`
	ResolvedViolations []string `json:"resolved_violations,omitempty"`
}

// AppendHistory appends one JSONL line to the history file at path.
// The file is created if it does not exist.
func AppendHistory(path string, report *AuditReport, scopeIDs []string) error {
	rec := HistoryRecord{
		Timestamp:      report.Timestamp,
		RunID:          report.RunID,
		GitCommit:      gitCommitHash(),
		IntentsAudited: len(report.Results),
		Summary:        report.Summary,
		ScopeIDs:       scopeIDs,
	}

	// Collect top violations and missing tests (intent IDs only).
	for _, r := range report.Results {
		if r.Status == StatusCandidateViolation {
			rec.TopViolations = append(rec.TopViolations, r.IntentID)
		}
		if r.Status == StatusMissingTest {
			rec.TopMissing = append(rec.TopMissing, r.IntentID)
		}
	}

	// Compute regressions against previous run with matching scope.
	var prev *HistoryRecord
	var prevErr error
	if len(scopeIDs) > 0 {
		prev, prevErr = ReadLastHistoryWithScope(path, scopeIDs)
	} else {
		prev, prevErr = ReadLastHistory(path)
	}
	if prevErr == nil && prev != nil {
		reg := ComputeRegressions(prev, report)
		if len(reg.NewViolations) > 0 || len(reg.ResolvedViolations) > 0 {
			rec.Regressions = reg
		}
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal history record: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write history record: %w", err)
	}
	return nil
}

// ReadLastHistory reads the last non-empty line from a JSONL history file.
// Returns (nil, nil) if the file is empty. Returns an error if the file
// cannot be opened or the last line cannot be parsed.
func ReadLastHistory(path string) (*HistoryRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lastLine string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lastLine = line
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan history file: %w", err)
	}
	if lastLine == "" {
		return nil, nil
	}

	var rec HistoryRecord
	if err := json.Unmarshal([]byte(lastLine), &rec); err != nil {
		return nil, fmt.Errorf("parse last history record: %w", err)
	}
	return &rec, nil
}

// ReadLastHistoryWithScope reads the last JSONL line whose scope_ids match
// the given set (order-independent). Returns (nil, nil) if no matching entry
// exists. This ensures scoped audits only regress against prior runs with the
// same scope, preventing false regressions when a scoped run is compared
// against a full audit.
func ReadLastHistoryWithScope(path string, scopeIDs []string) (*HistoryRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var match *HistoryRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec HistoryRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if scopeMatches(rec.ScopeIDs, scopeIDs) {
			copied := rec
			match = &copied
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan history file: %w", err)
	}
	return match, nil
}

// scopeMatches returns true if both slices contain the same elements
// (order-independent). Two nil/empty slices match each other.
func scopeMatches(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			return false
		}
	}
	return true
}

// ComputeRegressions compares the previous history record against the current
// audit report and returns new/resolved violation intent IDs.
func ComputeRegressions(prev *HistoryRecord, current *AuditReport) *RegressionSummary {
	prevViolations := make(map[string]bool, len(prev.TopViolations))
	for _, id := range prev.TopViolations {
		prevViolations[id] = true
	}

	curViolations := make(map[string]bool)
	for _, r := range current.Results {
		if r.Status == StatusCandidateViolation {
			curViolations[r.IntentID] = true
		}
	}

	reg := &RegressionSummary{}

	// New violations: in current but not in previous.
	for id := range curViolations {
		if !prevViolations[id] {
			reg.NewViolations = append(reg.NewViolations, id)
		}
	}

	// Resolved violations: in previous but not in current.
	for id := range prevViolations {
		if !curViolations[id] {
			reg.ResolvedViolations = append(reg.ResolvedViolations, id)
		}
	}

	return reg
}

// NewRunID generates a unique run ID for history records.
func NewRunID() string {
	return fmt.Sprintf("audit-%s", uuid.New().String()[:8])
}

// gitCommitHash returns the current HEAD commit hash, or "" on error.
func gitCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
