package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/awareness/incidentpattern"
)

// registerAwarenessIncidentPatternTools registers the incident pattern matching tools.
// These give awareness proactive memory: before editing, Claude checks whether the
// current task resembles a past incident, reverted fix, or known architectural trap.
func registerAwarenessIncidentPatternTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.record_incident_pattern",
		Description: "Store a reusable failure pattern extracted from an incident. " +
			"Records files, symbols, invariants, dangerous edit shapes, failed fix attempts, and lessons. " +
			"Also writes a summary to AI Memory for cross-session recall. " +
			"Call this after closing an incident or approving a postmortem proposal.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id": {Type: "string", Description: "Incident ID, e.g. INC-2026-0001"},
				"title":       {Type: "string", Description: "Short title of the incident pattern"},
				"severity":    {Type: "string", Description: "critical | warning | info", Enum: []string{"critical", "warning", "info"}},
				"summary":     {Type: "string", Description: "What happened"},
				"failure_mode": {Type: "string", Description: "Short failure mode identifier, e.g. partial_authoritative_state_commit"},
				"root_cause":  {Type: "string", Description: "Root cause of the incident"},
				"lesson":      {Type: "string", Description: "The correct architectural lesson"},
				"files": {
					Type:        "array",
					Description: `Files involved: [{"path":"golang/...","role":"dispatch authority"}]`,
					Items:       &propSchema{Type: "object"},
				},
				"symbols": {
					Type:        "array",
					Description: `Symbols involved: [{"symbol":"promoteInstallResult","role":"failed fix site"}]`,
					Items:       &propSchema{Type: "object"},
				},
				"invariants": {
					Type:        "array",
					Description: `Invariants touched: [{"invariant_id":"install_result_must_be_atomic","relationship":"violated"}]`,
					Items:       &propSchema{Type: "object"},
				},
				"failed_fixes": {
					Type:        "array",
					Description: `Failed fix attempts: [{"description":"...","reverted":true,"revert_reason":"..."}]`,
					Items:       &propSchema{Type: "object"},
				},
				"edit_shapes": {
					Type:        "array",
					Description: `Dangerous edit shapes: [{"shape_kind":"split_authoritative_state_transition","description":"...","dangerous":true}]`,
					Items:       &propSchema{Type: "object"},
				},
				"proposals": {
					Type:        "array",
					Description: `Linked proposals: [{"proposal_id":"PROP-1","relationship":"rejected","reason":"..."}]`,
					Items:       &propSchema{Type: "object"},
				},
			},
			Required: []string{"incident_id", "title", "severity", "lesson"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded", "message": "awareness graph unavailable"}, nil
		}

		p := incidentpattern.IncidentPattern{
			IncidentID:  strArg(args, "incident_id"),
			Title:       strArg(args, "title"),
			Severity:    strArg(args, "severity"),
			Summary:     strArg(args, "summary"),
			FailureMode: strArg(args, "failure_mode"),
			RootCause:   strArg(args, "root_cause"),
			Lesson:      strArg(args, "lesson"),
		}

		p.Files = parsePatternFiles(args)
		p.Symbols = parsePatternSymbols(args)
		p.Invariants = parsePatternInvariants(args)
		p.FailedFixes = parseFailedFixes(args)
		p.EditShapes = parseEditShapes(args)
		p.Proposals = parsePatternProposals(args)

		store := incidentpattern.NewStore(st.g)
		stored, err := store.RecordPattern(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("record incident pattern: %w", err)
		}

		// Best-effort AI Memory write-through for cross-session recall.
		go writePatternToAIMemory(s, stored)

		return map[string]interface{}{
			"status":     "recorded",
			"pattern_id": stored.ID,
			"incident_id": stored.IncidentID,
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.match_incident_patterns",
		Description: "Check whether the current task resembles a known past incident. " +
			"Call this before editing files. If block=true, stop and read the referenced incident before continuing. " +
			"Provide as many signals as you know: files, symbols, components, invariants, proposed edit shape.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Current Claude session or run ID"},
				"task":       {Type: "string", Description: "What you are trying to do"},
				"intent":     {Type: "string", Description: "edit | review | diagnose"},
				"files": {
					Type:        "array",
					Description: "Files you plan to edit or read",
					Items:       &propSchema{Type: "string"},
				},
				"symbols": {
					Type:        "array",
					Description: "Go/proto symbols you plan to modify",
					Items:       &propSchema{Type: "string"},
				},
				"components": {
					Type:        "array",
					Description: "Service/component names involved",
					Items:       &propSchema{Type: "string"},
				},
				"invariants": {
					Type:        "array",
					Description: "Invariant IDs relevant to the task",
					Items:       &propSchema{Type: "string"},
				},
				"proposed_shape": {
					Type:        "array",
					Description: "Edit shape identifiers you plan to apply (e.g. split_authoritative_state_transition)",
					Items:       &propSchema{Type: "string"},
				},
				"diff_preview": {Type: "string", Description: "Optional diff preview for additional scoring"},
			},
			Required: []string{"session_id", "task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{
				"has_warning": false,
				"matches":     []interface{}{},
				"status":      "degraded",
				"trust":       awarenessTrustMap(st, false),
			}, nil
		}

		req := incidentpattern.IncidentMatchRequest{
			SessionID:   strArg(args, "session_id"),
			Task:        strArg(args, "task"),
			Intent:      strArg(args, "intent"),
			DiffPreview: strArg(args, "diff_preview"),
			Files:       strSliceArg(args, "files"),
			Symbols:     strSliceArg(args, "symbols"),
			Components:  strSliceArg(args, "components"),
			Invariants:  strSliceArg(args, "invariants"),
			ProposedShape: strSliceArg(args, "proposed_shape"),
		}

		matches, err := incidentpattern.Match(ctx, st.g, req)
		if err != nil {
			return nil, fmt.Errorf("match incident patterns: %w", err)
		}

		hasWarning := len(matches) > 0
		block := false
		for _, m := range matches {
			if m.Block {
				block = true
				break
			}
		}

		return map[string]interface{}{
			"has_warning": hasWarning,
			"block":       block,
			"matches":     matchesToMaps(matches),
			"trust":       awarenessTrustMap(st, hasWarning),
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.acknowledge_incident_warning",
		Description: "Acknowledge that you have read an incident and adjusted your plan. " +
			"After acknowledgement, awareness will not re-block for this session + incident pair. " +
			"Also writes the acknowledgement to AI Memory for audit trail.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":           {Type: "string", Description: "Current Claude session or run ID"},
				"incident_id":          {Type: "string", Description: "Incident ID being acknowledged"},
				"acknowledged_reason":  {Type: "string", Description: "Why you are proceeding — what you changed in your plan"},
			},
			Required: []string{"session_id", "incident_id", "acknowledged_reason"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}

		sessionID := strArg(args, "session_id")
		incidentID := strArg(args, "incident_id")
		reason := strArg(args, "acknowledged_reason")

		ack := incidentpattern.NewAcknowledgementStore(st.g)
		if err := ack.AcknowledgeIncident(ctx, sessionID, incidentID, reason); err != nil {
			return nil, fmt.Errorf("acknowledge incident: %w", err)
		}

		// Best-effort AI Memory write for audit trail.
		go writeAckToAIMemory(s, sessionID, incidentID, reason)

		return map[string]interface{}{"status": "acknowledged", "incident_id": incidentID}, nil
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func objectSlice(args map[string]interface{}, key string) []map[string]interface{} {
	raw, _ := args[key].([]interface{})
	var out []map[string]interface{}
	for _, v := range raw {
		if m, ok := v.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func parsePatternFiles(args map[string]interface{}) []incidentpattern.PatternFile {
	var out []incidentpattern.PatternFile
	for _, obj := range objectSlice(args, "files") {
		path, _ := obj["path"].(string)
		role, _ := obj["role"].(string)
		if path != "" {
			out = append(out, incidentpattern.PatternFile{Path: path, Role: role})
		}
	}
	return out
}

func parsePatternSymbols(args map[string]interface{}) []incidentpattern.PatternSymbol {
	var out []incidentpattern.PatternSymbol
	for _, obj := range objectSlice(args, "symbols") {
		sym, _ := obj["symbol"].(string)
		role, _ := obj["role"].(string)
		if sym != "" {
			out = append(out, incidentpattern.PatternSymbol{Symbol: sym, Role: role})
		}
	}
	return out
}

func parsePatternInvariants(args map[string]interface{}) []incidentpattern.PatternInvariant {
	var out []incidentpattern.PatternInvariant
	for _, obj := range objectSlice(args, "invariants") {
		id, _ := obj["invariant_id"].(string)
		rel, _ := obj["relationship"].(string)
		if id != "" {
			out = append(out, incidentpattern.PatternInvariant{InvariantID: id, Relationship: rel})
		}
	}
	return out
}

func parseFailedFixes(args map[string]interface{}) []incidentpattern.FailedFix {
	var out []incidentpattern.FailedFix
	for _, obj := range objectSlice(args, "failed_fixes") {
		desc, _ := obj["description"].(string)
		reverted, _ := obj["reverted"].(bool)
		reason, _ := obj["revert_reason"].(string)
		proposalID, _ := obj["proposal_id"].(string)
		commitHash, _ := obj["commit_hash"].(string)
		if desc != "" {
			out = append(out, incidentpattern.FailedFix{
				Description:  desc,
				Reverted:     reverted,
				RevertReason: reason,
				ProposalID:   proposalID,
				CommitHash:   commitHash,
			})
		}
	}
	return out
}

func parseEditShapes(args map[string]interface{}) []incidentpattern.EditShape {
	var out []incidentpattern.EditShape
	for _, obj := range objectSlice(args, "edit_shapes") {
		kind, _ := obj["shape_kind"].(string)
		desc, _ := obj["description"].(string)
		dangerous := true
		if d, ok := obj["dangerous"].(bool); ok {
			dangerous = d
		}
		if kind != "" {
			out = append(out, incidentpattern.EditShape{ShapeKind: kind, Description: desc, Dangerous: dangerous})
		}
	}
	return out
}

func parsePatternProposals(args map[string]interface{}) []incidentpattern.PatternProposal {
	var out []incidentpattern.PatternProposal
	for _, obj := range objectSlice(args, "proposals") {
		pid, _ := obj["proposal_id"].(string)
		rel, _ := obj["relationship"].(string)
		reason, _ := obj["reason"].(string)
		if pid != "" {
			out = append(out, incidentpattern.PatternProposal{ProposalID: pid, Relationship: rel, Reason: reason})
		}
	}
	return out
}

func matchesToMaps(matches []incidentpattern.IncidentPatternMatch) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(matches))
	for _, m := range matches {
		signals := make([]map[string]interface{}, 0, len(m.MatchedSignals))
		for _, s := range m.MatchedSignals {
			signals = append(signals, map[string]interface{}{
				"kind":        s.Kind,
				"value":       s.Value,
				"weight":      s.Weight,
				"explanation": s.Explanation,
			})
		}
		fixes := make([]map[string]interface{}, 0, len(m.FailedFixes))
		for _, ff := range m.FailedFixes {
			fixes = append(fixes, map[string]interface{}{
				"description":   ff.Description,
				"reverted":      ff.Reverted,
				"revert_reason": ff.RevertReason,
			})
		}
		out = append(out, map[string]interface{}{
			"pattern_id":       m.PatternID,
			"incident_id":      m.IncidentID,
			"title":            m.Title,
			"severity":         m.Severity,
			"score":            m.Score,
			"confidence":       m.Confidence,
			"block":            m.Block,
			"matched_signals":  signals,
			"failed_fixes":     fixes,
			"lesson":           m.Lesson,
			"warning":          m.Warning,
			"recommended_next": m.RecommendedNext,
		})
	}
	return out
}

