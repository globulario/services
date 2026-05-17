package preflight

import (
	"context"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/assurance"
	"github.com/globulario/services/golang/awareness/bundlesync"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/integrity"
)

func computeTrustEnvelope(ctx context.Context, g *graph.Graph, opts Options, r *Report) *assurance.TrustEnvelope {
	if r == nil {
		return nil
	}
	// PrimaryMatchKind picks the most-specific match kind so the trust
	// envelope's reason text stays honest. Order matters: failure_mode wins
	// when both are present (it's the most actionable axis); raw YAML is a
	// last-resort fallback. See docs/awareness/composed_path_failures.md
	// (TrustEnvelope match-kind).
	primaryKind := ""
	switch {
	case len(r.FailureModes) > 0:
		primaryKind = assurance.MatchKindFailureMode
	case len(r.Invariants) > 0:
		primaryKind = assurance.MatchKindInvariant
	case len(r.ForbiddenFixes) > 0:
		primaryKind = assurance.MatchKindForbiddenFix
	case len(r.RawKnowledgeMatches) > 0:
		primaryKind = assurance.MatchKindRawYAML
	}
	in := assurance.ComposeInputs{
		MatchFound:       len(r.Invariants) > 0 || len(r.FailureModes) > 0 || len(r.ForbiddenFixes) > 0 || len(r.RawKnowledgeMatches) > 0,
		PrimaryMatchKind: primaryKind,
	}
	if g != nil {
		stOpts := assurance.Options{DocsDir: opts.DocsDir}
		// Resolve the manifest path: explicit caller value first, then the
		// canonical install path. This makes the joined freshness pipeline
		// usable by every caller (CLI, MCP, in-process) without each one
		// having to re-derive the path. A missing/unreadable manifest is
		// silently OK — Staleness.BundlePresent stays false and the verdict
		// reflects the gap honestly.
		manifestPath := opts.BundleManifestPath
		if manifestPath == "" {
			manifestPath = bundlesync.DefaultManifestPath()
		}
		if m, mErr := bundlesync.LoadManifest(manifestPath); mErr == nil {
			stOpts.Manifest = m
		}
		if st, err := assurance.CheckStaleness(ctx, g, stOpts); err == nil {
			in.Staleness = st
		}
		if len(r.FailureModes) > 0 {
			if cov, err := assurance.ComputeCoverage(ctx, g); err == nil {
				in.PerFailureMode = chooseFailureModeCoverage(cov, r.FailureModes)
			}
		}
	}
	env := assurance.Compose(in)
	return &env
}

// chooseFailureModeCoverage picks the worst-coverage entry among the matched
// failure_modes, since the trust envelope must reflect the weakest link in the
// match set. Uses CoverageReport.CoverageFor (O(1) per id) so the whole pass
// is O(len(matched)) instead of O(len(PerFailureMode) * len(matched)).
func chooseFailureModeCoverage(cov *assurance.CoverageReport, matched []string) *assurance.FailureModeCoverage {
	if cov == nil || len(cov.PerFailureMode) == 0 || len(matched) == 0 {
		return nil
	}
	priority := map[string]int{"ORPHAN": 0, "PARTIAL": 1, "DETECTED": 2, "TESTED": 3, "ENFORCED": 4, "DEPRECATED": 5, "INTENTIONAL_GAP": 6}
	var best *assurance.FailureModeCoverage
	bestRank := 99
	for _, id := range matched {
		fm := cov.CoverageFor(id)
		if fm == nil {
			continue
		}
		rank := 99
		if r, ok := priority[fm.State]; ok {
			rank = r
		}
		if best == nil || rank < bestRank {
			best = fm
			bestRank = rank
		}
	}
	return best
}

func inferExperienceDomain(r *Report) string {
	for _, s := range r.Services {
		switch strings.ToLower(s) {
		case "workflow":
			return "workflow"
		case "repository":
			return "repository"
		case "cluster-controller", "cluster":
			return "cluster"
		}
	}
	if len(r.Classification) > 0 {
		switch r.Classification[0] {
		case ClassRuntimeIncident, ClassRetryLoop, ClassRestartStorm:
			return "workflow"
		}
	}
	return ""
}

