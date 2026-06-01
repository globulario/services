// retention.go — periodic pruning of terminal workflow runs.
//
// Keeps the most recent N terminal runs per cluster and deletes older ones.
// Also deletes their associated steps, events, artifact refs, and secondary
// index rows (runs_by_node, runs_by_component).
//
// This prevents unbounded partition growth in workflow_runs which causes
// tombstone pressure and high CPU on ScyllaDB reads.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gocql/gocql"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

const (
	// defaultRetentionKeep is the number of terminal runs to keep per cluster.
	defaultRetentionKeep = 1000

	// retentionScanInterval is how often the pruner runs.
	retentionScanInterval = 10 * time.Minute

	// retentionBatchSize is the max rows deleted per pruner cycle to avoid
	// overwhelming ScyllaDB with tombstones in a single pass.
	retentionBatchSize = 500
)

// retentionPruner deletes old terminal workflow runs beyond the retention limit.
type retentionPruner struct {
	getSession func() *gocql.Session
	clusterID  string
	keep       int
	logger     *slog.Logger
}

func newRetentionPruner(getSession func() *gocql.Session, clusterID string, log *slog.Logger) *retentionPruner {
	return &retentionPruner{
		getSession: getSession,
		clusterID:  clusterID,
		keep:       defaultRetentionKeep,
		logger:     log,
	}
}

// Start runs the pruner on a ticker. Call in a goroutine.
func (rp *retentionPruner) Start(ctx context.Context) {
	rp.logger.Info("retention: pruner started", "cluster_id", rp.clusterID, "keep", rp.keep)
	// Run once after a short startup delay.
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return
	}
	rp.logger.Info("retention: startup delay elapsed, running first prune")
	rp.prune()

	ticker := time.NewTicker(retentionScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rp.prune()
		case <-ctx.Done():
			return
		}
	}
}

// prune finds terminal runs beyond the keep limit and deletes them in batches.
func (rp *retentionPruner) prune() {
	sess := rp.getSession()
	if sess == nil {
		rp.logger.Warn("retention: no scylla session, skipping")
		return
	}
	rp.logger.Info("retention: scanning for old runs", "cluster_id", rp.clusterID, "keep", rp.keep)

	// Fetch all terminal runs ordered by started_at DESC (table clustering).
	// We keep the newest `rp.keep` and delete the rest.
	iter := sess.Query(
		`SELECT started_at, id, status, node_id, component_name
		 FROM workflow_runs
		 WHERE cluster_id = ?`,
		rp.clusterID,
	).Iter()

	type runKey struct {
		startedAt     time.Time
		id            string
		status        int
		nodeID        string
		componentName string
	}

	var toDelete []runKey
	kept := 0
	var rk runKey
	for iter.Scan(&rk.startedAt, &rk.id, &rk.status, &rk.nodeID, &rk.componentName) {
		if !isTerminalStatusInt(rk.status) {
			// Never delete active runs.
			continue
		}
		kept++
		if kept > rp.keep {
			toDelete = append(toDelete, rk)
			if len(toDelete) >= retentionBatchSize {
				break // Don't collect unbounded; we'll get the rest next cycle.
			}
		}
	}
	if err := iter.Close(); err != nil {
		rp.logger.Warn("retention: scan workflow_runs failed", "err", err)
		return
	}

	if len(toDelete) == 0 {
		return
	}

	rp.logger.Info("retention: pruning old workflow runs",
		"total_terminal", kept,
		"keep", rp.keep,
		"deleting", len(toDelete),
	)

	deleted := 0
	for _, rk := range toDelete {
		if err := rp.deleteRun(sess, rk.startedAt, rk.id, rk.nodeID, rk.componentName); err != nil {
			rp.logger.Warn("retention: delete run failed",
				"run_id", rk.id,
				"err", err,
			)
			continue
		}
		deleted++
	}

	rp.logger.Info("retention: pruning complete",
		"deleted", deleted,
		"remaining", kept-deleted,
	)
}

// deleteRun removes a run and its associated data from all tables.
func (rp *retentionPruner) deleteRun(sess *gocql.Session, startedAt time.Time, runID, nodeID, componentName string) error {
	// Main table.
	if err := sess.Query(
		`DELETE FROM workflow_runs WHERE cluster_id = ? AND started_at = ? AND id = ?`,
		rp.clusterID, startedAt, runID,
	).Exec(); err != nil {
		return fmt.Errorf("workflow_runs: %w", err)
	}

	// Secondary index: by node.
	if nodeID != "" {
		_ = sess.Query(
			`DELETE FROM workflow_runs_by_node WHERE cluster_id = ? AND node_id = ? AND started_at = ? AND run_id = ?`,
			rp.clusterID, nodeID, startedAt, runID,
		).Exec()
	}

	// Secondary index: by component.
	if componentName != "" {
		_ = sess.Query(
			`DELETE FROM workflow_runs_by_component WHERE cluster_id = ? AND component_name = ? AND started_at = ? AND run_id = ?`,
			rp.clusterID, componentName, startedAt, runID,
		).Exec()
	}

	// Steps.
	_ = sess.Query(
		`DELETE FROM workflow_steps WHERE cluster_id = ? AND run_id = ?`,
		rp.clusterID, runID,
	).Exec()

	// Events.
	_ = sess.Query(
		`DELETE FROM workflow_events WHERE cluster_id = ? AND run_id = ?`,
		rp.clusterID, runID,
	).Exec()

	// Artifact refs.
	_ = sess.Query(
		`DELETE FROM workflow_artifact_refs WHERE cluster_id = ? AND run_id = ?`,
		rp.clusterID, runID,
	).Exec()

	return nil
}

// isTerminalStatusInt wraps the existing isTerminalStatus for int values
// returned by ScyllaDB scans.
func isTerminalStatusInt(status int) bool {
	return isTerminalStatus(workflowpb.RunStatus(status))
}
