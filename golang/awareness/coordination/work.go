package coordination

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateWorkItem creates a new work item in a coordination run.
func (s *Store) CreateWorkItem(ctx context.Context, req CreateWorkItemRequest) (*CoordinationWorkItem, error) {
	now := time.Now().Unix()
	priority := req.Priority
	if priority == "" {
		priority = "normal"
	}

	wi := &CoordinationWorkItem{
		ID:                "WORK-" + uuid.New().String()[:8],
		RunID:             req.RunID,
		Title:             req.Title,
		Description:       req.Description,
		Status:            "open",
		Priority:          priority,
		AssignedAgentID:   req.AssignedAgentID,
		RelatedFiles:      req.RelatedFiles,
		RelatedComponents: req.RelatedComponents,
		RelatedInvariants: req.RelatedInvariants,
		RelatedIncidents:  req.RelatedIncidents,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_work_items
		  (id, run_id, title, description, status, priority, assigned_agent_id, claimed_by_agent_id,
		   related_files, related_components, related_invariants, related_incidents,
		   created_at, updated_at, closed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		wi.ID, wi.RunID, wi.Title, wi.Description, wi.Status, wi.Priority,
		wi.AssignedAgentID, "",
		joinPipe(wi.RelatedFiles), joinPipe(wi.RelatedComponents),
		joinPipe(wi.RelatedInvariants), joinPipe(wi.RelatedIncidents),
		wi.CreatedAt, wi.UpdatedAt, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: create work item: %w", err)
	}
	return wi, nil
}

// ClaimWorkItem allows an agent to claim a work item.
// Returns an error if the work item is already claimed by another agent.
func (s *Store) ClaimWorkItem(ctx context.Context, runID, workItemID, agentID string) error {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		UPDATE coordination_work_items
		SET claimed_by_agent_id = ?, status = 'claimed', updated_at = ?
		WHERE id = ? AND run_id = ? AND (claimed_by_agent_id = '' OR claimed_by_agent_id = ? OR claimed_by_agent_id IS NULL)`,
		agentID, now, workItemID, runID, agentID,
	)
	if err != nil {
		return fmt.Errorf("coordination: claim work item: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("coordination: claim work item: work item %s is already claimed by another agent", workItemID)
	}
	return nil
}

// CompleteWorkItem marks a work item as done.
func (s *Store) CompleteWorkItem(ctx context.Context, runID, workItemID, agentID, summary string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE coordination_work_items
		SET status = 'done', closed_at = ?, updated_at = ?
		WHERE id = ? AND run_id = ?`,
		now, now, workItemID, runID,
	)
	return err
}

// ListWorkItems returns all work items for a run.
func (s *Store) ListWorkItems(ctx context.Context, runID string) ([]CoordinationWorkItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, title, description, status, priority, assigned_agent_id, claimed_by_agent_id,
		       related_files, related_components, related_invariants, related_incidents,
		       created_at, updated_at, closed_at
		FROM coordination_work_items WHERE run_id = ?
		ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list work items: %w", err)
	}
	defer rows.Close()

	var result []CoordinationWorkItem
	for rows.Next() {
		wi := CoordinationWorkItem{}
		var desc, assignedID, claimedID *string
		var relFiles, relComps, relInvs, relIncs *string
		var closedAt *int64
		if err := rows.Scan(
			&wi.ID, &wi.RunID, &wi.Title, &desc, &wi.Status, &wi.Priority,
			&assignedID, &claimedID,
			&relFiles, &relComps, &relInvs, &relIncs,
			&wi.CreatedAt, &wi.UpdatedAt, &closedAt,
		); err != nil {
			return nil, err
		}
		if desc != nil {
			wi.Description = *desc
		}
		if assignedID != nil {
			wi.AssignedAgentID = *assignedID
		}
		if claimedID != nil {
			wi.ClaimedByAgentID = *claimedID
		}
		if relFiles != nil {
			wi.RelatedFiles = splitPipe(*relFiles)
		}
		if relComps != nil {
			wi.RelatedComponents = splitPipe(*relComps)
		}
		if relInvs != nil {
			wi.RelatedInvariants = splitPipe(*relInvs)
		}
		if relIncs != nil {
			wi.RelatedIncidents = splitPipe(*relIncs)
		}
		if closedAt != nil {
			wi.ClosedAt = *closedAt
		}
		result = append(result, wi)
	}
	return result, rows.Err()
}
