package main

// tools_awareness_node_context.go — node-context navigation tools plus trust
// envelope helpers and service review stub. Stubs are used when the context
// package was removed from the standalone awareness module.

import (
	"context"
	"strings"

	"github.com/globulario/services/golang/awareness/assurance"
)

// registerAwarenessNodeContextTools registers stubs for the three node-centric navigation tools.
func registerAwarenessNodeContextTools(s *server, _ *awarenessState) {
	registerAwarenessNodeContext(s)
	registerAwarenessNeighborhood(s)
	registerAwarenessExplainNode(s)
}

func registerAwarenessNodeContext(s *server) {
	s.register(toolDef{
		Name:        "awareness.node_context",
		Description: "Show full architectural context for a graph node [not available — context package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {Type: "string", Description: "Node ID, service name, symbol name, file path, invariant ID, or failure mode ID"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "context package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessNeighborhood(s *server) {
	s.register(toolDef{
		Name:        "awareness.neighborhood",
		Description: "Show the BFS neighborhood of a graph node [not available — context package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node":  {Type: "string", Description: "Node ID or name (required)"},
				"depth": {Type: "integer", Description: "BFS depth (max 4)", Default: 1},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "context package was removed from standalone awareness module",
		}, nil
	})
}

func registerAwarenessExplainNode(s *server) {
	s.register(toolDef{
		Name:        "awareness.explain_node",
		Description: "Explain a graph node's role, risks, and edit warnings [not available — context package removed]",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node": {Type: "string", Description: "Node ID or name (required)"},
			},
			Required: []string{"node"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "not_available",
			"reason": "context package was removed from standalone awareness module",
		}, nil
	})
}

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

// ---- trust envelope helpers ----

func awarenessTrustMap(st *awarenessState, matchFound bool) map[string]interface{} {
	in := assurance.ComposeInputs{MatchFound: matchFound}
	if st != nil && st.g != nil {
		if s, err := assurance.CheckStaleness(context.Background(), st.g, assurance.Options{DocsDir: st.docsDir}); err == nil {
			in.Staleness = s
		}
	}
	env := assurance.Compose(in)
	return trustEnvelopeToMap(env)
}

func trustEnvelopeToMap(env assurance.TrustEnvelope) map[string]interface{} {
	return map[string]interface{}{
		"verdict":         string(env.Verdict),
		"confidence":      string(env.Confidence),
		"freshness":       string(env.Freshness),
		"coverage":        string(env.Coverage),
		"limitations":     append([]string(nil), env.Limitations...),
		"required_action": append([]string(nil), env.RequiredActions...),
	}
}

func trustFromConfidenceCoverage(st *awarenessState, confidence, graphCoverage string, matchFound bool, blindSpots []string) map[string]interface{} {
	env := assurance.Compose(assurance.ComposeInputs{MatchFound: matchFound})
	switch strings.ToLower(graphCoverage) {
	case "checked_with_matches":
		env.Coverage = assurance.TrustCoveragePartial
	case "checked_clean", "not_checked":
		env.Coverage = assurance.TrustCoverageNone
	}
	switch strings.ToLower(confidence) {
	case "high":
		env.Confidence = assurance.ConfidenceHigh
	case "medium":
		env.Confidence = assurance.ConfidenceMedium
	case "low":
		env.Confidence = assurance.ConfidenceLow
	default:
		env.Confidence = assurance.ConfidenceNone
	}
	if st != nil && st.g != nil {
		if s, err := assurance.CheckStaleness(context.Background(), st.g, assurance.Options{DocsDir: st.docsDir}); err == nil {
			env.Freshness = assurance.Compose(assurance.ComposeInputs{MatchFound: matchFound, Staleness: s}).Freshness
			if env.Freshness != assurance.FreshnessFresh && env.Verdict == assurance.TrustUsable {
				env.Verdict = assurance.TrustStale
			}
		}
	}
	if len(blindSpots) > 0 {
		env.Limitations = append(append([]string{}, env.Limitations...), blindSpots...)
	}
	return trustEnvelopeToMap(env)
}
