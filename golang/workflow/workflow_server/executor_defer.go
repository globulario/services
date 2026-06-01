// executor_defer.go — WF-DEFER scheduler hook (B2).
//
// The engine returns a non-terminal RunDeferred status when a step
// exhausts its in-run retry budget AND has a defer: policy. The
// workflow_server is responsible for two things:
//
//  1. Persisting the deferred state on workflow_runs so the cooldown
//     survives a controller / executor restart. The existing columns
//     `backoff_until_ms` and `retry_attempt` are reused — semantics
//     align (this run is in backoff; defer_count is a retry counter).
//
//  2. At dispatch time, refusing to start a new run for the same
//     correlation_id while the prior run is still in its defer
//     cooldown. This is the "scheduler skip" requirement: deferred
//     runs are skipped before defer_until and become eligible after.
//
// Scope notes
// -----------
// This commit deliberately keeps the loop minimal:
//   - No doctor abandonment when defer_count >= max_defers.
//   - No event-driven blocker-tag wakeup.
//   - No broad scheduler refactor — the existing supersedePriorRuns,
//     StartRun, and ExecuteWorkflow flow is preserved.
//   - Normal pending-run dispatch ordering is unchanged: the guard
//     only short-circuits if a deferred run is found.
// @awareness namespace=globular.platform
// @awareness component=platform_workflow.server
// @awareness file_role=workflow_deferred_execution_handler
// @awareness implements=globular.platform:intent.workflow.terminal_runs_must_be_bounded
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// shouldSkipForDeferral is the pure decision: given the latest run
// recorded for a correlation_id and the current wall-clock, should the
// scheduler refuse to dispatch a new run? Returns true only when the
// run is in RUN_STATUS_DEFERRED and BackoffUntilMs is in the future.
//
// Pulled out of the scylla path so it can be unit-tested without a
// session.
func shouldSkipForDeferral(latest *workflowpb.WorkflowRun, now time.Time) bool {
	if latest == nil {
		return false
	}
	if latest.GetStatus() != workflowpb.RunStatus_RUN_STATUS_DEFERRED {
		return false
	}
	return latest.GetBackoffUntilMs() > now.UnixMilli()
}

// findActiveDeferredRun returns the most-recent run for a
// correlation_id that is currently in RUN_STATUS_DEFERRED and whose
// backoff has not yet elapsed. Returns nil otherwise (no deferred run,
// or its cooldown is over and it's eligible to be re-dispatched).
//
// Mirrors the supersedePriorRuns scan pattern. ALLOW FILTERING is fine
// here — correlation_id sets are small (one operator story).
func (srv *server) findActiveDeferredRun(clusterID, correlationID string, now time.Time) *workflowpb.WorkflowRun {
	if correlationID == "" {
		return nil
	}
	sess := srv.getSession()
	if sess == nil {
		return nil
	}
	iter := sess.Query(`
		SELECT id, started_at, status, backoff_until_ms, retry_attempt, error_message
		FROM workflow_runs
		WHERE cluster_id=? AND correlation_id=? ALLOW FILTERING`,
		clusterID, correlationID,
	).PageSize(100).Iter()

	var (
		id, errMsg                string
		startedAt                 time.Time
		status, retryAttempt      int
		backoffMs                 int64
		latest                    *workflowpb.WorkflowRun
		latestStartedUnixMs int64 = -1
	)
	for iter.Scan(&id, &startedAt, &status, &backoffMs, &retryAttempt, &errMsg) {
		candidate := &workflowpb.WorkflowRun{
			Id:             id,
			CorrelationId:  correlationID,
			Status:         workflowpb.RunStatus(status),
			BackoffUntilMs: backoffMs,
			RetryAttempt:   int32(retryAttempt),
			ErrorMessage:   errMsg,
		}
		if !shouldSkipForDeferral(candidate, now) {
			continue
		}
		ms := startedAt.UnixMilli()
		if latest == nil || ms > latestStartedUnixMs {
			latest = candidate
			latestStartedUnixMs = ms
		}
	}
	_ = iter.Close()
	return latest
}

// wakeActiveDeferredRun is the WF-DEFER B4 redispatch primitive. It
// finds the active deferred run for (clusterID, correlationID) and
// sets backoff_until_ms to the wake instant — collapsing the cooldown
// window. defer_count and retry_attempt are preserved: the budget
// continues to count down toward abandonment if the wake fires
// prematurely. If no deferred run exists (already eligible, never
// deferred, or the run finished), this is a no-op success.
//
// Source-of-truth choice: B2's cooldown lives on workflow_runs (not
// the B3 row), so the wake also writes to workflow_runs. The B3
// counter row stays untouched.
func (srv *server) wakeActiveDeferredRun(ctx context.Context, clusterID, correlationID string, now time.Time) (string, bool, error) {
	dr := srv.findActiveDeferredRun(clusterID, correlationID, now)
	if dr == nil {
		return "", false, nil
	}
	sess, err := srv.getSessionOrError()
	if err != nil {
		return dr.GetId(), false, err
	}
	if err := srv.updateRunByID(sess, clusterID, dr.GetId(), func(startedAt time.Time) error {
		return sess.Query(`
			UPDATE workflow_runs SET
				backoff_until_ms=?,
				updated_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			now.UnixMilli(),
			now,
			clusterID, startedAt, dr.GetId(),
		).Exec()
	}); err != nil {
		return dr.GetId(), false, err
	}
	return dr.GetId(), true, nil
}

// recordRunDeferred persists Run.Defer state onto the workflow_runs row
// for the given run id. Called after the engine returns a deferred run.
//
// Reuses backoff_until_ms (when may we retry) and retry_attempt (how
// many cycles so far) — both already exist on the schema. Status flips
// to RUN_STATUS_DEFERRED. error_message carries the last step error
// for observability.
func (srv *server) recordRunDeferred(ctx context.Context, clusterID, runID string, ds *engine.DeferState) error {
	if ds == nil {
		return fmt.Errorf("recordRunDeferred: nil DeferState")
	}
	sess, err := srv.getSessionOrError()
	if err != nil {
		return err
	}
	now := time.Now()
	return srv.updateRunByID(sess, clusterID, runID, func(startedAt time.Time) error {
		return sess.Query(`
			UPDATE workflow_runs SET
				status=?,
				backoff_until_ms=?,
				retry_attempt=?,
				error_message=?,
				updated_at=?
			WHERE cluster_id=? AND started_at=? AND id=?`,
			int(workflowpb.RunStatus_RUN_STATUS_DEFERRED),
			ds.DeferUntil.UnixMilli(),
			int32(ds.DeferCount),
			ds.Reason,
			now,
			clusterID, startedAt, runID,
		).Exec()
	})
}
