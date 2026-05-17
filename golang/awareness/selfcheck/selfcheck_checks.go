package selfcheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/checkedit"
	"github.com/globulario/services/golang/awareness/debugsession"
	"github.com/globulario/services/golang/awareness/enforce"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/preflight"
	"github.com/globulario/services/golang/awareness/semantic"
)

func checkBuild(opts Options) CheckResult {
	cr := CheckResult{Kind: KindBuild, Name: "graph_db_exists_and_recent"}

	dbPath := opts.DBPath
	if dbPath == "" {
		cr.Status = StatusSkipped
		cr.Detail = "no DBPath provided — build check skipped"
		return cr
	}

	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("graph.db not found at %s — run 'globular awareness build'", dbPath)
		return cr
	}
	if err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("stat graph.db: %v", err)
		return cr
	}

	age := time.Since(info.ModTime())
	if age > 24*time.Hour {
		cr.Status = StatusWeak
		cr.Detail = fmt.Sprintf("graph.db is %.0fh old — consider rebuilding (run 'globular awareness build')", age.Hours())
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("graph.db exists, built %.0fm ago", age.Minutes())
	return cr
}

func checkAudit(ctx context.Context, g *graph.Graph, opts Options) CheckResult {
	cr := CheckResult{Kind: KindAudit, Name: "enforcement_audit"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — audit skipped"
		return cr
	}

	result := enforce.Audit(ctx, g, enforce.AuditOptions{
		RepoRoot:  opts.RepoPath,
		SrcDir:    opts.RepoPath,
		SkipDrift: true, // drift has its own check
	})

	if result.ErrorCount > 0 {
		cr.Status = StatusFail
		msgs := make([]string, 0, result.ErrorCount)
		for _, f := range result.Findings {
			if f.Severity == enforce.SeverityError {
				msgs = append(msgs, f.Message)
			}
		}
		cr.Detail = fmt.Sprintf("%d audit errors: %s", result.ErrorCount, strings.Join(msgs, "; "))
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("audit passed — %d warnings, %d info findings", result.WarningCount, result.InfoCount)
	if result.WarningCount > 3 {
		cr.Noisy = []string{fmt.Sprintf("%d audit warnings — consider triaging", result.WarningCount)}
		cr.Noisy = append(cr.Noisy, summarizeWarningGroups(result.Findings, 3)...)
	}
	if opts.Strict && opts.MaxTopWarningGroup >= 0 {
		topCount := maxWarningGroupCount(result.Findings)
		if topCount > opts.MaxTopWarningGroup {
			cr.Status = StatusFail
			cr.Detail = fmt.Sprintf(
				"audit warning-group threshold exceeded: top warning group count=%d > max=%d",
				topCount, opts.MaxTopWarningGroup)
		}
	}
	return cr
}

func summarizeWarningGroups(findings []enforce.Finding, maxGroups int) []string {
	if maxGroups <= 0 {
		return nil
	}
	var warnings []enforce.Finding
	for _, f := range findings {
		if f.Severity == enforce.SeverityWarning {
			warnings = append(warnings, f)
		}
	}
	if len(warnings) == 0 {
		return nil
	}
	groups := enforce.GroupFindings(warnings)
	if len(groups) > maxGroups {
		groups = groups[:maxGroups]
	}
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, fmt.Sprintf("top warning group: %s (%d) — %s", g.Code, g.Count, g.SuggestedAction))
	}
	return out
}

func maxWarningGroupCount(findings []enforce.Finding) int {
	var warnings []enforce.Finding
	for _, f := range findings {
		if f.Severity == enforce.SeverityWarning {
			warnings = append(warnings, f)
		}
	}
	if len(warnings) == 0 {
		return 0
	}
	groups := enforce.GroupFindings(warnings)
	if len(groups) == 0 {
		return 0
	}
	return groups[0].Count
}

