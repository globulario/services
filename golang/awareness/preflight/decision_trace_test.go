package preflight

// decision_trace_test.go — Phase 1 acceptance tests for the per-finding
// navigation layer. See claude_codex_awareness_context_navigation_improvement.md
// for the full multi-phase plan; these tests pin the contract the later
// phases must keep stable.

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDecisionTrace_FailureModeMatchProducesTrace covers the most common
// case: a failure_mode is matched in the graph, so the trace identifies the
// finding, carries graph-sourced evidence, and links the matched required
// tests + forbidden_fixes as pivots.
func TestDecisionTrace_FailureModeMatchProducesTrace(t *testing.T) {
	r := &Report{
		FailureModes:   []string{"workflow.resume_poisoning"},
		Invariants:     []string{"workflow_receipts_required"},
		ForbiddenFixes: []string{"resume_without_receipt"},
		RequiredTests:  []string{"TestResumeRequiresReceipt"},
		MatchedAliases: []string{"workflow.resume_poisoning"},
		Confidence:     ConfidenceMedium,
		GraphFreshness: &GraphFreshnessReport{Stale: false},
	}
	traces := buildDecisionTraces(r)
	if len(traces) == 0 {
		t.Fatal("expected at least one decision trace, got 0")
	}
	var fmTrace *DecisionTrace
	for i := range traces {
		if traces[i].FindingType == FindingFailureMode && traces[i].FindingID == "workflow.resume_poisoning" {
			fmTrace = &traces[i]
			break
		}
	}
	if fmTrace == nil {
		t.Fatalf("no failure_mode trace for workflow.resume_poisoning; got %+v", traces)
	}
	// MatchedBy must include graph + alias entries.
	var hasGraph, hasAlias bool
	for _, ev := range fmTrace.MatchedBy {
		switch ev.Source {
		case "graph":
			hasGraph = true
		case "alias":
			hasAlias = true
		}
	}
	if !hasGraph {
		t.Errorf("failure_mode trace missing graph evidence: %+v", fmTrace.MatchedBy)
	}
	if !hasAlias {
		t.Errorf("failure_mode trace missing alias evidence: %+v", fmTrace.MatchedBy)
	}
	// Pivots must include the matched required_test and forbidden_fix.
	pivotKinds := pivotKindSet(fmTrace.Pivots)
	for _, want := range []string{"required_test", "forbidden_fix", "source_invariant"} {
		if !pivotKinds[want] {
			t.Errorf("missing pivot kind %q in %+v", want, fmTrace.Pivots)
		}
	}
	// Every trace must carry at least one falsifier (generic counts).
	if len(fmTrace.Falsifiers) == 0 {
		t.Error("failure_mode trace missing falsifier")
	}
}

// TestDecisionTrace_InvariantMatchProducesTrace covers the invariant axis.
func TestDecisionTrace_InvariantMatchProducesTrace(t *testing.T) {
	r := &Report{
		Invariants:     []string{"workflow_receipts_required"},
		ForbiddenFixes: []string{"resume_without_receipt"},
		Confidence:     ConfidenceHigh,
		GraphFreshness: &GraphFreshnessReport{Stale: false},
	}
	traces := buildDecisionTraces(r)
	if len(traces) != 2 {
		t.Fatalf("expected 2 traces (1 invariant + 1 forbidden_fix), got %d: %+v", len(traces), traces)
	}
	var invTrace *DecisionTrace
	for i := range traces {
		if traces[i].FindingType == FindingInvariant {
			invTrace = &traces[i]
			break
		}
	}
	if invTrace == nil {
		t.Fatal("no invariant trace produced")
	}
	if invTrace.FindingID != "workflow_receipts_required" {
		t.Errorf("FindingID = %q", invTrace.FindingID)
	}
	if invTrace.MatchedBy[0].Source != "graph" {
		t.Errorf("expected graph evidence; got %q", invTrace.MatchedBy[0].Source)
	}
	pivotKinds := pivotKindSet(invTrace.Pivots)
	if !pivotKinds["forbidden_fix"] {
		t.Errorf("invariant trace missing forbidden_fix pivot: %+v", invTrace.Pivots)
	}
}

