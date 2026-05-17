package graph

import (
	"context"
	"fmt"
	"strings"
)

func (g *Graph) PromoteExperienceLesson(ctx context.Context, expID string, target string, summary string) (*ExperiencePromotionResult, error) {
	readiness, err := g.EvaluatePromotionReadiness(ctx, expID)
	if err != nil {
		return nil, err
	}
	if !readiness.Ready {
		return nil, fmt.Errorf("promotion blocked: %s", strings.Join(readiness.Reasons, ", "))
	}
	target = strings.TrimSpace(strings.ToLower(target))
	if target != "forbidden_fix" && target != "invariant" {
		return nil, fmt.Errorf("target must be forbidden_fix or invariant")
	}
	expNodeID := "experience:" + expID
	expNode, err := g.FindNode(ctx, expNodeID)
	if err != nil {
		return nil, err
	}
	if expNode == nil {
		return nil, fmt.Errorf("experience not found: %s", expID)
	}
	candidateKind := NodeTypeAntiPattern
	edgeKind := EdgeProducedForbiddenFixCandidate
	prefix := "candidate_forbidden_fix:"
	if target == "invariant" {
		candidateKind = NodeTypeLesson
		edgeKind = EdgeProducedInvariantCandidate
		prefix = "candidate_invariant:"
	}
	candidateID := prefix + expID
	if strings.TrimSpace(summary) == "" {
		summary = expNode.Summary
	}
	if err := g.AddNode(ctx, Node{
		ID:      candidateID,
		Type:    candidateKind,
		Name:    expID,
		Summary: summary,
		Metadata: map[string]any{
			"status":          "candidate",
			"requires_review": true,
		},
	}); err != nil {
		return nil, err
	}
	if err := g.AddEdge(ctx, Edge{Src: expNodeID, Kind: edgeKind, Dst: candidateID}); err != nil {
		return nil, err
	}
	return &ExperiencePromotionResult{
		ExperienceID: expID,
		Target:       target,
		CandidateID:  candidateID,
		Status:       "candidate_recorded",
	}, nil
}

func (g *Graph) EvaluatePromotionReadiness(ctx context.Context, expID string) (*ExperiencePromotionReadiness, error) {
	rec, err := g.GetExperience(ctx, expID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("experience not found: %s", expID)
	}
	out := &ExperiencePromotionReadiness{
		ExperienceID:     expID,
		ObservationCount: len(rec.Observations),
	}
	for _, a := range rec.Attempts {
		switch strings.ToLower(strings.TrimSpace(a.Status)) {
		case "success":
			out.WorkedAttempts++
		case "failed":
			out.FailedAttempts++
		}
	}
	out.EvidenceStrength = deriveEvidenceStrength(rec.Observations)
	if rec.Scorecard != nil {
		out.Verdict = rec.Scorecard.Verdict
	}
	if out.Verdict == "" {
		out.Verdict = deriveVerdict(rec.Experience.Status, len(rec.Attempts), len(rec.Observations), rec.Experience.Lesson)
	}
	edges, _ := g.OutgoingEdges(ctx, "experience:"+expID)
	for _, e := range edges {
		if e.Kind == EdgeContradictedBy && strings.HasPrefix(e.Dst, "experience:") {
			out.ContradictedBy = append(out.ContradictedBy, strings.TrimPrefix(e.Dst, "experience:"))
		}
		if e.Kind == EdgeSupersedes && strings.HasPrefix(e.Dst, "experience:") {
			out.SupersededBy = append(out.SupersededBy, strings.TrimPrefix(e.Dst, "experience:"))
		}
	}
	reasons := []string{}
	if strings.TrimSpace(rec.Experience.Lesson) == "" {
		reasons = append(reasons, "missing_lesson")
	}
	if out.WorkedAttempts == 0 {
		reasons = append(reasons, "no_successful_attempt")
	}
	if out.ObservationCount == 0 {
		reasons = append(reasons, "no_observations")
	}
	if out.EvidenceStrength < 0.65 {
		reasons = append(reasons, "insufficient_evidence_strength")
	}
	if len(out.ContradictedBy) > 0 {
		reasons = append(reasons, "contradicted_by_newer_experience")
	}
	if strings.EqualFold(out.Verdict, "unproven") || strings.EqualFold(out.Verdict, "weak") {
		reasons = append(reasons, "verdict_not_strong_enough")
	}
	out.Reasons = uniqueStrings(reasons)
	out.Ready = len(out.Reasons) == 0
	return out, nil
}

