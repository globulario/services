// Package behavioral_rdf is the Scylla-backed projection reader + export tooling
// for the behavioral-memory RDF projection (behavioral/rdf).
//
// It lives OUTSIDE behavioral/ because it imports the gocql driver — keeping the
// generic kernel (and behavioral/rdf) free of any database dependency. It is a
// READ-ONLY, one-shot admin path: full-table scans (no ALLOW FILTERING, no WHERE)
// filtered in Go by project/domain. It is NEVER on the runtime path and never
// touches CheckAction / ResolveGovernedContext.
package behavioral_rdf

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/rdf"
	"github.com/gocql/gocql"
)

// ScyllaReader reads behavioral-memory rows for projection via full-table scans.
type ScyllaReader struct{ session *gocql.Session }

// NewScyllaReader builds a reader over an existing gocql session.
func NewScyllaReader(session *gocql.Session) *ScyllaReader { return &ScyllaReader{session: session} }

var _ rdf.Reader = (*ScyllaReader)(nil)

func (r *ScyllaReader) scope(project, domain, p, d string) bool {
	if project != "" && p != project {
		return false
	}
	if domain != "" && d != domain {
		return false
	}
	return true
}

// Read scans the behavioral_memory tables and returns a scoped Bundle.
func (r *ScyllaReader) Read(ctx context.Context, opts rdf.ReadOptions) (*rdf.Bundle, error) {
	b := &rdf.Bundle{}
	q := func(cql string, scan func(*gocql.Iter) error) error {
		iter := r.session.Query(cql).WithContext(ctx).Iter()
		if err := scan(iter); err != nil {
			_ = iter.Close()
			return err
		}
		return iter.Close()
	}

	if err := q(`SELECT project, domain, id, kind, source_ref, status, memory_id, metadata FROM behavioral_memory.signals`, func(it *gocql.Iter) error {
		var p, d, id, kind, sref, status, mem string
		var meta map[string]string
		for it.Scan(&p, &d, &id, &kind, &sref, &status, &mem, &meta) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Signals = append(b.Signals, api.Signal{ID: id, Project: p, Domain: api.DomainRef(d), Kind: api.SignalKind(kind),
					SourceRef: sref, Status: api.GovernanceStatus(status), Provenance: api.Provenance{MemoryID: mem}, Metadata: meta})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan signals: %w", err)
	}

	if err := q(`SELECT project, domain, id, signal_id, status, subject_entity, predicate, object_value FROM behavioral_memory.claims`, func(it *gocql.Iter) error {
		var p, d, id, sig, status, subj, pred, obj string
		for it.Scan(&p, &d, &id, &sig, &status, &subj, &pred, &obj) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Claims = append(b.Claims, api.Claim{ID: id, Project: p, Domain: api.DomainRef(d), SignalID: sig,
					Status: api.GovernanceStatus(status), SubjectEntity: subj, Predicate: pred, ObjectValue: obj})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan claims: %w", err)
	}

	if err := q(`SELECT project, domain, id, target_kind, target_id, lane, observed_from, satisfies FROM behavioral_memory.evidence`, func(it *gocql.Iter) error {
		var p, d, id, tk, tid, lane, obsFrom string
		var sat []string
		for it.Scan(&p, &d, &id, &tk, &tid, &lane, &obsFrom, &sat) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Evidence = append(b.Evidence, api.Evidence{ID: id, Project: p, Domain: api.DomainRef(d), TargetKind: tk, TargetID: tid,
					Lane: api.EvidenceLane(lane), ObservedFrom: obsFrom, Satisfies: toReqRefs(sat)})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan evidence: %w", err)
	}

	if err := q(`SELECT project, domain, id, title, governs_refs FROM behavioral_memory.authorities`, func(it *gocql.Iter) error {
		var p, d, id, title string
		var gov []string
		for it.Scan(&p, &d, &id, &title, &gov) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Authorities = append(b.Authorities, api.Authority{ID: id, Project: p, Domain: api.DomainRef(d), Title: title, GovernsRefs: gov})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan authorities: %w", err)
	}

	if err := q(`SELECT project, domain, id, title FROM behavioral_memory.conditions`, func(it *gocql.Iter) error {
		var p, d, id, title string
		for it.Scan(&p, &d, &id, &title) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Conditions = append(b.Conditions, api.Condition{ID: id, Project: p, Domain: api.DomainRef(d), Title: title})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan conditions: %w", err)
	}

	if err := q(`SELECT project, domain, id, kind, resolution, left_ref, right_ref FROM behavioral_memory.contradictions`, func(it *gocql.Iter) error {
		var p, d, id, kind, res, left, right string
		for it.Scan(&p, &d, &id, &kind, &res, &left, &right) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Contradictions = append(b.Contradictions, api.Contradiction{ID: id, Project: p, Domain: api.DomainRef(d), Kind: kind, Resolution: res, LeftRef: left, RightRef: right})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan contradictions: %w", err)
	}

	if err := q(`SELECT project, domain, id, title, status, risk_level, applies_when, authorities, required_evidence, forbidden_moves, promotion_decision_id, revocation_rule_id, superseded_by, narrowed_by, source_refs, generated_from FROM behavioral_memory.principles`, func(it *gocql.Iter) error {
		var p, d, id, title, status, risk, pdid, rrid, sup, narr string
		var aw, auth, req, forb, src, gen []string
		for it.Scan(&p, &d, &id, &title, &status, &risk, &aw, &auth, &req, &forb, &pdid, &rrid, &sup, &narr, &src, &gen) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Principles = append(b.Principles, api.Principle{ID: id, Project: p, Domain: api.DomainRef(d), Title: title,
					Status: api.GovernanceStatus(status), RiskLevel: risk, AppliesWhen: toCondRefs(aw), Authorities: toAuthRefs(auth),
					RequiredEvidence: toReqRefs(req), ForbiddenMoves: toFmRefs(forb), PromotionDecisionID: pdid, RevocationRuleID: rrid,
					SupersededBy: sup, NarrowedBy: narr, SourceRefs: src, GeneratedFrom: gen})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan principles: %w", err)
	}

	if err := q(`SELECT project, domain, id, principle_id, decision, verdict, missing_evidence FROM behavioral_memory.promotion_decisions`, func(it *gocql.Iter) error {
		var p, d, id, pid, dec, verdict string
		var miss []string
		for it.Scan(&p, &d, &id, &pid, &dec, &verdict, &miss) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.PromotionDecisions = append(b.PromotionDecisions, api.PromotionDecisionRecord{ID: id, Project: p, Domain: api.DomainRef(d),
					PrincipleID: pid, Decision: api.PromotionDecision(dec), Verdict: verdict, MissingEvidence: miss})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan promotion_decisions: %w", err)
	}

	if err := q(`SELECT project, domain, id, principle_id, action FROM behavioral_memory.revocation_rules`, func(it *gocql.Iter) error {
		var p, d, id, pid, action string
		for it.Scan(&p, &d, &id, &pid, &action) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.RevocationRules = append(b.RevocationRules, api.RevocationRule{ID: id, Project: p, Domain: api.DomainRef(d), PrincipleID: pid, Action: action})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan revocation_rules: %w", err)
	}

	if err := q(`SELECT project, domain, id, status, checked_against_principles, forbidden_matched, missing_evidence FROM behavioral_memory.action_checks`, func(it *gocql.Iter) error {
		var p, d, id, status string
		var checked, forb, miss []string
		for it.Scan(&p, &d, &id, &status, &checked, &forb, &miss) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.ActionChecks = append(b.ActionChecks, api.ActionCheck{ID: id, Project: p, Domain: api.DomainRef(d), Status: status,
					CheckedAgainstPrinciples: checked, ForbiddenMatched: toFmRefs(forb), MissingEvidence: toReqRefs(miss)})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan action_checks: %w", err)
	}

	if err := q(`SELECT project, domain, id, action_check_id, status, theme, supports_principles, weakens_principles FROM behavioral_memory.outcomes`, func(it *gocql.Iter) error {
		var p, d, id, acid, status, theme string
		var sup, weak []string
		for it.Scan(&p, &d, &id, &acid, &status, &theme, &sup, &weak) {
			if r.scope(opts.Project, opts.Domain, p, d) {
				b.Outcomes = append(b.Outcomes, api.Outcome{ID: id, Project: p, Domain: api.DomainRef(d), ActionCheckID: acid, Status: status,
					Theme: theme, SupportsPrinciples: sup, WeakensPrinciples: weak})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan outcomes: %w", err)
	}

	return b, nil
}

func toReqRefs(in []string) []api.RequiredEvidenceRef {
	out := make([]api.RequiredEvidenceRef, len(in))
	for i, v := range in {
		out[i] = api.RequiredEvidenceRef(v)
	}
	return out
}
func toCondRefs(in []string) []api.ConditionRef {
	out := make([]api.ConditionRef, len(in))
	for i, v := range in {
		out[i] = api.ConditionRef(v)
	}
	return out
}
func toAuthRefs(in []string) []api.AuthorityRef {
	out := make([]api.AuthorityRef, len(in))
	for i, v := range in {
		out[i] = api.AuthorityRef(v)
	}
	return out
}
func toFmRefs(in []string) []api.ForbiddenMoveRef {
	out := make([]api.ForbiddenMoveRef, len(in))
	for i, v := range in {
		out[i] = api.ForbiddenMoveRef(v)
	}
	return out
}
