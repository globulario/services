// Package selfcheck runs the awareness system against itself — detecting
// false silences, noise, stale graph facts, and MCP safety regressions.
//
// It is observational only: it never mutates approved YAML files, never
// promotes proposals, and never auto-approves anything.
package selfcheck

import (
	"context"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
)

// CheckKind identifies which self-check sub-step produced a result.
type CheckKind string

const (
	KindBuild          CheckKind = "build"
	KindAudit          CheckKind = "audit"
	KindCoverage       CheckKind = "annotation_coverage"
	KindDrift          CheckKind = "graph_drift"
	KindSmoke          CheckKind = "preflight_smoke"
	KindNodeContext    CheckKind = "node_context_smoke"
	KindSemanticPath   CheckKind = "semantic_path_smoke"
	KindDebugSession   CheckKind = "debug_session_smoke"
	KindCheckEdit      CheckKind = "check_edit_smoke"
	KindPreflightAudit CheckKind = "preflight_audit_smoke"
	KindMCPDiscovery   CheckKind = "mcp_tool_discovery"
)

// CheckStatus is the outcome of a single self-check step.
type CheckStatus string

const (
	StatusPass    CheckStatus = "PASS"
	StatusFail    CheckStatus = "FAIL"
	StatusWeak    CheckStatus = "WEAK"    // matched but without strong evidence
	StatusSkipped CheckStatus = "SKIPPED" // check not applicable (e.g., no graph)
)

// CheckResult is the outcome of one self-check step.
type CheckResult struct {
	Kind          CheckKind
	Name          string
	Status        CheckStatus
	Detail        string   // human-readable explanation
	FalseSilences []string // expected invariants/fixes NOT found
	Noisy         []string // unexpected warnings or over-fired results
	Missing       []string // missing aliases, tests, etc.
}

// Options configures a self-check run.
type Options struct {
	RepoPath string // repo root (empty = use cwd)
	DocsDir  string // path to docs/awareness
	DBPath   string // path to graph.db (empty = no graph)
	Strict   bool   // if true, Report.StrictFail is set when any check fails
	// MaxTopWarningGroup, when >= 0, fails self-check strict mode if the largest
	// warning group in audit exceeds this count.
	MaxTopWarningGroup int
}

// Report is the complete self-check output.
type Report struct {
	GeneratedAt          time.Time
	Pass                 bool
	StrictFail           bool // true when Strict=true and any FAIL exists
	Checks               []CheckResult
	FalseSilences        []string // aggregated from all smoke checks
	NoisySections        []string
	MissingAliases       []string
	MissingTests         []string
	StaleRefs            []string
	MCPIssues            []string
	RecommendedFixes     []string
	ShouldCreateIncident bool
}

// SmokeCase defines one synthetic preflight scenario used to verify the
// awareness graph surfaces expected constraints for a given task string.
type SmokeCase struct {
	Name               string
	Task               string
	ExpectedInvariants []string
	ExpectedForbidden  []string
	Description        string
}

// SmokeCases is the canonical battery. Exported so tests can reference it.
var SmokeCases = []SmokeCase{
	{
		Name: "annotation_false_positive",
		Task: "annotation validator false positive check in production source",
		ExpectedInvariants: []string{
			"awareness.annotation_scanner.production_source_only",
		},
		Description: "Annotation scanner must restrict scanning to production source, not test fixtures.",
	},
	{
		Name: "desired_hash_mismatch",
		Task: "desired_hash mismatch task — hash never converges after deploy",
		ExpectedInvariants: []string{
			"infra.desired_hash_consistency",
		},
		ExpectedForbidden: []string{
			"use_raw_artifact_digest_as_desired_hash",
		},
		Description: "Desired-hash instability must surface infra.desired_hash_consistency and block the raw-digest forbidden fix.",
	},
	{
		Name: "missing_key_stopped_runtime",
		Task: "missing key stopped service — absence is not destructive intent",
		ExpectedInvariants: []string{
			"critical_state.absence_is_not_destructive_intent",
		},
		Description: "A missing etcd key must never be treated as a delete intent for a running service.",
	},
	{
		Name: "command_package_missing_unit",
		Task: "COMMAND package missing systemd unit file — runtime proof mismatch",
		ExpectedInvariants: []string{
			"runtime.installed_state_must_match_package_kind",
		},
		Description: "COMMAND packages must not require systemd units; the doctor must use package_kind as authority.",
	},
	{
		Name: "runtime_observation_desired_state",
		Task: "runtime observation created desired state — heartbeat should not set desired",
		ExpectedInvariants: []string{
			"infra.heartbeat_not_desired_authority",
		},
		Description: "Heartbeat must never write desired state. Only the controller reconciler has that authority.",
	},
}

// Run executes all self-check steps and returns a consolidated Report.
// g may be nil; graph-dependent checks are skipped gracefully.
func Run(ctx context.Context, opts Options, g *graph.Graph) (*Report, error) {
	r := &Report{
		GeneratedAt: time.Now(),
	}

	var allResults []CheckResult

	// 1. Build check — verify graph.db exists and was built recently.
	allResults = append(allResults, checkBuild(opts))

	// 2. Full enforcement audit.
	allResults = append(allResults, checkAudit(ctx, g, opts))

	// 3. Annotation coverage.
	allResults = append(allResults, checkAnnotationCoverage(ctx, g, opts))

	// 4. Graph drift.
	allResults = append(allResults, checkGraphDrift(ctx, g, opts))

	// 5. Preflight smoke cases.
	for _, sc := range SmokeCases {
		allResults = append(allResults, checkSmoke(ctx, g, opts, sc))
	}

	// 6. Node-context smoke.
	allResults = append(allResults, checkNodeContext(ctx, g, opts))

	// 7. Semantic path smoke.
	allResults = append(allResults, checkSemanticPath(ctx, g))

	// 8. Debug-session smoke.
	allResults = append(allResults, checkDebugSession(ctx, g, opts))

	// 9. Check-edit smoke.
	allResults = append(allResults, checkCheckEdit(ctx, g, opts))

	// 10. Preflight-audit smoke.
	allResults = append(allResults, checkPreflightAudit(ctx, g))

	// 11. MCP tool discovery smoke.
	allResults = append(allResults, checkMCPDiscovery(opts))

	r.Checks = allResults

	// Aggregate across all checks.
	failCount := 0
	for _, cr := range allResults {
		if cr.Status == StatusFail {
			failCount++
		}
		r.FalseSilences = append(r.FalseSilences, cr.FalseSilences...)
		r.NoisySections = append(r.NoisySections, cr.Noisy...)
		r.MCPIssues = appendIfKind(r.MCPIssues, cr, KindMCPDiscovery)
		r.StaleRefs = appendIfKind(r.StaleRefs, cr, KindDrift)
		r.MissingAliases = append(r.MissingAliases, filterMissingAliases(cr)...)
		r.MissingTests = append(r.MissingTests, filterMissingTests(cr)...)
	}

	r.Pass = failCount == 0
	r.ShouldCreateIncident = failCount > 0
	r.RecommendedFixes = buildRecommendedFixes(allResults)

	if opts.Strict && !r.Pass {
		r.StrictFail = true
	}

	return r, nil
}

// CheckOptions is an alias to Options used inside individual check functions
// to avoid the redundant type name.
type CheckOptions = Options
