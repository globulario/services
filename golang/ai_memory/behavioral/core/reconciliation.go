package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

const (
	findingBehavioralMissingAWG  = "BEHAVIORAL_CANDIDATE_MISSING_AWG_MAPPING"
	findingRuntimeContradictsAWG = "RUNTIME_CONTRADICTS_AWG"
	findingRuntimeReinforcesAWG  = "RUNTIME_REINFORCES_AWG"
	findingAWGMissingBehavioral  = "AWG_RUNTIME_RELEVANT_WITHOUT_BEHAVIORAL_CANDIDATE"
	findingAWGMissingTestMapping = "AWG_MAPPING_MISSING_TEST_CANDIDATE"
)

func uniqStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func proposedAWGIDs(prefix string, theme string, count int) []string {
	if count > 0 {
		return nil
	}
	theme = strings.TrimSpace(theme)
	if theme == "" {
		theme = "runtime.theme"
	}
	repl := strings.NewReplacer(" ", "_", "/", ".", "-", "_", ":", "_")
	return []string{prefix + "." + repl.Replace(theme)}
}

func summarizeOutcomes(outcomes []api.Outcome) (failures, successes, severe int32) {
	for _, o := range outcomes {
		switch o.Status {
		case "failure", "blocked", "reverted":
			failures++
		case "success":
			successes++
		}
		if o.Severe {
			severe++
		}
	}
	return
}

// GenerateReconciliationReport creates an advisory bridge artifact between
// behavioral-memory and AWG. It surfaces drift/reinforcement and proposed
// review links only; it never mutates either governance surface.
func (s *Service) GenerateReconciliationReport(ctx context.Context, req *api.GenerateReconciliationReportRequest) (*api.GenerateReconciliationReportResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Actor) == "" {
		return nil, fmt.Errorf("actor is required")
	}

	var (
		candidate *api.PromotionCandidate
		err       error
		theme     = strings.TrimSpace(req.Theme)
	)
	if req.PromotionCandidateID != "" {
		candidate, err = s.store.GetPromotionCandidate(ctx, req.Project, string(req.Domain), req.PromotionCandidateID)
		if err != nil {
			return nil, fmt.Errorf("generate reconciliation report: load promotion candidate: %w", err)
		}
		if theme == "" {
			theme = candidate.Theme
		}
	}
	if theme == "" && req.RuntimeRelevant {
		return nil, fmt.Errorf("theme is required when runtime_relevant is true and no promotion candidate is supplied")
	}

	outcomes, err := s.store.ListOutcomesByTheme(ctx, req.Project, string(req.Domain), theme)
	if err != nil && theme != "" {
		return nil, fmt.Errorf("generate reconciliation report: list outcomes: %w", err)
	}
	failures, successes, severe := summarizeOutcomes(outcomes)
	findingSet := map[string]bool{}
	if candidate != nil && len(req.AWGInvariantIDs) == 0 && len(req.AWGFailureModeIDs) == 0 && len(req.AWGTestIDs) == 0 {
		findingSet[findingBehavioralMissingAWG] = true
	}
	if req.RuntimeRelevant && candidate == nil && (len(req.AWGInvariantIDs) > 0 || len(req.AWGFailureModeIDs) > 0 || len(req.AWGTestIDs) > 0) {
		findingSet[findingAWGMissingBehavioral] = true
	}
	if (len(req.AWGInvariantIDs) > 0 || len(req.AWGFailureModeIDs) > 0) && len(req.AWGTestIDs) == 0 {
		findingSet[findingAWGMissingTestMapping] = true
	}
	if candidate != nil && (len(req.AWGInvariantIDs) > 0 || len(req.AWGFailureModeIDs) > 0) {
		switch {
		case failures > 0 || severe > 0:
			findingSet[findingRuntimeContradictsAWG] = true
		case len(outcomes) > 0 && failures == 0 && severe == 0 && successes == int32(len(outcomes)):
			findingSet[findingRuntimeReinforcesAWG] = true
		}
	}

	findings := make([]string, 0, len(findingSet))
	for f := range findingSet {
		findings = append(findings, f)
	}
	sort.Strings(findings)

	report := api.ReconciliationReport{
		ID:                        newID(),
		Project:                   req.Project,
		Domain:                    req.Domain,
		Theme:                     theme,
		AWGInvariantIDs:           uniqStrings(req.AWGInvariantIDs),
		AWGFailureModeIDs:         uniqStrings(req.AWGFailureModeIDs),
		AWGTestIDs:                uniqStrings(req.AWGTestIDs),
		Findings:                  findings,
		OutcomeCount:              int32(len(outcomes)),
		FailureCount:              failures,
		SuccessCount:              successes,
		SevereCount:               severe,
		ProposedAWGInvariantIDs:   proposedAWGIDs("invariant", theme, len(req.AWGInvariantIDs)),
		ProposedAWGFailureModeIDs: proposedAWGIDs("failure_mode", theme, len(req.AWGFailureModeIDs)),
		ProposedAWGTestIDs:        proposedAWGIDs("test", theme, len(req.AWGTestIDs)),
		Actor:                     req.Actor,
		CreatedAt:                 time.Now().Unix(),
		Metadata:                  map[string]string{},
	}
	if candidate != nil {
		report.PromotionCandidateID = candidate.ID
	}
	if req.RuntimeRelevant && candidate == nil {
		report.ProposedBehavioralTheme = theme
	}
	if len(findings) == 0 {
		report.Summary = "behavioral-memory and AWG inputs are aligned with no surfaced reconciliation drift"
	} else {
		report.Summary = "reconciliation surfaced: " + strings.Join(findings, ", ")
	}
	if err := s.store.PutReconciliationReport(ctx, &report); err != nil {
		return nil, fmt.Errorf("generate reconciliation report: persist: %w", err)
	}
	return &api.GenerateReconciliationReportResponse{Report: report}, nil
}

// ListReconciliationReports returns stored advisory bridge artifacts.
func (s *Service) ListReconciliationReports(ctx context.Context, req *api.ListReconciliationReportsRequest) (*api.ListReconciliationReportsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	reports, err := s.store.ListReconciliationReports(ctx, req.Project, string(req.Domain), req.Theme, req.PromotionCandidateID, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list reconciliation reports: %w", err)
	}
	return &api.ListReconciliationReportsResponse{Reports: reports}, nil
}
