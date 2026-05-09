package main

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
)

func registerAwarenessAgentUsageTools(s *server, st *awarenessState) {
	registerPreEditContextTool(s, st)
	registerAgentUsageReportTool(s, st)
}

// registerPreEditContextTool registers awareness.pre_edit_context — a combined
// tool that returns invariant context for a file AND records a pre-edit usage
// event so skip-rate tracking stays accurate.
func registerPreEditContextTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.pre_edit_context",
		Description: `Return invariant and risk context for a file you are about to edit.

Combines awareness.file_invariant_context with usage event recording so that
skip-rate metrics stay accurate. Call this BEFORE editing any file.

Returns:
- invariants linked to the file (implements, enforces, configures, observes)
- edit warnings from forbidden_actions on each invariant
- required tests that must pass after editing
- blind_spots if the file is not indexed in the graph

Also records a pre_edit_context usage event to the agent usage log.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "Relative file path (e.g. 'golang/cluster_controller/server.go')",
				},
				"session_id": {
					Type:        "string",
					Description: "Optional: session identifier for usage tracking",
				},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file := strArg(args, "file")
		if file == "" {
			return map[string]interface{}{"error": "file is required"}, nil
		}

		// Record the usage event regardless of graph availability.
		if st.g != nil {
			_ = st.g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
				ID:           fmt.Sprintf("pre_edit_%s_%d", sanitizeID(file), time.Now().UnixNano()),
				Agent:        "claude",
				SessionIDHash: strArg(args, "session_id"),
				Tool:         "awareness.pre_edit_context",
				Operation:    "called",
				TaskType:     "pre_edit",
			})
		}

		if st.g == nil {
			return map[string]interface{}{
				"file":       file,
				"error":      "graph unavailable — run 'globular awareness build' first",
				"blind_spots": []string{"graph not available — static file analysis only"},
			}, nil
		}
		return buildFileInvariantContext(ctx, st.g, file)
	})
}

// registerAgentUsageReportTool registers awareness.agent_usage_report — returns
// aggregate usage stats for the configured window (default 7 days).
func registerAgentUsageReportTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.agent_usage_report",
		Description: `Return aggregate agent usage statistics for the awareness toolset.

Reports:
- sessions_total: distinct sessions over the window
- preflight_calls: how often awareness.preflight was called
- preflight_skip_rate_pct: percentage of sessions without a preflight call
- pre_edit_context_calls: pre-edit context lookups
- agent_context_calls: agent context lookups
- scan_violations_calls: scan-violations calls
- commits_without_integrity_check: commits that bypassed graph integrity check

Use this to detect whether agents are actually using awareness.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"window_days": {
					Type:        "integer",
					Description: "Look-back window in days (default 7)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		windowDays := 7
		if v, ok := args["window_days"]; ok {
			switch w := v.(type) {
			case float64:
				windowDays = int(w)
			case int:
				windowDays = w
			}
		}
		if windowDays < 1 {
			windowDays = 1
		}
		if windowDays > 90 {
			windowDays = 90
		}

		if st.g == nil {
			return map[string]interface{}{
				"status":     "no_data",
				"window_days": windowDays,
				"error":      "graph unavailable — run 'globular awareness build' first",
			}, nil
		}

		summary, err := st.g.QueryAgentUsageSummary(ctx, windowDays)
		if err != nil {
			return nil, fmt.Errorf("agent_usage_report: %w", err)
		}

		return map[string]interface{}{
			"agent_usage": map[string]interface{}{
				"window_days":                       summary.WindowDays,
				"sessions":                          summary.SessionsTotal,
				"preflight_runs":                    summary.PreflightCalls,
				"preflight_skip_rate":               summary.PreflightSkipRatePct,
				"pre_edit_context_runs":             summary.PreEditContextCalls,
				"agent_context_runs":                summary.AgentContextCalls,
				"scan_violations_runs":              summary.ScanViolationsCalls,
				"commits_without_graph_integrity":   summary.CommitsWithoutIntegrityCheck,
				"status":                            summary.Status,
				"recommended_action":                summary.RecommendedAction,
			},
		}, nil
	})
}

// sanitizeID converts a file path to a short safe string for use in event IDs.
func sanitizeID(s string) string {
	const maxLen = 40
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '/' || c == '.' || c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return string(out)
}
