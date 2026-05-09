package coordination

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RecordCoordinationWarning records a warning in a coordination run.
func (s *Store) RecordCoordinationWarning(ctx context.Context, req RecordWarningRequest) (*CoordinationWarning, error) {
	now := time.Now().Unix()
	w := &CoordinationWarning{
		ID:               "WARN-" + uuid.New().String()[:8],
		RunID:            req.RunID,
		AgentID:          req.AgentID,
		WarningType:      req.WarningType,
		Severity:         req.Severity,
		Message:          req.Message,
		RelatedFile:      req.RelatedFile,
		RelatedComponent: req.RelatedComponent,
		RelatedIncident:  req.RelatedIncident,
		Status:           StatusActive,
		CreatedAt:        now,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_warnings
		  (id, run_id, agent_id, warning_type, severity, message,
		   related_file, related_component, related_incident, status, created_at, acknowledged_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.RunID, w.AgentID, w.WarningType, w.Severity, w.Message,
		w.RelatedFile, w.RelatedComponent, w.RelatedIncident, w.Status, w.CreatedAt, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: record warning: %w", err)
	}
	return w, nil
}

// ListActiveWarnings returns all active warnings for a run.
func (s *Store) ListActiveWarnings(ctx context.Context, runID string) ([]CoordinationWarning, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_id, warning_type, severity, message,
		       related_file, related_component, related_incident, status, created_at, acknowledged_at
		FROM coordination_warnings WHERE run_id = ? AND status = ?
		ORDER BY created_at ASC`,
		runID, StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list active warnings: %w", err)
	}
	defer rows.Close()

	return scanWarnings(rows)
}

// AcknowledgeWarning marks a warning as acknowledged.
func (s *Store) AcknowledgeWarning(ctx context.Context, runID, warningID string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE coordination_warnings SET status = 'acknowledged', acknowledged_at = ? WHERE id = ? AND run_id = ?`,
		now, warningID, runID,
	)
	return err
}

// RecordHandoff records a handoff note from one agent to another.
func (s *Store) RecordHandoff(ctx context.Context, req RecordHandoffRequest) (*CoordinationHandoffNote, error) {
	now := time.Now().Unix()
	h := &CoordinationHandoffNote{
		ID:           "HANDOFF-" + uuid.New().String()[:8],
		RunID:        req.RunID,
		FromAgentID:  req.FromAgentID,
		ToAgentID:    req.ToAgentID,
		WorkItemID:   req.WorkItemID,
		Title:        req.Title,
		Body:         req.Body,
		RelatedFiles: req.RelatedFiles,
		CreatedAt:    now,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_handoff_notes
		  (id, run_id, from_agent_id, to_agent_id, work_item_id, title, body, related_files, created_at, read_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.RunID, h.FromAgentID, h.ToAgentID, h.WorkItemID,
		h.Title, h.Body, joinPipe(h.RelatedFiles), h.CreatedAt, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: record handoff: %w", err)
	}
	return h, nil
}

// ListHandoffs returns handoff notes for a run, optionally filtered by target agent.
// If toAgentID is empty, returns all handoffs for the run.
func (s *Store) ListHandoffs(ctx context.Context, runID, toAgentID string) ([]CoordinationHandoffNote, error) {
	var rows interface {
		Next() bool
		Scan(...interface{}) error
		Err() error
		Close() error
	}
	var err error

	if toAgentID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, run_id, from_agent_id, to_agent_id, work_item_id, title, body, related_files, created_at, read_at
			FROM coordination_handoff_notes WHERE run_id = ? AND (to_agent_id = ? OR to_agent_id = '' OR to_agent_id IS NULL)
			ORDER BY created_at ASC`,
			runID, toAgentID,
		)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, run_id, from_agent_id, to_agent_id, work_item_id, title, body, related_files, created_at, read_at
			FROM coordination_handoff_notes WHERE run_id = ?
			ORDER BY created_at ASC`,
			runID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("coordination: list handoffs: %w", err)
	}
	defer rows.Close()

	var result []CoordinationHandoffNote
	for rows.Next() {
		h := CoordinationHandoffNote{}
		var toAgent, workItem, relFiles *string
		var readAt *int64
		if err := rows.Scan(
			&h.ID, &h.RunID, &h.FromAgentID, &toAgent, &workItem,
			&h.Title, &h.Body, &relFiles, &h.CreatedAt, &readAt,
		); err != nil {
			return nil, err
		}
		if toAgent != nil {
			h.ToAgentID = *toAgent
		}
		if workItem != nil {
			h.WorkItemID = *workItem
		}
		if relFiles != nil {
			h.RelatedFiles = splitPipe(*relFiles)
		}
		if readAt != nil {
			h.ReadAt = *readAt
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// MarkHandoffRead marks a handoff note as read.
func (s *Store) MarkHandoffRead(ctx context.Context, handoffID string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE coordination_handoff_notes SET read_at = ? WHERE id = ?`,
		now, handoffID,
	)
	return err
}

// scanWarnings is a shared scanner for coordination_warnings rows.
func scanWarnings(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]CoordinationWarning, error) {
	var result []CoordinationWarning
	for rows.Next() {
		w := CoordinationWarning{}
		var agentID, relFile, relComp, relInc *string
		var ackAt *int64
		if err := rows.Scan(
			&w.ID, &w.RunID, &agentID, &w.WarningType, &w.Severity, &w.Message,
			&relFile, &relComp, &relInc, &w.Status, &w.CreatedAt, &ackAt,
		); err != nil {
			return nil, err
		}
		if agentID != nil {
			w.AgentID = *agentID
		}
		if relFile != nil {
			w.RelatedFile = *relFile
		}
		if relComp != nil {
			w.RelatedComponent = *relComp
		}
		if relInc != nil {
			w.RelatedIncident = *relInc
		}
		if ackAt != nil {
			w.AcknowledgedAt = *ackAt
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
