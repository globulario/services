package rules

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Persistent heal audit trail
//
// Append-only JSONL file at /var/lib/globular/clusterdoctor/heal-audit.jsonl.
// One line per action (executed, skipped, or failed). Written by the leader
// only. Survives process restarts. Read via GetHealHistory RPC or CLI.
// ──────────────────────────────────────────────────────────────────────────────

const defaultAuditPath = "/var/lib/globular/clusterdoctor/heal-audit.jsonl"

// HealAuditRecord is one line in the JSONL audit file.
type HealAuditRecord struct {
	Timestamp   time.Time       `json:"ts"`
	CycleID     string          `json:"cycle_id"`
	InvariantID string          `json:"invariant_id"`
	EntityRef   string          `json:"entity_ref"`
	Node        string          `json:"node"`
	Package     string          `json:"package"`
	Disposition HealDisposition `json:"disposition"`
	Action      string          `json:"action"`
	Executed    bool            `json:"executed"`
	Verified    bool            `json:"verified"`
	Error       string          `json:"error,omitempty"`
}

// HealAuditStore handles append + read for the JSONL audit file.
type HealAuditStore struct {
	path string
	mu   sync.Mutex
}

// NewHealAuditStore creates an audit store at the given path.
// Creates the directory if it doesn't exist.
func NewHealAuditStore(path string) *HealAuditStore {
	if path == "" {
		path = defaultAuditPath
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return &HealAuditStore{path: path}
}

// Append writes one audit record as a single JSONL line.
func (s *HealAuditStore) Append(rec HealAuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	defer f.Close()
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal audit record: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	return nil
}

// AppendReport writes all results from a HealReport as individual audit records.
func (s *HealAuditStore) AppendReport(report HealReport) {
	cycleID := report.Timestamp.Format("20060102T150405")
	for _, r := range report.Results {
		if !r.Executed && r.Error == "" && r.Disposition != HealAuto {
			continue // skip observe/propose with no action — keep the file focused
		}
		node, pkg := parseEntityRef(r.EntityRef)
		rec := HealAuditRecord{
			Timestamp:   r.Timestamp,
			CycleID:     cycleID,
			InvariantID: r.InvariantID,
			EntityRef:   r.EntityRef,
			Node:        node,
			Package:     pkg,
			Disposition: r.Disposition,
			Action:      r.Action,
			Executed:    r.Executed,
			Verified:    r.Verified,
			Error:       r.Error,
		}
		_ = s.Append(rec) // best-effort — don't crash the healer on audit failure
	}
}

// HealHistoryFilter controls what records are returned by ReadHistory.
type HealHistoryFilter struct {
	Node         string
	Package      string
	InvariantID  string
	ExecutedOnly bool
	FailuresOnly bool
	Limit        int
	Since        time.Time
}

// ReadHistory reads the JSONL file and returns matching records, newest first.
func (s *HealAuditStore) ReadHistory(filter HealHistoryFilter) ([]HealAuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no history yet
		}
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	defer f.Close()

	var all []HealAuditRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec HealAuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // skip corrupt lines
		}
		if matchesFilter(rec, filter) {
			all = append(all, rec)
		}
	}

	// Reverse for newest-first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	if filter.Limit > 0 && len(all) > filter.Limit {
		all = all[:filter.Limit]
	}
	return all, nil
}

func matchesFilter(rec HealAuditRecord, f HealHistoryFilter) bool {
	if f.Node != "" && rec.Node != f.Node && !strings.HasPrefix(rec.Node, f.Node) {
		return false
	}
	if f.Package != "" && rec.Package != f.Package {
		return false
	}
	if f.InvariantID != "" && rec.InvariantID != f.InvariantID {
		return false
	}
	if f.ExecutedOnly && !rec.Executed {
		return false
	}
	if f.FailuresOnly && rec.Error == "" {
		return false
	}
	if !f.Since.IsZero() && rec.Timestamp.Before(f.Since) {
		return false
	}
	return true
}

func parseEntityRef(ref string) (node, pkg string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ref
}
