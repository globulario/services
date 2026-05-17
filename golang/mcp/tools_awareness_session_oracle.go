package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/awareness/sessionoracle"
)

func registerAwarenessSessionOracleTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.session.start",
		Description: "Start a new agent session in the oracle. Call at the beginning of work. Returns the session ID to pass to all subsequent session tools.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Optional explicit session ID (default: auto-generated SES-XXXXXXXX)"},
				"title":      {Type: "string", Description: "Short session title (e.g. 'Fix install retry loop')"},
				"objective":  {Type: "string", Description: "What the session is trying to accomplish"},
				"actor":      {Type: "string", Description: "Who is running the session (default: claude)"},
				"repo_root":  {Type: "string", Description: "Absolute path to the repository root"},
				"parent_session_id": {Type: "string", Description: "Parent session ID if this session continues prior work"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		sess, err := o.StartSession(ctx, sessionoracle.StartSessionRequest{
			ID:              strArg(args, "session_id"),
			Title:           strArg(args, "title"),
			Objective:       strArg(args, "objective"),
			Actor:           strArg(args, "actor"),
			RepoRoot:        strArg(args, "repo_root"),
			ParentSessionID: strArg(args, "parent_session_id"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":     "started",
			"session_id": sess.ID,
			"branch":     sess.Branch,
			"commit":     sess.GitCommitStart,
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.record_file_touch",
		Description: "Record when a file is read, edited, or created during the session. Also cooperates with Stale Context Detection for read/inspect actions.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Session ID from session.start"},
				"path":       {Type: "string", Description: "File path (absolute or repo-relative)"},
				"action":     {Type: "string", Description: "read | edit | create | delete | rename | test | inspect"},
				"reason":     {Type: "string", Description: "Why this file was accessed"},
				"turn_index": {Type: "number", Description: "Current conversation turn index"},
			},
			Required: []string{"session_id", "path", "action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		ft, err := o.RecordFileTouch(ctx,
			strArg(args, "session_id"),
			strArg(args, "path"),
			strArg(args, "action"),
			strArg(args, "reason"),
			intArgDefault(args, "turn_index", 0))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":             "recorded",
			"touch_id":           ft.ID,
			"sequence":           ft.Sequence,
			"fingerprint_before": ft.FingerprintBefore,
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.record_decision",
		Description: "Record an architectural or engineering decision made during the session, including rationale and alternatives considered.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":               {Type: "string", Description: "Session ID"},
				"title":                    {Type: "string", Description: "Short decision title"},
				"decision":                 {Type: "string", Description: "The decision made"},
				"rationale":                {Type: "string", Description: "Why this decision was made"},
				"alternatives_considered":  {Type: "array", Items: &propSchema{Type: "string"}, Description: "Other approaches that were considered and rejected"},
				"related_files":            {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files involved in this decision"},
				"related_invariants":       {Type: "array", Items: &propSchema{Type: "string"}, Description: "Invariant IDs this decision upholds or relates to"},
				"related_incidents":        {Type: "array", Items: &propSchema{Type: "string"}, Description: "Incident IDs that motivated this decision"},
				"confidence":               {Type: "string", Description: "high | medium | low (default: medium)"},
			},
			Required: []string{"session_id", "title", "decision", "rationale"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		d, err := o.RecordDecision(ctx, sessionoracle.RecordDecisionRequest{
			SessionID:              strArg(args, "session_id"),
			Title:                  strArg(args, "title"),
			Decision:               strArg(args, "decision"),
			Rationale:              strArg(args, "rationale"),
			AlternativesConsidered: strSliceArg(args, "alternatives_considered"),
			RelatedFiles:           strSliceArg(args, "related_files"),
			RelatedInvariants:      strSliceArg(args, "related_invariants"),
			RelatedIncidents:       strSliceArg(args, "related_incidents"),
			Confidence:             strArg(args, "confidence"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "recorded", "decision_id": d.ID}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.record_assumption",
		Description: "Record an unverified assumption made during the session. Assumptions appear in the resume snapshot until resolved.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":      {Type: "string", Description: "Session ID"},
				"assumption":      {Type: "string", Description: "The assumption being made"},
				"basis":           {Type: "string", Description: "Evidence or reasoning for this assumption"},
				"validation_plan": {Type: "string", Description: "How to verify this assumption"},
				"related_files":   {Type: "string", Description: "Files relevant to this assumption"},
			},
			Required: []string{"session_id", "assumption"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		a, err := o.RecordAssumption(ctx, sessionoracle.RecordAssumptionRequest{
			SessionID:      strArg(args, "session_id"),
			Assumption:     strArg(args, "assumption"),
			Basis:          strArg(args, "basis"),
			ValidationPlan: strArg(args, "validation_plan"),
			RelatedFiles:   strArg(args, "related_files"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "recorded", "assumption_id": a.ID}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.record_unfinished",
		Description: "Record a task that was not completed during the session. Unfinished items feed the recommended_next_action in the resume snapshot.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":        {Type: "string", Description: "Session ID"},
				"title":             {Type: "string", Description: "Short task title"},
				"description":       {Type: "string", Description: "What needs to be done"},
				"priority":          {Type: "string", Description: "critical | high | medium | low"},
				"reason_unfinished": {Type: "string", Description: "Why this was not completed now"},
				"next_action":       {Type: "string", Description: "Specific first step for the next session"},
				"related_files":     {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files involved"},
				"related_tests":     {Type: "array", Items: &propSchema{Type: "string"}, Description: "Test targets that need to pass"},
				"related_incidents": {Type: "array", Items: &propSchema{Type: "string"}, Description: "Related incident IDs"},
			},
			Required: []string{"session_id", "title", "description"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		w, err := o.RecordUnfinishedWork(ctx, sessionoracle.RecordUnfinishedWorkRequest{
			SessionID:        strArg(args, "session_id"),
			Title:            strArg(args, "title"),
			Description:      strArg(args, "description"),
			Priority:         strArg(args, "priority"),
			ReasonUnfinished: strArg(args, "reason_unfinished"),
			NextAction:       strArg(args, "next_action"),
			RelatedFiles:     strSliceArg(args, "related_files"),
			RelatedTests:     strSliceArg(args, "related_tests"),
			RelatedIncidents: strSliceArg(args, "related_incidents"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "recorded", "work_id": w.ID}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.record_test_result",
		Description: "Record the result of running tests during the session. Failed tests surface in recommended_next_action.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":     {Type: "string", Description: "Session ID"},
				"command":        {Type: "string", Description: "Test command that was run"},
				"status":         {Type: "string", Description: "passed | failed | skipped | error"},
				"summary":        {Type: "string", Description: "Brief summary of results"},
				"output_excerpt": {Type: "string", Description: "Key lines from test output"},
				"related_files":  {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files involved"},
			},
			Required: []string{"session_id", "command", "status"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		r, err := o.RecordTestResult(ctx, sessionoracle.RecordTestResultRequest{
			SessionID:     strArg(args, "session_id"),
			Command:       strArg(args, "command"),
			Status:        strArg(args, "status"),
			Summary:       strArg(args, "summary"),
			OutputExcerpt: strArg(args, "output_excerpt"),
			RelatedFiles:  strSliceArg(args, "related_files"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "recorded", "test_result_id": r.ID}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.close",
		Description: "Close a session, build a resume snapshot, and optionally push a compact durable summary to AI Memory. Returns the snapshot with recommended_next_action.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":        {Type: "string", Description: "Session ID to close"},
				"push_to_ai_memory": {Type: "boolean", Description: "If true, push compact summary to AI Memory service (requires AI Memory reachable)"},
			},
			Required: []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		pushMem := boolArg(args, "push_to_ai_memory")
		var bridge sessionoracle.AIMemoryBridge
		if pushMem {
			bridge = sessionoracle.NoopBridge() // real bridge requires live AI Memory endpoint
		}
		snap, err := o.CloseSession(ctx, strArg(args, "session_id"), pushMem, bridge)
		if err != nil {
			return nil, err
		}
		unfinCount := 0
		for _, w := range snap.Unfinished {
			if w.Status == "open" || w.Status == "in_progress" {
				unfinCount++
			}
		}
		return map[string]interface{}{
			"status":                 "closed",
			"resume_snapshot_id":     snap.ID,
			"summary":                snap.Summary,
			"recommended_next_action": snap.RecommendedNextAction,
			"unfinished_count":       unfinCount,
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.resume",
		Description: "Resume a specific session by ID. Returns the structured oracle snapshot including stale context warnings, incident warnings, decisions, and recommended next action.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Session ID to resume"},
			},
			Required: []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		snap, err := o.ResumeSession(ctx, strArg(args, "session_id"))
		if err != nil {
			return nil, err
		}
		return snap, nil
	})

	s.register(toolDef{
		Name:        "awareness.session.resume_latest",
		Description: "Resume the most recent open session for the given repo root. If no open session exists, falls back to the most recently closed session.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"repo_root": {Type: "string", Description: "Absolute path to the repository root"},
			},
			Required: []string{"repo_root"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		o := sessionoracle.New(st.g)
		snap, err := o.ResumeLatestOpenSession(ctx, strArg(args, "repo_root"))
		if err != nil {
			return map[string]interface{}{"found": false, "error": err.Error()}, nil
		}
		return map[string]interface{}{"found": true, "session_id": snap.SessionID, "snapshot": snap}, nil
	})
}
