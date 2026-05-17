package graph

import (
	"context"
	"sort"
	"strings"
)

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
