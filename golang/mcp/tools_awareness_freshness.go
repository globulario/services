package main

// tools_awareness_freshness.go — stubs after contextfreshness package was removed
// from standalone awareness module. The stale-context detection MCP tools are
// not available in this build.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// registerAwarenessFreshnessTools registers the stale-context detection tools.
// The tools are stubs — contextfreshness was removed from standalone awareness module.
func registerAwarenessFreshnessTools(s *server, _ *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.record_context_read",
		Description: "Record that you just read a source file (returns sha256 fingerprint). [contextfreshness not available — fingerprint only]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Session ID (required)"},
				"path":       {Type: "string", Description: "File path that was read (required)"},
				"reason":     {Type: "string", Description: "Why it was read"},
				"tool":       {Type: "string", Description: "Tool that performed the read"},
				"turn_index": {Type: "integer", Description: "Conversation turn index"},
			},
			Required: []string{"session_id", "path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		path := strArg(args, "path")
		fp := fingerprintFile(path)
		return map[string]interface{}{
			"status":      "recorded",
			"path":        path,
			"fingerprint": fp,
			"note":        "contextfreshness staleness tracking not available — fingerprint only",
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.check_stale_context",
		Description: "Check whether files read earlier have changed [not available — contextfreshness package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Session ID (required)"},
				"paths":      {Type: "array", Description: "File paths to check", Items: &propSchema{Type: "string"}},
				"turn_index": {Type: "integer", Description: "Current turn index"},
			},
			Required: []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "contextfreshness package was removed from standalone awareness module",
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.check_session_freshness",
		Description: "Show all stale context files for a session [not available — contextfreshness package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id": {Type: "string", Description: "Session ID (required)"},
				"turn_index": {Type: "integer", Description: "Current turn index"},
			},
			Required: []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "contextfreshness package was removed from standalone awareness module",
		}, nil
	})
}

// fingerprintFile computes the sha256 of a file's content, or "" if unreadable.
func fingerprintFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
