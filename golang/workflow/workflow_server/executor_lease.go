// executor_lease.go implements durable run ownership for centralized
// workflow execution. Each active run is owned by exactly one executor
// instance via a ScyllaDB lease. Heartbeats keep the lease alive;
// orphan detection claims stale leases for resumption.
//
// See docs/architecture/HA-control-plane-design.md §Class C.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

// ── Schema ───────────────────────────────────────────────────────────────────

const createExecutorLeasesTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.executor_leases (
    run_id           text PRIMARY KEY,
    executor_id      text,
    heartbeat_at     bigint,
    started_at       bigint,
    last_progress_at bigint
)`

// leaseHeartbeatInterval is how often an owning executor refreshes heartbeat_at.
// A package var (not const) so tests can shorten it. Keep well under
// orphanHeartbeatTimeout so a healthy executor never looks orphaned.
var leaseHeartbeatInterval = 10 * time.Second

// revokedExecutorID is the tombstone owner the reaper writes onto a hung run's
// lease. It belongs to no live executor, so (a) the prior owner's heartbeat CAS
// (IF executor_id = <prior>) fails and it stops, and (b) any orphan scanner sees
// executor_id != self with a backdated heartbeat and claims the run for resume.
const revokedExecutorID = "__reaper_revoked__"

// ── Lease Manager ────────────────────────────────────────────────────────────

// executorLeaseManager manages run ownership leases. It provides claim,
// heartbeat, release, and orphan scanning operations.
type executorLeaseManager struct {
	srv        *server
	executorID string

	mu        sync.Mutex
	ownedRuns map[string]context.CancelFunc // run_id → heartbeat cancel

	// Orphan scanner backoff state.
	scanFailures     int           // consecutive scan-cycle failures
	scanBackoff      time.Duration // current backoff duration
	scanBackoffUntil time.Time     // skip scans until this time
}

func newExecutorLeaseManager(srv *server) *executorLeaseManager {
	hostname, _ := os.Hostname()
	return &executorLeaseManager{
		srv:        srv,
		executorID: fmt.Sprintf("executor:%s:%d", hostname, os.Getpid()),
		ownedRuns:  make(map[string]context.CancelFunc),
	}
}

// ClaimRun attempts to claim ownership of a run via ScyllaDB LWT.
// Returns true if the claim succeeded (this executor now owns the run).
// Returns false if another executor already owns it.
func (m *executorLeaseManager) ClaimRun(ctx context.Context, runID string) (bool, error) {
	sess := m.srv.getSession()
	if sess == nil {
		// No ScyllaDB session — the lease fence is unavailable. Log a
		// warning and proceed (single-node bootstrap or degraded multi-node).
		// The caller should treat this as degraded: the run will execute
		// but is not fenced against concurrent executors.
		// See meta.fallback_must_degrade_semantics.
		slog.Warn("lease: ClaimRun proceeding without ScyllaDB fence (session nil) — run is unfenced",
			"run_id", runID)
		return true, nil
	}

	now := time.Now().UnixMilli()

	// LWT: INSERT IF NOT EXISTS — only succeeds if no current owner.
	// last_progress_at is seeded to now so a freshly-claimed run has a valid
	// progress clock before its first step completes (see RecordProgress).
	applied, err := sess.Query(`
		INSERT INTO workflow.executor_leases (run_id, executor_id, heartbeat_at, started_at, last_progress_at)
		VALUES (?, ?, ?, ?, ?)
		IF NOT EXISTS`,
		runID, m.executorID, now, now, now,
	).ScanCAS(nil, nil, nil, nil, nil)

	if err != nil {
		return false, fmt.Errorf("claim run %s: %w", runID, err)
	}

	if applied {
		// Start heartbeat goroutine.
		hbCtx, cancel := context.WithCancel(context.Background())
		m.mu.Lock()
		m.ownedRuns[runID] = cancel
		m.mu.Unlock()
		go m.heartbeatLoop(hbCtx, runID)
		return true, nil
	}

	// INSERT failed — another executor holds the lease.
	// Check if that lease is stale (heartbeat older than orphanHeartbeatTimeout).
	// If so, steal it immediately rather than waiting for the orphan scanner.
	cutoff := time.Now().Add(-orphanHeartbeatTimeout).UnixMilli()
	var existingExecutor string
	var existingHeartbeat int64
	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()
	if err := sess.Query(`
		SELECT executor_id, heartbeat_at FROM workflow.executor_leases WHERE run_id = ?`,
		runID,
	).WithContext(readCtx).Scan(&existingExecutor, &existingHeartbeat); err != nil {
		// Can't read the lease — treat as actively owned.
		return false, nil
	}
	if existingHeartbeat >= cutoff {
		// Lease is fresh — respect active ownership.
		return false, nil
	}

	// Stale lease: steal it using the same LWT as the orphan scanner.
	slog.Info("executor lease: stale lease on ClaimRun, stealing",
		"run_id", runID,
		"stale_executor", existingExecutor,
		"heartbeat_age_ms", time.Now().UnixMilli()-existingHeartbeat)
	stolen, err := m.claimOrphan(ctx, runID, existingHeartbeat)
	if err != nil {
		return false, fmt.Errorf("steal stale lease for run %s: %w", runID, err)
	}
	return stolen, nil
}

// ReleaseRun removes the lease for a completed run and stops its heartbeat.
func (m *executorLeaseManager) ReleaseRun(runID string) {
	m.mu.Lock()
	if cancel, ok := m.ownedRuns[runID]; ok {
		cancel()
		delete(m.ownedRuns, runID)
	}
	m.mu.Unlock()

	sess := m.srv.getSession()
	if sess == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	applied, err := sess.Query(`
		DELETE FROM workflow.executor_leases
		WHERE run_id = ?
		IF executor_id = ?`,
		runID, m.executorID,
	).WithContext(ctx).ScanCAS(nil)
	if err != nil {
		slog.Warn("executor lease: release failed", "run_id", runID, "err", err)
	}
	if applied == false {
		slog.Warn("executor lease: release skipped, not owner",
			"run_id", runID, "executor_id", m.executorID)
	}
}

// heartbeatLoop updates the heartbeat_at timestamp every 10 seconds.
// Stops when the context is cancelled (run completed or released).
func (m *executorLeaseManager) heartbeatLoop(ctx context.Context, runID string) {
	ticker := time.NewTicker(leaseHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sess := m.srv.getSession()
			if sess == nil {
				continue
			}
			now := time.Now().UnixMilli()
			applied, err := sess.Query(`
				UPDATE workflow.executor_leases SET heartbeat_at = ?
				WHERE run_id = ?
				IF executor_id = ?`,
				now, runID, m.executorID,
			).ScanCAS(nil)
			if err != nil {
				slog.Warn("executor lease: heartbeat failed",
					"run_id", runID, "err", err)
				continue
			}
			if !applied {
				slog.Warn("executor lease: ownership lost, stopping heartbeat",
					"run_id", runID, "executor_id", m.executorID)
				m.mu.Lock()
				if cancel, ok := m.ownedRuns[runID]; ok {
					cancel()
					delete(m.ownedRuns, runID)
				}
				m.mu.Unlock()
				return
			}
		}
	}
}

// RecordProgress stamps last_progress_at=now for a run this executor owns.
// Called on every step completion (executionRecorder.onStepDone) so the reaper
// can distinguish an executor that is alive AND advancing steps from one that is
// alive but hung (heartbeat fresh, no step progress). Best-effort and fenced by
// executor ownership: a failed or non-owned update is logged, never fatal — a
// missed stamp only makes the run look slightly staler, which the generous
// progressDeadline absorbs. No-op when the ScyllaDB fence is unavailable.
func (m *executorLeaseManager) RecordProgress(runID string) {
	sess := m.srv.getSession()
	if sess == nil {
		return
	}
	now := time.Now().UnixMilli()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := sess.Query(`
		UPDATE workflow.executor_leases SET last_progress_at = ?
		WHERE run_id = ?
		IF executor_id = ?`,
		now, runID, m.executorID,
	).WithContext(ctx).ScanCAS(nil); err != nil {
		slog.Debug("executor lease: record progress failed",
			"run_id", runID, "err", err)
	}
}

// RevokeLease forces a run's lease to look orphaned so the orphan scanner
// resumes it. The reaper calls this when an executor is alive (heartbeating) but
// has made no step progress past the deadline — a hung-but-heartbeating executor
// that neither the heartbeat check nor the normal orphan scan would recover.
//
// It tombstones the owner (executor_id = revokedExecutorID) and backdates the
// heartbeat to 0, fenced on currentOwner so a lease that changed hands
// concurrently is left alone (meta.competing_writers_must_converge_or_be_fenced).
// After this: the prior owner's next heartbeat CAS fails and it stops; the orphan
// scanner sees a stale, foreign lease and claims it via claimOrphan, which drives
// ResumeRun. Resume then closes an all-terminal run (empty Execute → idempotent
// FinishRun) or re-runs the remaining/RUNNING step (actors are idempotent).
// Returns true if the revoke was applied (this run was still owned by currentOwner).
func (m *executorLeaseManager) RevokeLease(runID, currentOwner string) (bool, error) {
	sess := m.srv.getSession()
	if sess == nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	applied, err := sess.Query(`
		UPDATE workflow.executor_leases
		SET executor_id = ?, heartbeat_at = ?
		WHERE run_id = ?
		IF executor_id = ?`,
		revokedExecutorID, int64(0), runID, currentOwner,
	).WithContext(ctx).ScanCAS(nil)
	if err != nil {
		return false, fmt.Errorf("revoke lease %s: %w", runID, err)
	}
	// If this executor happens to be the (local) owner, stop its heartbeat now
	// rather than waiting for the CAS to fail on the next tick.
	if applied {
		m.mu.Lock()
		if cancel, ok := m.ownedRuns[runID]; ok {
			cancel()
			delete(m.ownedRuns, runID)
		}
		m.mu.Unlock()
	}
	return applied, nil
}

// ── Orphan Scanner ───────────────────────────────────────────────────────────

const (
	orphanHeartbeatTimeout = 30 * time.Second
	orphanScanInterval     = 15 * time.Second
)

// StartOrphanScanner runs a background goroutine that periodically scans
// for orphaned runs (heartbeat older than orphanHeartbeatTimeout) and
// attempts to claim them for resumption.
func (m *executorLeaseManager) StartOrphanScanner(ctx context.Context) {
	if m.srv.getSession() == nil {
		return // no ScyllaDB = single-node mode
	}

	go func() {
		// Recover immediately on startup so a freshly (re)started instance adopts
		// orphaned (stale-lease) and stranded (unleased EXECUTING) runs without
		// waiting a full scan interval — this is the fast-failover path when an
		// executor instance dies and another takes over.
		m.scanAndClaimOrphans(ctx)
		m.scanAndClaimUnleasedRuns(ctx)

		ticker := time.NewTicker(orphanScanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.scanAndClaimOrphans(ctx)
				m.scanAndClaimUnleasedRuns(ctx)
			}
		}
	}()
}

const (
	orphanBackoffMin = 1 * time.Second
	orphanBackoffMax = 2 * time.Minute
)

func (m *executorLeaseManager) scanAndClaimOrphans(ctx context.Context) {
	// Backoff: skip this scan if we're in a backoff period.
	if time.Now().Before(m.scanBackoffUntil) {
		return
	}
	// Session may be nil if ScyllaDB was closed during shutdown.
	sess := m.srv.getSession()
	if sess == nil {
		return
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-orphanHeartbeatTimeout).UnixMilli()

	iter := sess.Query(`
		SELECT run_id, executor_id, heartbeat_at FROM workflow.executor_leases`,
	).WithContext(scanCtx).Iter()

	var runID, executorID string
	var heartbeatAt int64
	cycleFailures := 0

	for iter.Scan(&runID, &executorID, &heartbeatAt) {
		if heartbeatAt < cutoff && executorID != m.executorID {
			// Stale lease — attempt to claim via LWT.
			slog.Info("executor lease: orphan detected",
				"run_id", runID,
				"stale_executor", executorID,
				"heartbeat_age_ms", time.Now().UnixMilli()-heartbeatAt)

			claimed, err := m.claimOrphan(ctx, runID, heartbeatAt)
			if err != nil {
				cycleFailures++
				// Rate-limit error logs: only log first 3 per cycle.
				if cycleFailures <= 3 {
					slog.Warn("executor lease: claim orphan failed",
						"run_id", runID, "err", err,
						"cycle_failures", cycleFailures)
				}
				continue
			}
			if claimed {
				slog.Info("executor lease: claimed orphan, resuming",
					"run_id", runID)
				go func(rID string) {
					resumeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()
					if err := m.srv.resumeOrphanedRun(resumeCtx, rID); err != nil {
						slog.Warn("executor lease: resume failed",
							"run_id", rID, "err", err)
					}
					m.ReleaseRun(rID)
				}(runID)
			}
		}
	}

	if err := iter.Close(); err != nil {
		slog.Warn("executor lease: orphan scan iteration error", "err", err)
		cycleFailures++
	}

	// Log suppressed errors.
	if cycleFailures > 3 {
		slog.Warn("executor lease: orphan scan cycle completed with errors",
			"total_failures", cycleFailures, "suppressed", cycleFailures-3)
	}

	// Update backoff state.
	if cycleFailures > 0 {
		m.scanFailures++
		if m.scanBackoff == 0 {
			m.scanBackoff = orphanBackoffMin
		} else {
			m.scanBackoff *= 2
			if m.scanBackoff > orphanBackoffMax {
				m.scanBackoff = orphanBackoffMax
			}
		}
		m.scanBackoffUntil = time.Now().Add(m.scanBackoff)
		slog.Warn("executor lease: orphan scanner backing off",
			"consecutive_failures", m.scanFailures,
			"backoff", m.scanBackoff,
			"next_scan_after", m.scanBackoffUntil.Format(time.RFC3339))
	} else {
		// Reset on clean cycle.
		if m.scanFailures > 0 {
			slog.Info("executor lease: orphan scanner recovered",
				"previous_failures", m.scanFailures)
		}
		m.scanFailures = 0
		m.scanBackoff = 0
		m.scanBackoffUntil = time.Time{}
	}
}

func (m *executorLeaseManager) claimOrphan(ctx context.Context, runID string, staleHeartbeat int64) (bool, error) {
	sess := m.srv.getSession()
	if sess == nil {
		return false, fmt.Errorf("scylla session unavailable")
	}
	now := time.Now().UnixMilli()

	// LWT: only succeed if heartbeat hasn't changed (no one else claimed it).
	// Seed last_progress_at to now so the resuming executor starts with a fresh
	// progress clock (the prior owner's stalled progress must not immediately
	// re-trip the reaper against the new owner).
	applied, err := sess.Query(`
		UPDATE workflow.executor_leases
		SET executor_id = ?, heartbeat_at = ?, last_progress_at = ?
		WHERE run_id = ?
		IF heartbeat_at = ?`,
		m.executorID, now, now, runID, staleHeartbeat,
	).ScanCAS(nil)

	if err != nil {
		return false, fmt.Errorf("claim orphan %s: %w", runID, err)
	}

	if applied {
		// Start heartbeat for the claimed run.
		hbCtx, cancel := context.WithCancel(context.Background())
		m.mu.Lock()
		m.ownedRuns[runID] = cancel
		m.mu.Unlock()
		go m.heartbeatLoop(hbCtx, runID)
	}

	return applied, nil
}

// runIsRecoverableWhenUnleased decides whether a run that currently has NO
// executor lease should be claimed and driven to completion by any available
// instance. Only RUN_STATUS_EXECUTING qualifies: it means "this run is supposed
// to be running but nobody owns it" — the orphaned/stranded case. Terminal and
// DEFERRED runs are done or parked; BLOCKED runs are intentionally waiting for
// operator approval and MUST NOT be resumed. Pure so the policy is unit-testable.
func runIsRecoverableWhenUnleased(status int) bool {
	return workflowpb.RunStatus(status) == workflowpb.RunStatus_RUN_STATUS_EXECUTING
}

// scanAndClaimUnleasedRuns realises "any engine instance can load and progress
// any run from shared state": it finds EXECUTING runs that have NO lease (the
// stranded case — e.g. an executor that returned with FinishRun still failing,
// whose deferred ReleaseRun then deleted the lease), claims each via the same
// LWT fence as ClaimRun, and drives it through ResumeRun (close it if all steps
// are already terminal, else re-run the remainder).
//
// This complements the orphan scanner (stale-lease) and the reaper (hung, fresh
// lease): together they guarantee every EXECUTING run has exactly one live owner
// or is recovered. A sync-executing run always holds a lease (ClaimRun precedes
// StartRun), so it is never seen here — only genuinely unowned runs are claimed
// (meta.competing_writers_must_converge_or_be_fenced).
func (m *executorLeaseManager) scanAndClaimUnleasedRuns(ctx context.Context) {
	if time.Now().Before(m.scanBackoffUntil) {
		return
	}
	sess := m.srv.getSession()
	if sess == nil {
		return
	}
	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Snapshot the set of run_ids that currently hold a lease. Any EXECUTING run
	// NOT in this set is unowned.
	leased := make(map[string]struct{})
	lit := sess.Query(`SELECT run_id FROM workflow.executor_leases`).WithContext(scanCtx).Iter()
	var leaseRunID string
	for lit.Scan(&leaseRunID) {
		leased[leaseRunID] = struct{}{}
	}
	if err := lit.Close(); err != nil {
		slog.Warn("executor lease: unleased scan (lease list) error", "err", err)
		return
	}

	rit := sess.Query(`SELECT id, status FROM workflow.workflow_runs LIMIT 500 ALLOW FILTERING`).
		WithContext(scanCtx).Iter()
	var (
		runID   string
		status  int
		claimed int
	)
	for rit.Scan(&runID, &status) {
		if !runIsRecoverableWhenUnleased(status) {
			continue
		}
		if _, ok := leased[runID]; ok {
			continue // owned — leave to heartbeat/orphan/reaper
		}
		ok, err := m.ClaimRun(ctx, runID)
		if err != nil {
			slog.Warn("executor lease: claim unleased run failed", "run_id", runID, "err", err)
			continue
		}
		if !ok {
			continue // another instance claimed it first
		}
		claimed++
		slog.Info("executor lease: claimed unleased EXECUTING run, resuming", "run_id", runID)
		go func(rID string) {
			resumeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := m.srv.resumeOrphanedRun(resumeCtx, rID); err != nil {
				slog.Warn("executor lease: unleased-run resume failed", "run_id", rID, "err", err)
			}
			m.ReleaseRun(rID)
		}(runID)
	}
	if err := rit.Close(); err != nil {
		slog.Warn("executor lease: unleased scan (runs) error", "err", err)
	}
	if claimed > 0 {
		slog.Info("executor lease: recovered unleased EXECUTING runs", "claimed", claimed)
	}
}