func inferExperienceCapability(r *Report) string {
	for _, s := range r.Services {
		if strings.EqualFold(s, "workflow") {
			return "workflow.defer"
		}
	}
	return ""
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func computeConfidence(r *Report, g *graph.Graph) (Coverage, Confidence, string, []string, ConfidenceFactors) {
	// Graph coverage.
	var graphCov CoverageState
	if g == nil {
		graphCov = CoverageNotChecked
	} else if len(r.Invariants) > 0 || len(r.FailureModes) > 0 {
		graphCov = CoverageCheckedWithMatch
	} else {
		graphCov = CoverageCheckedClean
	}
	if r.GraphFreshness != nil && r.GraphFreshness.Stale {
		graphCov = CoverageStale
	}

	// Raw YAML coverage — the raw fallback always runs in step 12.
	// We track whether it ran (it always does) and whether it found matches.
	var rawYAMLCov CoverageState
	if len(r.RawKnowledgeMatches) > 0 {
		rawYAMLCov = CoverageCheckedWithMatch
	} else {
		rawYAMLCov = CoverageCheckedClean
	}

	// Runtime coverage.
	var runtimeCov CoverageState
	if r.Runtime == nil || !r.Runtime.Included {
		runtimeCov = CoverageNoop
	} else {
		hasEvidence := len(r.Runtime.DoctorFindings) > 0 ||
			len(r.Runtime.ServiceStatuses) > 0 ||
			len(r.Runtime.WorkflowReceipts) > 0 ||
			len(r.Runtime.StateDeltas) > 0 ||
			len(r.Runtime.MatchedInvariants) > 0 ||
			len(r.Runtime.MatchedFailureModes) > 0
		if hasEvidence {
			runtimeCov = CoverageCheckedWithMatch
		} else {
			runtimeCov = CoverageCheckedClean
		}
	}

	// Metrics coverage.
	var metricsCov CoverageState
	if r.Runtime == nil || !r.Runtime.Included {
		metricsCov = CoverageNoop
	} else if len(r.Runtime.MetricWarnings) > 0 {
		metricsCov = CoverageCheckedWithMatch
	} else {
		metricsCov = CoverageCheckedClean
	}

	cov := Coverage{
		Graph:         graphCov,
		RawYAML:       rawYAMLCov,
		Runtime:       runtimeCov,
		Metrics:       metricsCov,
		CodeScan:      CoverageNotChecked, // scan_violations is a separate tool
		IncidentStore: CoverageNotChecked,
	}

	var blindSpots []string
	if graphCov == CoverageNotChecked {
		blindSpots = append(blindSpots, "graph unavailable — static pattern matching only")
	}
	if graphCov == CoverageStale {
		blindSpots = append(blindSpots, "graph stale — knowledge may be incomplete")
	}
	if runtimeCov == CoverageNoop {
		blindSpots = append(blindSpots, "runtime snapshot not collected — no live cluster evidence")
	}
	if metricsCov == CoverageNoop {
		blindSpots = append(blindSpots, "metrics not available — resource saturation not assessed")
	}
	if cov.CodeScan == CoverageNotChecked {
		blindSpots = append(blindSpots, "code violation scan not run — use awareness.scan_violations")
	}

	// Score the confidence.
	// graph checked (any non-not_checked state except stale):
	graphChecked := graphCov == CoverageCheckedClean || graphCov == CoverageCheckedWithMatch
	rawYAMLChecked := rawYAMLCov == CoverageCheckedClean || rawYAMLCov == CoverageCheckedWithMatch
	runtimeActive := runtimeCov == CoverageCheckedClean || runtimeCov == CoverageCheckedWithMatch
	metricsActive := metricsCov == CoverageCheckedClean || metricsCov == CoverageCheckedWithMatch

	score := 0
	if graphChecked {
		score++
	}
	if rawYAMLChecked {
		score++
	}
	if runtimeActive {
		score++
	}
	if metricsActive {
		score++
	}

	var conf Confidence
	var reason string
	switch {
	case score >= 3:
		conf = ConfidenceHigh
		reason = "graph, raw YAML, and runtime evidence all collected"
	case score == 2:
		conf = ConfidenceMedium
		reason = "static analysis complete; runtime evidence partial or unavailable"
	case score == 1:
		conf = ConfidenceLow
		reason = "only partial static analysis available; runtime sources unavailable"
	default:
		conf = ConfidenceUnknown
		reason = "no awareness data collected — graph missing and runtime unavailable"
	}

	// Override if graph is stale and no runtime compensates.
	if graphCov == CoverageStale && !runtimeActive {
		if conf == ConfidenceHigh {
			conf = ConfidenceMedium
			reason += "; demoted due to stale graph"
		}
	}

	factors := ConfidenceFactors{
		Coverage:        cov.Graph,
		Provenance:      "graph+raw_yaml",
		GraphFreshness:  graphCov,
		PathQuality:     "trusted_or_declared",
		RuntimeEvidence: runtimeCov,
	}
	if r.GraphFilteredByTrustCount > 0 {
		factors.PathQuality = "mixed_trust_paths"
	}
	if r.GraphMatchCount == 0 && r.RawYAMLMatchCount > 0 {
		factors.Provenance = "raw_yaml_fallback"
	}
	if g == nil {
		factors.Provenance = "raw_yaml_only"
	}

	return cov, conf, reason, blindSpots, factors
}

func computeSafetyStatus(r *Report) SafetyStatus {
	if r == nil {
		return SafetyStatusUnknownNotSafe
	}
	sensitive := hasClass(r.Classification, ClassArchitectureSensitive) ||
		hasClass(r.Classification, ClassConvergenceRisk)
	unknownImpact := hasClass(r.Classification, ClassUnknownImpact)
	graphStaleNoRuntime := r.Coverage.Graph == CoverageStale && r.Coverage.Runtime == CoverageNoop
	if sensitive && (unknownImpact || graphStaleNoRuntime) {
		return SafetyStatusUnknownNotSafe
	}
	return SafetyStatusProceed
}

func computeDegradedMode(r *Report) DegradedModePlaybook {
	p := DegradedModePlaybook{}
	if r == nil {
		p.Enabled = true
		p.Reason = "preflight report unavailable"
		return p
	}

	graphUnavailable := r.Coverage.Graph == CoverageNotChecked
	graphStale := r.Coverage.Graph == CoverageStale
	sensitive := hasClass(r.Classification, ClassArchitectureSensitive) || hasClass(r.Classification, ClassConvergenceRisk)
	if !(graphUnavailable || graphStale || r.SafetyStatus == SafetyStatusUnknownNotSafe) {
		return p
	}

	p.Enabled = true
	if graphUnavailable {
		p.Reason = "graph unavailable"
	} else if graphStale {
		p.Reason = "graph stale"
	} else {
		p.Reason = "insufficient trusted evidence"
	}
	p.AllowedNextSteps = []string{
		"Run `globular awareness build --clean` to refresh graph evidence.",
		"Re-run preflight with the exact task and target files.",
		"Use raw YAML matches as guidance only; require tests before any behavior change.",
	}
	p.BlockedActions = []string{
		"Do not apply destructive or runtime-stop changes from inferred intent.",
		"Do not treat no-match as safe for architecture-sensitive tasks.",
		"Do not promote confidence to high without refreshed graph or runtime evidence.",
	}
	p.StopConditions = []string{
		"Stop and escalate if preflight still returns UNKNOWN_NOT_SAFE after rebuild.",
		"Stop if required tests cannot be identified for a sensitive task.",
	}
	if sensitive {
		p.StopConditions = append(p.StopConditions,
			"Stop if any proposed fix violates listed forbidden fixes.")
	}
	return p
}

func computeRiskTier(r *Report, g *graph.Graph) RiskTier {
	if r == nil {
		return RiskHigh
	}
	if hasClass(r.Classification, ClassArchitectureSensitive) ||
		hasClass(r.Classification, ClassConvergenceRisk) ||
		hasClass(r.Classification, ClassDependencyCycle) ||
		hasClass(r.Classification, ClassPackageAdmission) ||
		hasClass(r.Classification, ClassRuntimeIncident) {
		return RiskHigh
	}
	if hasClass(r.Classification, ClassLocalCodeChange) && len(r.Files) == 0 {
		// Only low risk when the graph was available and confirmed no file impact.
		// With a nil graph, zero file matches means the graph did not cover this
		// task — that is a coverage gap, not confirmed low impact.
		if g != nil {
			return RiskLow
		}
	}
	return RiskMedium
}

func applyLowRiskFastPath(r *Report) bool {
	if r == nil {
		return false
	}
	if r.RiskTier != RiskLow {
		return false
	}
	if hasClass(r.Classification, ClassUnknownImpact) {
		return false
	}
	// Fast-path is only for truly local/no-context tasks. If awareness already
	// matched aliases or architectural facts, keep full context fidelity.
	if len(r.MatchedAliases) > 0 || len(r.Invariants) > 0 || len(r.FailureModes) > 0 {
		return false
	}
	if r.GraphFreshness != nil && r.GraphFreshness.Stale {
		return false
	}
	// FastPathApplied is a pure signal — no data is truncated here.
	// Context reduction for presentation belongs in the render layer, not
	// on the shared Report struct where truncation is unrecoverable.
	r.Warnings = append(r.Warnings, "FAST_PATH_APPLIED: low-risk task; graph confirmed no architectural impact.")
	return true
}

func noAwarenessFactsMatched(r *Report) bool {
	if r == nil {
		return true
	}
	hasFixLedgerMatch := false
	if r.DidWeFix != nil {
		hasFixLedgerMatch = len(r.DidWeFix.FixCases) > 0
	}
	hasPackageContext := len(r.Packages) > 0 || r.PackageAdmission != nil
	hasRuntimeEvidence := false
	if r.Runtime != nil {
		hasRuntimeEvidence = len(r.Runtime.DoctorFindings) > 0 ||
			len(r.Runtime.ServiceStatuses) > 0 ||
			len(r.Runtime.WorkflowReceipts) > 0 ||
			len(r.Runtime.StateDeltas) > 0 ||
			len(r.Runtime.MatchedInvariants) > 0 ||
			len(r.Runtime.MatchedFailureModes) > 0
	}

	return len(r.Invariants) == 0 &&
		len(r.FailureModes) == 0 &&
		!hasFixLedgerMatch &&
		len(r.MatchedAliases) == 0 &&
		!hasPackageContext &&
		!hasRuntimeEvidence
}

// lowTrustLevels are VerificationLevel values that indicate a match is present
// in the graph but not yet fully validated. The match is kept in the main result
// lists — it is NOT suppressed — but also reported in FilteredMatches with the
// reason so callers can lower their confidence accordingly.
var lowTrustLevels = map[string]bool{
	integrity.TrustStale:    true,
	integrity.TrustInferred: true,
	integrity.TrustProposal: true,
	integrity.TrustInvalid:  true,
}

// trustReasonFor maps a trust level to the human-readable reason reported in
// FilteredMatch.Reason. When the node has no trust metadata the reason is
// "missing_provenance" — not an error, but worth surfacing.
func trustReasonFor(level string) string {
	switch level {
	case integrity.TrustStale:
		return "stale"
	case integrity.TrustInferred:
		return "inferred"
	case integrity.TrustProposal:
		return "proposal"
	case integrity.TrustInvalid:
		return "invalid"
	default:
		return level
	}
}

// checkNodesTrust inspects matched invariant/failure_mode/forbidden_fix node
// metadata for low trust provenance. It does not remove the matches from the
// main result; it returns a parallel FilteredMatch slice so callers know which
// graph findings carry reduced confidence.
func checkNodesTrust(ctx context.Context, g *graph.Graph, invariantIDs, failureModeIDs, forbiddenFixIDs []string) []FilteredMatch {
	var out []FilteredMatch

	check := func(id, kind, nodePrefix string) {
		nodeID := nodePrefix + id
		n, err := g.FindNode(ctx, nodeID)
		if err != nil || n == nil {
			return
		}
		tl, _ := n.Metadata["trust_level"].(string)
		if tl == "" {
			tl, _ = n.Metadata["verification_level"].(string)
		}
		if tl != "" && lowTrustLevels[tl] {
			out = append(out, FilteredMatch{
				ID:         id,
				Kind:       kind,
				Reason:     trustReasonFor(tl),
				TrustLevel: tl,
			})
		}
	}

	for _, id := range invariantIDs {
		check(id, "invariant", "invariant:")
	}
	for _, id := range failureModeIDs {
		check(id, "failure_mode", "failure_mode:")
	}
	for _, id := range forbiddenFixIDs {
		check(id, "forbidden_fix", "forbidden_fix:")
	}
	return out
}
