package main

// tools_awareness_service_review.go — stub after analysis.ReviewService /
// ServiceDesignReview were removed from the standalone awareness module.
// The tool is kept in the registry as "not_available" so MCP clients get
// a clear error rather than a missing-tool error.

import (
	"context"
)

// registerReviewServiceTool registers awareness.review_service as a stub.
// It is called from registerSelfReviewTools (self_review_tool.go).
func registerReviewServiceTool(s *server, _ *awarenessState) {
	s.register(toolDef{
		Name: "awareness.review_service",
		Description: "Design-level review of a named Globular service in the awareness graph " +
			"[not available — analysis.ReviewService removed from standalone module]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service": {
					Type:        "string",
					Description: "Service ID, proto service name, or display name.",
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'text' (default) or 'json'.",
					Enum:        []string{"text", "json"},
				},
			},
			Required: []string{"service"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "analysis.ReviewService / ServiceDesignReview were removed from the standalone awareness module",
		}, nil
	})
}
