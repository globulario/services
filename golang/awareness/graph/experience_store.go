package graph

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ExperienceEntry struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind,omitempty"`
	Domain         string  `json:"domain,omitempty"`
	Capability     string  `json:"capability,omitempty"`
	Status         string  `json:"status,omitempty"`
	Summary        string  `json:"summary,omitempty"`
	GoalOriginal   string  `json:"goal_original,omitempty"`
	GoalNormalized string  `json:"goal_normalized,omitempty"`
	GoalVerb       string  `json:"goal_verb,omitempty"`
	GoalObject     string  `json:"goal_object,omitempty"`
	StrategyID     string  `json:"strategy_id,omitempty"`
	Lesson         string  `json:"lesson,omitempty"`
	NextTimeHint   string  `json:"next_time_hint,omitempty"`
	CreatedBy      string  `json:"created_by,omitempty"`
	ReviewedBy     string  `json:"reviewed_by,omitempty"`
	CreatedAt      int64   `json:"created_at,omitempty"`
	UpdatedAt      int64   `json:"updated_at,omitempty"`
}

type ExperienceAttempt struct {
	ID           string `json:"id"`
	ExperienceID string `json:"experience_id"`
	StrategyID   string `json:"strategy_id,omitempty"`
	Action       string `json:"action,omitempty"`
	Rationale    string `json:"rationale,omitempty"`
	Outcome      string `json:"outcome,omitempty"`
	Status       string `json:"status,omitempty"`
	CreatedAt    int64  `json:"created_at,omitempty"`
}

type ExperienceObservation struct {
	ID           string  `json:"id"`
	ExperienceID string  `json:"experience_id"`
	AttemptID    string  `json:"attempt_id,omitempty"`
	Type         string  `json:"type,omitempty"`
	Summary      string  `json:"summary,omitempty"`
	Source       string  `json:"source,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	CreatedAt    int64   `json:"created_at,omitempty"`
}

type ExperienceScorecard struct {
	Success          float64 `json:"success"`
	EvidenceStrength float64 `json:"evidence_strength"`
	ReuseValue       float64 `json:"reuse_value"`
	Specificity      float64 `json:"specificity"`
	RiskReduction    float64 `json:"risk_reduction"`
	Confidence       float64 `json:"confidence"`
	FinalScore       float64 `json:"final_score"`
	Verdict          string  `json:"verdict"`
}

type ExperiencePromotionResult struct {
	ExperienceID string `json:"experience_id"`
	Target       string `json:"target"`
	CandidateID  string `json:"candidate_id"`
	Status       string `json:"status"`
}

type ExperiencePromotionReadiness struct {
	ExperienceID     string   `json:"experience_id"`
	Ready            bool     `json:"ready"`
	Reasons          []string `json:"reasons,omitempty"`
	WorkedAttempts   int      `json:"worked_attempts"`
	FailedAttempts   int      `json:"failed_attempts"`
	ObservationCount int      `json:"observation_count"`
	EvidenceStrength float64  `json:"evidence_strength"`
	Verdict          string   `json:"verdict,omitempty"`
	ContradictedBy   []string `json:"contradicted_by,omitempty"`
	SupersededBy     []string `json:"superseded_by,omitempty"`
}

type ExperienceLinkInput struct {
	ClosureEntryID       string
	InvariantIDs         []string
	ForbiddenFixIDs      []string
	TouchedFiles         []string
	ChangedSymbols       []string
	AvoidedForbiddenFixs []string
}

const SeedWorkflowDeferExperienceID = "exp.workflow.defer.b2_smoke_success"

type ExperienceRecord struct {
	Experience   ExperienceEntry
	Attempts     []ExperienceAttempt
	Observations []ExperienceObservation
	Scorecard    *ExperienceScorecard
}

type ExperienceSearchQuery struct {
	Goal            string
	Domain          string
	Capability      string
	Files           []string
	Symbols         []string
	InvariantIDs    []string
	ForbiddenFixIDs []string
	Limit           int
}

type ExperienceSearchHit struct {
	ExperienceID  string   `json:"experience_id"`
	Score         float64  `json:"score"`
	Summary       string   `json:"summary"`
	StrategyID    string   `json:"strategy_id"`
	Hint          string   `json:"hint"`
	Status        string   `json:"status"`
	Domain        string   `json:"domain"`
	Capability    string   `json:"capability"`
	Lesson        string   `json:"lesson"`
	Verdict       string   `json:"verdict,omitempty"`
	FinalScore    float64  `json:"final_score,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
	WorkedPaths   []string `json:"worked_paths,omitempty"`
	FailedPaths   []string `json:"failed_paths,omitempty"`
	EvidenceTypes []string `json:"evidence_types,omitempty"`
}

