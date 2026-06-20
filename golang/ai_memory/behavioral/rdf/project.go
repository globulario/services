package rdf

import (
	"strings"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// Derived instance kinds (not Scylla tables) for lineage subjects.
const (
	kindBackfilledMemory = api.EntityKind("backfilled_memory")
	kindOpsSource        = api.EntityKind("operational_knowledge_source")
)

// Project renders a Bundle to a deterministic, duplicate-free N-Triples document.
// It is pure: no Scylla, no runtime calls (never invokes CheckAction /
// ResolveGovernedContext).
func Project(b *Bundle) []byte { return newProjector(b).run().ntriples() }

// ProjectTriples is Project but returns the triple count too (for validation).
func ProjectTriples(b *Bundle) ([]byte, int) {
	ts := newProjector(b).run()
	return ts.ntriples(), ts.count()
}

type projector struct {
	b  *Bundle
	ts *tripleSet
}

func newProjector(b *Bundle) *projector { return &projector{b: b, ts: newTripleSet()} }

// ── emit helpers ──────────────────────────────────────────────────────────────

func (p *projector) typ(subject, classIRIStr string) { p.ts.add(subject, iri(rdfType), classIRIStr) }
func (p *projector) label(subject, val string) {
	if val != "" {
		p.ts.add(subject, iri(rdfsLabel), literal(val))
	}
}
func (p *projector) lit(subject, predName, val string) {
	if val != "" {
		p.ts.add(subject, predIRI(predName), literal(val))
	}
}
func (p *projector) rel(subject, predName, objectIRI string) {
	p.ts.add(subject, predIRI(predName), objectIRI)
}

// emitRefs links subject→each ref (as an instance IRI of kind) and optionally
// types the referenced instance.
func (p *projector) emitRefs(subject, predName string, kind api.EntityKind, refs []string, alsoTypes ...string) {
	for _, r := range refs {
		if r == "" {
			continue
		}
		obj := instanceIRI(kind, r)
		p.rel(subject, predName, obj)
		for _, c := range alsoTypes {
			p.typ(obj, c)
		}
	}
}

func refStrs[T ~string](in []T) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if string(v) != "" {
			out = append(out, string(v))
		}
	}
	return out
}

func (p *projector) run() *tripleSet {
	for i := range p.b.Signals {
		p.signal(&p.b.Signals[i])
	}
	for i := range p.b.Claims {
		p.claim(&p.b.Claims[i])
	}
	for i := range p.b.Evidence {
		p.evidence(&p.b.Evidence[i])
	}
	for i := range p.b.Authorities {
		p.authority(&p.b.Authorities[i])
	}
	for i := range p.b.Conditions {
		p.condition(&p.b.Conditions[i])
	}
	for i := range p.b.Contradictions {
		p.contradiction(&p.b.Contradictions[i])
	}
	for i := range p.b.Principles {
		p.principle(&p.b.Principles[i])
	}
	for i := range p.b.PromotionDecisions {
		p.promotion(&p.b.PromotionDecisions[i])
	}
	for i := range p.b.RevocationRules {
		p.revocation(&p.b.RevocationRules[i])
	}
	for i := range p.b.ActionChecks {
		p.actionCheck(&p.b.ActionChecks[i])
	}
	for i := range p.b.Outcomes {
		p.outcome(&p.b.Outcomes[i])
	}
	return p.ts
}

// ── per-entity projection (the §7 mapping table) ──────────────────────────────

func (p *projector) signal(s *api.Signal) {
	subj := instanceIRI(api.KindSignal, s.ID)
	p.typ(subj, classIRI(ClassSignal))
	p.lit(subj, PredGovernanceStatus, string(s.Status))
	p.lit(subj, PredSignalKind, string(s.Kind))
	p.lit(subj, PredSourceRef, s.SourceRef)
	if s.Provenance.MemoryID != "" {
		mem := instanceIRI(kindBackfilledMemory, s.Provenance.MemoryID)
		p.rel(subj, PredBackfilledFromMemory, mem)
		p.typ(mem, classIRI(ClassBackfilledMemory))
	}
	for _, r := range splitMeta(s.Metadata["source_refs"]) {
		p.lit(subj, PredSourceRef, r)
	}
}

