package domain

import "github.com/globulario/services/golang/ai_memory/behavioral/api"

// PrincipleFromSeed materializes one domain-owned seed template into the exact
// PROPOSED principle shape persisted by LoadCatalogs. Keeping this conversion in
// one place matters because the runtime learner also uses seed principles as
// candidate templates: startup and learning must not interpret the same pack in
// two subtly different ways.
//
// The returned principle is still only PROPOSED. This helper confers no runtime
// authority and never promotes anything.
func PrincipleFromSeed(project string, d Domain, ps PrincipleSeed) api.Principle {
	name := d.Name()
	return api.Principle{
		ID:       ps.ID,
		Project:  project,
		Domain:   api.DomainRef(name),
		Title:    ps.Title,
		AppliesWhen:      toRefs[api.ConditionRef](ps.AppliesWhen),
		Authorities:      toRefs[api.AuthorityRef](ps.Authorities),
		RequiredEvidence: toRefs[api.RequiredEvidenceRef](ps.RequiredEvidence),
		ForbiddenMoves:   toRefs[api.ForbiddenMoveRef](ps.ForbiddenMoves),
		RecommendedAction: ps.RecommendedAction,
		RiskLevel:         ps.RiskLevel,
		RevocationRule:    ps.RevocationRule,
		PromotionReason:   ps.PromotionReason,
		Status:            api.StatusProposedPrinciple,
		Version:           1,
		ProposedBy:        "seed:" + name,
		SourceRefs:        ps.SourceRefs,
		GeneratedFrom:     ps.GeneratedFrom,
		Provenance:        api.Provenance{AgentID: "seed:" + name},
		Metadata:          seedMeta(name),
	}
}
