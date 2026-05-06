package mcp_test

import (
	"testing"
)

func TestAllToolsHaveNonEmptyDescription(t *testing.T) {
	s, _ := makeTestServer(t)
	for _, name := range s.ToolNames() {
		def := s.ToolDef(name)
		if def == nil {
			t.Errorf("tool %q: ToolDef returned nil", name)
			continue
		}
		if def.Name == "" {
			t.Errorf("tool %q: Name is empty", name)
		}
		if def.Description == "" {
			t.Errorf("tool %q: Description is empty", name)
		}
		if def.InputSchema.Type != "object" {
			t.Errorf("tool %q: InputSchema.Type must be 'object', got %q", name, def.InputSchema.Type)
		}
		_ = def.InputSchema.Properties // ensure Properties is accessible
	}
}

func TestRequiredToolsHaveRequiredFields(t *testing.T) {
	s, _ := makeTestServer(t)

	mustHaveRequired := map[string][]string{
		"awareness.preflight":             {"task"},
		"awareness.agent_context":         {"task"},
		"awareness.impact_file":           {"file"},
		"awareness.did_we_fix":            {"task"},
		"awareness.pattern_status":        {"pattern"},
		"awareness.validate_package":      {"path"},
		"awareness.package_context":       {"path"},
		"awareness.propose_from_incident": {"incident_id"},
		"awareness.validate_proposal":     {"file"},
		"awareness.approve_proposal":      {"file"},
	}

	for toolName, wantRequired := range mustHaveRequired {
		def := s.ToolDef(toolName)
		if def == nil {
			t.Errorf("tool %q not found", toolName)
			continue
		}
		reqSet := make(map[string]bool, len(def.InputSchema.Required))
		for _, r := range def.InputSchema.Required {
			reqSet[r] = true
		}
		for _, field := range wantRequired {
			if !reqSet[field] {
				t.Errorf("tool %q: field %q should be required", toolName, field)
			}
		}
	}
}