func (g *Graph) CreateExperience(ctx context.Context, e ExperienceEntry) (*ExperienceEntry, error) {
	now := time.Now().Unix()
	if e.ID == "" {
		e.ID = fmt.Sprintf("exp.%d", now)
	}
	e.CreatedAt = now
	e.UpdatedAt = now

	g.expMu.Lock()
	cp := e
	g.experiences[e.ID] = &cp
	g.expMu.Unlock()

	_ = g.writeJSON("experience/entries", e.ID, &e)

	_ = g.AddNode(ctx, Node{ID: "experience:" + e.ID, Type: NodeTypeExperience, Name: e.ID, Summary: e.Summary,
		Metadata: map[string]any{"domain": e.Domain, "capability": e.Capability, "status": e.Status, "kind": e.Kind}})
	if e.GoalNormalized != "" || e.GoalOriginal != "" {
		goalID := "goal_pattern:" + e.ID
		_ = g.AddNode(ctx, Node{ID: goalID, Type: NodeTypeGoalPattern, Name: firstNonEmpty(e.GoalNormalized, e.GoalOriginal),
			Summary: e.GoalOriginal, Metadata: map[string]any{"verb": e.GoalVerb, "object": e.GoalObject}})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + e.ID, Kind: EdgePursuedGoal, Dst: goalID})
	}
	if e.StrategyID != "" {
		strategyNodeID := "strategy:" + e.StrategyID
		_ = g.AddNode(ctx, Node{ID: strategyNodeID, Type: NodeTypeStrategy, Name: e.StrategyID, Summary: e.StrategyID})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + e.ID, Kind: EdgeUsedStrategy, Dst: strategyNodeID})
	}
	if e.Capability != "" {
		capID := "capability:" + e.Capability
		_ = g.AddNode(ctx, Node{ID: capID, Type: "capability", Name: e.Capability})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + e.ID, Kind: EdgeRelatedToCapability, Dst: capID})
	}
	return &e, nil
}

func (g *Graph) AddExperienceAttempt(ctx context.Context, a ExperienceAttempt) (*ExperienceAttempt, error) {
	if a.ExperienceID == "" {
		return nil, fmt.Errorf("experience_id is required")
	}
	now := time.Now().Unix()
	if a.ID == "" {
		a.ID = fmt.Sprintf("attempt.%s.%d", a.ExperienceID, now)
	}
	a.CreatedAt = now

	g.expMu.Lock()
	cp := a
	g.expAttempts[a.ExperienceID] = append(g.expAttempts[a.ExperienceID], &cp)
	g.expMu.Unlock()

	_ = g.writeJSON("experience/attempts", a.ID, &a)

	attemptNodeID := "attempt:" + a.ID
	_ = g.AddNode(ctx, Node{ID: attemptNodeID, Type: NodeTypeAttempt, Name: a.ID, Summary: a.Action,
		Metadata: map[string]any{"status": a.Status, "outcome": a.Outcome}})
	_ = g.AddEdge(ctx, Edge{Src: "experience:" + a.ExperienceID, Kind: EdgeHasAttempt, Dst: attemptNodeID})
	if a.StrategyID != "" {
		strategyNodeID := "strategy:" + a.StrategyID
		_ = g.AddNode(ctx, Node{ID: strategyNodeID, Type: NodeTypeStrategy, Name: a.StrategyID, Summary: a.StrategyID})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + a.ExperienceID, Kind: EdgeUsedStrategy, Dst: strategyNodeID})
	}
	return &a, nil
}

