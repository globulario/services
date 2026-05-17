package graph

import (
	"context"
	"fmt"
	"time"
)

// UpsertRuntimeSnapshot stores a serialized snapshot.
func (g *Graph) UpsertRuntimeSnapshot(ctx context.Context, id string, capturedAt int64, nodeID, clusterID string, snapshotJSON []byte) error {
	if g.readOnly {
		return fmt.Errorf("UpsertRuntimeSnapshot %s: graph is read-only", id)
	}
	now := time.Now().Unix()
	rec := &runtimeSnapshotRecord{
		ID:           id,
		CapturedAt:   capturedAt,
		NodeID:       nodeID,
		ClusterID:    clusterID,
		SnapshotJSON: string(snapshotJSON),
		CreatedAt:    now,
	}

	g.snapshotMu.Lock()
	// Upsert: replace if same ID.
	replaced := false
	for i, s := range g.snapshots {
		if s.ID == id {
			g.snapshots[i] = rec
			replaced = true
			break
		}
	}
	if !replaced {
		g.snapshots = append(g.snapshots, rec)
	}
	sortSnapshotsByTime(g.snapshots)
	g.snapshotMu.Unlock()

	return g.writeJSON("snapshots", id, rec)
}

// LatestRuntimeSnapshot returns the most-recent snapshot JSON, or nil if none.
func (g *Graph) LatestRuntimeSnapshot(ctx context.Context) ([]byte, error) {
	g.snapshotMu.RLock()
	defer g.snapshotMu.RUnlock()
	if len(g.snapshots) == 0 {
		return nil, nil
	}
	return []byte(g.snapshots[0].SnapshotJSON), nil
}

// GetRuntimeSnapshotByID returns the stored snapshot JSON for the given ID.
// Returns (nil, nil) if no snapshot with that ID exists.
func (g *Graph) GetRuntimeSnapshotByID(ctx context.Context, id string) ([]byte, error) {
	g.snapshotMu.RLock()
	defer g.snapshotMu.RUnlock()
	for _, s := range g.snapshots {
		if s.ID == id {
			return []byte(s.SnapshotJSON), nil
		}
	}
	return nil, nil
}