// TestDecisionTrace_RawYAMLFallbackIsLabeled is the load-bearing test for
// the Phase 1 honesty rule: a raw-yaml-only fallback match must NOT look
// like graph proof. Its FindingType is raw_knowledge, its evidence Source
// is raw_yaml, and its confidence cannot exceed "low".
func TestDecisionTrace_RawYAMLFallbackIsLabeled(t *testing.T) {
	r := &Report{
		RawKnowledgeMatches: []RawKnowledgeMatch{{
			Source:       "failure_modes.yaml",
			Kind:         "failure_mode",
			ID:           "etcd.leader_instability",
			Score:        3,
			MatchedTerms: []string{"leader", "etcd"},
		}},
		Confidence:     ConfidenceMedium, // would have been medium without fallback gating
		GraphFreshness: &GraphFreshnessReport{Stale: false},
	}
	traces := buildDecisionTraces(r)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d: %+v", len(traces), traces)
	}
	tr := traces[0]
	if tr.FindingType != FindingRawKnowledge {
		t.Errorf("FindingType = %q, want raw_knowledge", tr.FindingType)
	}
	if tr.MatchedBy[0].Source != "raw_yaml" {
		t.Errorf("evidence source = %q, want raw_yaml", tr.MatchedBy[0].Source)
	}
	if tr.Confidence != ConfidenceLow {
		t.Errorf("raw fallback confidence = %q, want low", tr.Confidence)
	}
	if len(tr.Warnings) == 0 {
		t.Error("raw fallback trace should warn it is not graph proof")
	}
}

// TestDecisionTrace_RuntimeMatchAttachesRuntimeEvidence covers the
// IncludeRuntime=true path: a failure_mode matched by both graph and the
// live runtime overlay should carry two MatchedBy entries.
func TestDecisionTrace_RuntimeMatchAttachesRuntimeEvidence(t *testing.T) {
	r := &Report{
		FailureModes: []string{"workflow.resume_poisoning"},
		Runtime: &RuntimeSection{
			Included:            true,
			MatchedFailureModes: []string{"workflow.resume_poisoning"},
		},
		LiveOverlay:    &LiveOverlayFreshness{Status: "fresh"},
		Confidence:     ConfidenceHigh,
		GraphFreshness: &GraphFreshnessReport{Stale: false},
	}
	traces := buildDecisionTraces(r)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	var hasGraph, hasRuntime bool
	var runtimeFreshness string
	for _, ev := range traces[0].MatchedBy {
		switch ev.Source {
		case "graph":
			hasGraph = true
		case "runtime":
			hasRuntime = true
			runtimeFreshness = ev.Freshness
		}
	}
	if !hasGraph || !hasRuntime {
		t.Errorf("expected graph + runtime evidence; got %+v", traces[0].MatchedBy)
	}
	if runtimeFreshness != "fresh" {
		t.Errorf("runtime evidence freshness = %q, want fresh", runtimeFreshness)
	}
}

// TestDecisionTrace_StaleGraphCapsEvidenceConfidence makes sure agents can't
// read a stale-graph match as high-confidence: per-evidence confidence is
// capped even when the Report-level Confidence enum is High.
func TestDecisionTrace_StaleGraphCapsEvidenceConfidence(t *testing.T) {
	r := &Report{
		FailureModes:   []string{"x"},
		Confidence:     ConfidenceHigh,
		GraphFreshness: &GraphFreshnessReport{Stale: true},
	}
	traces := buildDecisionTraces(r)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	ev := traces[0].MatchedBy[0]
	if ev.Confidence > 0.5 {
		t.Errorf("stale graph evidence confidence = %v, want <= 0.5", ev.Confidence)
	}
	if ev.Freshness != "stale" {
		t.Errorf("freshness label = %q, want stale", ev.Freshness)
	}
	// Falsifier should reflect the stale graph and recommend a rebuild.
	if !strings.Contains(traces[0].Falsifiers[0].Command, "awareness build") {
		t.Errorf("stale-graph falsifier should suggest rebuild; got %+v", traces[0].Falsifiers)
	}
}

