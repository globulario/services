package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

func registerAwarenessExperienceTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.experience_start",
		Description: "Start an experience ledger entry for a goal.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"goal": {Type: "string"}, "domain": {Type: "string"}, "capability": {Type: "string"},
			"kind": {Type: "string"}, "summary": {Type: "string"}, "strategy": {Type: "string"},
		}, Required: []string{"goal"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded", "message": "awareness graph unavailable"}, nil
		}
		goal := strArg(args, "goal")
		if goal == "" {
			return nil, fmt.Errorf("goal is required")
		}
		e, err := st.g.CreateExperience(ctx, graph.ExperienceEntry{
			Kind:           firstNonEmpty(strArg(args, "kind"), "debugging_experience"),
			Domain:         strArg(args, "domain"),
			Capability:     strArg(args, "capability"),
			Status:         "unproven",
			Summary:        strArg(args, "summary"),
			GoalOriginal:   goal,
			GoalNormalized: expNormalizeGoal(goal),
			GoalVerb:       expGoalVerb(goal),
			GoalObject:     expGoalObject(goal),
			StrategyID:     strArg(args, "strategy"),
			CreatedBy:      "mcp",
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"experience_id": e.ID, "status": "started"}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_record_attempt",
		Description: "Record a strategy attempt in an experience.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id": {Type: "string"}, "strategy": {Type: "string"}, "action": {Type: "string"},
			"rationale": {Type: "string"}, "outcome": {Type: "string"}, "status": {Type: "string"},
		}, Required: []string{"experience_id", "action"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		a, err := st.g.AddExperienceAttempt(ctx, graph.ExperienceAttempt{
			ExperienceID: strArg(args, "experience_id"),
			StrategyID:   strArg(args, "strategy"),
			Action:       strArg(args, "action"),
			Rationale:    strArg(args, "rationale"),
			Outcome:      strArg(args, "outcome"),
			Status:       firstNonEmpty(strArg(args, "status"), "inconclusive"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"attempt_id": a.ID, "status": "recorded"}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_add_observation",
		Description: "Add an observation to an experience.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id": {Type: "string"}, "type": {Type: "string"}, "summary": {Type: "string"},
			"source": {Type: "string"}, "confidence": {Type: "number"},
		}, Required: []string{"experience_id", "summary"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		o, err := st.g.AddExperienceObservation(ctx, graph.ExperienceObservation{
			ExperienceID: strArg(args, "experience_id"),
			Type:         firstNonEmpty(strArg(args, "type"), "operator_note"),
			Summary:      strArg(args, "summary"),
			Source:       strArg(args, "source"),
			Confidence:   numArg(args, "confidence", 0.7),
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"observation_id": o.ID, "status": "recorded"}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_close",
		Description: "Close an experience and attach lesson/hint.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id": {Type: "string"}, "status": {Type: "string"}, "lesson": {Type: "string"},
			"next_time_hint": {Type: "string"},
		}, Required: []string{"experience_id", "status"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		err := st.g.CloseExperience(ctx, strArg(args, "experience_id"), strArg(args, "status"), strArg(args, "lesson"), strArg(args, "next_time_hint"), nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "closed"}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_search_similar",
		Description: "Search similar experiences by goal/domain/capability.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"goal": {Type: "string"}, "domain": {Type: "string"}, "capability": {Type: "string"}, "limit": {Type: "number"},
		}, Required: []string{"goal"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded", "matches": []any{}}, nil
		}
		hits, err := st.g.SearchSimilarExperiences(ctx, graph.ExperienceSearchQuery{
			Goal:       strArg(args, "goal"),
			Domain:     strArg(args, "domain"),
			Capability: strArg(args, "capability"),
			Limit:      int(numArg(args, "limit", 5)),
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"matches": hits}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_get",
		Description: "Get a single experience with attempts and observations.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{"experience_id": {Type: "string"}}, Required: []string{"experience_id"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		rec, err := st.g.GetExperience(ctx, strArg(args, "experience_id"))
		if err != nil {
			return nil, err
		}
		if rec == nil {
			return map[string]any{"status": "not_found"}, nil
		}
		return rec, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_promote_lesson",
		Description: "Record a review-gated candidate from an experience lesson (does not auto-promote).",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id": {Type: "string"},
			"to":            {Type: "string"},
			"summary":       {Type: "string"},
		}, Required: []string{"experience_id", "to"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		res, err := st.g.PromoteExperienceLesson(
			ctx,
			strArg(args, "experience_id"),
			strArg(args, "to"),
			strArg(args, "summary"),
		)
		if err != nil {
			return nil, err
		}
		return res, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_seed_workflow_defer",
		Description: "Seed the canonical workflow-defer experience used by preflight retrieval validation.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		e, err := st.g.SeedWorkflowDeferExperience(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "seeded", "experience_id": e.ID}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_link_artifacts",
		Description: "Link closure/invariants/forbidden-fixes/files/symbols to an experience.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id":           {Type: "string"},
			"closure_entry_id":        {Type: "string"},
			"invariant_ids":           {Type: "array", Items: &propSchema{Type: "string"}},
			"forbidden_fix_ids":       {Type: "array", Items: &propSchema{Type: "string"}},
			"avoided_forbidden_fixes": {Type: "array", Items: &propSchema{Type: "string"}},
			"touched_files":           {Type: "array", Items: &propSchema{Type: "string"}},
			"changed_symbols":         {Type: "array", Items: &propSchema{Type: "string"}},
		}, Required: []string{"experience_id"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		err := st.g.LinkExperienceArtifacts(ctx, strArg(args, "experience_id"), graph.ExperienceLinkInput{
			ClosureEntryID:       strArg(args, "closure_entry_id"),
			InvariantIDs:         strSliceArg(args, "invariant_ids"),
			ForbiddenFixIDs:      strSliceArg(args, "forbidden_fix_ids"),
			AvoidedForbiddenFixs: strSliceArg(args, "avoided_forbidden_fixes"),
			TouchedFiles:         strSliceArg(args, "touched_files"),
			ChangedSymbols:       strSliceArg(args, "changed_symbols"),
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "linked"}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_link_relation",
		Description: "Link one experience to another (contradicted_by|supersedes|similar_to).",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id": {Type: "string"},
			"relation":      {Type: "string"},
			"target_id":     {Type: "string"},
		}, Required: []string{"experience_id", "relation", "target_id"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		err := st.g.LinkExperienceRelation(ctx, strArg(args, "experience_id"), strArg(args, "relation"), strArg(args, "target_id"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "linked"}, nil
	})

	s.register(toolDef{
		Name:        "awareness.experience_promotion_check",
		Description: "Evaluate promotion readiness for an experience lesson.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]propSchema{
			"experience_id": {Type: "string"},
		}, Required: []string{"experience_id"}},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return map[string]any{"status": "degraded"}, nil
		}
		return st.g.EvaluatePromotionReadiness(ctx, strArg(args, "experience_id"))
	})
}

func numArg(args map[string]interface{}, key string, def float64) float64 {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return def
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func expNormalizeGoal(goal string) string {
	g := strings.ToLower(strings.TrimSpace(goal))
	r := strings.NewReplacer("_", " ", ".", " ", "-", " ", "/", " ", "  ", " ")
	return strings.Join(strings.Fields(r.Replace(g)), " ")
}

func expGoalVerb(goal string) string {
	parts := strings.Fields(expNormalizeGoal(goal))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func expGoalObject(goal string) string {
	parts := strings.Fields(expNormalizeGoal(goal))
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], "_")
}