func (p *projector) claim(c *api.Claim) {
	subj := instanceIRI(api.KindClaim, c.ID)
	p.typ(subj, classIRI(ClassClaim))
	p.lit(subj, PredGovernanceStatus, string(c.Status))
	p.lit(subj, PredSubjectEntity, c.SubjectEntity)
	p.lit(subj, PredPredicate, c.Predicate)
	p.lit(subj, PredObjectValue, c.ObjectValue)
	if c.SignalID != "" {
		sig := instanceIRI(api.KindSignal, c.SignalID)
		p.rel(subj, PredDerivedFromSignal, sig)
		p.rel(sig, PredProducesClaim, subj) // inverse, first-class
	}
}

func (p *projector) evidence(e *api.Evidence) {
	subj := instanceIRI(api.KindEvidence, e.ID)
	p.typ(subj, classIRI(ClassEvidence))
	if e.Lane == api.LaneRuntimeRequired || e.Lane == api.LaneHybrid {
		p.typ(subj, awgIRI(AWGRuntimeEvidence)) // AWG-compatible
	}
	p.lit(subj, PredEvidenceLane, string(e.Lane))
	if e.TargetID != "" {
		tgt := instanceIRI(targetKind(e.TargetKind), e.TargetID)
		p.rel(subj, PredSupportsTarget, tgt)
		p.rel(tgt, PredSupportedBy, subj)
	}
	if e.ObservedFrom != "" {
		p.rel(subj, PredObservedFrom, instanceIRI(api.KindSignal, e.ObservedFrom))
	}
	p.emitRefs(subj, PredSatisfies, api.KindRequiredEvidence, refStrs(e.Satisfies), classIRI(ClassRequiredEvidence), awgIRI(AWGRequiredEvidence))
}

func (p *projector) authority(a *api.Authority) {
	subj := instanceIRI(api.KindAuthority, a.ID)
	p.typ(subj, classIRI(ClassAuthority))
	p.label(subj, a.Title)
	for _, ref := range a.GovernsRefs {
		if obj := canonicalToIRI(ref); obj != "" {
			p.rel(subj, PredGoverns, obj)
		} else {
			p.lit(subj, PredGoverns, ref)
		}
	}
}

func (p *projector) condition(c *api.Condition) {
	subj := instanceIRI(api.KindCondition, c.ID)
	p.typ(subj, classIRI(ClassCondition))
	p.label(subj, c.Title)
}

func (p *projector) contradiction(c *api.Contradiction) {
	subj := instanceIRI(api.KindContradiction, c.ID)
	p.typ(subj, classIRI(ClassContradiction))
	p.lit(subj, PredResolution, c.Resolution)
	for _, ref := range []string{c.LeftRef, c.RightRef} {
		if ref == "" {
			continue
		}
		p.rel(instanceIRI(api.KindClaim, ref), PredContradictedBy, subj)
	}
}

func (p *projector) principle(pr *api.Principle) {
	subj := instanceIRI(api.KindPrinciple, pr.ID)
	p.typ(subj, classIRI(ClassPrinciple))
	p.label(subj, pr.Title)
	p.lit(subj, PredGovernanceStatus, string(pr.Status))
	p.lit(subj, PredRiskLevel, pr.RiskLevel)
	p.emitRefs(subj, PredAppliesWhen, api.KindCondition, refStrs(pr.AppliesWhen))
	p.emitRefs(subj, PredGovernedBy, api.KindAuthority, refStrs(pr.Authorities))
	p.emitRefs(subj, PredRequiresEvidence, api.KindRequiredEvidence, refStrs(pr.RequiredEvidence), classIRI(ClassRequiredEvidence), awgIRI(AWGRequiredEvidence))
	p.emitRefs(subj, PredForbidsMove, api.KindForbiddenMove, refStrs(pr.ForbiddenMoves), classIRI(ClassForbiddenMove), awgIRI(AWGForbiddenRepairMove))
	if pr.PromotionDecisionID != "" {
		p.rel(subj, PredPromotedBy, instanceIRI(api.KindPromotionDecision, pr.PromotionDecisionID))
	}
	if pr.RevocationRuleID != "" {
		p.rel(subj, PredRevokedBy, instanceIRI(api.KindRevocationRule, pr.RevocationRuleID))
	}
	if pr.SupersededBy != "" {
		p.rel(subj, PredSupersededBy, instanceIRI(api.KindPrinciple, pr.SupersededBy))
	}
	if pr.NarrowedBy != "" {
		p.rel(subj, PredNarrowedBy, instanceIRI(api.KindRevocationRule, pr.NarrowedBy))
	}
	generated := false
	for _, ref := range pr.GeneratedFrom {
		if strings.HasPrefix(ref, "opsknowledge:") {
			generated = true
			src := instanceIRI(kindOpsSource, strings.TrimPrefix(ref, "opsknowledge:"))
			p.rel(subj, PredGeneratedFrom, src)
			p.typ(src, classIRI(ClassOperationalKnowledge))
		} else {
			p.lit(subj, PredGeneratedFrom, ref)
		}
	}
	if generated {
		p.typ(subj, classIRI(ClassGeneratedPrinciple))
	}
	for _, ref := range pr.SourceRefs {
		p.lit(subj, PredSourceRef, ref)
		if strings.HasPrefix(ref, "ai-memory:") {
			mem := instanceIRI(kindBackfilledMemory, strings.TrimPrefix(ref, "ai-memory:"))
			p.rel(subj, PredBackfilledFromMemory, mem)
			p.typ(mem, classIRI(ClassBackfilledMemory))
		}
	}
}