// writePatternToAIMemory writes a pattern summary to AI Memory for cross-session recall.
// Called in a goroutine — failure is logged but not fatal.
func writePatternToAIMemory(s *server, p incidentpattern.IncidentPattern) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.clients.get(ctx, memoryEndpoint())
	if err != nil {
		log.Printf("incidentpattern: ai_memory write-through unavailable: %v", err)
		return
	}
	client := ai_memorypb.NewAiMemoryServiceClient(conn)

	content := fmt.Sprintf("Incident: %s\nTitle: %s\nRoot cause: %s\nLesson: %s",
		p.IncidentID, p.Title, p.RootCause, p.Lesson)

	_, err = client.Store(ctx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project:   "globular-services",
			Type:      ai_memorypb.MemoryType_FEEDBACK,
			Title:     fmt.Sprintf("Incident pattern: %s — %s", p.IncidentID, p.Title),
			Content:   content,
			Tags:      []string{"incident_pattern", p.IncidentID, p.Severity},
			Metadata:  map[string]string{"incident_id": p.IncidentID, "pattern_id": p.ID, "severity": p.Severity},
			AgentId:   "awareness-mcp",
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	})
	if err != nil {
		log.Printf("incidentpattern: ai_memory store failed for %s: %v", p.IncidentID, err)
	}
}

// writeAckToAIMemory records an acknowledgement in AI Memory for audit trail.
func writeAckToAIMemory(s *server, sessionID, incidentID, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.clients.get(ctx, memoryEndpoint())
	if err != nil {
		return
	}
	client := ai_memorypb.NewAiMemoryServiceClient(conn)

	_, err = client.Store(ctx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project:        "globular-services",
			Type:           ai_memorypb.MemoryType_DECISION,
			Title:          fmt.Sprintf("Incident %s acknowledged in session %s", incidentID, sessionID),
			Content:        reason,
			Tags:           []string{"incident_pattern", "acknowledged", incidentID},
			Metadata:       map[string]string{"incident_id": incidentID, "session_id": sessionID},
			ConversationId: sessionID,
			AgentId:        "awareness-mcp",
			CreatedAt:      time.Now().Unix(),
			UpdatedAt:      time.Now().Unix(),
		},
	})
	if err != nil {
		log.Printf("incidentpattern: ai_memory ack store failed for %s: %v", incidentID, err)
	}
}

// ensure import is used.
var _ = strings.ToLower
