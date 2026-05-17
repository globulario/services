package graph

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GraphFreshness describes the freshness of the awareness graph.
type GraphFreshness struct {
	BuiltAt             time.Time
	BuiltAtUnix         int64
	AgeSeconds          float64
	KnowledgeMtime      time.Time
	KnowledgeSourceHash string
	Stale               bool
	StaleReason         string
	RebuildRecommended  bool
	MaxAgeExceeded      bool
}

// LatestBuildTime returns the most recent build timestamp from in-memory builds.
// Returns (zero time, false, nil) if no build records exist.
func (g *Graph) LatestBuildTime(ctx context.Context) (time.Time, bool, error) {
	g.buildMu.RLock()
	var maxTS int64
	var found bool
	for _, b := range g.builds {
		if b.CreatedAt > maxTS {
			maxTS = b.CreatedAt
			found = true
		}
	}
	g.buildMu.RUnlock()

	if found && maxTS != 0 {
		return time.Unix(maxTS, 0).UTC(), true, nil
	}

	return time.Time{}, false, nil
}

// knowledgeFiles are the canonical YAML files that contribute graph nodes
// and edges to the awareness knowledge base.
var knowledgeFiles = []string{
	"failure_modes.yaml", "invariants.yaml", "convergence_rules.yaml",
	"forbidden_fixes.yaml", "design_patterns.yaml", "patterns.yaml",
	"services.yaml",
	"detector_mapping.yaml",
}

// KnowledgeFiles returns a copy of the canonical list.
func KnowledgeFiles() []string {
	out := make([]string, len(knowledgeFiles))
	copy(out, knowledgeFiles)
	return out
}

// Freshness computes graph freshness relative to the knowledge YAML files in docsDir.
func (g *Graph) Freshness(ctx context.Context, docsDir string) GraphFreshness {
	return g.FreshnessAt(ctx, docsDir, time.Now())
}

// FreshnessAt is the clock-injectable form of Freshness.
func (g *Graph) FreshnessAt(ctx context.Context, docsDir string, now time.Time) GraphFreshness {
	if now.IsZero() {
		now = time.Now()
	}
	builtAt, ok, _ := g.LatestBuildTime(ctx)
	if !ok {
		return GraphFreshness{
			Stale:              true,
			StaleReason:        "no graph build record found — run 'globular awareness build'",
			RebuildRecommended: true,
		}
	}

	f := GraphFreshness{
		BuiltAt:     builtAt,
		BuiltAtUnix: builtAt.Unix(),
		AgeSeconds:  now.Sub(builtAt).Seconds(),
	}

	// Check max age (24h).
	if f.AgeSeconds > 24*3600 {
		f.MaxAgeExceeded = true
		f.Stale = true
		f.StaleReason = fmt.Sprintf("graph is %.1f hours old — run 'globular awareness build'", f.AgeSeconds/3600)
	}

	// Check knowledge file mtimes.
	if docsDir != "" {
		knowledgeMtime := latestMtime(docsDir, knowledgeFiles)
		f.KnowledgeMtime = knowledgeMtime

		if !knowledgeMtime.IsZero() && knowledgeMtime.After(builtAt) {
			f.Stale = true
			f.StaleReason = "knowledge YAML files modified after last graph build — run 'globular awareness build'"
		}

		f.KnowledgeSourceHash = computeKnowledgeHash(docsDir)
	}

	f.RebuildRecommended = f.Stale
	return f
}

func computeKnowledgeHash(docsDir string) string {
	h := sha256.New()
	for _, name := range knowledgeFiles {
		data, err := os.ReadFile(filepath.Join(docsDir, name))
		if err != nil {
			continue
		}
		h.Write([]byte(name))
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
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
