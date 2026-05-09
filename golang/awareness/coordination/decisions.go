package coordination

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RecordCoordinationDecision records a decision made during a coordination run.
func (s *Store) RecordCoordinationDecision(ctx context.Context, req RecordDecisionRequest) (*CoordinationDecision, error) {
	now := time.Now().Unix()
	d := &CoordinationDecision{
		ID:                "DEC-" + uuid.New().String()[:8],
		RunID:             req.RunID,
		AgentID:           req.AgentID,
		Title:             req.Title,
		Decision:          req.Decision,
		Rationale:         req.Rationale,
		Scope:             req.Scope,
		RelatedFiles:      req.RelatedFiles,
		RelatedComponents: req.RelatedComponents,
		RelatedInvariants: req.RelatedInvariants,
		RelatedIncidents:  req.RelatedIncidents,
		Binding:           req.Binding,
		SupersededBy:      "",
		CreatedAt:         now,
	}

	bindingInt := 0
	if d.Binding {
		bindingInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_decisions
		  (id, run_id, agent_id, title, decision, rationale, scope,
		   related_files, related_components, related_invariants, related_incidents,
		   binding, superseded_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.RunID, d.AgentID, d.Title, d.Decision, d.Rationale, d.Scope,
		joinPipe(d.RelatedFiles), joinPipe(d.RelatedComponents),
		joinPipe(d.RelatedInvariants), joinPipe(d.RelatedIncidents),
		bindingInt, d.SupersededBy, d.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: record decision: %w", err)
	}
	return d, nil
}

// ListRelevantDecisions returns decisions that are relevant to the given files or components,
// plus all global-scope decisions.
func (s *Store) ListRelevantDecisions(ctx context.Context, runID string, files []string, components []string) ([]CoordinationDecision, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_id, title, decision, rationale, scope,
		       related_files, related_components, related_invariants, related_incidents,
		       binding, superseded_by, created_at
		FROM coordination_decisions WHERE run_id = ?
		ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list decisions: %w", err)
	}

	// Drain all rows first before any nested queries.
	type rawDec struct {
		id, runID, agentID, title, decision, rationale, scope string
		relFiles, relComps, relInvs, relIncs                  string
		binding                                                int
		supersededBy                                           string
		createdAt                                              int64
	}
	var raw []rawDec
	for rows.Next() {
		var d rawDec
		var supersededBy *string
		if err := rows.Scan(
			&d.id, &d.runID, &d.agentID, &d.title, &d.decision, &d.rationale, &d.scope,
			&d.relFiles, &d.relComps, &d.relInvs, &d.relIncs,
			&d.binding, &supersededBy, &d.createdAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		if supersededBy != nil {
			d.supersededBy = *supersededBy
		}
		raw = append(raw, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []CoordinationDecision
	for _, d := range raw {
		// Include if global scope, or if any matching file/component.
		include := d.scope == "global"
		if !include && len(files) > 0 {
			for _, f := range files {
				if strings.Contains(d.relFiles, f) {
					include = true
					break
				}
			}
		}
		if !include && len(components) > 0 {
			for _, c := range components {
				if strings.Contains(d.relComps, c) {
					include = true
					break
				}
			}
		}
		if !include && len(files) == 0 && len(components) == 0 {
			include = true
		}
		if !include {
			continue
		}
		dec := CoordinationDecision{
			ID:                d.id,
			RunID:             d.runID,
			AgentID:           d.agentID,
			Title:             d.title,
			Decision:          d.decision,
			Rationale:         d.rationale,
			Scope:             d.scope,
			RelatedFiles:      splitPipe(d.relFiles),
			RelatedComponents: splitPipe(d.relComps),
			RelatedInvariants: splitPipe(d.relInvs),
			RelatedIncidents:  splitPipe(d.relIncs),
			Binding:           d.binding == 1,
			SupersededBy:      d.supersededBy,
			CreatedAt:         d.createdAt,
		}
		result = append(result, dec)
	}
	return result, nil
}

// OverrideDecision marks a decision as superseded and records a conflict event.
func (s *Store) OverrideDecision(ctx context.Context, runID, agentID, decisionID, reason, evidence string) (*CoordinationConflict, error) {
	_, err := s.db.ExecContext(ctx,
		`UPDATE coordination_decisions SET superseded_by = 'overridden' WHERE id = ? AND run_id = ?`,
		decisionID, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: override decision: %w", err)
	}

	conflict := CoordinationConflict{
		RunID:        runID,
		ConflictType: "decision_conflict",
		Severity:     "warning",
		AgentA:       agentID,
		Message:      fmt.Sprintf("agent %s overrode decision %s: %s", agentID, decisionID, reason),
		Resolution:   evidence,
		Status:       "open",
		CreatedAt:    time.Now().Unix(),
	}
	return s.RecordConflict(ctx, conflict)
}

// RecordCoordinationAssumption records an assumption made during a run.
func (s *Store) RecordCoordinationAssumption(ctx context.Context, runID, agentID, assumption, basis, validationPlan, relatedFiles string) (*CoordinationAssumption, error) {
	now := time.Now().Unix()
	a := &CoordinationAssumption{
		ID:             "ASMP-" + uuid.New().String()[:8],
		RunID:          runID,
		AgentID:        agentID,
		Assumption:     assumption,
		Basis:          basis,
		Status:         "unverified",
		ValidationPlan: validationPlan,
		RelatedFiles:   relatedFiles,
		CreatedAt:      now,
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_assumptions
		  (id, run_id, agent_id, assumption, basis, status, validation_plan, related_files, created_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RunID, a.AgentID, a.Assumption, a.Basis,
		a.Status, a.ValidationPlan, a.RelatedFiles, a.CreatedAt, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: record assumption: %w", err)
	}
	return a, nil
}

// ListAssumptions returns all assumptions for a run.
func (s *Store) ListAssumptions(ctx context.Context, runID string) ([]CoordinationAssumption, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_id, assumption, basis, status, validation_plan, related_files, created_at, resolved_at
		FROM coordination_assumptions WHERE run_id = ?
		ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: list assumptions: %w", err)
	}
	defer rows.Close()

	var result []CoordinationAssumption
	for rows.Next() {
		a := CoordinationAssumption{}
		var basis, vplan, relFiles *string
		var resolvedAt *int64
		if err := rows.Scan(
			&a.ID, &a.RunID, &a.AgentID, &a.Assumption, &basis,
			&a.Status, &vplan, &relFiles, &a.CreatedAt, &resolvedAt,
		); err != nil {
			return nil, err
		}
		if basis != nil {
			a.Basis = *basis
		}
		if vplan != nil {
			a.ValidationPlan = *vplan
		}
		if relFiles != nil {
			a.RelatedFiles = *relFiles
		}
		if resolvedAt != nil {
			a.ResolvedAt = *resolvedAt
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
