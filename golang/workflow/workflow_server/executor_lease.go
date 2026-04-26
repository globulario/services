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
)

// ── Schema ───────────────────────────────────────────────────────────────────

const createExecutorLeasesTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.executor_leases (
    run_id       text PRIMARY KEY,
    executor_id  text,
    heartbeat_at bigint,
    started_at   bigint
)`

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
	if m.srv.session == nil {
		return true, nil // no ScyllaDB = single-node mode, always succeed
	}

	now := time.Now().UnixMilli()

	// LWT: INSERT IF NOT EXISTS — only succeeds if no current owner.
	applied, err := m.srv.session.Query(`
		INSERT INTO workflow.executor_leases (run_id, executor_id, heartbeat_at, started_at)
		VALUES (?, ?, ?, ?)
		IF NOT EXISTS`,
		runID, m.executorID, now, now,
	).ScanCAS(nil, nil, nil, nil)

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

	return false, nil // another executor owns it
}

// ReleaseRun removes the lease for a completed run and stops its heartbeat.
func (m *executorLeaseManager) ReleaseRun(runID string) {
	m.mu.Lock()
	if cancel, ok := m.ownedRuns[runID]; ok {
		cancel()
		delete(m.ownedRuns, runID)
	}
	m.mu.Unlock()

	if m.srv.session == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	applied, err := m.srv.session.Query(`
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
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.srv.session == nil {
				continue
			}
			now := time.Now().UnixMilli()
			applied, err := m.srv.session.Query(`
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

// ── Orphan Scanner ───────────────────────────────────────────────────────────

const (
	orphanHeartbeatTimeout = 30 * time.Second
	orphanScanInterval     = 15 * time.Second
)

// StartOrphanScanner runs a background goroutine that periodically scans
// for orphaned runs (heartbeat older than orphanHeartbeatTimeout) and
// attempts to claim them for resumption.
func (m *executorLeaseManager) StartOrphanScanner(ctx context.Context) {
	if m.srv.session == nil {
		return // no ScyllaDB = single-node mode
	}

	go func() {
		ticker := time.NewTicker(orphanScanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.scanAndClaimOrphans(ctx)
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
	if m.srv.session == nil {
		return
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-orphanHeartbeatTimeout).UnixMilli()

	iter := m.srv.session.Query(`
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
	now := time.Now().UnixMilli()

	// LWT: only succeed if heartbeat hasn't changed (no one else claimed it).
	applied, err := m.srv.session.Query(`
		UPDATE workflow.executor_leases
		SET executor_id = ?, heartbeat_at = ?
		WHERE run_id = ?
		IF heartbeat_at = ?`,
		m.executorID, now, runID, staleHeartbeat,
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