func (p *projector) promotion(d *api.PromotionDecisionRecord) {
	subj := instanceIRI(api.KindPromotionDecision, d.ID)
	p.typ(subj, classIRI(ClassPromotionDecision))
	p.typ(subj, awgIRI(AWGPromotionDecision)) // AWG-compatible
	p.lit(subj, PredVerdict, d.Verdict)
	p.lit(subj, PredCheckStatus, string(d.Decision))
	if d.PrincipleID != "" {
		p.rel(subj, PredDecides, instanceIRI(api.KindPrinciple, d.PrincipleID))
	}
	p.emitRefs(subj, PredMissingEvidence, api.KindRequiredEvidence, d.MissingEvidence)
}

func (p *projector) revocation(r *api.RevocationRule) {
	subj := instanceIRI(api.KindRevocationRule, r.ID)
	p.typ(subj, classIRI(ClassRevocationRule))
	p.lit(subj, PredCheckStatus, r.Action)
	if r.PrincipleID != "" {
		p.rel(subj, PredRevokes, instanceIRI(api.KindPrinciple, r.PrincipleID))
	}
}

func (p *projector) actionCheck(a *api.ActionCheck) {
	subj := instanceIRI(api.KindActionCheck, a.ID)
	p.typ(subj, classIRI(ClassActionCheck))
	p.lit(subj, PredCheckStatus, a.Status)
	p.emitRefs(subj, PredCheckedAgainst, api.KindPrinciple, a.CheckedAgainstPrinciples)
	p.emitRefs(subj, PredBlockedBy, api.KindForbiddenMove, refStrs(a.ForbiddenMatched), classIRI(ClassForbiddenMove))
	p.emitRefs(subj, PredMissingEvidence, api.KindRequiredEvidence, refStrs(a.MissingEvidence))
}

func (p *projector) outcome(o *api.Outcome) {
	subj := instanceIRI(api.KindOutcome, o.ID)
	p.typ(subj, classIRI(ClassOutcome))
	p.typ(subj, awgIRI(AWGOutcomeFeedback)) // AWG-compatible
	p.lit(subj, PredOutcomeStatus, o.Status)
	p.lit(subj, PredGroupedByTheme, o.Theme)
	if o.ActionCheckID != "" {
		p.rel(subj, PredResultedFrom, instanceIRI(api.KindActionCheck, o.ActionCheckID))
	}
	p.emitRefs(subj, PredSupportsPrinciple, api.KindPrinciple, o.SupportsPrinciples)
	p.emitRefs(subj, PredWeakensPrinciple, api.KindPrinciple, o.WeakensPrinciples)
}

// ── small helpers ─────────────────────────────────────────────────────────────

func targetKind(k string) api.EntityKind {
	switch k {
	case "principle":
		return api.KindPrinciple
	default:
		return api.KindClaim
	}
}

// canonicalToIRI expands a "behavioral:<kind>/<id>" canonical ref to a full
// instance IRI, or returns "" if the ref is not a canonical URI.
func canonicalToIRI(ref string) string {
	const pfx = "behavioral:"
	if !strings.HasPrefix(ref, pfx) {
		return ""
	}
	return iri(instanceBase + escapeIRIPath(strings.TrimPrefix(ref, pfx)))
}

func splitMeta(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