// TestDecisionTrace_NoMatchReturnsEmptyNotNil pins the contract from the
// design doc: NO_MATCH must NOT fabricate a trace. The trust envelope is
// the authority on safety in that case; a synthetic trace would compete.
// Empty slice (length 0) rather than nil so JSON serialization shows
// "decision_traces": [] explicitly.
func TestDecisionTrace_NoMatchReturnsEmptyNotNil(t *testing.T) {
	r := &Report{Confidence: ConfidenceUnknown}
	traces := buildDecisionTraces(r)
	if traces == nil {
		t.Fatal("buildDecisionTraces returned nil, want empty slice")
	}
	if len(traces) != 0 {
		t.Errorf("expected 0 traces under NO_MATCH, got %d: %+v", len(traces), traces)
	}
	// JSON shape check — round-trip the wrapping Report and ensure the
	// field appears as an array, not null.
	r.DecisionTraces = traces
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"decision_traces":[]`) {
		t.Errorf("expected decision_traces:[] in JSON; got: %s", raw)
	}
}

// TestDecisionTrace_RawYAMLCoveredByGraphIsNotDuplicated guards against the
// double-emission bug: when the graph already matched a failure_mode AND the
// raw-yaml fallback also caught the same id, only one trace should appear
// (the graph one, FindingType=failure_mode), not two.
func TestDecisionTrace_RawYAMLCoveredByGraphIsNotDuplicated(t *testing.T) {
	r := &Report{
		FailureModes: []string{"etcd.leader_instability"},
		RawKnowledgeMatches: []RawKnowledgeMatch{{
			Source: "failure_modes.yaml",
			Kind:   "failure_mode",
			ID:     "etcd.leader_instability",
		}},
		Confidence:     ConfidenceMedium,
		GraphFreshness: &GraphFreshnessReport{Stale: false},
	}
	traces := buildDecisionTraces(r)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace (graph wins, fallback suppressed), got %d: %+v", len(traces), traces)
	}
	if traces[0].FindingType != FindingFailureMode {
		t.Errorf("FindingType = %q, want failure_mode (graph wins)", traces[0].FindingType)
	}
}

// TestDecisionTrace_DeterministicOrdering pins the ordering contract so the
// JSON output is stable across runs — important for both diff-based tests
// and for human-readable change logs.
func TestDecisionTrace_DeterministicOrdering(t *testing.T) {
	r := &Report{
		FailureModes:   []string{"b", "a"},
		Invariants:     []string{"i2", "i1"},
		ForbiddenFixes: []string{"f2", "f1"},
		Confidence:     ConfidenceMedium,
		GraphFreshness: &GraphFreshnessReport{Stale: false},
	}
	traces := buildDecisionTraces(r)
	// failure_modes first, then invariants, then forbidden_fixes; within
	// each group, sorted by id.
	wantOrder := []struct {
		ft FindingType
		id string
	}{
		{FindingFailureMode, "a"},
		{FindingFailureMode, "b"},
		{FindingInvariant, "i1"},
		{FindingInvariant, "i2"},
		{FindingForbiddenFix, "f1"},
		{FindingForbiddenFix, "f2"},
	}
	if len(traces) != len(wantOrder) {
		t.Fatalf("trace count = %d, want %d", len(traces), len(wantOrder))
	}
	for i, want := range wantOrder {
		if traces[i].FindingType != want.ft || traces[i].FindingID != want.id {
			t.Errorf("traces[%d] = {%s, %s}, want {%s, %s}",
				i, traces[i].FindingType, traces[i].FindingID, want.ft, want.id)
		}
	}
}

// pivotKindSet collects unique Kind values for assertion convenience.
func pivotKindSet(pivots []ContextPivot) map[string]bool {
	out := map[string]bool{}
	for _, p := range pivots {
		out[p.Kind] = true
	}
	return out
}
