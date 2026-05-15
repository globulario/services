package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPreflightReturnsCompactEnvelope(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch after deploy",
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b)
	}

	// Default response is a compact envelope, not the full canonical JSON.
	for _, key := range []string{"output_profile", "safety_status", "risk_tier", "confidence", "agent_context"} {
		if _, ok := m[key]; !ok {
			t.Errorf("compact envelope missing key %q", key)
		}
	}

	// output_profile must default to compact.
	if m["output_profile"] != "compact" {
		t.Errorf("expected output_profile=compact, got %v", m["output_profile"])
	}

	// Full canonical fields must NOT be present at top level in compact mode.
	for _, banned := range []string{"classification", "invariants", "failure_modes", "did_we_fix", "decision_traces", "experience_hints"} {
		if _, ok := m[banned]; ok {
			t.Errorf("compact envelope must not include %q at top level", banned)
		}
	}
}

func TestPreflightDefaultRuntimePolicyIsNever(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch after deploy",
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	if m["runtime_policy"] != "never" {
		t.Errorf("expected runtime_policy=never by default, got %v", m["runtime_policy"])
	}

	// Runtime section must not be present in default compact response.
	if _, ok := m["runtime"]; ok {
		t.Error("runtime section must not appear in compact response by default")
	}
}

func TestPreflightDegradesMissingGraph(t *testing.T) {
	s, _ := newAwarenessDegradedServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch",
	})
	if err != nil {
		t.Fatalf("expected degraded result, not error: %v", err)
	}

	b, _ := json.Marshal(result)
	// Degraded warning appears in agent_context or warnings.
	if !strings.Contains(string(b), "no graph DB") && !strings.Contains(string(b), "no graph") {
		t.Errorf("degraded preflight must include 'no graph DB' warning, got: %s", b)
	}
}

func TestPreflightWithRuntimeDetailIncludesRuntimeSection(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task":                   "desired_hash mismatch",
		"runtime_policy":         "auto",
		"include_runtime_detail": true,
		"runtime_window":         "5m",
		// Use a generous budget: this test checks key presence, not byte limits.
		// Live cluster runtime data can be large; the budget test is separate.
		"max_bytes": 500000,
	})
	if err != nil {
		t.Fatalf("preflight with runtime: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	// The "runtime" key must be present (may be null if collection failed).
	if _, ok := m["runtime"]; !ok {
		t.Error("expected 'runtime' key when include_runtime_detail=true")
	}
}

func TestPreflightClassifiesDesiredHashAsMismatch(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch between controller and node-agent",
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, _ := json.Marshal(result)
	bs := string(b)
	// In compact mode classification appears in agent_context as human-readable text.
	// Accept both the enum form (forensic/JSON) and the human-readable form (compact agent).
	if !strings.Contains(bs, "STATE_MISMATCH") && !strings.Contains(bs, "State mismatch detected") {
		t.Errorf("expected state-mismatch signal for desired_hash task, got: %s", bs)
	}
}

// TestPreflightFullJSONRequiresForensic verifies that format=json alone does not
// return the canonical full report — it must be paired with output_profile=forensic
// or full_json=true.
func TestPreflightFullJSONRequiresForensic(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task":   "desired_hash mismatch",
		"format": "json",
		// no output_profile=forensic and no full_json=true
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	// Without forensic profile, response must still be compact envelope.
	if _, ok := m["agent_context"]; !ok {
		t.Errorf("expected compact envelope with agent_context, got: %s", b)
	}
	// Canonical fields like "classification" must not be present at top level.
	if _, ok := m["classification"]; ok {
		t.Errorf("format=json without forensic profile must not expose canonical classification field: %s", b)
	}
}

// TestPreflightForensicFullJSON verifies that output_profile=forensic with full_json=true
// returns the canonical report fields.
func TestPreflightForensicFullJSON(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task":           "debug unknown cluster failure",
		"output_profile": "forensic",
		"full_json":      true,
	})
	if err != nil {
		t.Fatalf("preflight forensic: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	// Forensic + full_json must include canonical fields and metadata.
	if m["output_profile"] != "forensic" {
		t.Errorf("expected output_profile=forensic, got %v", m["output_profile"])
	}
	if m["full_json"] != true {
		t.Errorf("expected full_json=true in forensic response")
	}
	// Canonical fields from the JSON report must be present.
	for _, key := range []string{"task", "safety_status", "risk_tier", "confidence"} {
		if _, ok := m[key]; !ok {
			t.Errorf("forensic full_json response missing key %q", key)
		}
	}
}

// TestPreflightByteBudgetEnforcement verifies that responses exceeding max_bytes
// are truncated gracefully with valid JSON and truncated=true.
func TestPreflightByteBudgetEnforcement(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task":      "desired_hash mismatch",
		"max_bytes": 100, // extremely small to trigger truncation
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("truncated response must be valid JSON: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("truncated response must be valid JSON: %v\n%s", err, b)
	}

	if m["truncated"] != true {
		t.Errorf("expected truncated=true when max_bytes exceeded, got: %s", b)
	}
	if _, ok := m["truncation_reason"]; !ok {
		t.Errorf("expected truncation_reason field when truncated=true")
	}
	// Essential safety fields must survive truncation.
	for _, key := range []string{"safety_status", "risk_tier"} {
		if _, ok := m[key]; !ok {
			t.Errorf("truncated response missing essential field %q", key)
		}
	}
}

// TestPreflightNextContextHandles verifies that the compact envelope includes
// follow-up handles for agents to request deeper context progressively.
func TestPreflightNextContextHandles(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task":  "desired_hash mismatch",
		"files": []interface{}{"golang/repository/reconcile.go"},
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	handles, ok := m["next_context_handles"]
	if !ok {
		t.Fatalf("compact envelope must include next_context_handles: %s", b)
	}

	arr, ok := handles.([]interface{})
	if !ok || len(arr) == 0 {
		t.Errorf("next_context_handles must be a non-empty array: %v", handles)
	}

	// Each handle must have tool and args fields.
	for i, h := range arr {
		hm, ok := h.(map[string]interface{})
		if !ok {
			t.Errorf("handle[%d] must be an object", i)
			continue
		}
		if _, ok := hm["tool"]; !ok {
			t.Errorf("handle[%d] missing 'tool' field", i)
		}
		if _, ok := hm["args"]; !ok {
			t.Errorf("handle[%d] missing 'args' field", i)
		}
	}
}
