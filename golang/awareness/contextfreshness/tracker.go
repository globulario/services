// Package contextfreshness tracks which files an agent session has read and
// detects when those files change, preventing the agent from acting on stale context.
//
// The freshness ledger answers: "Is the version of this file that the agent
// remembers still the current version?" — a question the awareness graph
// alone cannot answer.
package contextfreshness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/google/uuid"
)

// Tracker records context reads and detects staleness.
// It is backed by JSON files in the awareness graph's data directory,
// or an in-memory map when the graph has no data directory.
type Tracker struct {
	mu      sync.Mutex
	dataDir string // <graph.DataDir()>/context_reads; "" = in-memory

	// memReads and memWarnings hold records when dataDir == "".
	memReads    []*contextReadFile
	memWarnings map[string]*staleWarningFile
}

// New returns a Tracker backed by the given awareness graph data directory.
func New(g *graph.Graph) *Tracker {
	dir := ""
	if d := g.DataDir(); d != "" {
		dir = filepath.Join(d, "context_reads")
	}
	return &Tracker{
		dataDir:     dir,
		memWarnings: make(map[string]*staleWarningFile),
	}
}

// contextReadFile is the on-disk format for a ContextRead record.
type contextReadFile struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
	SizeBytes   int64  `json:"size_bytes"`
	ModTimeUnix int64  `json:"mod_time_unix"`
	GitCommit   string `json:"git_commit"`
	ReadReason  string `json:"read_reason"`
	ReadTool    string `json:"read_tool"`
	TurnIndex   int    `json:"turn_index"`
	CreatedAt   int64  `json:"created_at"`
}

// staleWarningFile is the on-disk format for a StaleContextWarning record.
type staleWarningFile struct {
	ID                 string `json:"id"`
	SessionID          string `json:"session_id"`
	Path               string `json:"path"`
	ReadFingerprint    string `json:"read_fingerprint"`
	CurrentFingerprint string `json:"current_fingerprint"`
	ReadTurnIndex      int    `json:"read_turn_index"`
	CurrentTurnIndex   int    `json:"current_turn_index"`
	Severity           string `json:"severity"`
	Message            string `json:"message"`
	CreatedAt          int64  `json:"created_at"`
	AcknowledgedAt     int64  `json:"acknowledged_at,omitempty"`
}

func (t *Tracker) readsDir() string {
	if t.dataDir == "" {
		return ""
	}
	_ = os.MkdirAll(t.dataDir, 0o755)
	return t.dataDir
}

func (t *Tracker) warningsDir() string {
	if t.dataDir == "" {
		return ""
	}
	d := filepath.Join(filepath.Dir(t.dataDir), "stale_warnings")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// sanitizeID converts an id to a filesystem-safe component.
func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_", ".", "_")
	return r.Replace(id)
}

// RecordContextRead records that the agent consumed path at its current fingerprint.
func (t *Tracker) RecordContextRead(ctx context.Context, sessionID, path, readReason, readTool string, turnIndex int) (*ContextRead, error) {
	snap, err := Fingerprint(path)
	if err != nil {
		return nil, fmt.Errorf("contextfreshness: fingerprint %s: %w", path, err)
	}
	now := time.Now().Unix()
	cr := &ContextRead{
		ID:          uuid.New().String(),
		SessionID:   sessionID,
		Path:        path,
		Fingerprint: snap.Fingerprint,
		SizeBytes:   snap.SizeBytes,
		ModTimeUnix: snap.ModTimeUnix,
		GitCommit:   snap.GitCommit,
		ReadReason:  readReason,
		ReadTool:    readTool,
		TurnIndex:   turnIndex,
		CreatedAt:   now,
	}

	rec := &contextReadFile{
		ID:          cr.ID,
		SessionID:   cr.SessionID,
		Path:        cr.Path,
		Fingerprint: cr.Fingerprint,
		SizeBytes:   cr.SizeBytes,
		ModTimeUnix: cr.ModTimeUnix,
		GitCommit:   cr.GitCommit,
		ReadReason:  cr.ReadReason,
		ReadTool:    cr.ReadTool,
		TurnIndex:   cr.TurnIndex,
		CreatedAt:   cr.CreatedAt,
	}

	t.mu.Lock()
	if t.dataDir == "" {
		// In-memory mode.
		t.memReads = append(t.memReads, rec)
	} else {
		dir := t.readsDir()
		if dir != "" {
			err = writeJSONAtomic(filepath.Join(dir, sanitizeID(cr.ID)+".json"), rec)
		}
	}
	t.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("contextfreshness: write context_read: %w", err)
	}

	return cr, nil
}

