package main_test

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAllToolOutputsAreJSONSerializable(t *testing.T) {
	s, docsDir := makeTestServer(t)

	// Write a minimal incident bundle for propose_from_incident.
	writeIncidentFixture(t, docsDir)

	toolCalls := []struct {
		name string
		args map[string]interface{}
	}{
		{"awareness.preflight", map[string]interface{}{"task": "test task"}},
		{"awareness.agent_context", map[string]interface{}{"task": "test task"}},
		{"awareness.impact_file", map[string]interface{}{"file": "golang/foo.go"}},
		{"awareness.did_we_fix", map[string]interface{}{"task": "desired_hash fix"}},
		{"awareness.pattern_status", map[string]interface{}{"pattern": "desired"}},
		{"awareness.fix_status", map[string]interface{}{"pattern": "desired"}},
		{"awareness.runtime_snapshot", map[string]interface{}{}},
		{"awareness.validate_package", map[string]interface{}{"path": "/nonexistent"}},
	}

	for _, tc := range toolCalls {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := s.CallTool(context.Background(), tc.name, tc.args)
			// Result may be nil on error — that is OK. When non-nil it must marshal.
			if result != nil {
				if _, err := json.Marshal(result); err != nil {
					t.Errorf("tool %q returned non-serializable result: %v", tc.name, err)
				}
			}
		})
	}
}

func TestDidWeFixReturnsFIxLedgerResult(t *testing.T) {
	s, _ := makeTestServer(t)

	result, err := s.CallTool(context.Background(), "awareness.did_we_fix", map[string]interface{}{
		"task": "desired_hash drift",
	})
	if err != nil {
		t.Fatalf("did_we_fix: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	for _, key := range []string{"status", "fix_cases", "remaining_gaps", "next_action"} {
		if _, ok := m[key]; !ok {
			t.Errorf("did_we_fix missing key %q", key)
		}
	}
}