func checkAnnotationCoverage(ctx context.Context, g *graph.Graph, opts Options) CheckResult {
	cr := CheckResult{Kind: KindCoverage, Name: "annotation_coverage"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — coverage check skipped"
		return cr
	}

	watchlistPath := ""
	if opts.DocsDir != "" {
		watchlistPath = filepath.Join(opts.DocsDir, "high_risk_files.yaml")
	}

	result := enforce.AnnotationCoverage(ctx, g, enforce.AnnotationCoverageOptions{
		RepoRoot:      opts.RepoPath,
		SrcDir:        opts.RepoPath,
		WatchlistPath: watchlistPath,
		DocsDir:       opts.DocsDir,
	})

	if result.ErrorCount > 0 {
		cr.Status = StatusWeak
		msgs := make([]string, 0, result.ErrorCount)
		for _, f := range result.Findings {
			if f.Severity == enforce.SeverityError {
				msgs = append(msgs, f.File+": "+f.Message)
			}
		}
		cr.Detail = fmt.Sprintf("%d coverage gaps: %s", result.ErrorCount, strings.Join(msgs, "; "))
		cr.Missing = msgs
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = "annotation coverage OK"
	return cr
}

func checkGraphDrift(ctx context.Context, g *graph.Graph, opts CheckOptions) CheckResult {
	cr := CheckResult{Kind: KindDrift, Name: "graph_drift"}

	if g == nil || opts.RepoPath == "" {
		cr.Status = StatusSkipped
		cr.Detail = "no graph or repo path — drift check skipped"
		return cr
	}

	findings := enforce.AuditDrift(ctx, g, opts.RepoPath)

	errors := 0
	var stale []string
	for _, f := range findings {
		if f.Severity == enforce.SeverityError {
			errors++
			stale = append(stale, f.Message)
		}
	}

	if errors > 0 {
		cr.Status = StatusWeak
		cr.Detail = fmt.Sprintf("%d stale graph references: %s", errors, strings.Join(stale, "; "))
		cr.Missing = stale
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = "no stale graph references"
	return cr
}

// checkSmoke runs a single preflight smoke case and verifies expected invariants fire.
func checkSmoke(ctx context.Context, g *graph.Graph, opts Options, sc SmokeCase) CheckResult {
	cr := CheckResult{
		Kind: KindSmoke,
		Name: "smoke:" + sc.Name,
	}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — smoke case skipped"
		return cr
	}

	r, err := preflight.Run(ctx, preflight.Options{
		Task:    sc.Task,
		DocsDir: opts.DocsDir,
	}, g)
	if err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("preflight.Run failed: %v", err)
		return cr
	}

	// Check expected invariants.
	invSet := strSet(r.Invariants)
	for _, expected := range sc.ExpectedInvariants {
		if !invSet[expected] {
			cr.FalseSilences = append(cr.FalseSilences,
				fmt.Sprintf("smoke:%s — expected invariant %q not surfaced for task %q", sc.Name, expected, sc.Task))
		}
	}

	// Check expected forbidden fixes.
	ffSet := normalizeSet(r.ForbiddenFixes)
	for _, expected := range sc.ExpectedForbidden {
		if !ffSet[normalizeToken(expected)] {
			cr.FalseSilences = append(cr.FalseSilences,
				fmt.Sprintf("smoke:%s — expected forbidden fix %q not surfaced for task %q", sc.Name, expected, sc.Task))
		}
	}

	if len(cr.FalseSilences) > 0 {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("%d false silences: %s", len(cr.FalseSilences), strings.Join(cr.FalseSilences, "; "))
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("expected invariants %v surfaced", sc.ExpectedInvariants)
	return cr
}

func checkNodeContext(ctx context.Context, g *graph.Graph, opts Options) CheckResult {
	cr := CheckResult{Kind: KindNodeContext, Name: "node_context_smoke"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — node context smoke skipped"
		return cr
	}

	// Use tasks known to hit heartbeat/authority invariants.
	tasks := []string{
		"heartbeat writes desired state — authority violation",
		"runtime promoted to desired heartbeat authority",
	}
	var result analysis.AgentContextResult
	var err error
	for _, task := range tasks {
		_, result, err = analysis.GenerateAgentContext(ctx, g, task, analysis.AgentContextHints{})
		if err != nil {
			cr.Status = StatusFail
			cr.Detail = fmt.Sprintf("GenerateAgentContext: %v", err)
			return cr
		}
		if len(result.InvariantIDs) > 0 || len(result.FailureModeIDs) > 0 {
			cr.Status = StatusPass
			cr.Detail = fmt.Sprintf("node context returned %d invariants, %d failure modes",
				len(result.InvariantIDs), len(result.FailureModeIDs))
			return cr
		}
	}
	cr.Status = StatusWeak
	cr.Detail = "node-context smoke returned no invariants or failure modes — possible false silence for architectural task"
	cr.FalseSilences = []string{"node_context_smoke: no context returned for heartbeat-authority task"}
	return cr
}

func checkSemanticPath(ctx context.Context, g *graph.Graph) CheckResult {
	cr := CheckResult{Kind: KindSemanticPath, Name: "semantic_path_smoke"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — semantic path smoke skipped"
		return cr
	}

	// Run a path query between two well-known invariant nodes.
	// The path may be empty (sparse graph) but the call must not error.
	fromID := "invariant:infra.desired_hash_consistency"
	toID := "invariant:infra.heartbeat_not_desired_authority"

	_, err := semantic.ShortestPath(ctx, g, fromID, toID, semantic.PathOptions{MaxDepth: 6})
	if err != nil && !isNotFoundErr(err) {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("semantic path query failed: %v", err)
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("semantic path query completed without error (from %s → %s)", fromID, toID)
	return cr
}

func checkDebugSession(ctx context.Context, g *graph.Graph, opts Options) CheckResult {
	cr := CheckResult{Kind: KindDebugSession, Name: "debug_session_smoke"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — debug session smoke skipped"
		return cr
	}

	sess, err := debugsession.Run(ctx, debugsession.Options{
		Task:    "restart storm after controller deploy — convergence failure",
		DocsDir: opts.DocsDir,
	}, g)
	if err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("debugsession.Run: %v", err)
		return cr
	}
	if sess == nil {
		cr.Status = StatusFail
		cr.Detail = "debugsession.Run returned nil session"
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("debug session produced %d root cause paths, confidence=%s",
		len(sess.LikelyRootCausePaths), sess.Confidence)
	return cr
}

func checkCheckEdit(ctx context.Context, g *graph.Graph, opts Options) CheckResult {
	cr := CheckResult{Kind: KindCheckEdit, Name: "check_edit_smoke"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — check-edit smoke skipped"
		return cr
	}

	// A known high-risk file — must be index-able by the awareness graph.
	smokeFile := "golang/cluster_controller/cluster_controller_server/release_reconciler.go"

	result, err := checkedit.Run(ctx, g, checkedit.Options{File: smokeFile})
	if err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("checkedit.Run: %v", err)
		return cr
	}

	// We don't assert specific forbidden fixes — just that the call completed and
	// the high-risk file has at least one awareness signal (forbidden fix or code smell).
	if len(result.Warnings) > 0 && !result.HasIssues {
		cr.Status = StatusWeak
		cr.Detail = "check-edit completed with warnings but no issues — high-risk file may be missing annotations"
		cr.Noisy = result.Warnings
		return cr
	}

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("check-edit completed: has_issues=%v, %d forbidden fixes, %d code smells",
		result.HasIssues, len(result.ForbiddenFixes), len(result.CodeSmells))
	return cr
}

func checkPreflightAudit(ctx context.Context, g *graph.Graph) CheckResult {
	cr := CheckResult{Kind: KindPreflightAudit, Name: "preflight_audit_smoke"}

	if g == nil {
		cr.Status = StatusSkipped
		cr.Detail = "no graph — preflight audit smoke skipped"
		return cr
	}

	records, err := g.QueryPreflightAudits(ctx, 0, "")
	if err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("QueryPreflightAudits: %v", err)
		return cr
	}

	cr.Status = StatusPass
	if len(records) == 0 {
		cr.Detail = "preflight audit DB is readable — no records yet (run preflight with --write-audit to populate)"
	} else {
		cr.Detail = fmt.Sprintf("preflight audit DB readable — %d records found", len(records))
	}
	return cr
}