func (g *Graph) AddExperienceObservation(ctx context.Context, o ExperienceObservation) (*ExperienceObservation, error) {
	if o.ExperienceID == "" {
		return nil, fmt.Errorf("experience_id is required")
	}
	now := time.Now().Unix()
	if o.ID == "" {
		o.ID = fmt.Sprintf("obs.%s.%d", o.ExperienceID, now)
	}
	o.CreatedAt = now

	g.expMu.Lock()
	cp := o
	g.expObs[o.ExperienceID] = append(g.expObs[o.ExperienceID], &cp)
	g.expMu.Unlock()

	_ = g.writeJSON("experience/observations", o.ID, &o)

	obsNodeID := "observation:" + o.ID
	_ = g.AddNode(ctx, Node{ID: obsNodeID, Type: NodeTypeObservation, Name: o.ID, Summary: o.Summary,
		Metadata: map[string]any{"type": o.Type, "confidence": o.Confidence, "source": o.Source}})
	if o.AttemptID != "" {
		_ = g.AddEdge(ctx, Edge{Src: "attempt:" + o.AttemptID, Kind: EdgeObservedDuring, Dst: obsNodeID})
	} else {
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + o.ExperienceID, Kind: EdgeObservedDuring, Dst: obsNodeID})
	}
	return &o, nil
}

func (g *Graph) CloseExperience(ctx context.Context, expID string, status string, lesson string, nextHint string, score *ExperienceScorecard) error {
	now := time.Now().Unix()

	g.expMu.Lock()
	exp := g.experiences[expID]
	if exp != nil {
		exp.Status = status
		exp.Lesson = lesson
		exp.NextTimeHint = nextHint
		exp.UpdatedAt = now
		_ = g.writeJSON("experience/entries", expID, exp)
	}
	g.expMu.Unlock()

	n, _ := g.FindNode(ctx, "experience:"+expID)
	meta := map[string]any{"status": status}
	if n != nil && n.Metadata != nil {
		for k, v := range n.Metadata {
			meta[k] = v
		}
		meta["status"] = status
	}
	_ = g.AddNode(ctx, Node{ID: "experience:" + expID, Type: NodeTypeExperience, Name: expID, Summary: lesson, Metadata: meta})
	if lesson != "" {
		lessonID := "lesson:" + expID
		_ = g.AddNode(ctx, Node{ID: lessonID, Type: NodeTypeLesson, Name: expID, Summary: lesson})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + expID, Kind: EdgeProducedLesson, Dst: lessonID})
	}
	if nextHint != "" {
		hintID := "next_time_hint:" + expID
		_ = g.AddNode(ctx, Node{ID: hintID, Type: NodeTypeNextTimeHint, Name: expID, Summary: nextHint})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + expID, Kind: EdgeSuggestsNext, Dst: hintID})
	}
	if score != nil {
		g.expMu.RLock()
		attempts := g.expAttempts[expID]
		observations := g.expObs[expID]
		g.expMu.RUnlock()

		var obsSlice []ExperienceObservation
		for _, o := range observations {
			obsSlice = append(obsSlice, *o)
		}
		if score.EvidenceStrength == 0 {
			score.EvidenceStrength = deriveEvidenceStrength(obsSlice)
		}
		if score.Verdict == "" {
			statusStr := ""
			if exp != nil {
				statusStr = exp.Status
			}
			score.Verdict = deriveVerdict(statusStr, len(attempts), len(observations), lesson)
		}
		if score.FinalScore == 0 {
			score.FinalScore = (score.Success + score.EvidenceStrength + score.ReuseValue + score.Specificity + score.RiskReduction + score.Confidence) / 6.0
		}
		scoreID := "scorecard:" + expID
		_ = g.AddNode(ctx, Node{ID: scoreID, Type: NodeTypeScorecard, Name: expID, Summary: score.Verdict,
			Metadata: map[string]any{
				"success": score.Success, "evidence_strength": score.EvidenceStrength,
				"reuse_value": score.ReuseValue, "specificity": score.Specificity,
				"risk_reduction": score.RiskReduction, "confidence": score.Confidence,
				"final_score": score.FinalScore,
			}})
		_ = g.AddEdge(ctx, Edge{Src: "experience:" + expID, Kind: EdgeValidatedBy, Dst: scoreID})
	}
	return nil
}

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

