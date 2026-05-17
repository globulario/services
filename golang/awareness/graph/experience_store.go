package graph

import (
	"context"
	"fmt"
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
