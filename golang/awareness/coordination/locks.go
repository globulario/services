package coordination

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// expireLocksAt marks all active locks that have expired as of `now`.
func (s *Store) expireLocksAt(ctx context.Context, now int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE coordination_file_locks SET status = ? WHERE status = ? AND expires_at < ?`,
		StatusExpired, StatusActive, now,
	)
	return err
}

// ExpireOldLocks expires all locks that have passed their expiry time.
func (s *Store) ExpireOldLocks(ctx context.Context) error {
	return s.expireLocksAt(ctx, time.Now().Unix())
}

// AcquireFileLock attempts to acquire an exclusive lock on a file for a run.
// Returns (lock, nil, nil) on success, (nil, conflict, nil) when blocked,
// or (nil, nil, err) on a database error.
func (s *Store) AcquireFileLock(ctx context.Context, req AcquireFileLockRequest) (*FileLock, *LockConflict, error) {
	now := time.Now().Unix()

	ttl := req.TTL
	if ttl == 0 {
		switch req.LockKind {
		case LockDoNotTouch:
			ttl = TTLDoNotTouchLock
		default:
			ttl = TTLEditLock
		}
	}
	expires := now + ttl

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("coordination: acquire lock: begin tx: %w", err)
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	// Step 1: expire old locks.
	if _, err := tx.ExecContext(ctx,
		`UPDATE coordination_file_locks SET status = ? WHERE status = ? AND expires_at < ?`,
		StatusExpired, StatusActive, now,
	); err != nil {
		return nil, nil, fmt.Errorf("coordination: acquire lock: expire old: %w", err)
	}

	// Step 2: check for binding do_not_touch decision for this path.
	if req.LockKind == LockEdit || req.LockKind == LockRename || req.LockKind == LockDelete {
		decRows, err := tx.QueryContext(ctx, `
			SELECT id, agent_id, related_files FROM coordination_decisions
			WHERE run_id = ? AND binding = 1 AND (superseded_by = '' OR superseded_by IS NULL)`,
			req.RunID,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("coordination: acquire lock: check decisions: %w", err)
		}
		type decRow struct {
			id           string
			agentID      string
			relatedFiles string
		}
		var decs []decRow
		for decRows.Next() {
			var d decRow
			if err := decRows.Scan(&d.id, &d.agentID, &d.relatedFiles); err != nil {
				decRows.Close()
				return nil, nil, err
			}
			decs = append(decs, d)
		}
		decRows.Close()

		for _, d := range decs {
			if strings.Contains(d.relatedFiles, req.Path) {
				if err := tx.Rollback(); err == nil {
					tx = nil
				}
				return nil, &LockConflict{
					Type:         "do_not_touch_violation",
					Path:         req.Path,
					OwnerAgentID: d.agentID,
					Message:      fmt.Sprintf("binding do_not_touch decision %s prevents %s on %s", d.id, req.LockKind, req.Path),
				}, nil
			}
		}
	}

	// Step 3: check for existing active lock.
	lockRows, err := tx.QueryContext(ctx, `
		SELECT id, agent_id, lock_kind, expires_at FROM coordination_file_locks
		WHERE run_id = ? AND path = ? AND status = ?`,
		req.RunID, req.Path, StatusActive,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("coordination: acquire lock: check existing: %w", err)
	}
	type existingLock struct {
		id        string
		agentID   string
		lockKind  string
		expiresAt int64
	}
	var existing []existingLock
	for lockRows.Next() {
		var el existingLock
		if err := lockRows.Scan(&el.id, &el.agentID, &el.lockKind, &el.expiresAt); err != nil {
			lockRows.Close()
			return nil, nil, err
		}
		existing = append(existing, el)
	}
	lockRows.Close()

	for _, el := range existing {
		if el.agentID != req.AgentID {
			// Conflict: another agent holds the lock.
			if err := tx.Rollback(); err == nil {
				tx = nil
			}
			return nil, &LockConflict{
				Type:         "file_lock_conflict",
				Path:         req.Path,
				OwnerAgentID: el.agentID,
				Message:      fmt.Sprintf("file %s is locked (%s) by agent %s", req.Path, el.lockKind, el.agentID),
			}, nil
		}
		// Same agent: renew the lock.
		if _, err := tx.ExecContext(ctx,
			`UPDATE coordination_file_locks SET expires_at = ?, updated_at = ? WHERE id = ?`,
			expires, now, el.id,
		); err != nil {
			return nil, nil, fmt.Errorf("coordination: renew lock: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, fmt.Errorf("coordination: renew lock commit: %w", err)
		}
		tx = nil
		// Return the renewed lock.
		lk, err2 := s.getLockByID(ctx, el.id)
		if err2 != nil {
			// Construct from known data.
			lk = &FileLock{
				ID:        el.id,
				RunID:     req.RunID,
				AgentID:   req.AgentID,
				Path:      req.Path,
				LockKind:  el.lockKind,
				Reason:    req.Reason,
				Status:    StatusActive,
				ExpiresAt: expires,
				CreatedAt: now,
			}
		}
		return lk, nil, nil
	}

	// Step 4: insert new lock.
	id := "LOCK-" + uuid.New().String()[:8]
	lk := &FileLock{
		ID:        id,
		RunID:     req.RunID,
		AgentID:   req.AgentID,
		Path:      req.Path,
		LockKind:  req.LockKind,
		Reason:    req.Reason,
		Status:    StatusActive,
		CreatedAt: now,
		ExpiresAt: expires,
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO coordination_file_locks
		  (id, run_id, agent_id, path, lock_kind, reason, fingerprint_at_lock, status, created_at, expires_at, released_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lk.ID, lk.RunID, lk.AgentID, lk.Path, lk.LockKind, lk.Reason, "",
		lk.Status, lk.CreatedAt, lk.ExpiresAt, nil,
	); err != nil {
		return nil, nil, fmt.Errorf("coordination: insert lock: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("coordination: acquire lock commit: %w", err)
	}
	tx = nil
	return lk, nil, nil
}

// getLockByID retrieves a single lock by its ID.
func (s *Store) getLockByID(ctx context.Context, lockID string) (*FileLock, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, run_id, agent_id, path, lock_kind, reason, fingerprint_at_lock,
		       status, created_at, expires_at, released_at
		FROM coordination_file_locks WHERE id = ?`, lockID)

	lk := &FileLock{}
	var fp *string
	var releasedAt *int64
	if err := row.Scan(
		&lk.ID, &lk.RunID, &lk.AgentID, &lk.Path, &lk.LockKind, &lk.Reason, &fp,
		&lk.Status, &lk.CreatedAt, &lk.ExpiresAt, &releasedAt,
	); err != nil {
		return nil, err
	}
	if fp != nil {
		lk.FingerprintAtLock = *fp
	}
	if releasedAt != nil {
		lk.ReleasedAt = *releasedAt
	}
	return lk, nil
}