func (g *Graph) GetExperience(ctx context.Context, expID string) (*ExperienceRecord, error) {
	g.expMu.RLock()
	exp := g.experiences[expID]
	attempts := g.expAttempts[expID]
	observations := g.expObs[expID]
	g.expMu.RUnlock()

	if exp == nil {
		return nil, nil
	}
	rec := &ExperienceRecord{Experience: *exp}
	for _, a := range attempts {
		rec.Attempts = append(rec.Attempts, *a)
	}
	for _, o := range observations {
		rec.Observations = append(rec.Observations, *o)
	}

	// Check for scorecard node.
	if n, _ := g.FindNode(ctx, "scorecard:"+expID); n != nil {
		s := &ExperienceScorecard{}
		s.Success = toFloat(n.Metadata["success"])
		s.EvidenceStrength = toFloat(n.Metadata["evidence_strength"])
		s.ReuseValue = toFloat(n.Metadata["reuse_value"])
		s.Specificity = toFloat(n.Metadata["specificity"])
		s.RiskReduction = toFloat(n.Metadata["risk_reduction"])
		s.Confidence = toFloat(n.Metadata["confidence"])
		s.FinalScore = toFloat(n.Metadata["final_score"])
		s.Verdict = n.Summary
		rec.Scorecard = s
	}

	return rec, nil
}