// CheckStaleContext checks the given paths for staleness relative to what the
// session read earlier.
func (t *Tracker) CheckStaleContext(ctx context.Context, sessionID string, paths []string, currentTurnIndex int, severity string) ([]StaleContextWarning, error) {
	if severity == "" {
		severity = SeverityCritical
	}
	var warnings []StaleContextWarning
	for _, path := range paths {
		w, err := t.checkPath(ctx, sessionID, path, currentTurnIndex, severity)
		if err != nil {
			return nil, err
		}
		if w != nil {
			warnings = append(warnings, *w)
		}
	}
	return warnings, nil
}

// checkPath checks a single path. Returns nil when the file is fresh or untracked.
func (t *Tracker) checkPath(ctx context.Context, sessionID, path string, currentTurnIndex int, severity string) (*StaleContextWarning, error) {
	cr := t.latestContextRead(sessionID, path)
	if cr == nil {
		// No read recorded for this path in this session → not stale, just untracked.
		return nil, nil
	}

	currentFP, deleted := currentFingerprintOrDeleted(path)
	if !deleted && currentFP == cr.Fingerprint {
		return nil, nil // still fresh
	}

	w := buildWarning(sessionID, path, cr, currentFP, currentTurnIndex, severity)
	if err := t.persistWarning(ctx, w); err != nil {
		return nil, err
	}
	return &w, nil
}

// latestContextRead returns the most recent ContextRead for (sessionID, path), or nil.
func (t *Tracker) latestContextRead(sessionID, path string) *ContextRead {
	var candidates []*contextReadFile

	if t.dataDir == "" {
		// In-memory mode.
		t.mu.Lock()
		for _, r := range t.memReads {
			if r.SessionID == sessionID && r.Path == path {
				cp := *r
				candidates = append(candidates, &cp)
			}
		}
		t.mu.Unlock()
	} else {
		dir := t.readsDir()
		if dir == "" {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var rec contextReadFile
			if err := json.Unmarshal(data, &rec); err != nil {
				continue
			}
			if rec.SessionID != sessionID || rec.Path != path {
				continue
			}
			r := rec
			candidates = append(candidates, &r)
		}
	}

	var latest *contextReadFile
	for _, r := range candidates {
		if latest == nil || r.CreatedAt > latest.CreatedAt {
			latest = r
		}
	}
	if latest == nil {
		return nil
	}
	return &ContextRead{
		ID:          latest.ID,
		SessionID:   latest.SessionID,
		Path:        latest.Path,
		Fingerprint: latest.Fingerprint,
		SizeBytes:   latest.SizeBytes,
		ModTimeUnix: latest.ModTimeUnix,
		GitCommit:   latest.GitCommit,
		ReadReason:  latest.ReadReason,
		ReadTool:    latest.ReadTool,
		TurnIndex:   latest.TurnIndex,
		CreatedAt:   latest.CreatedAt,
	}
}

// AcknowledgeWarning marks a stale_context_warning as acknowledged.
func (t *Tracker) AcknowledgeWarning(ctx context.Context, warningID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.dataDir == "" {
		// In-memory mode.
		if w, ok := t.memWarnings[warningID]; ok {
			w.AcknowledgedAt = time.Now().Unix()
		}
		return nil
	}

	dir := t.warningsDir()
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, sanitizeID(warningID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // not found, silently ignore
	}
	var w staleWarningFile
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	w.AcknowledgedAt = time.Now().Unix()
	return writeJSONAtomic(path, &w)
}

func (t *Tracker) persistWarning(ctx context.Context, w StaleContextWarning) error {
	rec := &staleWarningFile{
		ID:                 w.ID,
		SessionID:          w.SessionID,
		Path:               w.Path,
		ReadFingerprint:    w.ReadFingerprint,
		CurrentFingerprint: w.CurrentFingerprint,
		ReadTurnIndex:      w.ReadTurnIndex,
		CurrentTurnIndex:   w.CurrentTurnIndex,
		Severity:           w.Severity,
		Message:            w.Message,
		CreatedAt:          w.CreatedAt,
		AcknowledgedAt:     w.AcknowledgedAt,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.dataDir == "" {
		// In-memory mode.
		t.memWarnings[w.ID] = rec
		return nil
	}

	dir := t.warningsDir()
	if dir == "" {
		return nil
	}
	return writeJSONAtomic(filepath.Join(dir, sanitizeID(w.ID)+".json"), rec)
}
