package main

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/awareness/enforce"
	"github.com/globulario/awareness/graph"
)

func registerAwarenessAgentUsageTools(s *server, st *awarenessState) {
	registerPreEditContextTool(s, st)
	registerAgentUsageReportTool(s, st)
	registerPreCommitCheckTool(s, st)
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
				"file":        file,
				"error":       "graph unavailable — run 'globular awareness build' first",
				"blind_spots": []string{"graph not available — static file analysis only"},
				"trust":       awarenessTrustMap(st, false),
			}, nil
		}
		out, err := buildFileInvariantContext(ctx, st.g, file)
		if err != nil {
			return nil, err
		}
		// Match-found = at least one invariant linked to the file. The
		// "warning: file not indexed" path returns invariants=[] which yields
		// match_found=false, so verdict is unknown — never trusted.
		matchFound := false
		if invs, ok := out["invariants"].([]map[string]interface{}); ok {
			matchFound = len(invs) > 0
		} else if invs, ok := out["invariants"].([]interface{}); ok {
			matchFound = len(invs) > 0
		}
		out["trust"] = awarenessTrustMap(st, matchFound)
		return out, nil
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

// registerPreCommitCheckTool registers awareness.pre_commit_check — runs
// impact_path + scan_violations + graph_integrity summary before committing,
// and records a usage event so commits_without_integrity_check is accurate.
func registerPreCommitCheckTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.pre_commit_check",
		Description: `Run before marking work complete or committing changes.

Summarises:
- scan_violations: critical findings in changed files
- graph_integrity: shape check on the awareness graph (escalated severities)
- required_tests: tests that must pass based on changed files
- proposal_queue: whether learn_from_fix should be called

Records a pre_commit_check usage event. If this tool is NOT called before a
commit, commits_without_integrity_check is incremented in the usage report.

Use this as the final awareness gate before saying "done".`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"files": {
					Type:        "array",
					Description: "Changed files (relative paths). Used for scan_violations and impact path.",
					Items:       &propSchema{Type: "string"},
				},
				"session_id": {
					Type:        "string",
					Description: "Optional session identifier for usage tracking.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		files := getStrSlice(args, "files")
		sessionID := strArg(args, "session_id")

		// Record usage so this commit does NOT increment commits_without_integrity_check.
		if st.g != nil {
			_ = st.g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
				ID:                fmt.Sprintf("pre_commit_%d", time.Now().UnixNano()),
				Agent:             "claude",
				SessionIDHash:     sessionID,
				Tool:              "commit.graph_integrity",
				Operation:         "called",
				TaskType:          "pre_commit",
				ChangedFilesCount: len(files),
			})
		}

		result := map[string]interface{}{
			"files_checked": files,
		}

		// Graph integrity summary (shape check only — no repo scan needed).
		if st.g != nil {
			result["graph_integrity"] = runPreCommitIntegritySummary(ctx, st)
		} else {
			result["graph_integrity"] = map[string]interface{}{
				"status":  "unavailable",
				"message": "graph not available — run 'globular awareness build' first",
			}
		}

		// Scan violations summary for changed files.
		if len(files) > 0 && st.repoRoot != "" {
			result["scan_summary"] = fmt.Sprintf("run 'globular awareness scan-violations --paths %s' to check violations", joinPaths(files))
		} else {
			result["scan_summary"] = "no files provided — run awareness.scan_violations manually"
		}

		// Proposal queue status.
		queueSection, _ := buildQueueSection(st.docsDir, 24.0)
		result["proposal_queue"] = map[string]interface{}{
			"status":            queueSection.Status,
			"pending_proposals": queueSection.PendingProposals,
			"stale_proposals":   queueSection.StaleProposals,
		}
		if queueSection.PendingProposals > 0 {
			result["learn_from_fix_recommended"] = true
		}

		return result, nil
	})
}

// runPreCommitIntegritySummary returns a compact graph integrity summary for the
// pre_commit_check tool. Shape checks only — no repo scan required.
func runPreCommitIntegritySummary(ctx context.Context, st *awarenessState) map[string]interface{} {
	ciRes := enforce.GraphIntegrityCICheck(ctx, st.g, enforce.CICheckOptions{
		MaxRequiredTestNoPath: 100,
		RepoRoot:              st.repoRoot,
		DocsDir:               st.docsDir,
	})
	status := "ok"
	if ciRes.ErrorCount > 0 {
		status = "critical"
	} else if ciRes.WarningCount > 0 {
		status = "warning"
	}
	return map[string]interface{}{
		"status":        status,
		"error_count":   ciRes.ErrorCount,
		"warning_count": ciRes.WarningCount,
		"pass":          ciRes.Pass,
	}
}

func joinPaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	result := paths[0]
	for _, p := range paths[1:] {
		result += " " + p
	}
	return result
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
