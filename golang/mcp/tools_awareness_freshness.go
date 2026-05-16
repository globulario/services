package main

import (
	"context"
	"fmt"

	"github.com/globulario/awareness/contextfreshness"
)

// registerAwarenessFreshnessTools registers the stale-context detection tools.
// These let Claude record which files it has read and check for staleness before
// editing or reasoning from them.
func registerAwarenessFreshnessTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.record_context_read",
		Description: "Record that you just read a source file. " +
			"Call this after every Read tool call so awareness can detect if the file changes before you act on it. " +
			"Returns the sha256 fingerprint captured at read time.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Current Claude session or run ID"},
				"path":       {Type: "string", Description: "Absolute or repo-relative path to the file read"},
				"read_reason": {Type: "string", Description: "Why you read the file (e.g. 'debug install retry loop')"},
				"read_tool":  {Type: "string", Description: "Tool used to read (e.g. Read, Bash, WebFetch)"},
				"turn_index": {Type: "number", Description: "Approximate turn number in the conversation"},
			},
			Required: []string{"session_id", "path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded", "message": "awareness graph unavailable"}, nil
		}
		sessionID := strArg(args, "session_id")
		path := strArg(args, "path")
		if path == "" {
			return nil, fmt.Errorf("path is required")
		}
		reason := strArg(args, "read_reason")
		tool := strArg(args, "read_tool")
		turnIndex := intArgDefault(args, "turn_index", 0)

		tr := contextfreshness.New(st.g)
		cr, err := tr.RecordContextRead(ctx, sessionID, path, reason, tool, turnIndex)
		if err != nil {
			return nil, fmt.Errorf("record context read: %w", err)
		}
		return map[string]interface{}{
			"path":        cr.Path,
			"fingerprint": cr.Fingerprint,
			"turn_index":  cr.TurnIndex,
			"status":      "recorded",
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.check_stale_context",
		Description: "Check whether files you read earlier in this session have changed. " +
			"Call this before editing any file. If stale=true, re-read the listed files before proceeding. " +
			"Critical severity means the file changed and you are about to act on it — stop and re-read.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Current Claude session or run ID"},
				"paths": {
					Type:        "array",
					Description: "Files to check for staleness",
					Items:       &propSchema{Type: "string"},
				},
				"current_turn_index": {Type: "number", Description: "Current turn number in the conversation"},
			},
			Required: []string{"session_id", "paths"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"stale": false, "warnings": []interface{}{}, "status": "degraded"}, nil
		}
		sessionID := strArg(args, "session_id")
		rawPaths, _ := args["paths"].([]interface{})
		var paths []string
		for _, p := range rawPaths {
			if s, ok := p.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("paths must be a non-empty array")
		}
		currentTurn := intArgDefault(args, "current_turn_index", 0)

		tr := contextfreshness.New(st.g)
		warnings, err := tr.CheckStaleContext(ctx, sessionID, paths, currentTurn, contextfreshness.SeverityCritical)
		if err != nil {
			return nil, fmt.Errorf("check stale context: %w", err)
		}

		return buildFreshnessResponse(warnings, len(paths)), nil
	})

	s.register(toolDef{
		Name: "awareness.check_session_freshness",
		Description: "Check ALL files read in this session for staleness. " +
			"Call this before making architecture decisions that span multiple files. " +
			"Returns a list of stale files and a human-readable summary.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":         {Type: "string", Description: "Current Claude session or run ID"},
				"current_turn_index": {Type: "number", Description: "Current turn number in the conversation"},
			},
			Required: []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"stale": false, "stale_files": []string{}, "status": "degraded"}, nil
		}
		sessionID := strArg(args, "session_id")
		currentTurn := intArgDefault(args, "current_turn_index", 0)

		tr := contextfreshness.New(st.g)
		warnings, err := tr.CheckAllSessionReads(ctx, sessionID, currentTurn)
		if err != nil {
			return nil, fmt.Errorf("check session freshness: %w", err)
		}

		staleFiles := make([]string, 0, len(warnings))
		for _, w := range warnings {
			staleFiles = append(staleFiles, w.Path)
		}

		msg := "All session reads are fresh."
		if len(warnings) > 0 {
			msg = fmt.Sprintf(
				"%d previously-read file(s) changed after they were read. Re-read them before editing or making architecture decisions.",
				len(warnings))
		}

		return map[string]interface{}{
			"stale":       len(warnings) > 0,
			"stale_files": staleFiles,
			"message":     msg,
			"warnings":    warningsToMaps(warnings),
		}, nil
	})
}

func buildFreshnessResponse(warnings []contextfreshness.StaleContextWarning, checkedCount int) map[string]interface{} {
	stale := len(warnings) > 0
	ws := warningsToMaps(warnings)
	result := map[string]interface{}{
		"stale":         stale,
		"warnings":      ws,
		"checked_files": checkedCount,
	}
	if !stale {
		result["message"] = "All checked files are fresh — safe to proceed."
	}
	return result
}

func warningsToMaps(warnings []contextfreshness.StaleContextWarning) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, map[string]interface{}{
			"path":                w.Path,
			"severity":            w.Severity,
			"message":             w.Message,
			"read_turn_index":     w.ReadTurnIndex,
			"current_turn_index":  w.CurrentTurnIndex,
			"read_fingerprint":    w.ReadFingerprint,
			"current_fingerprint": w.CurrentFingerprint,
		})
	}
	return out
}
