package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

const (
	coverageCandidateDefaultMinRepeats = 2
	coverageLearnerActor               = "behavioral.coverage_learner"
)

// learningAuditStore decorates the persistence port at the one place every
// CheckAction already passes: its durable audit write. The underlying ActionCheck
// is persisted FIRST. Learning is then best-effort and may add derived evidence
// plus a promotion candidate, after which the audit row is refreshed with a
// visible learning status.
//
// This keeps two very different failure policies separate:
//   - failure to persist the gate verdict is fatal to CheckAction;
//   - failure to learn from an already-valid verdict must never change whether
//     the proposed action is allowed or blocked.
//
// Embedding Store forwards the rest of the persistence contract unchanged.
type learningAuditStore struct {
	store.Store
	service *Service
}

func (l *learningAuditStore) RecordActionCheck(ctx context.Context, a *api.ActionCheck) error {
	if err := l.Store.RecordActionCheck(ctx, a); err != nil {
		return err
	}
	if a == nil || a.Governed || l.service == nil {
		return nil
	}

	res, err := l.service.learnFromCoverageGap(ctx, a)
	// A domain absent from the registry has no semantic vocabulary from which a
	// generic learner could safely derive a candidate. Preserve the original
	// audit row unchanged rather than pretending that "no pack" means "known
	// domain with no template".
	if !res.DomainKnown && err == nil {
		return nil
	}
	if a.Metadata == nil {
		a.Metadata = map[string]string{}
	}
	switch {
	case err != nil:
		a.Metadata["learning_status"] = "degraded"
		a.Metadata["learning_error"] = err.Error()
	case res.MatchedTemplates == 0:
		a.Metadata["learning_status"] = "uncovered_no_template"
		a.RecommendedSteps = append(a.RecommendedSteps,
			"behavioral memory observed this governance gap, but the domain has no explicit proposed-principle template that matches it")
	default:
		a.Metadata["learning_status"] = "recorded"
		a.Metadata["learning_templates_matched"] = fmt.Sprintf("%d", res.MatchedTemplates)
		a.Metadata["learning_candidates_queued"] = fmt.Sprintf("%d", res.QueuedCandidates)
		if res.QueuedCandidates > 0 {
			a.RecommendedSteps = append(a.RecommendedSteps,
				fmt.Sprintf("behavioral memory queued %d promotion candidate(s) for human review; nothing was auto-promoted", res.QueuedCandidates))
		} else {
			a.RecommendedSteps = append(a.RecommendedSteps,
				"behavioral memory recorded this coverage gap; a matching candidate will enter the review queue when the pattern repeats")
		}
	}

	// Best-effort annotation only. The original audit row is already durable, so
	// a failure here must not turn a valid gate verdict into a transport failure.
	_ = l.Store.RecordActionCheck(ctx, a)
	return nil
}

type coverageLearningResult struct {
	DomainKnown      bool
	MatchedTemplates int
	QueuedCandidates int
}

// learnFromCoverageGap converts one ungoverned ActionCheck into non-authoritative
// learning material. It never creates, mutates, promotes, or indexes a principle.
// The only eligible templates are explicit PROPOSED principles already persisted
// from the registered domain pack. The generic kernel therefore invents no
// condition, authority, evidence, or forbidden-move refs.
func (s *Service) learnFromCoverageGap(ctx context.Context, ac *api.ActionCheck) (coverageLearningResult, error) {
	var out coverageLearningResult
	if ac == nil || ac.Governed || s.registry == nil {
		return out, nil
	}
	pack, ok := s.registry.Lookup(string(ac.Domain))
	if !ok || pack == nil {
		return out, nil
	}
	out.DomainKnown = true
	aliasIdx := s.forbiddenAliasIndex(ac.Domain)

	var failures []string
	for _, seed := range pack.Catalogs().Principles {
		p, err := s.store.GetPrinciple(ctx, ac.Project, string(ac.Domain), seed.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// Registry/store split is a degraded learning state, not an excuse
				// to manufacture a proposal from an in-memory template. Slice A
				// makes this impossible during normal startup; keep it loud here.
				failures = append(failures, fmt.Sprintf("seed principle %s is registered but not persisted", seed.ID))
				continue
			}
			failures = append(failures, fmt.Sprintf("load seed principle %s: %v", seed.ID, err))
			continue
		}
		if p.Status != api.StatusProposedPrinciple && p.Status != api.StatusUnspecified {
			continue // governed/terminal knowledge is not candidate material
		}
		if !coverageTemplateMatches(p, ac, aliasIdx) {
			continue
		}

		out.MatchedTemplates++
		queued, err := s.recordCoverageSupport(ctx, ac, p)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", p.ID, err))
			continue
		}
		if queued {
			out.QueuedCandidates++
		}
	}
	if len(failures) > 0 {
		return out, fmt.Errorf("coverage learning: %s", strings.Join(failures, "; "))
	}
	return out, nil
}

