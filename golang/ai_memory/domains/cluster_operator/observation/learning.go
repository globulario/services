package observation

// learning.go bridges a domain-owned seed principle to the candidate draft the
// governance API takes. It is a translation, not a decision: every field comes
// from the seed, and nothing is defaulted, inferred, or filled in when absent.
//
// A missing field must stay missing. Substituting a plausible risk level or
// revocation rule here would put a governance claim into a human's review queue
// that no domain author ever wrote — and it would be indistinguishable from one
// they did.

import (
	"sync"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
)

// learningPack is the domain pack used to resolve templates. Built once and
// cached: parsing the embedded catalogs on every observation would put YAML
// parsing on the learning loop's path for no benefit, since the catalogs are
// compiled in and cannot change at runtime.
//
// A construction failure is remembered, not retried. If the embedded catalogs
// do not parse, they will not parse on the next tick either, and retrying would
// turn one packaging defect into a per-tick cost forever.
var (
	learningPackOnce sync.Once
	learningPack     *cluster_operator.Pack
)

func loadLearningPack() *cluster_operator.Pack {
	learningPackOnce.Do(func() {
		p, err := cluster_operator.New()
		if err != nil {
			// Leave learningPack nil — CandidateDraftFor then reports "no
			// template", which degrades learning without touching any verdict.
			return
		}
		learningPack = p
	})
	return learningPack
}

// CandidateDraftFor returns the draft this domain proposes for an observed
// pattern, or ok=false when the domain has no template for it.
//
// ok=false is the normal, honest answer for an unrecognized pattern. The caller
// must queue nothing rather than queue something generic: a candidate whose
// scope nobody established costs a human the review AND teaches them the queue
// is noise.
func CandidateDraftFor(obs domain.LearningObservation) (CandidateDraft, bool) {
	pack := loadLearningPack()
	if pack == nil {
		return CandidateDraft{}, false
	}
	seed, ok := domain.CandidateTemplateFor(pack, obs)
	if !ok {
		return CandidateDraft{}, false
	}
	return CandidateDraft{
		Title:             seed.Title,
		AppliesWhen:       append([]string(nil), seed.AppliesWhen...),
		Authorities:       append([]string(nil), seed.Authorities...),
		RequiredEvidence:  append([]string(nil), seed.RequiredEvidence...),
		RecommendedAction: seed.RecommendedAction,
		RiskLevel:         seed.RiskLevel,
		PromotionReason:   seed.PromotionReason,
		RevocationRule:    seed.RevocationRule,
	}, true
}
