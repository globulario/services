package preflight

// decision_trace.go — Phase 1 of the context-navigation effort. See
// claude_codex_awareness_context_navigation_improvement.md for the full
// design. Phase 1 deliberately keeps the scope tight:
//
//   - the types live on Report (see report.go)
//   - traces are populated from data preflight already collects
//   - no new graph traversal, no new analysis/contextnav package, no MCP tool
//
// What ships:
//
//   - one DecisionTrace per matched failure_mode, invariant, forbidden_fix
//   - one DecisionTrace per raw-knowledge fallback match (graph missed but
//     the source YAML matched — the trace foregrounds the fallback so it
//     can't be mistaken for graph proof)
//   - one DecisionTrace per matched runtime failure_mode (IncludeRuntime=true)
//   - MatchedBy carries Source = graph | raw_yaml | runtime | alias so the
//     reader can tell graph proof apart from fallback heuristics
//   - Pivots reference the failure_mode's source_invariant, the required
//     tests that touch the match set, the forbidden_fixes that fire for it,
//     and the matched aliases that triggered the lookup
//   - one generic Falsifier (Phase 6 will add per-failure_mode templates)
//   - Confidence inherits the Report's overall confidence, but raw-yaml-only
//     traces are capped at "low" so callers can't read fallback as proof
//
// What does NOT ship in Phase 1 (intentionally, with hooks in the types):
//
//   - OwnerContext (layer/service/package inference) — empty struct, Phase 3
//   - NextActions (remediation commands) — empty slice, Phase 7
//   - per-failure_mode Falsifiers — generic only, Phase 6
//
// The discipline: every shipped type must be the SAME shape it'll have in
// Phase 10 so MCP consumers don't have to chase schema changes. Adding fields
// later is additive; renaming or restructuring is not.

import (
	"fmt"
	"sort"
)

// rawKnowledgeFallbackHint is the generic falsifier used when no
// failure_mode-specific template exists yet. It tells the agent how to
// invalidate the match without prescribing the diagnosis. Phase 6 swaps in
// per-failure_mode templates.
const rawKnowledgeFallbackHint = "Re-run preflight after a graph rebuild and a live-snapshot collect; if the same finding does not reappear, the original match was a stale/fallback artifact."

// buildDecisionTraces composes a per-finding navigation layer from data the
// rest of preflight.Run already collected. Pure: no graph or storage I/O.
// Stable ordering so JSON output is deterministic across runs.
func buildDecisionTraces(r *Report) []DecisionTrace {
	if r == nil {
		return []DecisionTrace{}
	}

	// matchedAliases lifts to a set so each EvidenceRef can decide whether an
	// alias also helped trigger it. Aliases are a weaker signal than graph
	// edges, so they appear in MatchedBy but cap the per-evidence confidence.
	aliasSet := make(map[string]bool, len(r.MatchedAliases))
	for _, a := range r.MatchedAliases {
		aliasSet[a] = true
	}

	// Match runtime ids back to whatever failure_mode names they hit so the
	// runtime evidence lands on the right trace rather than orphaning.
	runtimeFMSet := map[string]bool{}
	if r.Runtime != nil {
		for _, fm := range r.Runtime.MatchedFailureModes {
			runtimeFMSet[fm] = true
		}
	}
	runtimeInvSet := map[string]bool{}
	if r.Runtime != nil {
		for _, id := range r.Runtime.MatchedInvariants {
			runtimeInvSet[id] = true
		}
	}

	// Track whether a raw-yaml id is already covered by a graph match so we
	// don't double-emit the same finding under two FindingTypes.
	covered := map[string]bool{}
	for _, id := range r.Invariants {
		covered["invariant:"+id] = true
	}
	for _, id := range r.FailureModes {
		covered["failure_mode:"+id] = true
	}
	for _, id := range r.ForbiddenFixes {
		covered["forbidden_fix:"+id] = true
	}

	traces := make([]DecisionTrace, 0,
		len(r.Invariants)+len(r.FailureModes)+len(r.ForbiddenFixes)+len(r.RawKnowledgeMatches))

	// failure_mode traces first — they're the most actionable axis.
	for _, fmID := range r.FailureModes {
		traces = append(traces, traceForFailureMode(r, fmID, aliasSet, runtimeFMSet))
	}
	// invariant traces.
	for _, invID := range r.Invariants {
		traces = append(traces, traceForInvariant(r, invID, aliasSet, runtimeInvSet))
	}
	// forbidden_fix traces — these are warnings about specific avoid-this
	// patterns, so they get their own surface in MatchedBy.
	for _, ffID := range r.ForbiddenFixes {
		traces = append(traces, traceForForbiddenFix(r, ffID, aliasSet))
	}
	// raw-yaml-only fallback traces. Only emit when the same id is not
	// already covered by a graph trace above — otherwise the same finding
	// would appear twice with different FindingTypes and confuse the reader.
	for _, raw := range r.RawKnowledgeMatches {
		nsID := raw.Kind + ":" + raw.ID
		if covered[nsID] {
			continue
		}
		traces = append(traces, traceForRawKnowledge(r, raw))
	}

	// Deterministic order: failure_modes by id, then invariants, then
	// forbidden_fixes, then raw_knowledge. Already grouped by build order
	// above; only sort within each type-group to keep the report stable.
	sort.SliceStable(traces, func(i, j int) bool {
		if traces[i].FindingType != traces[j].FindingType {
			return findingTypeRank(traces[i].FindingType) < findingTypeRank(traces[j].FindingType)
		}
		return traces[i].FindingID < traces[j].FindingID
	})
	return traces
}

