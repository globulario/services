package preflight

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/analysis/contextnav"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/runtime"
)

// Options configures a preflight run.
type Options struct {
	Task               string                 // required: task description
	Files              []string               // optional: files to run impact analysis on
	PackagePath        string                 // optional: path to package dir with awareness.yaml
	Phase              string                 // optional: dependency phase filter for cycle detection
	DocsDir            string                 // path to docs/awareness (aliases, fix_cases, guardrails)
	RepoRoot           string                 // optional: repo root for Go file coverage computation
	IncludeRuntime     bool                   // collect live runtime snapshot
	RuntimeWindow      time.Duration          // lookback window for events/workflows (default 15m)
	Bridge             *runtime.RuntimeBridge // optional: if nil and IncludeRuntime, uses noop bridge
	WriteAudit         bool                   // if true, persist a PreflightAuditRecord to the graph DB after the run
	GitSHA             string                 // optional: current git commit SHA for the audit record
	Now                time.Time              // injectable clock for tests; zero → time.Now()
	BundleManifestPath string                 // optional: path to installed awareness-bundle manifest.json; populates Staleness.BundlePresent
}

// Run executes the full preflight and returns a structured report.
// g may be nil — in that case graph-dependent sections are skipped with a warning.
func Run(ctx context.Context, opts Options, g *graph.Graph) (*Report, error) {
	r := &Report{
		Task:     opts.Task,
		Files:    opts.Files,
		Cycles:   []CycleWarning{},
		DidWeFix: &DidWeFixSection{},
	}

	// 1. Classify task.
	r.Classification = ClassifyTask(opts.Task)

	// 2. Load aliases from docs dir (silent on missing).
	aliasMap := loadAliases(opts.DocsDir)

	// 3. Load fix cases and guardrails.
	fixCases := loadFixCases(opts.DocsDir)

	// 4. Alias matching — collect which aliases fired.
	r.MatchedAliases = matchAliases(opts.Task, aliasMap)

	// 5. Agent context from graph (invariants, failure modes, forbidden fixes, tests, searches, services).
	r.GraphAvailable = g != nil
	if g != nil {
		acResult, warn := runAgentContext(ctx, g, opts.Task, opts.Files, aliasMap)
		if warn != "" {
			r.Warnings = append(r.Warnings, warn)
		}
		r.Invariants = acResult.InvariantIDs
		r.FailureModes = acResult.FailureModeIDs
		r.ForbiddenFixes = acResult.ForbiddenFixes
		r.RequiredTests = acResult.RequiredTests
		r.RequiredSearches = acResult.RequiredSearches
		r.Services = acResult.ServiceNames
		r.GraphMatchCount = len(r.Invariants) + len(r.FailureModes) + len(r.ForbiddenFixes)

		// 5b. Trust filter: annotate low-trust graph matches into FilteredMatches.
		// Low-trust matches are still present in the main result lists (per spec:
		// do not suppress them entirely) but are also reported under FilteredMatches
		// so callers understand why graph_filtered_by_trust_count > 0.
		r.FilteredMatches = checkNodesTrust(ctx, g, r.Invariants, r.FailureModes, r.ForbiddenFixes)
		r.GraphFilteredByTrustCount = len(r.FilteredMatches)
	} else {
		r.Warnings = append(r.Warnings, "no graph DB — graph-dependent sections skipped (run 'globular awareness build' first)")
		r.Warnings = append(r.Warnings, "UNKNOWN_IMPACT: graph unavailable — use awareness.preflight with graph for reliable classification")
	}

	// 6. Impact analysis per file (annotation-priority first, then transitive traversal).
	if g != nil && len(opts.Files) > 0 {
		r = mergeImpact(ctx, g, opts.Files, r)
	}

	// 7. Package context + admission if --package provided.
	if opts.PackagePath != "" {
		r.Classification = appendClass(r.Classification, ClassPackageAdmission)
		pas, pkgNames := runPackageAdmission(ctx, g, opts.PackagePath)
		r.PackageAdmission = pas
		r.Packages = unique(append(r.Packages, pkgNames...))
	}

	// 8. Cycle detection if --phase provided.
	if g != nil && opts.Phase != "" {
		cycleWarnings, err := runCycles(ctx, g, opts.Phase)
		if err != nil {
			r.Warnings = append(r.Warnings, "cycle detection: "+err.Error())
		} else if len(cycleWarnings) > 0 {
			r.Cycles = cycleWarnings
			r.Classification = appendClass(r.Classification, ClassDependencyCycle)
		}
	}

	// 9. Fix-ledger: did-we-fix.
	r.DidWeFix = runDidWeFix(opts.Task, fixCases, aliasMap)

	// 10. Aggregate required tests from impact results (dedup with acResult already merged).
	r.RequiredTests = unique(r.RequiredTests)

	// 11. Aggregate forbidden fixes (dedup).
	r.ForbiddenFixes = unique(r.ForbiddenFixes)
	r.ForbiddenFixes = append(r.ForbiddenFixes, guardrailForbiddenFixes(opts.Task, opts.DocsDir)...)
	r.ForbiddenFixes = unique(r.ForbiddenFixes)

	// 11b. Collect code smells and design context from invariants.
	// Use both graph-matched invariants and alias-matched invariant IDs so that
	// design patterns surface even when GenerateAgentContext doesn't traverse
	// to those invariant nodes via graph edges.
	if g != nil {
		invIDSet := make(map[string]bool)
		for _, id := range r.Invariants {
			invIDSet["invariant:"+id] = true
		}
		for _, alias := range r.MatchedAliases {
			invIDSet["invariant:"+alias] = true
		}
		if len(invIDSet) > 0 {
			invNodeIDs := make([]string, 0, len(invIDSet))
			for id := range invIDSet {
				invNodeIDs = append(invNodeIDs, id)
			}
			// Legacy pattern nodes (patterns.yaml).
			if smells, err := g.CodeSmellsForInvariants(ctx, invNodeIDs); err == nil {
				r.CodeSmells = smells
			}
			// Design pattern layer (design_patterns.yaml).
			if dc, err := g.DesignContextForInvariants(ctx, invNodeIDs); err == nil {
				r.DesignPatterns = unique(dc.DesignPatterns)
				r.AntiPatterns = unique(dc.AntiPatterns)
				r.CodeSmells = unique(append(r.CodeSmells, dc.CodeSmells...))
			}
		}
	}

	// 12. Raw YAML fallback: graph/query NO_MATCH is not proof of safety.
	// The source awareness YAML files are the authority; the graph is an index.
	// Cross-check them directly so stale graph nodes or query misses do not
	// create false confidence.
	rawMatches := RawKnowledgeFallback(opts.Task, opts.Files, opts.DocsDir)
	r.RawKnowledgeMatches = rawMatches
	r.RawYAMLMatchCount = len(rawMatches)
	if len(rawMatches) > 0 {
		r = mergeRawKnowledgeMatches(r, rawMatches)
	}

	// 13. False-silence detection and UNKNOWN_IMPACT gating.
	noFacts := noAwarenessFactsMatched(r)
	if noFacts {
		r.Warnings = append(r.Warnings, "NO_AWARENESS_MATCH: no awareness facts matched this task. This does not prove the task is safe.")
	} else if len(rawMatches) > 0 {
		r.Classification = appendClass(r.Classification, ClassStaticFallback)
		r.Warnings = append(r.Warnings, "RAW_KNOWLEDGE_CROSSCHECK: source YAML matched relevant awareness facts; graph/query silence must not be treated as safe.")
	}
	if hasClass(r.Classification, ClassArchitectureSensitive) && noFacts {
		r.Classification = appendClass(r.Classification, ClassUnknownImpact)
	}

	// 14. Build recommended investigation order.
	if g != nil {
		domain := inferExperienceDomain(r)
		capability := inferExperienceCapability(r)
		hits, err := g.SearchSimilarExperiences(ctx, graph.ExperienceSearchQuery{
			Goal:            opts.Task,
			Domain:          domain,
			Capability:      capability,
			Files:           opts.Files,
			InvariantIDs:    r.Invariants,
			ForbiddenFixIDs: r.ForbiddenFixes,
			Limit:           3,
		})
		if err != nil {
			r.Warnings = append(r.Warnings, "experience search failed: "+err.Error())
		} else {
			for _, h := range hits {
				r.ExperienceHints = append(r.ExperienceHints, ExperienceHint{
					ExperienceID:  h.ExperienceID,
					Score:         h.Score,
					Strategy:      h.StrategyID,
					Hint:          h.Hint,
					Status:        h.Status,
					Summary:       h.Summary,
					Verdict:       h.Verdict,
					FinalScore:    h.FinalScore,
					Reasons:       h.Reasons,
					WorkedPaths:   h.WorkedPaths,
					FailedPaths:   h.FailedPaths,
					EvidenceTypes: h.EvidenceTypes,
				})
			}
		}
	}

	// 14. Build recommended investigation order.
	r.RecommendedOrder = buildInvestigationOrder(r)

	// 15. Build agent instruction.
	r.AgentInstruction = buildAgentInstruction(r)

	// 16. Runtime snapshot (optional).
	if opts.IncludeRuntime {
		r = mergeRuntime(ctx, opts, g, r)
	}

	// 17. Durable audit record (optional).
	if opts.WriteAudit && g != nil {
		rec := graph.PreflightAuditRecord{
			Task:           opts.Task,
			GitSHA:         opts.GitSHA,
			Files:          r.Files,
			ForbiddenFixes: r.ForbiddenFixes,
			Invariants:     r.Invariants,
			CodeSmells:     r.CodeSmells,
		}
		if err := g.InsertPreflightAudit(ctx, rec); err != nil {
			r.Warnings = append(r.Warnings, "preflight-audit write failed: "+err.Error())
		}
	}

	// 18. Graph freshness + collector health.
	if g != nil && opts.DocsDir != "" {
		f := g.Freshness(ctx, opts.DocsDir)
		gfr := &GraphFreshnessReport{
			BuiltAt:             formatTime(f.BuiltAt),
			AgeSeconds:          f.AgeSeconds,
			Stale:               f.Stale,
			StaleReason:         f.StaleReason,
			KnowledgeMtime:      formatTime(f.KnowledgeMtime),
			KnowledgeSourceHash: f.KnowledgeSourceHash,
			RebuildRecommended:  f.RebuildRecommended,
		}
		// Include last build duration and collector health if available.
		if rec, err := g.LatestBuildRecord(ctx); err == nil && rec != nil {
			gfr.LastBuildDurationMs = rec.Stats.DurationMs
			for _, ch := range rec.CollectorHealth {
				r.CollectorHealth = append(r.CollectorHealth, CollectorHealthSummary{
					CollectorID:  ch.CollectorID,
					Status:       ch.Status,
					NodesEmitted: ch.NodesEmitted,
					Error:        ch.Error,
				})
				if ch.Status == "error" {
					r.Warnings = append(r.Warnings,
						fmt.Sprintf("COLLECTOR_ERROR: %s failed: %s — graph may be missing nodes from this tier", ch.CollectorID, ch.Error))
					r.BlindSpots = append(r.BlindSpots,
						fmt.Sprintf("collector %s errored: nodes from this source may be absent from the graph", ch.CollectorID))
				}
			}
		}
		r.GraphFreshness = gfr
		if f.Stale && len(r.RawKnowledgeMatches) > 0 {
			r.Warnings = append(r.Warnings, "GRAPH_STALE: "+f.StaleReason+". Raw YAML matched — treat graph as incomplete.")
		}
	}

	// 18a. Go file coverage — measures what fraction of eligible Go files are indexed.
	// Low coverage lowers confidence and is reported as a blind spot.
	if opts.RepoRoot != "" {
		cov := computeGoFileCoverage(ctx, g, opts.RepoRoot)
		r.GoFileCoverage = cov
		if cov.ConfidenceImpact == "high" {
			r.BlindSpots = append(r.BlindSpots, cov.BlindSpots...)
			r.Warnings = append(r.Warnings, fmt.Sprintf(
				"UNKNOWN_IMPACT_LOW_COVERAGE: graph indexes only %.1f%% of eligible Go files — NO_MATCH results cannot be trusted as safe",
				cov.CoveragePercentGoFiles))
		} else if cov.ConfidenceImpact == "medium" {
			r.BlindSpots = append(r.BlindSpots, cov.BlindSpots...)
		}
	}

	// 18b. Live overlay freshness — separate from graph freshness.
	// Missing or stale live overlay is a blind spot for runtime-sensitive tasks.
	if g != nil {
		r.LiveOverlay = ComputeLiveOverlayFreshness(ctx, g, opts.Now)
		if r.LiveOverlay.Status == "absent" {
			r.BlindSpots = append(r.BlindSpots,
				"live overlay absent — run 'globular awareness live-snapshot' or enable systemd timer for scheduled refresh")
		} else if r.LiveOverlay.Status == "stale" {
			r.Warnings = append(r.Warnings,
				fmt.Sprintf("LIVE_OVERLAY_STALE: live mirror is %.0fs old (threshold %ds) — runtime evidence may be outdated",
					r.LiveOverlay.AgeSeconds, LiveOverlayStaleSeconds))
		} else if r.LiveOverlay.Status == "failed" || r.LiveOverlay.Status == "partial" {
			r.BlindSpots = append(r.BlindSpots,
				fmt.Sprintf("live overlay %s — some collectors could not refresh runtime evidence", r.LiveOverlay.Status))
		}
	}

	// 18c. Risk tier and optional low-risk fast path.
	r.RiskTier = computeRiskTier(r, g)
	r.FastPathApplied = applyLowRiskFastPath(r)

	// 19. Confidence model.
	r.Coverage, r.Confidence, r.ConfidenceReason, r.BlindSpots, r.ConfidenceFactors = computeConfidence(r, g)
	// Demote confidence if Go file coverage is critically low: the graph cannot
	// give reliable NO_MATCH answers for files it has never seen.
	if r.GoFileCoverage != nil && r.GoFileCoverage.ConfidenceImpact == "high" {
		if r.Confidence == ConfidenceHigh {
			r.Confidence = ConfidenceMedium
			r.ConfidenceReason += "; demoted: graph Go-file coverage < 70%"
		}
	}
	// Merge GoFileCoverage blind spots (computeConfidence builds its own slice).
	if r.GoFileCoverage != nil && len(r.GoFileCoverage.BlindSpots) > 0 {
		r.BlindSpots = unique(append(r.BlindSpots, r.GoFileCoverage.BlindSpots...))
	}
	r.SafetyStatus = computeSafetyStatus(r)
	r.DegradedMode = computeDegradedMode(r)
	if r.SafetyStatus == SafetyStatusUnknownNotSafe {
		r.Warnings = append(r.Warnings,
			"UNKNOWN_NOT_SAFE: evidence is incomplete for a sensitive task. Rebuild graph and collect runtime evidence before code changes.")
	}
	r.Trust = computeTrustEnvelope(ctx, g, opts, r)

	// Per-finding decision traces (analysis/contextnav).
	//
	// Phase 8 ordering contract: r.Trust MUST be computed before
	// buildContextnavInputs runs, so the adapter can lift TrustVerdict /
	// TrustConfidence / TrustFreshness / TrustReason into BuildInputs
	// and the trust_envelope EvidenceRef lands on every non-rawKnowledge
	// trace. Re-ordering these two lines silently empties the trust
	// evidence on every trace — keep them adjacent in this order.
	//
	// Pure composition over data already in the Report; produces an empty
	// slice when no findings matched so the JSON shape stays
	// "decision_traces: []" rather than null — agents must not read the
	// absence of a key as the absence of risk. The trust envelope above
	// remains the authority for NO_MATCH safety.
	r.DecisionTraces = contextnav.Build(buildContextnavInputs(ctx, r, g, opts.Task, opts.Files))

	return r, nil
}
