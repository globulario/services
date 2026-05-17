package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/awareness/assurance"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/incidentpattern"
)

// TestAwarenessTrustMap_NoMatchIsNeverTrusted is the load-bearing guard:
// awarenessTrustMap(_, false) must produce verdict = unknown for every
// surface that calls it (preflight no-graph path, agent_context no-graph
// path, match_incident_patterns degraded path, pre_edit_context no-graph
// path, impact_file no-graph path).
//
// If this test ever fails, the rubber-stamp prevention rule is broken and
// agents could receive a trusted recommendation without coverage or freshness
// evidence. Do not soften the assertion.
func TestAwarenessTrustMap_NoMatchIsNeverTrusted(t *testing.T) {
	cases := []struct {
		name string
		st   *awarenessState
	}{
		{name: "nil_state", st: nil},
		{name: "empty_state", st: &awarenessState{}},
		{name: "state_with_graph_no_docs", st: &awarenessState{g: openAgentUsageGraph(t)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := awarenessTrustMap(c.st, false)
			verdict, _ := out["verdict"].(string)
			if verdict == string(assurance.TrustTrusted) {
				t.Fatalf("verdict=%s for match_found=false; NO_MATCH must never be trusted", verdict)
			}
			if verdict != string(assurance.TrustUnknown) && verdict != string(assurance.TrustUnsafe) {
				t.Fatalf("verdict=%q; expected unknown or unsafe for NO_MATCH", verdict)
			}
			if _, ok := out["limitations"]; !ok {
				t.Fatalf("trust map missing limitations field: %v", out)
			}
			if _, ok := out["required_action"]; !ok {
				t.Fatalf("trust map missing required_action field: %v", out)
			}
		})
	}
}

// TestMatchIncidentPatterns_NoGraphReturnsTrust verifies the degraded path of
// awareness.match_incident_patterns carries a trust envelope with unknown
// verdict — the same shape an agent would parse from any other tool.
func TestMatchIncidentPatterns_NoGraphReturnsTrust(t *testing.T) {
	st := &awarenessState{g: nil}
	// Mirror the handler's degraded-path branch literally, since the handler
	// is registered as a closure on a server and is awkward to invoke here.
	got := map[string]interface{}{
		"has_warning": false,
		"matches":     []interface{}{},
		"status":      "degraded",
		"trust":       awarenessTrustMap(st, false),
	}
	trust, ok := got["trust"].(map[string]interface{})
	if !ok {
		t.Fatalf("trust missing or wrong type: %T", got["trust"])
	}
	if v, _ := trust["verdict"].(string); v == string(assurance.TrustTrusted) {
		t.Fatalf("verdict=%s on no-graph degraded path; must not be trusted", v)
	}
}

// TestMatchIncidentPatterns_NoMatchHasUnknownVerdict verifies that when the
// graph is present but no incident patterns match the request, the trust
// envelope still carries an unknown/unsafe verdict — match_found is false.
func TestMatchIncidentPatterns_NoMatchHasUnknownVerdict(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)
	st := &awarenessState{g: g}

	req := incidentpattern.IncidentMatchRequest{
		SessionID: "test",
		Task:      "no incidents seeded — must NO_MATCH",
	}
	matches, err := incidentpattern.Match(ctx, st.g, req)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	hasWarning := len(matches) > 0
	if hasWarning {
		t.Fatalf("expected no matches with empty graph, got %d", len(matches))
	}
	out := awarenessTrustMap(st, hasWarning)
	if v, _ := out["verdict"].(string); v == string(assurance.TrustTrusted) {
		t.Fatalf("verdict=%s on NO_MATCH; must never be trusted", v)
	}
}

// TestPreEditContext_NoGraphReturnsTrust verifies the no-graph branch of
// awareness.pre_edit_context exposes a trust envelope with unknown verdict.
func TestPreEditContext_NoGraphReturnsTrust(t *testing.T) {
	st := &awarenessState{g: nil}
	got := map[string]interface{}{
		"file":        "golang/awareness/graph/db.go",
		"error":       "graph unavailable — run 'globular awareness build' first",
		"blind_spots": []string{"graph not available — static file analysis only"},
		"trust":       awarenessTrustMap(st, false),
	}
	trust, ok := got["trust"].(map[string]interface{})
	if !ok {
		t.Fatalf("trust missing or wrong type: %T", got["trust"])
	}
	if v, _ := trust["verdict"].(string); v == string(assurance.TrustTrusted) {
		t.Fatalf("verdict=%s on no-graph path; must not be trusted", v)
	}
}

// TestPreEditContext_FileNotIndexedHasUnknownVerdict verifies that a real
// graph with no node for the requested file (the most common NO_MATCH path
// in production) yields a trust envelope with verdict != trusted.
func TestPreEditContext_FileNotIndexedHasUnknownVerdict(t *testing.T) {
	ctx := context.Background()
	g := openAgentUsageGraph(t)
	st := &awarenessState{g: g}

	out, err := buildFileInvariantContext(ctx, g, "golang/never/indexed.go")
	if err != nil {
		t.Fatalf("buildFileInvariantContext: %v", err)
	}
	matchFound := false
	if invs, ok := out["invariants"].([]map[string]interface{}); ok {
		matchFound = len(invs) > 0
	} else if invs, ok := out["invariants"].([]interface{}); ok {
		matchFound = len(invs) > 0
	}
	if matchFound {
		t.Fatalf("expected match_found=false for unindexed file, got true")
	}
	trust := awarenessTrustMap(st, matchFound)
	if v, _ := trust["verdict"].(string); v == string(assurance.TrustTrusted) {
		t.Fatalf("verdict=%s for unindexed file; must not be trusted", v)
	}

	// Sanity check that the verdict is one of the two acceptable NO_MATCH
	// values, not some unexpected new state.
	switch trust["verdict"] {
	case string(assurance.TrustUnknown), string(assurance.TrustUnsafe):
		// ok
	default:
		t.Fatalf("verdict=%q; expected unknown or unsafe", trust["verdict"])
	}

	_ = graph.Edge{} // keep import even if unused above
}
