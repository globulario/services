package graph

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// GraphFreshness describes the freshness of the awareness graph.
type GraphFreshness struct {
	BuiltAt        time.Time
	BuiltAtUnix    int64
	AgeSeconds     float64
	KnowledgeMtime time.Time
	Stale          bool
	StaleReason    string
}

// LatestBuildTime queries graph_builds for the most recent created_at timestamp.
// Returns (zero time, false, nil) if no build records exist.
func (g *Graph) LatestBuildTime(ctx context.Context) (time.Time, bool, error) {
	// Use sql.NullInt64 to handle NULL when the table is empty.
	var ts nullInt64
	err := g.db.QueryRowContext(ctx, `SELECT MAX(created_at) FROM graph_builds`).Scan(&ts)
	if err != nil {
		return time.Time{}, false, err
	}
	if !ts.Valid || ts.Value == 0 {
		return time.Time{}, false, nil
	}
	return time.Unix(ts.Value, 0).UTC(), true, nil
}

// nullInt64 is a simple nullable int64 scanner.
type nullInt64 struct {
	Value int64
	Valid bool
}

func (n *nullInt64) Scan(src interface{}) error {
	if src == nil {
		n.Valid = false
		n.Value = 0
		return nil
	}
	switch v := src.(type) {
	case int64:
		n.Value = v
		n.Valid = true
	case float64:
		n.Value = int64(v)
		n.Valid = true
	default:
		n.Valid = false
	}
	return nil
}

// Freshness computes graph freshness relative to the knowledge YAML files in docsDir.
// docsDir is the docs/awareness directory (e.g. /path/to/docs/awareness).
func (g *Graph) Freshness(ctx context.Context, docsDir string) GraphFreshness {
	builtAt, ok, _ := g.LatestBuildTime(ctx)
	if !ok {
		return GraphFreshness{
			Stale:       true,
			StaleReason: "no graph build record found — run 'globular awareness build'",
		}
	}

	f := GraphFreshness{
		BuiltAt:    builtAt,
		BuiltAtUnix: builtAt.Unix(),
		AgeSeconds: time.Since(builtAt).Seconds(),
	}

	// Check knowledge file mtimes.
	knowledgeMtime := latestMtime(docsDir, []string{
		"failure_modes.yaml", "invariants.yaml", "convergence_rules.yaml",
		"forbidden_fixes.yaml", "design_patterns.yaml", "patterns.yaml",
	})
	f.KnowledgeMtime = knowledgeMtime

	if !knowledgeMtime.IsZero() && knowledgeMtime.After(builtAt) {
		f.Stale = true
		f.StaleReason = "knowledge YAML files modified after last graph build — run 'globular awareness build'"
	}
	return f
}

func latestMtime(dir string, files []string) time.Time {
	var latest time.Time
	for _, name := range files {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}
