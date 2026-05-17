package main

// tools_awareness_failurelearning.go — stubs after failurelearning package was
// removed from standalone awareness module. The failure-learning MCP tools are
// not available in this build. The tools are registered as stubs that return
// a "not available" response so the MCP schema remains discoverable.

import (
	"context"
)

func registerAwarenessFailureLearningTools(s *server, st *awarenessState) {
	notAvailable := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "failurelearning package was removed from standalone awareness module",
		}, nil
	}

	stubTool := func(name, desc string) {
		s.register(toolDef{
			Name:        name,
			Description: desc + " (not available — failurelearning package removed)",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]propSchema{},
			},
		}, notAvailable)
	}

	stubTool("awareness.failure_learning.propose", "Propose a Failure Graph update")
	stubTool("awareness.failure_learning.propose_from_incident", "Propose a Failure Graph update from an incident")
	stubTool("awareness.failure_learning.propose_from_session", "Propose a Failure Graph update from a session")
	stubTool("awareness.failure_learning.list_pending", "List pending Failure Graph learning proposals")
	stubTool("awareness.failure_learning.show", "Show a Failure Graph learning proposal")
	stubTool("awareness.failure_learning.review", "Review a Failure Graph learning proposal")
	stubTool("awareness.failure_learning.apply", "Apply an approved Failure Graph learning proposal")
	stubTool("awareness.failure_learning.reject", "Reject a Failure Graph learning proposal")
	stubTool("awareness.failure_learning.check_closure", "Check closure has a Failure Graph learning proposal")
}