// ReleaseFileLock releases a lock held by a specific agent.
func (s *Store) ReleaseFileLock(ctx context.Context, runID, lockID, agentID string) error {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`UPDATE coordination_file_locks SET status = ?, released_at = ? WHERE id = ? AND agent_id = ? AND run_id = ?`,
		StatusReleased, now, lockID, agentID, runID,
	)
	if err != nil {
		return fmt.Errorf("coordination: release lock: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("coordination: release lock: lock %s not found or not owned by agent %s", lockID, agentID)
	}
	return nil
}

// ListActiveLocks returns all active locks for a run.
func (s *Store) ListActiveLocks(ctx context.Context, runID string) ([]FileLock, error) {
	now := time.Now().Unix()
	// Expire first.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE coordination_file_locks SET status = ? WHERE status = ? AND expires_at < ?`,
		StatusExpired, StatusActive, now,
	)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_id, path, lock_kind, reason, fingerprint_at_lock,
		       status, created_at, expires_at, released_at
		FROM coordination_file_locks WHERE run_id = ? AND status = ?`,
		runID, StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list active locks: %w", err)
	}
	defer rows.Close()

	var result []FileLock
	for rows.Next() {
		lk := FileLock{}
		var fp *string
		var releasedAt *int64
		if err := rows.Scan(
			&lk.ID, &lk.RunID, &lk.AgentID, &lk.Path, &lk.LockKind, &lk.Reason, &fp,
			&lk.Status, &lk.CreatedAt, &lk.ExpiresAt, &releasedAt,
		); err != nil {
			return nil, err
		}
		if fp != nil {
			lk.FingerprintAtLock = *fp
		}
		if releasedAt != nil {
			lk.ReleasedAt = *releasedAt
		}
		result = append(result, lk)
	}
	return result, rows.Err()
}
