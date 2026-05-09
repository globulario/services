package main

// tools_awareness_coordination.go: MCP tools for Agent Coordination Memory.
//
// Tools registered under the prefix "awareness.coordination.":
//   start_run, join, snapshot, claim_file, lock_file, release_lock,
//   record_decision, record_handoff, detect_conflicts, close_run, override_decision.

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/coordination"
)

func registerAwarenessCoordinationTools(s *server, st *awarenessState) {
	// ── start_run ────────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.start_run",
		Description: "Start a new multi-agent coordination run. Returns a run_id that all participating agents must pass to subsequent coordination tools.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"id":        {Type: "string", Description: "Optional explicit run ID (default: auto-generated RUN-XXXXXXXX)"},
				"title":     {Type: "string", Description: "Short human-readable title for the run"},
				"objective": {Type: "string", Description: "What the multi-agent run is trying to accomplish"},
				"owner":     {Type: "string", Description: "Agent ID of the run owner"},
				"repo":      {Type: "string", Description: "Absolute path to the repository root"},
				"branch":    {Type: "string", Description: "Git branch (auto-detected from repo if omitted)"},
			},
			Required: []string{"title", "objective"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		run, err := store.StartCoordinationRun(ctx, coordination.StartCoordinationRunRequest{
			ID:           strArg(args, "id"),
			Title:        strArg(args, "title"),
			Objective:    strArg(args, "objective"),
			OwnerAgentID: strArg(args, "owner"),
			RepoRoot:     strArg(args, "repo"),
			Branch:       strArg(args, "branch"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status": "started",
			"run_id": run.ID,
			"branch": run.Branch,
			"commit": run.GitCommitStart,
		}, nil
	})

	// ── join ────────────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.join",
		Description: "Join a coordination run as an agent participant. Returns an agent_id to use in all subsequent calls.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":     {Type: "string", Description: "Coordination run ID from start_run"},
				"agent_name": {Type: "string", Description: "Human-readable agent name (e.g. 'claude-refactor')"},
				"agent_kind": {Type: "string", Description: "Kind of agent: claude | gpt | human | ci"},
				"session_id": {Type: "string", Description: "Session ID from session.start (optional)"},
				"role":       {Type: "string", Description: "Role in this run: coder | reviewer | planner | executor"},
			},
			Required: []string{"run_id", "agent_name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		a, err := store.JoinCoordinationRun(ctx, coordination.JoinCoordinationRunRequest{
			RunID:     strArg(args, "run_id"),
			AgentName: strArg(args, "agent_name"),
			AgentKind: strArg(args, "agent_kind"),
			SessionID: strArg(args, "session_id"),
			Role:      strArg(args, "role"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":   "joined",
			"agent_id": a.ID,
			"run_id":   a.RunID,
		}, nil
	})

	// ── snapshot ────────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.snapshot",
		Description: "Get the full coordination run snapshot: agents, work items, file claims, locks, decisions, warnings, conflicts, handoffs. Call this to orient yourself before editing files.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":   {Type: "string", Description: "Coordination run ID"},
				"agent_id": {Type: "string", Description: "Your agent ID (used to filter unread handoffs)"},
				"files":    {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files you plan to work on (used to filter relevant decisions)"},
			},
			Required: []string{"run_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		snap, err := store.GetCoordinationSnapshot(ctx,
			strArg(args, "run_id"),
			strArg(args, "agent_id"),
			strSliceArg(args, "files"),
		)
		if err != nil {
			return nil, err
		}
		return snap, nil
	})

	// ── claim_file ───────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.claim_file",
		Description: "Declare intent to read or edit a file. Other agents will see this in the snapshot. Use before AcquireFileLock for edit-heavy workflows.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":     {Type: "string", Description: "Coordination run ID"},
				"agent_id":   {Type: "string", Description: "Your agent ID"},
				"file":       {Type: "string", Description: "File path to claim"},
				"claim_kind": {Type: "string", Description: "read | investigate | likely_edit | do_not_touch"},
				"reason":     {Type: "string", Description: "Why you are claiming this file"},
				"ttl":        {Type: "number", Description: "TTL in seconds (0 = use default based on kind)"},
			},
			Required: []string{"run_id", "agent_id", "file", "claim_kind"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		c, err := store.ClaimFile(ctx, coordination.ClaimFileRequest{
			RunID:     strArg(args, "run_id"),
			AgentID:   strArg(args, "agent_id"),
			Path:      strArg(args, "file"),
			ClaimKind: strArg(args, "claim_kind"),
			Reason:    strArg(args, "reason"),
			TTL:       int64(intArgDefault(args, "ttl", 0)),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":   "claimed",
			"claim_id": c.ID,
			"path":     c.Path,
		}, nil
	})

	// ── lock_file ────────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.lock_file",
		Description: "Acquire an exclusive file lock before making edits. Returns {status:locked} on success or {status:blocked, conflict:{...}} when another agent holds the lock or a binding do_not_touch decision exists.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":    {Type: "string", Description: "Coordination run ID"},
				"agent_id":  {Type: "string", Description: "Your agent ID"},
				"file":      {Type: "string", Description: "File path to lock"},
				"lock_kind": {Type: "string", Description: "edit | rename | delete | semantic_boundary | do_not_touch"},
				"reason":    {Type: "string", Description: "Why you need this lock"},
				"ttl":       {Type: "number", Description: "TTL in seconds (0 = use default)"},
			},
			Required: []string{"run_id", "agent_id", "file", "lock_kind", "reason"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		lk, conflict, err := store.AcquireFileLock(ctx, coordination.AcquireFileLockRequest{
			RunID:    strArg(args, "run_id"),
			AgentID:  strArg(args, "agent_id"),
			Path:     strArg(args, "file"),
			LockKind: strArg(args, "lock_kind"),
			Reason:   strArg(args, "reason"),
			TTL:      int64(intArgDefault(args, "ttl", 0)),
		})
		if err != nil {
			return nil, err
		}
		if conflict != nil {
			return map[string]interface{}{
				"status":   "blocked",
				"conflict": conflict,
			}, nil
		}
		return map[string]interface{}{
			"status":  "locked",
			"lock_id": lk.ID,
			"path":    lk.Path,
		}, nil
	})

	// ── release_lock ─────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.release_lock",
		Description: "Release a file lock after edits are complete. Always release locks when done editing to unblock other agents.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":   {Type: "string", Description: "Coordination run ID"},
				"agent_id": {Type: "string", Description: "Your agent ID"},
				"lock_id":  {Type: "string", Description: "Lock ID from lock_file"},
			},
			Required: []string{"run_id", "agent_id", "lock_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		if err := store.ReleaseFileLock(ctx,
			strArg(args, "run_id"),
			strArg(args, "lock_id"),
			strArg(args, "agent_id"),
		); err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "released"}, nil
	})

	// ── record_decision ──────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.record_decision",
		Description: "Record an architectural or operational decision that other agents in the run must respect. Binding decisions block conflicting file locks.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":     {Type: "string", Description: "Coordination run ID"},
				"agent_id":   {Type: "string", Description: "Your agent ID"},
				"title":      {Type: "string", Description: "Short decision title"},
				"decision":   {Type: "string", Description: "The decision statement"},
				"rationale":  {Type: "string", Description: "Why this decision was made"},
				"scope":      {Type: "string", Description: "global | file | component | service"},
				"files":      {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files covered by this decision"},
				"components": {Type: "array", Items: &propSchema{Type: "string"}, Description: "Components covered"},
				"invariants": {Type: "array", Items: &propSchema{Type: "string"}, Description: "Invariant IDs this upholds"},
				"incidents":  {Type: "array", Items: &propSchema{Type: "string"}, Description: "Incident IDs that motivated this"},
				"binding":    {Type: "boolean", Description: "If true, this decision blocks conflicting locks (default: false)"},
			},
			Required: []string{"run_id", "agent_id", "title", "decision", "rationale", "scope"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		d, err := store.RecordCoordinationDecision(ctx, coordination.RecordDecisionRequest{
			RunID:             strArg(args, "run_id"),
			AgentID:           strArg(args, "agent_id"),
			Title:             strArg(args, "title"),
			Decision:          strArg(args, "decision"),
			Rationale:         strArg(args, "rationale"),
			Scope:             strArg(args, "scope"),
			RelatedFiles:      strSliceArg(args, "files"),
			RelatedComponents: strSliceArg(args, "components"),
			RelatedInvariants: strSliceArg(args, "invariants"),
			RelatedIncidents:  strSliceArg(args, "incidents"),
			Binding:           boolArg(args, "binding"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":      "recorded",
			"decision_id": d.ID,
			"binding":     d.Binding,
		}, nil
	})

	// ── record_handoff ───────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.record_handoff",
		Description: "Record a handoff note from one agent to another. The target agent will see it in their snapshot's handoff_notes section.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":       {Type: "string", Description: "Coordination run ID"},
				"from_agent":   {Type: "string", Description: "Your agent ID"},
				"to_agent":     {Type: "string", Description: "Target agent ID (empty = broadcast to all)"},
				"work_item_id": {Type: "string", Description: "Related work item ID (optional)"},
				"title":        {Type: "string", Description: "Short handoff title"},
				"body":         {Type: "string", Description: "Full handoff notes — what you did, what remains, what to watch out for"},
				"files":        {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files relevant to the handoff"},
			},
			Required: []string{"run_id", "from_agent", "title", "body"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		h, err := store.RecordHandoff(ctx, coordination.RecordHandoffRequest{
			RunID:        strArg(args, "run_id"),
			FromAgentID:  strArg(args, "from_agent"),
			ToAgentID:    strArg(args, "to_agent"),
			WorkItemID:   strArg(args, "work_item_id"),
			Title:        strArg(args, "title"),
			Body:         strArg(args, "body"),
			RelatedFiles: strSliceArg(args, "files"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":     "recorded",
			"handoff_id": h.ID,
		}, nil
	})

	// ── detect_conflicts ─────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.detect_conflicts",
		Description: "Detect all conflicts in a coordination run: overlapping edit claims, do_not_touch violations, lock conflicts. Returns all open conflicts.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id": {Type: "string", Description: "Coordination run ID"},
			},
			Required: []string{"run_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		conflicts, err := store.DetectCoordinationConflicts(ctx, strArg(args, "run_id"))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"conflicts": conflicts,
			"count":     len(conflicts),
		}, nil
	})

	// ── close_run ────────────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.close_run",
		Description: "Close a coordination run. Always closes the run but reports active locks, open conflicts, and other blockers in the snapshot's recommended_rules.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id": {Type: "string", Description: "Coordination run ID to close"},
			},
			Required: []string{"run_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		snap, err := store.CloseCoordinationRun(ctx, strArg(args, "run_id"))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":            "closed",
			"run_id":            snap.Run.ID,
			"recommended_rules": snap.RecommendedRules,
			"snapshot":          snap,
		}, nil
	})

	// ── override_decision ────────────────────────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.coordination.override_decision",
		Description: "Override a binding decision made by another agent. Records a conflict event. Use only in genuine emergencies — prefer amending the decision cooperatively.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"run_id":      {Type: "string", Description: "Coordination run ID"},
				"agent_id":    {Type: "string", Description: "Your agent ID"},
				"decision_id": {Type: "string", Description: "Decision ID to override"},
				"reason":      {Type: "string", Description: "Why you are overriding this decision"},
				"evidence":    {Type: "string", Description: "Evidence or justification supporting the override"},
			},
			Required: []string{"run_id", "agent_id", "decision_id", "reason"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		store := coordination.New(st.g)
		conflict, err := store.OverrideDecision(ctx,
			strArg(args, "run_id"),
			strArg(args, "agent_id"),
			strArg(args, "decision_id"),
			strArg(args, "reason"),
			strArg(args, "evidence"),
		)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":      "overridden",
			"conflict_id": conflict.ID,
			"message":     conflict.Message,
		}, nil
	})
}