func findingTypeRank(t FindingType) int {
	switch t {
	case FindingFailureMode:
		return 0
	case FindingInvariant:
		return 1
	case FindingForbiddenFix:
		return 2
	case FindingRawKnowledge:
		return 3
	case FindingRuntime:
		return 4
	case FindingExperience:
		return 5
	}
	return 99
}

func traceForFailureMode(r *Report, fmID string, aliasSet, runtimeFMSet map[string]bool) DecisionTrace {
	t := DecisionTrace{
		FindingID:   fmID,
		FindingType: FindingFailureMode,
		Confidence:  r.Confidence,
		MatchedBy:   []EvidenceRef{},
		Pivots:      []ContextPivot{},
		NextActions: []DiagnosticAction{},
		Falsifiers:  []Falsifier{genericFalsifier(r)},
	}

	t.MatchedBy = append(t.MatchedBy, EvidenceRef{
		Source:     "graph",
		NodeID:     "failure_mode:" + fmID,
		Confidence: graphEvidenceConfidence(r),
		Freshness:  graphFreshnessLabel(r),
		Reason:     "matched against awareness graph failure_modes",
	})
	if aliasSet[fmID] {
		t.MatchedBy = append(t.MatchedBy, EvidenceRef{
			Source:     "alias",
			NodeID:     fmID,
			Confidence: 0.55,
			Reason:     "task phrase matched the failure_mode's context_aliases",
		})
	}
	if runtimeFMSet[fmID] {
		t.MatchedBy = append(t.MatchedBy, EvidenceRef{
			Source:     "runtime",
			NodeID:     "runtime:" + fmID,
			Confidence: runtimeEvidenceConfidence(r),
			Freshness:  runtimeFreshnessLabel(r),
			Reason:     "live runtime evidence indicated this failure_mode",
		})
	}

	// Source invariants this failure_mode violates aren't tracked separately
	// in the Phase 1 Report, so pivot to every matched invariant — the agent
	// can disambiguate via the graph in a follow-up call. Mark as inferred.
	for _, invID := range r.Invariants {
		t.Pivots = append(t.Pivots, ContextPivot{
			Kind:        "source_invariant",
			ID:          "invariant:" + invID,
			WhyRelevant: "co-matched invariant; failure_mode likely violates it",
			Confidence:  0.6,
		})
	}
	for _, tn := range r.RequiredTests {
		t.Pivots = append(t.Pivots, ContextPivot{
			Kind:        "required_test",
			ID:          tn,
			WhyRelevant: "test obligation surfaced by graph for this match set",
			Confidence:  0.8,
		})
	}
	for _, ffID := range r.ForbiddenFixes {
		t.Pivots = append(t.Pivots, ContextPivot{
			Kind:        "forbidden_fix",
			ID:          ffID,
			WhyRelevant: "do-not-do pattern paired with this match set",
			Confidence:  0.85,
		})
	}
	return t
}

func traceForInvariant(r *Report, invID string, aliasSet, runtimeInvSet map[string]bool) DecisionTrace {
	t := DecisionTrace{
		FindingID:   invID,
		FindingType: FindingInvariant,
		Confidence:  r.Confidence,
		MatchedBy:   []EvidenceRef{},
		Pivots:      []ContextPivot{},
		NextActions: []DiagnosticAction{},
		Falsifiers:  []Falsifier{genericFalsifier(r)},
	}
	t.MatchedBy = append(t.MatchedBy, EvidenceRef{
		Source:     "graph",
		NodeID:     "invariant:" + invID,
		Confidence: graphEvidenceConfidence(r),
		Freshness:  graphFreshnessLabel(r),
		Reason:     "matched against awareness graph invariants",
	})
	if aliasSet[invID] {
		t.MatchedBy = append(t.MatchedBy, EvidenceRef{
			Source: "alias", NodeID: invID, Confidence: 0.55,
			Reason: "task phrase matched the invariant's context_aliases",
		})
	}
	if runtimeInvSet[invID] {
		t.MatchedBy = append(t.MatchedBy, EvidenceRef{
			Source:     "runtime",
			NodeID:     "runtime:" + invID,
			Confidence: runtimeEvidenceConfidence(r),
			Freshness:  runtimeFreshnessLabel(r),
			Reason:     "live runtime evidence implicated this invariant",
		})
	}
	for _, ffID := range r.ForbiddenFixes {
		t.Pivots = append(t.Pivots, ContextPivot{
			Kind:        "forbidden_fix",
			ID:          ffID,
			WhyRelevant: "do-not-do pattern paired with this invariant",
			Confidence:  0.85,
		})
	}
	for _, tn := range r.RequiredTests {
		t.Pivots = append(t.Pivots, ContextPivot{
			Kind:        "required_test",
			ID:          tn,
			WhyRelevant: "test obligation surfaced by graph for this invariant",
			Confidence:  0.8,
		})
	}
	return t
}