// coverageTemplateMatches mirrors the runtime's engagement law for a proposed
// template: a caller-declared condition engages it, or one of its forbidden
// moves matches the action/target. condition.always alone is deliberately NOT an
// engagement signal; otherwise every universal high-risk seed would become a
// candidate for every unrelated action in the domain.
func coverageTemplateMatches(p *api.Principle, ac *api.ActionCheck, aliasIdx map[string][]string) bool {
	declared := map[api.ConditionRef]bool{}
	for _, c := range ac.Conditions {
		declared[c] = true
	}
	for _, c := range p.AppliesWhen {
		if c != AlwaysConditionRef && declared[c] {
			return true
		}
	}
	for _, fm := range p.ForbiddenMoves {
		if forbiddenRefMatches(fm, ac.ActionType, ac.Target, aliasIdx) {
			return true
		}
	}
	return false
}

func coverageEvidenceID(actionCheckID, principleID string) string {
	sum := sha256.Sum256([]byte(actionCheckID + "|" + principleID))
	return "coverage_gap_evidence." + hex.EncodeToString(sum[:8])
}

func coverageRepeatThreshold(p *api.Principle) int32 {
	// A candidate is a review question, not authority. Letting an irreversible
	// uncovered action happen once without even surfacing the question is too
	// quiet; lower-risk gaps keep the two-observation noise filter.
	if p != nil && p.RiskLevel == "irreversible" {
		return 1
	}
	return coverageCandidateDefaultMinRepeats
}

func containsString(in []string, want string) bool {
	for _, v := range in {
		if v == want {
			return true
		}
	}
	return false
}

// recordCoverageSupport records the fact that THIS exact audit check exposed a
// coverage gap, then accumulates it against a deterministic candidate derived
// from an already-persisted proposed principle.
//
// The evidence targets the ActionCheck, not the Principle. This is load-bearing:
// the promotion gate asks whether evidence targets a principle. A coverage-gap
// observation proves only that governance lacked reach; attaching it to the
// principle would accidentally help satisfy promotion evidence.
func (s *Service) recordCoverageSupport(ctx context.Context, ac *api.ActionCheck, p *api.Principle) (bool, error) {
	theme := "coverage_gap." + p.ID
	cid := candidateID(ac.Project, ac.Domain, theme, *p)
	evidenceID := coverageEvidenceID(ac.ID, p.ID)

	ev := &api.Evidence{
		ID:             evidenceID,
		Project:        ac.Project,
		Domain:         ac.Domain,
		TargetKind:     "action_check",
		TargetID:       ac.ID,
		Kind:           "governance_coverage_gap",
		Lane:           api.LaneStaticOnly,
		Result:         "observed",
		SourceKind:     "behavioral_action_check",
		SourceRef:      ac.ID,
		EntityRef:      ac.Target,
		AuthorityLevel: api.ObservationAuthorityDerived,
		ObservedAt:     ac.CreatedAt,
		Payload:        ac.Explanation,
		Provenance: api.Provenance{
			AgentID:   coverageLearnerActor,
			SourceRef: ac.ID,
			CreatedAt: ac.CreatedAt,
			UpdatedAt: ac.CreatedAt,
		},
		Metadata: map[string]string{
			"learning_kind":      "governance_coverage_gap",
			"candidate_template": p.ID,
		},
	}
	if err := s.store.PutEvidence(ctx, ev); err != nil {
		return false, fmt.Errorf("record coverage-gap evidence: %w", err)
	}

	now := time.Now().Unix()
	candidate, err := s.store.GetPromotionCandidate(ctx, ac.Project, string(ac.Domain), cid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return false, fmt.Errorf("load coverage candidate: %w", err)
	}
	if errors.Is(err, store.ErrNotFound) {
		candidate = &api.PromotionCandidate{
			ID:                      cid,
			Project:                 ac.Project,
			Domain:                  ac.Domain,
			Theme:                   theme,
			Status:                  api.PromotionCandidateStatusUnspecified,
			Title:                   p.Title,
			Rationale:               "an ungoverned runtime action matched this explicit domain-owned proposed principle; repeated coverage gaps should be reviewed for promotion",
			DraftPrinciple:          *p,
			GeneratedBy:             coverageLearnerActor,
			CreatedAt:               now,
			MaterializedPrincipleID: p.ID,
			Metadata: map[string]string{
				"candidate_kind": "COVERAGE_GAP_PROPOSED_PRINCIPLE",
			},
		}
	}

	if !containsString(candidate.SupportingEvidenceIDs, evidenceID) {
		candidate.SupportingEvidenceIDs = append(candidate.SupportingEvidenceIDs, evidenceID)
	}
	candidate.RepeatCount = int32(len(candidate.SupportingEvidenceIDs))
	candidate.Summary = fmt.Sprintf("%d ungoverned action check(s) matched proposed principle %q", candidate.RepeatCount, p.ID)
	candidate.UpdatedAt = now

	// UNSPECIFIED is the private accumulation state: persisted so an observation
	// is not lost, but omitted from the normal QUEUED review list. Irreversible
	// gaps surface on the first observation; all others keep a two-hit filter.
	if candidate.Status == api.PromotionCandidateStatusUnspecified && candidate.RepeatCount >= coverageRepeatThreshold(p) {
		candidate.Status = api.PromotionCandidateStatusQueued
	}
	if err := s.store.UpsertPromotionCandidate(ctx, candidate); err != nil {
		return false, fmt.Errorf("persist coverage candidate: %w", err)
	}
	return candidate.Status == api.PromotionCandidateStatusQueued, nil
}
