package coordination

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

// gitBranchAndCommit returns the current git branch and commit hash.
// It returns empty strings on any error.
func gitBranchAndCommit(repoRoot string) (branch, commit string) {
	if repoRoot == "" {
		return "", ""
	}
	bOut, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err == nil {
		branch = strings.TrimSpace(string(bOut))
	}
	cOut, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err == nil {
		commit = strings.TrimSpace(string(cOut))
	}
	return branch, commit
}

// StartCoordinationRun creates a new coordination run.
func (s *Store) StartCoordinationRun(ctx context.Context, req StartCoordinationRunRequest) (*CoordinationRun, error) {
	id := req.ID
	if id == "" {
		id = "RUN-" + uuid.New().String()[:8]
	}

	branch := req.Branch
	commit := ""
	if req.RepoRoot != "" {
		b, c := gitBranchAndCommit(req.RepoRoot)
		if branch == "" {
			branch = b
		}
		commit = c
	}

	now := time.Now().Unix()
	r := &CoordinationRun{
		ID:             id,
		Title:          req.Title,
		Objective:      req.Objective,
		Status:         StatusOpen,
		OwnerAgentID:   req.OwnerAgentID,
		RepoRoot:       req.RepoRoot,
		Branch:         branch,
		GitCommitStart: commit,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_runs
		  (id, title, objective, status, owner_agent_id, repo_root, branch, git_commit_start, git_commit_end, created_at, updated_at, closed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Title, r.Objective, r.Status,
		r.OwnerAgentID, r.RepoRoot, r.Branch, r.GitCommitStart, "",
		r.CreatedAt, r.UpdatedAt, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: start run: %w", err)
	}
	return r, nil
}

// GetRun retrieves a coordination run by ID.
func (s *Store) GetRun(ctx context.Context, runID string) (*CoordinationRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, objective, status, owner_agent_id, repo_root, branch,
		       git_commit_start, git_commit_end, created_at, updated_at, closed_at
		FROM coordination_runs WHERE id = ?`, runID)

	r := &CoordinationRun{}
	var ownerID, repoRoot, branch, commitStart, commitEnd *string
	var closedAt *int64
	if err := row.Scan(
		&r.ID, &r.Title, &r.Objective, &r.Status,
		&ownerID, &repoRoot, &branch, &commitStart, &commitEnd,
		&r.CreatedAt, &r.UpdatedAt, &closedAt,
	); err != nil {
		return nil, fmt.Errorf("coordination: get run %s: %w", runID, err)
	}
	if ownerID != nil {
		r.OwnerAgentID = *ownerID
	}
	if repoRoot != nil {
		r.RepoRoot = *repoRoot
	}
	if branch != nil {
		r.Branch = *branch
	}
	if commitStart != nil {
		r.GitCommitStart = *commitStart
	}
	if commitEnd != nil {
		r.GitCommitEnd = *commitEnd
	}
	if closedAt != nil {
		r.ClosedAt = *closedAt
	}
	return r, nil
}

// ListRuns returns all coordination runs, optionally filtered by status.
func (s *Store) ListRuns(ctx context.Context, status string) ([]CoordinationRun, error) {
	query := `SELECT id, title, objective, status, owner_agent_id, repo_root, branch,
		       git_commit_start, git_commit_end, created_at, updated_at, closed_at
		FROM coordination_runs`
	var args []interface{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("coordination: list runs: %w", err)
	}
	defer rows.Close()

	var result []CoordinationRun
	for rows.Next() {
		r := CoordinationRun{}
		var ownerID, repoRoot, branch, commitStart, commitEnd *string
		var closedAt *int64
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Objective, &r.Status,
			&ownerID, &repoRoot, &branch, &commitStart, &commitEnd,
			&r.CreatedAt, &r.UpdatedAt, &closedAt,
		); err != nil {
			return nil, fmt.Errorf("coordination: list runs scan: %w", err)
		}
		if ownerID != nil {
			r.OwnerAgentID = *ownerID
		}
		if repoRoot != nil {
			r.RepoRoot = *repoRoot
		}
		if branch != nil {
			r.Branch = *branch
		}
		if commitStart != nil {
			r.GitCommitStart = *commitStart
		}
		if commitEnd != nil {
			r.GitCommitEnd = *commitEnd
		}
		if closedAt != nil {
			r.ClosedAt = *closedAt
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// JoinCoordinationRun adds an agent participant to a run.
func (s *Store) JoinCoordinationRun(ctx context.Context, req JoinCoordinationRunRequest) (*AgentParticipant, error) {
	now := time.Now().Unix()
	a := &AgentParticipant{
		ID:          "AGENT-" + uuid.New().String()[:8],
		RunID:       req.RunID,
		AgentName:   req.AgentName,
		AgentKind:   req.AgentKind,
		SessionID:   req.SessionID,
		Role:        req.Role,
		Status:      AgentActive,
		HeartbeatAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_participants
		  (id, run_id, agent_name, agent_kind, session_id, role, status, heartbeat_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RunID, a.AgentName, a.AgentKind, a.SessionID, a.Role,
		a.Status, a.HeartbeatAt, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: join run: %w", err)
	}
	return a, nil
}

// HeartbeatAgent updates the last heartbeat for an agent.
func (s *Store) HeartbeatAgent(ctx context.Context, runID, agentID string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_participants SET heartbeat_at = ?, updated_at = ? WHERE id = ? AND run_id = ?`,
		now, now, agentID, runID,
	)
	return err
}

// LeaveCoordinationRun updates an agent's status when it leaves a run.
func (s *Store) LeaveCoordinationRun(ctx context.Context, runID, agentID, status string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_participants SET status = ?, updated_at = ? WHERE id = ? AND run_id = ?`,
		status, now, agentID, runID,
	)
	return err
}

// CloseCoordinationRun closes a coordination run and returns the final snapshot.
// It always closes the run (sets status=closed) but reports blockers in RecommendedRules.
func (s *Store) CloseCoordinationRun(ctx context.Context, runID string) (*CoordinationSnapshot, error) {
	now := time.Now().Unix()

	// Always close the run.
	_, err := s.db.ExecContext(ctx,
		`UPDATE coordination_runs SET status = ?, closed_at = ?, updated_at = ? WHERE id = ?`,
		StatusClosed, now, now, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: close run: %w", err)
	}

	// Build the snapshot to return.
	snap, err := s.GetCoordinationSnapshot(ctx, runID, "", nil)
	if err != nil {
		return nil, fmt.Errorf("coordination: close run snapshot: %w", err)
	}

	// Report blockers in RecommendedRules.
	activeLocks, _ := s.ListActiveLocks(ctx, runID)
	for _, lk := range activeLocks {
		snap.RecommendedRules = append(snap.RecommendedRules,
			fmt.Sprintf("WARNING: Run closed with active lock on %s (kind=%s, agent=%s)", lk.Path, lk.LockKind, lk.AgentID))
	}

	openConflicts, _ := s.ListConflicts(ctx, runID)
	for _, c := range openConflicts {
		if c.Status == "open" {
			snap.RecommendedRules = append(snap.RecommendedRules,
				fmt.Sprintf("WARNING: Run closed with open conflict: %s", c.Message))
		}
	}

	return snap, nil
}
