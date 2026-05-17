package main

import (
	"context"

	"github.com/globulario/services/golang/awareness/failuregraph"
)

// registerAwarenessFailureTools registers Failure Knowledge Graph MCP tools.
// These give awareness typed failure memory: when an agent encounters an error,
// Awareness can say what failure class it belongs to, how it was fixed, what
// wrong fixes to avoid, and what proof closes it.
func registerAwarenessFailureTools(s *server, st *awarenessState) {
	// ── awareness.failure.match_error ────────────────────────────────────────

	s.register(toolDef{
		Name: "awareness.failure.match_error",
		Description: "Match a raw error string against the Failure Knowledge Graph. " +
			"Returns the matched failure category, likely causes, known resolutions, " +
			"wrong fixes to avoid, and required regression tests. " +
			"Call this when you encounter an error before starting to diagnose.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"raw_error":    {Type: "string", Description: "The raw error message from logs or stderr"},
				"session_id":   {Type: "string", Description: "Current session or run ID"},
				"incident_id":  {Type: "string", Description: "Related incident ID if known"},
				"component":    {Type: "string", Description: "Go package or binary name, e.g. cluster-controller"},
				"service_name": {Type: "string", Description: "Service name, e.g. workflow-service"},
				"file_path":    {Type: "string", Description: "Source file being edited or implicated"},
				"semantic_atoms": {
					Type:        "array",
					Description: "Semantic diff atoms if available",
					Items:       &propSchema{Type: "string"},
				},
			},
			Required: []string{"raw_error"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded", "message": "awareness graph unavailable"}, nil
		}

		store := failuregraph.New(st.g)
		req := failuregraph.MatchErrorRequest{
			SessionID:   strArg(args, "session_id"),
			IncidentID:  strArg(args, "incident_id"),
			RawError:    strArg(args, "raw_error"),
			Component:   strArg(args, "component"),
			ServiceName: strArg(args, "service_name"),
			FilePath:    strArg(args, "file_path"),
		}
		if atoms, ok := args["semantic_atoms"].([]interface{}); ok {
			for _, a := range atoms {
				if str, ok := a.(string); ok {
					req.SemanticAtoms = append(req.SemanticAtoms, str)
				}
			}
		}

		exp, err := failuregraph.MatchError(ctx, store, req)
		if err != nil {
			return map[string]interface{}{"matched": false, "error": err.Error()}, nil
		}
		if exp == nil {
			return map[string]interface{}{
				"matched": false,
				"message": "No confident match found in Failure Knowledge Graph.",
			}, nil
		}

		causes := make([]string, len(exp.LikelyCauses))
		for i, c := range exp.LikelyCauses {
			causes[i] = c.Summary
		}
		resolutions := make([]string, len(exp.Resolutions))
		for i, r := range exp.Resolutions {
			resolutions[i] = r.Summary
		}
		wrongFixes := make([]string, len(exp.WrongFixes))
		for i, w := range exp.WrongFixes {
			wrongFixes[i] = w.Summary
		}
		tests := make([]string, len(exp.RequiredTests))
		for i, t := range exp.RequiredTests {
			tests[i] = t.Summary
		}

		return map[string]interface{}{
			"matched":    true,
			"confidence": exp.Confidence,
			"score":      exp.Score,
			"category": map[string]interface{}{
				"id":       exp.Category.ID,
				"name":     exp.Category.Name,
				"summary":  exp.Category.Summary,
				"severity": exp.Category.Severity,
			},
			"likely_causes":      causes,
			"resolutions":        resolutions,
			"wrong_fixes":        wrongFixes,
			"required_tests":     tests,
			"recommended_action": exp.RecommendedAction,
		}, nil
	})

	// ── awareness.failure.explain_category ──────────────────────────────────

	s.register(toolDef{
		Name: "awareness.failure.explain_category",
		Description: "Explain a failure category by ID or name. " +
			"Returns causes, resolutions, wrong fixes to avoid, regression tests, and invariants. " +
			"Use this to get the full picture before starting a fix.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"category_id": {Type: "string", Description: "Category ID (e.g. ERRCAT-installed_state_build_id_missing) or short name (e.g. installed_state_build_id_missing)"},
			},
			Required: []string{"category_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}

		store := failuregraph.New(st.g)
		catID := strArg(args, "category_id")
		if len(catID) > 0 && catID[:7] != "ERRCAT-" {
			catID = "ERRCAT-" + catID
		}

		exp, err := failuregraph.ExplainCategory(ctx, store, catID)
		if err != nil {
			return map[string]interface{}{"error": err.Error(), "category_id": catID}, nil
		}

		causes := nodeSummaries(exp.LikelyCauses)
		resolutions := nodeSummaries(exp.Resolutions)
		wrongFixes := nodeSummaries(exp.WrongFixes)
		tests := nodeSummaries(exp.RequiredTests)
		invariants := nodeSummaries(exp.RelatedInvariants)

		return map[string]interface{}{
			"category": map[string]interface{}{
				"id":       exp.Category.ID,
				"name":     exp.Category.Name,
				"summary":  exp.Category.Summary,
				"severity": exp.Category.Severity,
			},
			"common_causes":   causes,
			"resolutions":     resolutions,
			"wrong_fixes":     wrongFixes,
			"required_tests":  tests,
			"invariants":      invariants,
			"recommended_action": exp.RecommendedAction,
		}, nil
	})

	// ── awareness.failure.record_resolution ──────────────────────────────────

	s.register(toolDef{
		Name: "awareness.failure.record_resolution",
		Description: "Record a resolution recipe for a failure category. " +
			"Includes steps, forbidden steps, and verification criteria.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"category_id": {Type: "string", Description: "Failure category ID or name"},
				"title":       {Type: "string", Description: "Short resolution title"},
				"steps": {
					Type:        "array",
					Description: "Ordered resolution steps",
					Items:       &propSchema{Type: "string"},
				},
				"forbidden_steps": {
					Type:        "array",
					Description: "Steps that must NOT be taken",
					Items:       &propSchema{Type: "string"},
				},
				"verification": {
					Type:        "array",
					Description: "Verification criteria proving the fix is complete",
					Items:       &propSchema{Type: "string"},
				},
			},
			Required: []string{"category_id", "title"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}

		store := failuregraph.New(st.g)
		catID := strArg(args, "category_id")
		if len(catID) > 0 && catID[:7] != "ERRCAT-" {
			catID = "ERRCAT-" + catID
		}

		recipe := failuregraph.ResolutionRecipe{
			ResolutionID: catID,
			Title:        strArg(args, "title"),
			Steps:        strSliceArg(args, "steps"),
			ForbiddenSteps: strSliceArg(args, "forbidden_steps"),
			Verification: strSliceArg(args, "verification"),
		}

		saved, err := store.RecordResolutionRecipe(ctx, recipe)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"status": "recorded",
			"id":     saved.ID,
			"title":  saved.Title,
		}, nil
	})

	// ── awareness.failure.learn_from_incident ────────────────────────────────

	s.register(toolDef{
		Name: "awareness.failure.learn_from_incident",
		Description: "Extract failure knowledge from an incident and store it in the graph. " +
			"Creates category nodes plus cause, resolution, wrong-fix, and test edges. " +
			"Call this when closing an incident to make its knowledge reusable.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id":  {Type: "string", Description: "Incident ID, e.g. INC-2026-0007"},
				"category":     {Type: "string", Description: "Failure category name (will be created if it does not exist)"},
				"symptoms": {
					Type:        "array",
					Description: "Observed symptoms",
					Items:       &propSchema{Type: "string"},
				},
				"causes": {
					Type:        "array",
					Description: "Root causes",
					Items:       &propSchema{Type: "string"},
				},
				"resolutions": {
					Type:        "array",
					Description: "Resolutions applied",
					Items:       &propSchema{Type: "string"},
				},
				"wrong_fixes": {
					Type:        "array",
					Description: "Wrong fixes that were tried or should be avoided",
					Items:       &propSchema{Type: "string"},
				},
				"tests": {
					Type:        "array",
					Description: "Regression tests that prove closure",
					Items:       &propSchema{Type: "string"},
				},
			},
			Required: []string{"incident_id", "category"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}

		store := failuregraph.New(st.g)
		nodes, edges, err := failuregraph.LearnFromIncident(ctx, store,
			strArg(args, "incident_id"),
			strArg(args, "category"),
			strSliceArg(args, "symptoms"),
			strSliceArg(args, "causes"),
			strSliceArg(args, "resolutions"),
			strSliceArg(args, "wrong_fixes"),
			strSliceArg(args, "tests"),
		)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"status":        "learned",
			"created_nodes": nodes,
			"created_edges": edges,
			"categories":    []string{strArg(args, "category")},
		}, nil
	})

	// ── awareness.failure.find_similar ───────────────────────────────────────

	s.register(toolDef{
		Name: "awareness.failure.find_similar",
		Description: "Find failure categories similar to a given raw error. " +
			"Returns up to 5 ranked matches with explanations. " +
			"Use this when the exact category is unknown but you have an error string.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"raw_error": {Type: "string", Description: "Raw error string"},
				"component": {Type: "string", Description: "Component or package name"},
				"semantic_atoms": {
					Type:        "array",
					Description: "Semantic diff atoms if available",
					Items:       &propSchema{Type: "string"},
				},
				"limit": {Type: "integer", Description: "Maximum matches to return (default 5)"},
			},
			Required: []string{"raw_error"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]interface{}{"status": "degraded"}, nil
		}

		store := failuregraph.New(st.g)
		limit := 5
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		req := failuregraph.SimilarFailureRequest{
			RawError:      strArg(args, "raw_error"),
			Component:     strArg(args, "component"),
			SemanticAtoms: strSliceArg(args, "semantic_atoms"),
			Limit:         limit,
		}

		results, err := failuregraph.FindSimilar(ctx, store, req)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}

		matches := make([]map[string]interface{}, len(results))
		for i, exp := range results {
			matches[i] = map[string]interface{}{
				"category":           exp.Category.Name,
				"confidence":         exp.Confidence,
				"score":              exp.Score,
				"recommended_action": exp.RecommendedAction,
			}
		}
		return map[string]interface{}{"matches": matches}, nil
	})
}

// nodeSummaries extracts the Summary field from a slice of FailureNodes.
func nodeSummaries(nodes []failuregraph.FailureNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Summary != "" {
			out = append(out, n.Summary)
		}
	}
	return out
}

