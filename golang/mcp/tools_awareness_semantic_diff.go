package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/globulario/services/golang/awareness/semanticdiff"
	"github.com/globulario/services/golang/awareness/sessionoracle"
)

func registerAwarenessSemanticDiffTools(s *server, st *awarenessState) {
	// ── awareness.semantic_diff.interpret ────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.semantic_diff.interpret",
		Description: "Interpret a unified diff semantically against Globular's 4-layer state model. Returns verdict (allow/allow_with_warnings/block), severity, findings, and atoms.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":    {Type: "string", Description: "Session ID for correlation"},
				"task":          {Type: "string", Description: "What change is being made — used for context"},
				"diff_text":     {Type: "string", Description: "Unified diff text to interpret"},
				"diff_source":   {Type: "string", Description: "Where the diff came from (e.g. git, review, clipboard)"},
				"git_base":      {Type: "string", Description: "Git base ref (for attribution)"},
				"git_head":      {Type: "string", Description: "Git head ref (for attribution)"},
				"files":         {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files changed (optional, derived from diff)"},
				"require_clean": {Type: "boolean", Description: "If true, return error when verdict is block"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		diffText := strArg(args, "diff_text")
		if diffText == "" {
			return nil, fmt.Errorf("diff_text is required")
		}
		req := semanticdiff.SemanticDiffRequest{
			SessionID:    strArg(args, "session_id"),
			Task:         strArg(args, "task"),
			DiffText:     diffText,
			DiffSource:   strArg(args, "diff_source"),
			GitBase:      strArg(args, "git_base"),
			GitHead:      strArg(args, "git_head"),
			Files:        strSliceArg(args, "files"),
			RequireClean: boolArg(args, "require_clean"),
		}
		report, err := semanticdiff.InterpretSemanticDiff(ctx, req)
		if err != nil {
			return nil, err
		}

		// Persist to store if graph is available.
		if st.g != nil {
			sdStore := semanticdiff.NewStore(st.g)
			_ = sdStore.StoreReport(ctx, report)
			// Record to session oracle when session_id is provided.
			if req.SessionID != "" {
				recordSemanticDiffToSession(ctx, st, req.SessionID, report)
			}
		}

		if req.RequireClean && report.Verdict == semanticdiff.VerdictBlock {
			return nil, fmt.Errorf("semantic diff blocked: %s", report.Summary)
		}

		findings := make([]map[string]interface{}, 0, len(report.Findings))
		for _, f := range report.Findings {
			findings = append(findings, map[string]interface{}{
				"id":             f.ID,
				"kind":           f.Kind,
				"severity":       f.Severity,
				"file_path":      f.FilePath,
				"symbol":         f.Symbol,
				"layer_from":     f.LayerFrom,
				"layer_to":       f.LayerTo,
				"message":        f.Message,
				"evidence":       f.Evidence,
				"recommendation": f.Recommendation,
			})
		}
		return map[string]interface{}{
			"report_id":   report.ID,
			"verdict":     report.Verdict,
			"severity":    report.Severity,
			"summary":     report.Summary,
			"findings":    findings,
			"atom_count":  len(report.Atoms),
			"fingerprint": report.Fingerprint,
			"formatted":   semanticdiff.FormatReport(report),
		}, nil
	})

	// ── awareness.semantic_diff.from_git ─────────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.semantic_diff.from_git",
		Description: "Run semantic diff interpretation on the current git working tree diff. Executes 'git diff <base>' to get the diff, then interprets it.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":    {Type: "string", Description: "Session ID for correlation"},
				"task":          {Type: "string", Description: "What change is being made"},
				"git_base":      {Type: "string", Description: "Git base ref (default: HEAD)"},
				"git_head":      {Type: "string", Description: "Git head ref (default: working-tree)"},
				"require_clean": {Type: "boolean", Description: "If true, return error when verdict is block"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		gitBase := strArg(args, "git_base")
		if gitBase == "" {
			gitBase = "HEAD"
		}
		gitHead := strArg(args, "git_head")
		if gitHead == "" {
			gitHead = "working-tree"
		}

		var out []byte
		var cmdErr error
		if gitHead == "working-tree" {
			out, cmdErr = exec.CommandContext(ctx, "git", "diff", gitBase).Output()
		} else {
			out, cmdErr = exec.CommandContext(ctx, "git", "diff", gitBase, gitHead).Output()
		}
		if cmdErr != nil {
			return nil, fmt.Errorf("git diff failed: %w", cmdErr)
		}

		diffText := strings.TrimSpace(string(out))
		if diffText == "" {
			return map[string]interface{}{
				"verdict":  "allow",
				"severity": "info",
				"summary":  "ALLOW: No changes found in git diff — nothing to interpret.",
			}, nil
		}

		req := semanticdiff.SemanticDiffRequest{
			SessionID:    strArg(args, "session_id"),
			Task:         strArg(args, "task"),
			DiffText:     diffText,
			DiffSource:   "git",
			GitBase:      gitBase,
			GitHead:      gitHead,
			RequireClean: boolArg(args, "require_clean"),
		}
		report, err := semanticdiff.InterpretSemanticDiff(ctx, req)
		if err != nil {
			return nil, err
		}

		if st.g != nil {
			sdStore := semanticdiff.NewStore(st.g)
			_ = sdStore.StoreReport(ctx, report)
			if req.SessionID != "" {
				recordSemanticDiffToSession(ctx, st, req.SessionID, report)
			}
		}

		if req.RequireClean && report.Verdict == semanticdiff.VerdictBlock {
			return nil, fmt.Errorf("semantic diff blocked: %s", report.Summary)
		}

		findings := make([]map[string]interface{}, 0, len(report.Findings))
		for _, f := range report.Findings {
			findings = append(findings, map[string]interface{}{
				"id":             f.ID,
				"kind":           f.Kind,
				"severity":       f.Severity,
				"file_path":      f.FilePath,
				"symbol":         f.Symbol,
				"layer_from":     f.LayerFrom,
				"layer_to":       f.LayerTo,
				"message":        f.Message,
				"evidence":       f.Evidence,
				"recommendation": f.Recommendation,
			})
		}
		return map[string]interface{}{
			"report_id":   report.ID,
			"verdict":     report.Verdict,
			"severity":    report.Severity,
			"summary":     report.Summary,
			"findings":    findings,
			"atom_count":  len(report.Atoms),
			"fingerprint": report.Fingerprint,
			"formatted":   semanticdiff.FormatReport(report),
		}, nil
	})

	// ── awareness.semantic_diff.show_report ───────────────────────────────────

	s.register(toolDef{
		Name:        "awareness.semantic_diff.show_report",
		Description: "Load and return a previously stored semantic diff report by ID.",
		InputSchema: inputSchema{
			Type:     "object",
			Required: []string{"report_id"},
			Properties: map[string]propSchema{
				"report_id": {Type: "string", Description: "Report ID returned by interpret or from_git"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		reportID := strArg(args, "report_id")
		if reportID == "" {
			return nil, fmt.Errorf("report_id is required")
		}
		sdStore := semanticdiff.NewStore(st.g)
		report, err := sdStore.GetReport(ctx, reportID)
		if err != nil {
			return nil, err
		}
		return report, nil
	})
}

// recordSemanticDiffToSession records the semantic diff result as a session event,
// and emits a session warning when the verdict is block.
func recordSemanticDiffToSession(ctx context.Context, st *awarenessState, sessionID string, r *semanticdiff.SemanticDiffReport) {
	if st.g == nil || sessionID == "" {
		return
	}
	o := sessionoracle.New(st.g)

	title := "Semantic diff: " + r.Verdict
	body := r.Summary
	if r.Verdict == semanticdiff.VerdictBlock {
		body += "\n\nFindings:"
		for _, f := range r.Findings {
			if f.Severity == semanticdiff.SeverityForbidden || f.Severity == semanticdiff.SeverityCritical {
				body += "\n  [" + f.Severity + "] " + f.Message
			}
		}
	}

	_ = o.RecordSessionEvent(ctx, sessionID, "semantic_diff", title, body, map[string]string{
		"report_id":   r.ID,
		"verdict":     r.Verdict,
		"severity":    r.Severity,
		"fingerprint": r.Fingerprint,
	}, 0)

	if r.Verdict == semanticdiff.VerdictBlock {
		sev := "warning"
		if r.Severity == semanticdiff.SeverityForbidden {
			sev = "critical"
		}
		_, _ = o.RecordSessionWarning(ctx, sessionoracle.RecordSessionWarningRequest{
			SessionID:   sessionID,
			WarningType: "architecture",
			Severity:    sev,
			Message:     "Semantic diff blocked: " + r.Summary,
		})
	}
}
