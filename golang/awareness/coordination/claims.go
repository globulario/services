package coordination

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// defaultClaimTTL returns the default TTL for a claim kind.
func defaultClaimTTL(kind string) int64 {
	switch kind {
	case ClaimInvestigate:
		return TTLInvestigateClaim
	case ClaimLikelyEdit:
		return TTLLikelyEditClaim
	default:
		return TTLReadClaim
	}
}

// expireClaimsAt expires all active claims older than `now`.
func (s *Store) expireClaimsAt(ctx context.Context, now int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE coordination_file_claims SET status = ? WHERE status = ? AND expires_at IS NOT NULL AND expires_at < ?`,
		StatusExpired, StatusActive, now,
	)
	return err
}

// ClaimFile creates a file claim for an agent.
func (s *Store) ClaimFile(ctx context.Context, req ClaimFileRequest) (*FileClaim, error) {
	now := time.Now().Unix()

	ttl := req.TTL
	if ttl == 0 {
		ttl = defaultClaimTTL(req.ClaimKind)
	}
	expires := now + ttl

	id := "CLAIM-" + uuid.New().String()[:8]
	c := &FileClaim{
		ID:        id,
		RunID:     req.RunID,
		AgentID:   req.AgentID,
		Path:      req.Path,
		ClaimKind: req.ClaimKind,
		Reason:    req.Reason,
		Status:    StatusActive,
		CreatedAt: now,
		ExpiresAt: expires,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_file_claims
		  (id, run_id, agent_id, path, claim_kind, reason, status, created_at, expires_at, released_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.RunID, c.AgentID, c.Path, c.ClaimKind, c.Reason,
		c.Status, c.CreatedAt, c.ExpiresAt, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: claim file: %w", err)
	}
	return c, nil
}

// ReleaseClaim releases a file claim held by an agent.
func (s *Store) ReleaseClaim(ctx context.Context, runID, claimID, agentID string) error {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`UPDATE coordination_file_claims SET status = ?, released_at = ? WHERE id = ? AND agent_id = ? AND run_id = ?`,
		StatusReleased, now, claimID, agentID, runID,
	)
	if err != nil {
		return fmt.Errorf("coordination: release claim: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("coordination: release claim: claim %s not found or not owned by agent %s", claimID, agentID)
	}
	return nil
}

// ListActiveClaims returns all active claims for a run.
func (s *Store) ListActiveClaims(ctx context.Context, runID string) ([]FileClaim, error) {
	now := time.Now().Unix()
	_ = s.expireClaimsAt(ctx, now)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_id, path, claim_kind, reason, status, created_at, expires_at, released_at
		FROM coordination_file_claims WHERE run_id = ? AND status = ?`,
		runID, StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list active claims: %w", err)
	}
	defer rows.Close()

	return scanClaims(rows)
}

// ListActiveClaimsForPath returns all active claims for a specific path in a run.
func (s *Store) ListActiveClaimsForPath(ctx context.Context, runID, path string) ([]FileClaim, error) {
	now := time.Now().Unix()
	_ = s.expireClaimsAt(ctx, now)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_id, path, claim_kind, reason, status, created_at, expires_at, released_at
		FROM coordination_file_claims WHERE run_id = ? AND path = ? AND status = ?`,
		runID, path, StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list active claims for path: %w", err)
	}
	defer rows.Close()

	return scanClaims(rows)
}

func scanClaims(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]FileClaim, error) {
	var result []FileClaim
	for rows.Next() {
		c := FileClaim{}
		var reason *string
		var expiresAt, releasedAt *int64
		if err := rows.Scan(
			&c.ID, &c.RunID, &c.AgentID, &c.Path, &c.ClaimKind, &reason,
			&c.Status, &c.CreatedAt, &expiresAt, &releasedAt,
		); err != nil {
			return nil, err
		}
		if reason != nil {
			c.Reason = *reason
		}
		if expiresAt != nil {
			c.ExpiresAt = *expiresAt
		}
		if releasedAt != nil {
			c.ReleasedAt = *releasedAt
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
