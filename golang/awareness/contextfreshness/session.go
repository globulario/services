package contextfreshness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CheckAllSessionReads checks every file this session has read for staleness.
// Uses SeverityWarning — this is a background scan, not a pre-edit guard.
// For pre-edit guards use CheckStaleContext with SeverityCritical.
func (t *Tracker) CheckAllSessionReads(ctx context.Context, sessionID string, currentTurnIndex int) ([]StaleContextWarning, error) {
	paths := t.distinctSessionPaths(sessionID)
	return t.CheckStaleContext(ctx, sessionID, paths, currentTurnIndex, SeverityWarning)
}

// distinctSessionPaths returns all distinct file paths read in the given session.
func (t *Tracker) distinctSessionPaths(sessionID string) []string {
	seen := make(map[string]bool)

	if t.dataDir == "" {
		// In-memory mode.
		t.mu.Lock()
		for _, r := range t.memReads {
			if r.SessionID == sessionID && r.Path != "" {
				seen[r.Path] = true
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
			if rec.SessionID == sessionID && rec.Path != "" {
				seen[rec.Path] = true
			}
		}
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	return paths
}
