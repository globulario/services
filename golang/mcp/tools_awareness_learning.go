package main

// tools_awareness_learning.go — stubs for the propose_from_incident,
// validate_proposal, and approve_proposal MCP tools.
//
// The learning/failurelearning API (LoadIncidentBundle, GenerateProposalFromBundle,
// SaveProposal, LoadProposalFromFile, ValidateProposal, ApproveProposal,
// ValidationFail, etc.) was removed from the standalone awareness module during
// Phase 3. These tools are registered as stubs so MCP clients receive a clear
// "not_available" response rather than a missing-tool error.

import (
	"context"
)

func registerAwarenessLearningTools(s *server, _ *awarenessState) {
	s.register(toolDef{
		Name: "awareness.propose_from_incident",
		Description: "Generate a DRAFT awareness proposal from an incident bundle " +
			"[not available — learning.LoadIncidentBundle / GenerateProposalFromBundle removed from standalone module]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id": {Type: "string", Description: "Incident ID"},
				"output_name": {Type: "string", Description: "Output filename"},
			},
			Required: []string{"incident_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "learning.LoadIncidentBundle / GenerateProposalFromBundle were removed from standalone awareness module",
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.validate_proposal",
		Description: "Validate a proposal YAML against the awareness admission rules " +
			"[not available — learning.ValidateProposal removed from standalone module]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file":   {Type: "string", Description: "Path to the proposal YAML file"},
				"strict": {Type: "boolean", Description: "Treat warnings as failures"},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "learning.ValidateProposal / LoadProposalFromFile were removed from standalone awareness module",
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.approve_proposal",
		Description: "Validate a proposal and mark it APPROVED " +
			"[not available — learning.ApproveProposal removed from standalone module]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {Type: "string", Description: "Path to the proposal YAML file"},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "learning.ApproveProposal / LoadProposalFromFile were removed from standalone awareness module",
		}, nil
	})
}
