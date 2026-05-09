package coordination

import (
	"context"
	"fmt"
	"time"
)

// GetCoordinationSnapshot assembles the full state of a coordination run.
// agentID is used to filter unread handoff notes (empty = all handoffs).
// queryFiles filters decisions to only those relevant to the given files (empty = all decisions).
func (s *Store) GetCoordinationSnapshot(ctx context.Context, runID string, agentID string, queryFiles []string) (*CoordinationSnapshot, error) {
	now := time.Now().Unix()

	// 1. Load run.
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: get run: %w", err)
	}

	// 2. Load agents.
	agentRows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, agent_name, agent_kind, session_id, role, status, heartbeat_at, created_at, updated_at
		FROM agent_participants WHERE run_id = ?
		ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: agents: %w", err)
	}
	var agents []AgentParticipant
	for agentRows.Next() {
		a := AgentParticipant{}
		var sessionID, role *string
		var heartbeatAt *int64
		if err := agentRows.Scan(
			&a.ID, &a.RunID, &a.AgentName, &a.AgentKind, &sessionID, &role,
			&a.Status, &heartbeatAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			agentRows.Close()
			return nil, err
		}
		if sessionID != nil {
			a.SessionID = *sessionID
		}
		if role != nil {
			a.Role = *role
		}
		if heartbeatAt != nil {
			a.HeartbeatAt = *heartbeatAt
		}
		agents = append(agents, a)
	}
	agentRows.Close()
	if err := agentRows.Err(); err != nil {
		return nil, err
	}

	// 3. Load work items.
	workItems, err := s.ListWorkItems(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: work items: %w", err)
	}

	// 4. Load active file claims (expire old ones first).
	_ = s.expireClaimsAt(ctx, now)
	activeClaims, err := s.ListActiveClaims(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: claims: %w", err)
	}

	// 5. Load active file locks (expire old ones first).
	_ = s.expireLocksAt(ctx, now)
	activeLocks, err := s.ListActiveLocks(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: locks: %w", err)
	}

	// 6. Load decisions.
	decisions, err := s.ListRelevantDecisions(ctx, runID, queryFiles, nil)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: decisions: %w", err)
	}

	// 7. Load assumptions.
	assumptions, err := s.ListAssumptions(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: assumptions: %w", err)
	}

	// 8. Load active warnings.
	warnings, err := s.ListActiveWarnings(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: warnings: %w", err)
	}

	// 9. Load open conflicts.
	allConflicts, err := s.ListConflicts(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: conflicts: %w", err)
	}
	var openConflicts []CoordinationConflict
	for _, c := range allConflicts {
		if c.Status == "open" {
			openConflicts = append(openConflicts, c)
		}
	}

	// 10. Load handoffs (unread for agentID, or all if empty).
	handoffs, err := s.ListHandoffs(ctx, runID, agentID)
	if err != nil {
		return nil, fmt.Errorf("coordination: snapshot: handoffs: %w", err)
	}
	// Filter to unread if agent specified.
	var filteredHandoffs []CoordinationHandoffNote
	if agentID != "" {
		for _, h := range handoffs {
			if h.ReadAt == 0 {
				filteredHandoffs = append(filteredHandoffs, h)
			}
		}
	} else {
		filteredHandoffs = handoffs
	}

	// 11. Build RecommendedRules.
	var rules []string

	// Binding decisions.
	for _, d := range decisions {
		if d.Binding && d.SupersededBy == "" {
			files := ""
			if len(d.RelatedFiles) > 0 {
				files = fmt.Sprintf("%v", d.RelatedFiles)
			}
			rules = append(rules, fmt.Sprintf("Do not edit %s — binding decision by %s: %s", files, d.AgentID, d.Title))
		}
	}

	// Active locks held by other agents.
	for _, lk := range activeLocks {
		if agentID != "" && lk.AgentID == agentID {
			continue
		}
		rules = append(rules, fmt.Sprintf("File %s is locked for %s by %s", lk.Path, lk.LockKind, lk.AgentID))
	}

	// Open conflicts.
	for _, c := range openConflicts {
		rules = append(rules, fmt.Sprintf("CONFLICT: %s", c.Message))
	}

	// Critical/unacknowledged warnings.
	for _, w := range warnings {
		if w.Severity == "critical" || w.Severity == "error" {
			rules = append(rules, fmt.Sprintf("WARNING: %s", w.Message))
		}
	}

	return &CoordinationSnapshot{
		Run:              *run,
		Agents:           agents,
		WorkItems:        workItems,
		ActiveClaims:     activeClaims,
		ActiveLocks:      activeLocks,
		Decisions:        decisions,
		Assumptions:      assumptions,
		Warnings:         warnings,
		OpenConflicts:    openConflicts,
		HandoffNotes:     filteredHandoffs,
		RecommendedRules: rules,
	}, nil
}