// SeedWorkflowDeferExperience seeds a canonical workflow-defer experience. Idempotent.
func (g *Graph) SeedWorkflowDeferExperience(ctx context.Context) (*ExperienceEntry, error) {
	if existing, err := g.GetExperience(ctx, SeedWorkflowDeferExperienceID); err == nil && existing != nil {
		return &existing.Experience, nil
	}
	e, err := g.CreateExperience(ctx, ExperienceEntry{
		ID:             SeedWorkflowDeferExperienceID,
		Kind:           "debugging_experience",
		Domain:         "workflow",
		Capability:     "workflow.defer",
		Status:         "success",
		Summary:        "Preserved deferred workflow status across nested compileSubSteps and foreach aggregation.",
		GoalOriginal:   "Make failed runtime verification defer instead of retry hammering.",
		GoalNormalized: "workflow defer prevent retry hammer",
		GoalVerb:       "prevent",
		GoalObject:     "retry_hammer",
		StrategyID:     "trace_typed_error_across_boundaries",
		Lesson:         "For wrong workflow terminal status, trace typed semantic errors across compiler, executor, aggregation, and persistence boundaries.",
		NextTimeHint:   "Inspect typed error propagation before changing retry timing.",
		CreatedBy:      "seed",
	})
	if err != nil {
		return nil, err
	}
	_, _ = g.AddExperienceAttempt(ctx, ExperienceAttempt{
		ID:           "attempt.exp.workflow.defer.b2_smoke_success.001",
		ExperienceID: e.ID,
		StrategyID:   "trace_typed_error_across_boundaries",
		Action:       "inspect compileSubSteps defer propagation",
		Rationale:    "inner runtime status disagreed with final workflow status",
		Outcome:      "found defer policy dropped",
		Status:       "success",
	})
	_, _ = g.AddExperienceAttempt(ctx, ExperienceAttempt{
		ID:           "attempt.exp.workflow.defer.b2_smoke_success.002",
		ExperienceID: e.ID,
		StrategyID:   "trace_typed_error_across_boundaries",
		Action:       "inspect foreach aggregator error wrapping",
		Rationale:    "typed semantic errors can be dropped at aggregation boundary",
		Outcome:      "found typed StepDeferredError discarded",
		Status:       "success",
	})
	_, _ = g.AddExperienceObservation(ctx, ExperienceObservation{
		ID:           "obs.exp.workflow.defer.b2_smoke_success.test",
		ExperienceID: e.ID,
		Type:         "test",
		Summary:      "TestForeachWithDeferYieldsRunDeferred failed before fix and passed after fix",
		Source:       "TestForeachWithDeferYieldsRunDeferred",
		Confidence:   0.9,
	})
	_, _ = g.AddExperienceObservation(ctx, ExperienceObservation{
		ID:           "obs.exp.workflow.defer.b2_smoke_success.runtime",
		ExperienceID: e.ID,
		Type:         "runtime",
		Summary:      "RUN_STATUS_DEFERRED observed and skip-dispatch enforced during cooldown",
		Source:       "workflow runtime logs",
		Confidence:   0.85,
	})
	_ = g.CloseExperience(ctx, e.ID, "success", e.Lesson, e.NextTimeHint, &ExperienceScorecard{
		Success:          1.0,
		EvidenceStrength: 0.9,
		ReuseValue:       0.85,
		Specificity:      0.8,
		RiskReduction:    0.9,
		Confidence:       0.9,
		Verdict:          "strong",
	})
	return e, nil
}

