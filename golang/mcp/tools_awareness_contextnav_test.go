package main

// tools_awareness_contextnav_test.go — Phase 10 acceptance tests for
// the new awareness.decision_trace and awareness.finding_context tools.
// Pins:
//   - both tools register and the schema rejects missing required fields;
//   - finding_context returns a structured decision trace for a valid
//     prefixed finding id and degrades safely without a graph;
//   - decision_trace returns a degraded payload when the graph is missing.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// helper: build a server with the contextnav tools registered and an
// optional graph.
func newContextNavTestServer(t *testing.T, withGraph bool) *server {
	t.Helper()
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true
	s := newServer(cfg)

	st := &awarenessState{docsDir: t.TempDir(), nodeID: "test-node"}
	if withGraph {
		st.g = setupAwarenessTestGraph(t)
		t.Cleanup(func() { st.g.Close() })
	}
	registerAwarenessContextNavTools(s, st)
	return s
}

// TestAwarenessFindingContext_ReturnsStructuredTrace pins the happy
// path: a valid prefixed finding id returns a result with the trace
// fields populated.
func TestAwarenessFindingContext_ReturnsStructuredTrace(t *testing.T) {
	s := newContextNavTestServer(t, true)
	result, err := s.callTool(context.Background(), "awareness.finding_context", map[string]interface{}{
		"finding": "failure_mode:workflow.resume_poisoning",
		"task":    "workflow retry loop",
	})
	if err != nil {
		t.Fatalf("finding_context: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	b, _ := json.Marshal(result)
	raw := string(b)
	for _, want := range []string{
		`"finding":"failure_mode:workflow.resume_poisoning"`,
		`"decision_trace"`,
		`"finding_type":"failure_mode"`,
		`"finding_id":"workflow.resume_poisoning"`,
		// Template-matched falsifier should fire. The substring appears
		// inside the falsifier Command/HowToCheck — match the bare phrase
		// (JSON-escapes the surrounding quotes so we can't anchor with
		// a literal `"workflow retry loop"`).
		`workflow retry loop`,
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("missing substring %q in result: %s", want, raw)
		}
	}
}

// TestAwarenessFindingContext_RejectsMalformedID pins schema validation:
// a finding id without the kind prefix returns an error.
func TestAwarenessFindingContext_RejectsMalformedID(t *testing.T) {
	s := newContextNavTestServer(t, true)
	_, err := s.callTool(context.Background(), "awareness.finding_context", map[string]interface{}{
		"finding": "no_colon_at_all",
	})
	if err == nil {
		t.Error("expected error for malformed finding id, got nil")
	}
}

// TestAwarenessFindingContext_DegradesWithoutGraph pins the graceful
// fallback: when the graph is missing, the tool still returns a trace
// (just without graph-walked pivots / owner). The template falsifier
// still fires because it doesn't need a graph.
func TestAwarenessFindingContext_DegradesWithoutGraph(t *testing.T) {
	s := newContextNavTestServer(t, false)
	result, err := s.callTool(context.Background(), "awareness.finding_context", map[string]interface{}{
		"finding": "invariant:pki.ca_not_published",
	})
	if err != nil {
		t.Fatalf("finding_context (no graph): %v", err)
	}
	b, _ := json.Marshal(result)
	raw := string(b)
	if !strings.Contains(raw, `"finding":"invariant:pki.ca_not_published"`) {
		t.Errorf("missing finding key in result: %s", raw)
	}
	if !strings.Contains(raw, `"finding_type":"invariant"`) {
		t.Errorf("missing finding_type in result: %s", raw)
	}
	// Template falsifier for the PKI family should still fire.
	if !strings.Contains(raw, "SAN") && !strings.Contains(raw, "CA") {
		t.Errorf("expected PKI-family falsifier text; got: %s", raw)
	}
}

// TestAwarenessDecisionTrace_DegradedWithoutGraph pins the no-graph
// path: returns a structured "graph not available" payload rather than
// panicking. The error/hint must be surfaced so the agent knows to
// rebuild.
func TestAwarenessDecisionTrace_DegradedWithoutGraph(t *testing.T) {
	s := newContextNavTestServer(t, false)
	result, err := s.callTool(context.Background(), "awareness.decision_trace", map[string]interface{}{
		"task": "workflow retry loop",
	})
	if err != nil {
		t.Fatalf("decision_trace (no graph): %v", err)
	}
	b, _ := json.Marshal(result)
	raw := string(b)
	if !strings.Contains(raw, "awareness graph not available") {
		t.Errorf("expected degraded-mode error in result: %s", raw)
	}
}

// TestAwarenessDecisionTrace_MissingTaskRejected pins the schema
// contract: the task field is required.
func TestAwarenessDecisionTrace_MissingTaskRejected(t *testing.T) {
	s := newContextNavTestServer(t, false)
	_, err := s.callTool(context.Background(), "awareness.decision_trace", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing task, got nil")
	}
}
