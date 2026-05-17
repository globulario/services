package main

import (
	"testing"
)

func TestAllToolsHaveNonEmptyDescription(t *testing.T) {
	s, _ := newAwarenessTestServer(t)
	for name, rt := range s.tools {
		if rt.def.Name == "" {
			t.Errorf("tool %q: Name is empty", name)
		}
		if rt.def.Description == "" {
			t.Errorf("tool %q: Description is empty", name)
		}
		if rt.def.InputSchema.Type != "object" {
			t.Errorf("tool %q: InputSchema.Type must be 'object', got %q", name, rt.def.InputSchema.Type)
		}
	}
}

func TestRequiredToolsHaveRequiredFields(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	mustHaveRequired := map[string][]string{
		"awareness.preflight":             {"task"},
		"awareness.agent_context":         {"task"},
		"awareness.impact_file":           {"file"},
		"awareness.validate_package":      {"path"},
		"awareness.package_context":       {"path"},
		"awareness.propose_from_incident": {"incident_id"},
		"awareness.validate_proposal":     {"file"},
		"awareness.approve_proposal":      {"file"},
	}

	for toolName, wantRequired := range mustHaveRequired {
		rt, ok := s.tools[toolName]
		if !ok {
			t.Errorf("tool %q not found", toolName)
			continue
		}
		reqSet := make(map[string]bool, len(rt.def.InputSchema.Required))
		for _, r := range rt.def.InputSchema.Required {
			reqSet[r] = true
		}
		for _, field := range wantRequired {
			if !reqSet[field] {
				t.Errorf("tool %q: field %q should be required", toolName, field)
			}
		}
	}
}
