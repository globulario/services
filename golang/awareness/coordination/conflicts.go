package coordination

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RecordConflict records a coordination conflict.
func (s *Store) RecordConflict(ctx context.Context, c CoordinationConflict) (*CoordinationConflict, error) {
	if c.ID == "" {
		c.ID = "CONFLICT-" + uuid.New().String()[:8]
	}
	if c.CreatedAt == 0 {
		c.CreatedAt = time.Now().Unix()
	}
	if c.Status == "" {
		c.Status = "open"
	}

	var resolvedAt interface{} = nil
	if c.ResolvedAt != 0 {
		resolvedAt = c.ResolvedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_conflicts
		  (id, run_id, conflict_type, severity, agent_a, agent_b, path, symbol, message, resolution, status, created_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.RunID, c.ConflictType, c.Severity, c.AgentA, c.AgentB,
		c.Path, c.Symbol, c.Message, c.Resolution, c.Status, c.CreatedAt, resolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: record conflict: %w", err)
	}
	return &c, nil
}

// DetectCoordinationConflicts detects conflicts in a coordination run and returns them all.
func (s *Store) DetectCoordinationConflicts(ctx context.Context, runID string) ([]CoordinationConflict, error) {
	now := time.Now().Unix()

	// 1. Expire old claims before checking.
	_ = s.expireClaimsAt(ctx, now)

	// Load active claims.
	claimRows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, path, claim_kind FROM coordination_file_claims
		WHERE run_id = ? AND status = ?`,
		runID, StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: detect conflicts: claims: %w", err)
	}
	type claimEntry struct {
		id        string
		agentID   string
		path      string
		claimKind string
	}
	var claims []claimEntry
	for claimRows.Next() {
		var ce claimEntry
		if err := claimRows.Scan(&ce.id, &ce.agentID, &ce.path, &ce.claimKind); err != nil {
			claimRows.Close()
			return nil, err
		}
		claims = append(claims, ce)
	}
	claimRows.Close()

	// 2. Load binding do_not_touch decisions.
	decRows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, related_files FROM coordination_decisions
		WHERE run_id = ? AND binding = 1 AND (superseded_by = '' OR superseded_by IS NULL)`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: detect conflicts: decisions: %w", err)
	}
	type decEntry struct {
		id           string
		agentID      string
		relatedFiles string
	}
	var decs []decEntry
	for decRows.Next() {
		var d decEntry
		if err := decRows.Scan(&d.id, &d.agentID, &d.relatedFiles); err != nil {
			decRows.Close()
			return nil, err
		}
		decs = append(decs, d)
	}
	decRows.Close()

	// 3. Load active locks.
	lockRows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, path, lock_kind FROM coordination_file_locks
		WHERE run_id = ? AND status = ? AND expires_at > ?`,
		runID, StatusActive, now,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: detect conflicts: locks: %w", err)
	}
	type lockEntry struct {
		id       string
		agentID  string
		path     string
		lockKind string
	}
	var locks []lockEntry
	for lockRows.Next() {
		var le lockEntry
		if err := lockRows.Scan(&le.id, &le.agentID, &le.path, &le.lockKind); err != nil {
			lockRows.Close()
			return nil, err
		}
		locks = append(locks, le)
	}
	lockRows.Close()

	var newConflicts []CoordinationConflict

	// Detect overlapping edit claims from different agents on the same path.
	editClaimsByPath := make(map[string][]claimEntry)
	for _, c := range claims {
		if c.claimKind == ClaimLikelyEdit || c.claimKind == ClaimDoNotTouch {
			editClaimsByPath[c.path] = append(editClaimsByPath[c.path], c)
		}
	}
	for path, cs := range editClaimsByPath {
		if len(cs) < 2 {
			continue
		}
		for i := 0; i < len(cs); i++ {
			for j := i + 1; j < len(cs); j++ {
				if cs[i].agentID != cs[j].agentID {
					newConflicts = append(newConflicts, CoordinationConflict{
						RunID:        runID,
						ConflictType: "overlapping_edit_claim",
						Severity:     "warning",
						AgentA:       cs[i].agentID,
						AgentB:       cs[j].agentID,
						Path:         path,
						Message:      fmt.Sprintf("agents %s and %s both have edit claims on %s", cs[i].agentID, cs[j].agentID, path),
						Status:       "open",
						CreatedAt:    now,
					})
				}
			}
		}
	}

	// Detect do_not_touch violations: active edit/rename/delete locks on do_not_touch paths.
	for _, lk := range locks {
		if lk.lockKind != LockEdit && lk.lockKind != LockRename && lk.lockKind != LockDelete {
			continue
		}
		for _, d := range decs {
			if strings.Contains(d.relatedFiles, lk.path) {
				newConflicts = append(newConflicts, CoordinationConflict{
					RunID:        runID,
					ConflictType: "do_not_touch_violation",
					Severity:     "error",
					AgentA:       lk.agentID,
					Path:         lk.path,
					Message:      fmt.Sprintf("lock (%s) on %s violates binding do_not_touch decision %s", lk.lockKind, lk.path, d.id),
					Status:       "open",
					CreatedAt:    now,
				})
			}
		}
	}

	// Record newly detected conflicts.
	for _, c := range newConflicts {
		if _, err := s.RecordConflict(ctx, c); err != nil {
			// If conflict already exists, skip (best effort).
			_ = err
		}
	}

	// Load all open conflicts (including previously recorded ones).
	return s.ListConflicts(ctx, runID)
}

// ListConflicts returns all conflicts for a run.
func (s *Store) ListConflicts(ctx context.Context, runID string) ([]CoordinationConflict, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, conflict_type, severity, agent_a, agent_b, path, symbol, message, resolution, status, created_at, resolved_at
		FROM coordination_conflicts WHERE run_id = ?
		ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list conflicts: %w", err)
	}
	defer rows.Close()

	var result []CoordinationConflict
	for rows.Next() {
		c := CoordinationConflict{}
		var agentA, agentB, path, symbol, resolution *string
		var resolvedAt *int64
		if err := rows.Scan(
			&c.ID, &c.RunID, &c.ConflictType, &c.Severity,
			&agentA, &agentB, &path, &symbol, &c.Message, &resolution,
			&c.Status, &c.CreatedAt, &resolvedAt,
		); err != nil {
			return nil, err
		}
		if agentA != nil {
			c.AgentA = *agentA
		}
		if agentB != nil {
			c.AgentB = *agentB
		}
		if path != nil {
			c.Path = *path
		}
		if symbol != nil {
			c.Symbol = *symbol
		}
		if resolution != nil {
			c.Resolution = *resolution
		}
		if resolvedAt != nil {
			c.ResolvedAt = *resolvedAt
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
