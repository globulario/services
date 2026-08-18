package cluster_operator

// learning.go is where this pack says what an observed pattern MEANS.
//
// It replaces cluster-doctor's draftForInvariant table. That table lived in the
// doctor and authored full governance drafts — titles, authorities, evidence,
// risk levels, revocation rules — for a domain the doctor does not own. Two
// consequences followed, and both were live on master:
//
//  1. Anything absent from the table produced no candidate, so most findings and
//     every programming behavior could repeat forever without becoming
//     reviewable.
//  2. The table drifted from the pack. For the same rule, the doctor cited
//     authority.cluster.node_agent.runtime_state while this pack's
//     principle.cluster.restart_drifted_unit_with_observed_finding cites
//     authority.cluster.owner_service.runtime_state. Two hand-maintained copies
//     of one governance claim, disagreeing, with nothing to detect it.
//
// So the mapping here names a SEED ID and nothing else. It cannot drift from the
// catalogs, because it does not restate them — the draft is whatever the seed
// says at read time. TestLearningTemplatesResolve fails if a named seed stops
// existing, which is the failure the doctor's table could not detect: an
// unresolvable ref produced no error, just a silently empty review queue that
// reads exactly like a healthy cluster.

import "github.com/globulario/services/golang/ai_memory/behavioral/domain"

// learningTemplateByCondition disambiguates observations whose condition alone
// does not identify one seed.
//
// Only ambiguous cases belong here. condition.cluster.service.desired_observed_mismatch
// is claimed by two seeds — no_recovery_claim_without_authoritative_evidence and
// preserve_owner_executor_boundary — so generic matching correctly refuses to
// choose, and this pack states which one governs a repeated remediation
// pattern. Conditions claimed by exactly one seed are left to
// domain.MatchCandidateTemplate; listing them here would recreate the
// duplication this file exists to remove.
var learningTemplateByCondition = map[string]string{
	// A repeated desired/observed mismatch is an evidence problem before it is a
	// topology problem: the recurring failure is claiming recovery without the
	// owner authority having compared desired against observed. The
	// owner/executor boundary seed governs WHO may act, which is a different
	// question from whether recovery may be claimed at all.
	"condition.cluster.service.desired_observed_mismatch": "principle.cluster.no_recovery_claim_without_authoritative_evidence",
}

// CandidateTemplate implements domain.LearningSource.
//
// It returns only a seed id. The pack cannot hand back an arbitrary draft by
// design — a draft assembled here could name refs outside the catalogs, which
// is precisely the unresolvable-ref failure the boundary prevents.
func (p *Pack) CandidateTemplate(obs domain.LearningObservation) (string, bool) {
	for _, c := range obs.Conditions {
		if seedID, ok := learningTemplateByCondition[c]; ok {
			return seedID, true
		}
	}
	return "", false
}

// LearningTemplateSeedIDs returns every seed id this pack names as a template,
// so a test can assert each one resolves.
func LearningTemplateSeedIDs() []string {
	ids := make([]string, 0, len(learningTemplateByCondition))
	for _, id := range learningTemplateByCondition {
		ids = append(ids, id)
	}
	return ids
}

var _ domain.LearningSource = (*Pack)(nil)