func (g *Graph) LinkExperienceArtifacts(ctx context.Context, expID string, in ExperienceLinkInput) error {
	expNodeID := "experience:" + expID
	expNode, err := g.FindNode(ctx, expNodeID)
	if err != nil {
		return err
	}
	if expNode == nil {
		return fmt.Errorf("experience not found: %s", expID)
	}
	if in.ClosureEntryID != "" {
		closureID := ensurePrefixed(in.ClosureEntryID, "closure_entry:")
		_ = g.AddNode(ctx, Node{ID: closureID, Type: "closure_entry", Name: strings.TrimPrefix(closureID, "closure_entry:")})
		_ = g.AddEdge(ctx, Edge{Src: expNodeID, Kind: EdgeClosedBy, Dst: closureID})
	}
	for _, inv := range uniqueStrings(in.InvariantIDs) {
		if strings.TrimSpace(inv) == "" {
			continue
		}
		dst := ensurePrefixed(inv, "invariant:")
		_ = g.AddNode(ctx, Node{ID: dst, Type: NodeTypeInvariant, Name: strings.TrimPrefix(dst, "invariant:")})
		_ = g.AddEdge(ctx, Edge{Src: expNodeID, Kind: EdgeProtects, Dst: dst})
	}
	for _, ff := range uniqueStrings(in.ForbiddenFixIDs) {
		if strings.TrimSpace(ff) == "" {
			continue
		}
		dst := ensurePrefixed(ff, "forbidden_fix:")
		_ = g.AddNode(ctx, Node{ID: dst, Type: NodeTypeForbiddenFix, Name: strings.TrimPrefix(dst, "forbidden_fix:")})
		_ = g.AddEdge(ctx, Edge{Src: expNodeID, Kind: EdgeProducedForbiddenFixCandidate, Dst: dst})
	}
	for _, ff := range uniqueStrings(in.AvoidedForbiddenFixs) {
		if strings.TrimSpace(ff) == "" {
			continue
		}
		dst := ensurePrefixed(ff, "forbidden_fix:")
		_ = g.AddNode(ctx, Node{ID: dst, Type: NodeTypeForbiddenFix, Name: strings.TrimPrefix(dst, "forbidden_fix:")})
		_ = g.AddEdge(ctx, Edge{Src: expNodeID, Kind: EdgeAvoidedForbiddenFix, Dst: dst})
	}
	for _, f := range uniqueStrings(in.TouchedFiles) {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		dst := "source_file:" + f
		_ = g.AddNode(ctx, Node{ID: dst, Type: NodeTypeSourceFile, Name: f, Path: f})
		_ = g.AddEdge(ctx, Edge{Src: expNodeID, Kind: EdgeTouchesFile, Dst: dst})
	}
	for _, s := range uniqueStrings(in.ChangedSymbols) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		dst := "symbol:" + s
		_ = g.AddNode(ctx, Node{ID: dst, Type: NodeTypeSymbol, Name: s})
		_ = g.AddEdge(ctx, Edge{Src: expNodeID, Kind: EdgeChangedSymbol, Dst: dst})
	}
	return nil
}

func (g *Graph) LinkExperienceRelation(ctx context.Context, sourceExpID string, relation string, targetExpID string) error {
	src := "experience:" + sourceExpID
	dst := "experience:" + targetExpID
	srcNode, err := g.FindNode(ctx, src)
	if err != nil {
		return err
	}
	if srcNode == nil {
		return fmt.Errorf("experience not found: %s", sourceExpID)
	}
	dstNode, err := g.FindNode(ctx, dst)
	if err != nil {
		return err
	}
	if dstNode == nil {
		return fmt.Errorf("experience not found: %s", targetExpID)
	}
	var edgeKind string
	switch strings.ToLower(strings.TrimSpace(relation)) {
	case "contradicted_by", "contradicted":
		edgeKind = EdgeContradictedBy
	case "supersedes":
		edgeKind = EdgeSupersedes
	case "similar_to", "similar":
		edgeKind = EdgeSimilarTo
	default:
		return fmt.Errorf("unsupported relation: %s", relation)
	}
	return g.AddEdge(ctx, Edge{Src: src, Kind: edgeKind, Dst: dst})
}

func ensurePrefixed(v, prefix string) string {
	if strings.HasPrefix(v, prefix) {
		return v
	}
	return prefix + v
}