// checkMCPDiscovery verifies promote_proposal is NOT exposed via MCP.
// MCP safety invariant: awareness.mcp_must_not_expose_promotion.
// checkMCPDiscovery verifies the awareness.mcp_must_not_expose_promotion invariant
// by scanning the MCP registration source for forbidden calls.
// The standalone awareness/mcp server was removed in v1.2.20; the invariant is now
// enforced in golang/mcp/proposal_drain_tool.go (registerPromoteApprovedProposalsTool
// is deliberately not called from registerProposalDrainTools).
func checkMCPDiscovery(opts Options) CheckResult {
	cr := CheckResult{Kind: KindMCPDiscovery, Name: "mcp_promote_not_exposed"}

	// Locate the MCP proposal drain tool file relative to opts.DocsDir (../golang/mcp/).
	repoRoot := ""
	if opts.DocsDir != "" {
		// DocsDir is typically <repo>/docs/awareness — go up two levels.
		repoRoot = filepath.Join(opts.DocsDir, "..", "..")
	}
	if repoRoot == "" {
		cr.Status = StatusSkipped
		cr.Detail = "docs dir unknown — cannot verify MCP promotion invariant"
		return cr
	}

	drainFile := filepath.Join(repoRoot, "golang", "mcp", "proposal_drain_tool.go")
	data, err := os.ReadFile(drainFile)
	if err != nil {
		cr.Status = StatusSkipped
		cr.Detail = fmt.Sprintf("proposal_drain_tool.go not found (%v) — skipping MCP promotion check", err)
		return cr
	}

	// The invariant is violated if registerPromoteApprovedProposalsTool is called
	// from registerProposalDrainTools (i.e., the call is present and uncommented).
	// Look for a non-comment call to registerPromoteApprovedProposalsTool.
	// The function definition line starts with "func " — skip it.
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "func ") {
			continue
		}
		if strings.Contains(trimmed, "registerPromoteApprovedProposalsTool") {
			cr.Status = StatusFail
			cr.Detail = "MCP exposes promote_approved_proposals (must be CLI-only) — remove call from registerProposalDrainTools"
			return cr
		}
	}

	cr.Status = StatusPass
	cr.Detail = "MCP proposal_drain_tool.go does not call registerPromoteApprovedProposalsTool — promotion invariant holds"
	return cr
}