func (g *Graph) SearchSimilarExperiences(ctx context.Context, q ExperienceSearchQuery) ([]ExperienceSearchHit, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}

	g.expMu.RLock()
	var allExps []*ExperienceEntry
	for _, e := range g.experiences {
		cp := *e
		allExps = append(allExps, &cp)
	}
	g.expMu.RUnlock()

	goalTerms := tokenSet(strings.ToLower(q.Goal))
	fileSet := tokenSet(strings.ToLower(strings.Join(q.Files, " ")))
	symbolSet := tokenSet(strings.ToLower(strings.Join(q.Symbols, " ")))
	invariantSet := tokenSet(strings.ToLower(strings.Join(q.InvariantIDs, " ")))
	forbiddenSet := tokenSet(strings.ToLower(strings.Join(q.ForbiddenFixIDs, " ")))

	hits := []ExperienceSearchHit{}
	for _, e := range allExps {
		reasons := []string{}
		score := 0.0

		if q.Domain != "" && strings.EqualFold(q.Domain, e.Domain) {
			score += 0.15
			reasons = append(reasons, "domain-match")
		}
		if q.Capability != "" && strings.EqualFold(q.Capability, e.Capability) {
			score += 0.15
			reasons = append(reasons, "capability-match")
		}
		nodeTerms := tokenSet(strings.ToLower(strings.Join([]string{
			e.Summary, e.GoalOriginal, e.GoalNormalized, e.GoalVerb, e.GoalObject,
			e.Lesson, e.NextTimeHint, e.Domain, e.Capability,
		}, " ")))
		if len(goalTerms) > 0 {
			v := overlapRatio(goalTerms, nodeTerms)
			score += 0.35 * v
			if v > 0 {
				reasons = append(reasons, "goal-text-overlap")
			}
		}

		// Collect linked files, symbols, invariants, forbidden fixes from graph.
		var fileDsts, symbolDsts, invDsts, forbDsts []string
		var workedPaths, failedPaths []string
		expNodeID := "experience:" + e.ID
		if edges, err := g.OutgoingEdges(ctx, expNodeID); err == nil {
			for _, edge := range edges {
				switch edge.Kind {
				case EdgeTouchesFile:
					fileDsts = append(fileDsts, strings.TrimPrefix(edge.Dst, "source_file:"))
				case EdgeChangedSymbol:
					symbolDsts = append(symbolDsts, strings.TrimPrefix(edge.Dst, "symbol:"))
				case EdgeProtects:
					invDsts = append(invDsts, strings.TrimPrefix(edge.Dst, "invariant:"))
				case EdgeAvoidedForbiddenFix, EdgeProducedForbiddenFixCandidate:
					forbDsts = append(forbDsts, strings.TrimPrefix(edge.Dst, "forbidden_fix:"))
				}
			}
		}

		g.expMu.RLock()
		for _, a := range g.expAttempts[e.ID] {
			if strings.EqualFold(a.Status, "success") {
				workedPaths = append(workedPaths, a.Action)
			} else if strings.EqualFold(a.Status, "failed") {
				failedPaths = append(failedPaths, a.Action)
			}
		}
		var evidenceTypes []string
		seen := map[string]bool{}
		for _, o := range g.expObs[e.ID] {
			if o.Type != "" && !seen[o.Type] {
				seen[o.Type] = true
				evidenceTypes = append(evidenceTypes, o.Type)
			}
		}
		g.expMu.RUnlock()

		fileTerms := tokenSet(strings.ToLower(strings.Join(fileDsts, " ")))
		if len(fileSet) > 0 {
			v := overlapRatio(fileSet, fileTerms)
			score += 0.15 * v
			if v > 0 {
				reasons = append(reasons, "file-overlap")
			}
		}
		symbolTerms := tokenSet(strings.ToLower(strings.Join(symbolDsts, " ")))
		if len(symbolSet) > 0 {
			v := overlapRatio(symbolSet, symbolTerms)
			score += 0.1 * v
			if v > 0 {
				reasons = append(reasons, "symbol-overlap")
			}
		}
		invTerms := tokenSet(strings.ToLower(strings.Join(invDsts, " ")))
		if len(invariantSet) > 0 {
			v := overlapRatio(invariantSet, invTerms)
			score += 0.15 * v
			if v > 0 {
				reasons = append(reasons, "invariant-overlap")
			}
		}
		ffTerms := tokenSet(strings.ToLower(strings.Join(forbDsts, " ")))
		if len(forbiddenSet) > 0 {
			v := overlapRatio(forbiddenSet, ffTerms)
			score += 0.1 * v
			if v > 0 {
				reasons = append(reasons, "forbidden-fix-overlap")
			}
		}

		if score <= 0 {
			continue
		}

		// Get scorecard verdict.
		verdict := ""
		finalScore := 0.0
		if n, _ := g.FindNode(ctx, "scorecard:"+e.ID); n != nil {
			verdict = n.Summary
			finalScore = toFloat(n.Metadata["final_score"])
		}

		hits = append(hits, ExperienceSearchHit{
			ExperienceID:  e.ID,
			Score:         score,
			Summary:       e.Summary,
			StrategyID:    e.StrategyID,
			Hint:          e.NextTimeHint,
			Status:        e.Status,
			Domain:        e.Domain,
			Capability:    e.Capability,
			Lesson:        e.Lesson,
			Verdict:       verdict,
			FinalScore:    finalScore,
			Reasons:       uniqueStrings(reasons),
			WorkedPaths:   uniqueStrings(workedPaths),
			FailedPaths:   uniqueStrings(failedPaths),
			EvidenceTypes: uniqueStrings(evidenceTypes),
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func splitPipe(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.Split(v, "|")
}

func splitComma(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.Split(v, ",")
}

func stripPrefixes(in []string, prefix string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.TrimPrefix(s, prefix))
	}
	return out
}

func (g *Graph) listExperienceObservations(ctx context.Context, expID string) ([]ExperienceObservation, error) {
	g.expMu.RLock()
	ptrs := g.expObs[expID]
	out := make([]ExperienceObservation, len(ptrs))
	for i, o := range ptrs {
		out[i] = *o
	}
	g.expMu.RUnlock()
	return out, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	rep := strings.NewReplacer("_", " ", ".", " ", "/", " ", "-", " ", ",", " ", ":", " ")
	s = rep.Replace(s)
	for _, p := range strings.Fields(s) {
		if len(p) < 3 {
			continue
		}
		out[p] = true
	}
	return out
}

func overlapRatio(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	match := 0
	for k := range a {
		if b[k] {
			match++
		}
	}
	return float64(match) / float64(len(a))
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		k := strings.TrimSpace(s)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func ensurePrefixed(v, prefix string) string {
	if strings.HasPrefix(v, prefix) {
		return v
	}
	return prefix + v
}

func deriveVerdict(status string, attempts int, observations int, lesson string) string {
	if observations == 0 || strings.TrimSpace(lesson) == "" {
		return "unproven"
	}
	if strings.EqualFold(status, "success") && attempts > 0 {
		return "useful"
	}
	if strings.EqualFold(status, "failed") {
		return "weak"
	}
	return "unproven"
}

func deriveEvidenceStrength(observations []ExperienceObservation) float64 {
	if len(observations) == 0 {
		return 0
	}
	weightByType := map[string]float64{
		"test":          1.0,
		"runtime":       0.9,
		"prometheus":    0.8,
		"etcd":          0.8,
		"log":           0.7,
		"static_code":   0.6,
		"operator_note": 0.4,
	}
	sum := 0.0
	for _, o := range observations {
		w := weightByType[strings.ToLower(strings.TrimSpace(o.Type))]
		if w == 0 {
			w = 0.5
		}
		c := o.Confidence
		if c <= 0 {
			c = 0.5
		}
		if c > 1 {
			c = 1
		}
		sum += w * c
	}
	v := sum / float64(len(observations))
	if v > 1 {
		return 1
	}
	return v
}
