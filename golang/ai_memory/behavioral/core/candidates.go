package core

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

func normalizeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
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

func candidateID(project string, domain api.DomainRef, theme string, draft api.Principle) string {
	parts := []string{
		project,
		string(domain),
		theme,
		draft.Title,
		strings.Join(normalizeStrings(refStrings(draft.AppliesWhen)), ","),
		strings.Join(normalizeStrings(refStrings(draft.Authorities)), ","),
		strings.Join(normalizeStrings(refStrings(draft.RequiredEvidence)), ","),
		strings.Join(normalizeStrings(refStrings(draft.ForbiddenMoves)), ","),
		draft.RecommendedAction,
		draft.RiskLevel,
		draft.RevocationRule,
		draft.PromotionReason,
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "promotion_candidate." + hex.EncodeToString(sum[:8])
}

func collectOutcomeEvidenceIDs(outcomes []api.Outcome) []string {
	var out []string
	seen := map[string]bool{}
	for _, o := range outcomes {
		for _, id := range o.EvidenceIDs {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func collectOutcomeIDs(outcomes []api.Outcome) []string {
	out := make([]string, 0, len(outcomes))
	for _, o := range outcomes {
		if o.ID != "" {
			out = append(out, o.ID)
		}
	}
	sort.Strings(out)
	return out
}

func validateCandidateDraft(p *api.Principle) error {
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("draft principle title is required")
	}
	if len(p.AppliesWhen) == 0 {
		return fmt.Errorf("draft principle applies_when is required")
	}
	if len(p.Authorities) == 0 {
		return fmt.Errorf("draft principle authorities are required")
	}
	if len(p.RequiredEvidence) == 0 {
		return fmt.Errorf("draft principle required_evidence is required")
	}
	if strings.TrimSpace(p.PromotionReason) == "" {
		return fmt.Errorf("draft principle promotion_reason is required")
	}
	if strings.TrimSpace(p.RevocationRule) == "" {
		return fmt.Errorf("draft principle revocation_rule is required")
	}
	if !validRiskLevels[p.RiskLevel] {
		return fmt.Errorf("draft principle risk_level must be one of info|low|high|irreversible")
	}
	return nil
}

// GeneratePromotionCandidate creates or updates a human-review queue entry when
// a repeated outcome theme has enough support and the draft governance fields
// are explicit. It never creates a principle row and never auto-promotes.
func (s *Service) GeneratePromotionCandidate(ctx context.Context, req *api.GeneratePromotionCandidateRequest) (*api.GeneratePromotionCandidateResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Theme) == "" {
		return nil, fmt.Errorf("theme is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return nil, fmt.Errorf("actor is required")
	}
	if req.MinRepeats <= 0 {
		req.MinRepeats = 2
	}

	draft := req.DraftPrinciple
	draft.Project = req.Project
	draft.Domain = req.Domain
	draft.Status = api.StatusProposedPrinciple
	draft.ProposedBy = req.Actor
	if err := validateCandidateDraft(&draft); err != nil {
		return nil, err
	}

	outcomes, err := s.store.ListOutcomesByTheme(ctx, req.Project, string(req.Domain), req.Theme)
	if err != nil {
		return nil, fmt.Errorf("generate promotion candidate: list outcomes: %w", err)
	}
	if len(outcomes) < int(req.MinRepeats) {
		return nil, fmt.Errorf("generate promotion candidate: theme %q has %d outcomes, need at least %d", req.Theme, len(outcomes), req.MinRepeats)
	}

	supportingOutcomeIDs := collectOutcomeIDs(outcomes)
	supportingEvidenceIDs := normalizeStrings(append(collectOutcomeEvidenceIDs(outcomes), req.SupportingEvidenceIDs...))
	if len(supportingEvidenceIDs) == 0 {
		return nil, fmt.Errorf("generate promotion candidate: explicit supporting evidence is required")
	}

	now := time.Now().Unix()
	id := candidateID(req.Project, req.Domain, req.Theme, draft)
	if draft.ID == "" {
		draft.ID = id + ".draft"
	}
	if draft.Version == 0 {
		draft.Version = 1
	}
	if draft.Provenance.CreatedAt == 0 {
		draft.Provenance.CreatedAt = now
	}
	draft.Provenance.UpdatedAt = now

	candidate := api.PromotionCandidate{
		ID:                    id,
		Project:               req.Project,
		Domain:                req.Domain,
		Theme:                 req.Theme,
		Status:                api.PromotionCandidateStatusQueued,
		Title:                 draft.Title,
		Summary:               fmt.Sprintf("%d repeated outcome(s) for theme %q", len(outcomes), req.Theme),
		Rationale:             req.Rationale,
		SupportingOutcomeIDs:  supportingOutcomeIDs,
		SupportingEvidenceIDs: supportingEvidenceIDs,
		RepeatCount:           int32(len(outcomes)),
		DraftPrinciple:        draft,
		GeneratedBy:           req.Actor,
		CreatedAt:             now,
		UpdatedAt:             now,
		Metadata: map[string]string{
			"candidate_kind": "PROPOSED_PRINCIPLE",
		},
	}
	if existing, err := s.store.GetPromotionCandidate(ctx, req.Project, string(req.Domain), id); err == nil {
		candidate.Status = existing.Status
		candidate.CreatedAt = existing.CreatedAt
		candidate.MaterializedPrincipleID = existing.MaterializedPrincipleID
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("generate promotion candidate: load existing: %w", err)
	}
	if strings.TrimSpace(candidate.Rationale) == "" {
		candidate.Rationale = fmt.Sprintf("theme %q repeated %d time(s); explicit authority, condition, and evidence inputs supplied for review", req.Theme, len(outcomes))
	}
	if err := s.store.UpsertPromotionCandidate(ctx, &candidate); err != nil {
		return nil, fmt.Errorf("generate promotion candidate: persist: %w", err)
	}
	return &api.GeneratePromotionCandidateResponse{Candidate: candidate, OutcomeCount: int32(len(outcomes))}, nil
}

// ListPromotionCandidates returns queued review items. It is read-only and does
// not materialize principles.
func (s *Service) ListPromotionCandidates(ctx context.Context, req *api.ListPromotionCandidatesRequest) (*api.ListPromotionCandidatesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	list, err := s.store.ListPromotionCandidates(ctx, req.Project, string(req.Domain), req.Theme, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list promotion candidates: %w", err)
	}
	return &api.ListPromotionCandidatesResponse{Candidates: list}, nil
}