func traceForForbiddenFix(r *Report, ffID string, aliasSet map[string]bool) DecisionTrace {
	t := DecisionTrace{
		FindingID:   ffID,
		FindingType: FindingForbiddenFix,
		Confidence:  r.Confidence,
		MatchedBy: []EvidenceRef{{
			Source:     "graph",
			NodeID:     "forbidden_fix:" + ffID,
			Confidence: graphEvidenceConfidence(r),
			Freshness:  graphFreshnessLabel(r),
			Reason:     "matched against awareness graph forbidden_fixes",
		}},
		Pivots:      []ContextPivot{},
		NextActions: []DiagnosticAction{},
		Falsifiers:  []Falsifier{genericFalsifier(r)},
	}
	if aliasSet[ffID] {
		t.MatchedBy = append(t.MatchedBy, EvidenceRef{
			Source: "alias", NodeID: ffID, Confidence: 0.55,
			Reason: "task phrase matched the forbidden_fix's context_aliases",
		})
	}
	return t
}

func traceForRawKnowledge(r *Report, raw RawKnowledgeMatch) DecisionTrace {
	// Raw-yaml fallback is by definition NOT graph-proven. Cap confidence at
	// "low" so an agent can't read fallback output as strong evidence.
	conf := r.Confidence
	if conf != ConfidenceUnknown {
		conf = ConfidenceLow
	}
	t := DecisionTrace{
		FindingID:   raw.ID,
		FindingType: FindingRawKnowledge,
		Summary:     fmt.Sprintf("%s match from %s (graph missed; YAML caught it)", raw.Kind, raw.Source),
		Confidence:  conf,
		MatchedBy: []EvidenceRef{{
			Source:     "raw_yaml",
			NodeID:     raw.Kind + ":" + raw.ID,
			Confidence: 0.5,
			Reason:     fmt.Sprintf("matched %s in %s by terms: %v", raw.Kind, raw.Source, raw.MatchedTerms),
		}},
		Pivots:      []ContextPivot{},
		NextActions: []DiagnosticAction{},
		Falsifiers: []Falsifier{{
			Claim:      "this finding is real (not a stale YAML artifact)",
			HowToCheck: "rebuild the awareness graph and re-run preflight; if the same finding reappears under graph (not raw_yaml), it's load-bearing",
			Command:    "globular awareness build --clean",
		}},
		Warnings: []string{"fallback match — graph silence does not prove safety; treat as a hint, not proof"},
	}
	return t
}

func genericFalsifier(r *Report) Falsifier {
	if r.GraphFreshness != nil && r.GraphFreshness.Stale {
		return Falsifier{
			Claim:      "the graph path that produced this finding still exists after a fresh rebuild",
			HowToCheck: "rebuild the awareness graph and re-run preflight; the finding must reappear under the same FindingType",
			Command:    "globular awareness build --clean",
		}
	}
	return Falsifier{
		Claim:      "the graph path that produced this finding is intact",
		HowToCheck: rawKnowledgeFallbackHint,
	}
}

// graphEvidenceConfidence translates the Report's confidence enum into a
// 0..1 score for a graph EvidenceRef, with a stale-graph cap so high
// confidence cannot survive a stale graph in the per-evidence reading.
func graphEvidenceConfidence(r *Report) float64 {
	base := 0.85
	switch r.Confidence {
	case ConfidenceHigh:
		base = 0.9
	case ConfidenceMedium:
		base = 0.75
	case ConfidenceLow:
		base = 0.55
	case ConfidenceUnknown:
		base = 0.4
	}
	if r.GraphFreshness != nil && r.GraphFreshness.Stale && base > 0.5 {
		base = 0.5
	}
	return base
}

func runtimeEvidenceConfidence(r *Report) float64 {
	switch runtimeFreshnessLabel(r) {
	case "fresh":
		return 0.9
	case "stale":
		return 0.5
	}
	return 0.4
}

func graphFreshnessLabel(r *Report) string {
	if r.GraphFreshness == nil {
		return "unknown"
	}
	if r.GraphFreshness.Stale {
		return "stale"
	}
	return "fresh"
}

func runtimeFreshnessLabel(r *Report) string {
	if r.LiveOverlay == nil {
		return "unknown"
	}
	switch r.LiveOverlay.Status {
	case "fresh":
		return "fresh"
	case "stale":
		return "stale"
	case "absent":
		return "absent"
	}
	return "unknown"
}
