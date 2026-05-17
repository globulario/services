package graph

import (
	"context"
	"fmt"
	"time"
)

// BuildStats holds graph build statistics.
type BuildStats struct {
	Nodes                 int   `json:"nodes"`
	Edges                 int   `json:"edges"`
	Invariants            int   `json:"invariants"`
	FailureModes          int   `json:"failure_modes"`
	FilesScanned          int   `json:"files_scanned,omitempty"`
	KnowledgeFilesScanned int   `json:"knowledge_files_scanned,omitempty"`
	DurationMs            int64 `json:"duration_ms,omitempty"`
}

// CollectorHealthItem records the outcome of a single collector pass.
type CollectorHealthItem struct {
	CollectorID  string `json:"collector_id"`
	SourceTier   string `json:"source_tier,omitempty"`
	Status       string `json:"status"`
	NodesEmitted int    `json:"nodes_emitted"`
	Error        string `json:"error,omitempty"`
	Priority     string `json:"priority,omitempty"`
}

// BuildRecord is a single graph build record.
type BuildRecord struct {
	ID              string               `json:"id"`
	RepoRoot        string               `json:"repo_root"`
	GitCommit       string               `json:"git_commit,omitempty"`
	ReleaseID       string               `json:"release_id,omitempty"`
	CreatedAt       int64                `json:"created_at"`
	Stats           BuildStats           `json:"stats"`
	CollectorHealth []CollectorHealthItem `json:"collector_health,omitempty"`
}

// LiveSnapshotBuildID is the fixed build ID used for live overlay refresh records.
const LiveSnapshotBuildID = "live-snapshot"

// LatestBuildRecord returns the most recent static graph build row (excludes live snapshots).
func (g *Graph) LatestBuildRecord(ctx context.Context) (*BuildRecord, error) {
	g.buildMu.RLock()
	defer g.buildMu.RUnlock()
	var latest *BuildRecord
	for _, b := range g.builds {
		if b.ID == LiveSnapshotBuildID {
			continue
		}
		if latest == nil || b.CreatedAt > latest.CreatedAt {
			latest = b
		}
	}
	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

// LatestLiveSnapshotRecord returns the most recent live mirror refresh record.
func (g *Graph) LatestLiveSnapshotRecord(ctx context.Context) (*BuildRecord, error) {
	g.buildMu.RLock()
	defer g.buildMu.RUnlock()
	for _, b := range g.builds {
		if b.ID == LiveSnapshotBuildID {
			cp := *b
			return &cp, nil
		}
	}
	return nil, nil
}

// SetBuildCollectorHealth stores the collector health array for a build record.
func (g *Graph) SetBuildCollectorHealth(ctx context.Context, buildID string, items []CollectorHealthItem) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("SetBuildCollectorHealth: graph is read-only")
	}
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	for _, b := range g.builds {
		if b.ID == buildID {
			b.CollectorHealth = items
			return nil
		}
	}
	return nil // build not found — no-op
}

// UpsertBuildRecord records a completed graph build with its stats.
func (g *Graph) UpsertBuildRecord(ctx context.Context, id, repoRoot, gitCommit, releaseID string, stats BuildStats) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertBuildRecord: graph is read-only")
	}
	now := time.Now().Unix()
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	for _, b := range g.builds {
		if b.ID == id {
			b.RepoRoot = repoRoot
			b.GitCommit = gitCommit
			b.ReleaseID = releaseID
			b.CreatedAt = now
			b.Stats = stats
			return nil
		}
	}
	g.builds = append(g.builds, &BuildRecord{
		ID:        id,
		RepoRoot:  repoRoot,
		GitCommit: gitCommit,
		ReleaseID: releaseID,
		CreatedAt: now,
		Stats:     stats,
	})
	return nil
}

// UpsertBuildRecordAt is like UpsertBuildRecord but accepts an explicit
// Unix timestamp. Used for testing clock-dependent freshness logic.
func (g *Graph) UpsertBuildRecordAt(ctx context.Context, id, repoRoot, gitCommit, releaseID string, stats BuildStats, createdAt int64) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertBuildRecordAt: graph is read-only")
	}
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	for _, b := range g.builds {
		if b.ID == id {
			b.RepoRoot = repoRoot
			b.GitCommit = gitCommit
			b.ReleaseID = releaseID
			b.CreatedAt = createdAt
			b.Stats = stats
			return nil
		}
	}
	g.builds = append(g.builds, &BuildRecord{
		ID:        id,
		RepoRoot:  repoRoot,
		GitCommit: gitCommit,
		ReleaseID: releaseID,
		CreatedAt: createdAt,
		Stats:     stats,
	})
	return nil
}
