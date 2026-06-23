package core

// runtime.go implements the PR-4 runtime decision-support half:
// ResolveGovernedContext (what governed memory applies before action),
// CheckAction (is this proposed action allowed under promoted principles), and
// RecordOutcome (what happened afterward).
//
// These RPCs are READ + audit only with respect to governance: they never
// promote, revoke, or mutate principles, and they never run a cluster probe.
// They evaluate only already-recorded evidence and declared refs. The condition
// → promoted-principle bridge is the principles_by_condition index, which is
// written ONLY by promotion (see governance.go).

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// AlwaysConditionRef is the sentinel condition under which a principle is promoted
// when it must apply UNCONDITIONALLY — i.e. on every CheckAction / resolve, without
// the caller having to self-declare a matching runtime condition. A universal
// hard-rule (e.g. "never hot-swap a binary") is promoted under this so the gate has
// reach even when the caller passes no conditions. Being applicable is NOT being
// blocked: an always-applicable principle still only blocks when its forbidden move
// matches the proposed action. The owning domain pack must list condition.always in
// its condition catalog so principles may reference it. Kept in sync with the
// literal used in the seed YAML.
const AlwaysConditionRef api.ConditionRef = "condition.always"

// applicablePromotedPrinciples resolves the distinct promoted principles indexed
// under any of the given conditions PLUS the always-applicable sentinel. It also
// returns `declared`: the set of principle ids reached via a CALLER-DECLARED
// condition (not the condition.always sentinel). That distinction is the gate's:
// the sentinel makes a universal rule's forbidden-move check reach every action,
// but a principle reached ONLY via the sentinel must NOT impose its situational
// gates (evidence/authority/risk-approval) on an action it does not concern — see
// CheckAction's "engaged" set. The index holds only promoted mappings; the status
// guard is defensive against a stale index entry.
func (s *Service) applicablePromotedPrinciples(ctx context.Context, project string, domain api.DomainRef, conditions []api.ConditionRef) ([]api.Principle, map[string]bool, error) {
	seen := map[string]bool{}
	declared := map[string]bool{}
	var out []api.Principle
	// Sentinel first, then the caller's declared conditions. Principle-id dedup (seen)
	// keeps a principle indexed under both the sentinel and a specific condition from
	// double-counting, but `declared` is recorded BEFORE the dedup skip so a principle
	// seen first via the sentinel is still marked declared when a caller condition
	// also matches it.
	condSet := append([]api.ConditionRef{AlwaysConditionRef}, conditions...)
	for _, c := range condSet {
		ids, err := s.store.ListPrincipleIDsByCondition(ctx, project, string(domain), string(c))
		if err != nil {
			return nil, nil, err
		}
		for _, id := range ids {
			if c != AlwaysConditionRef {
				declared[id] = true
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			p, err := s.store.GetPrinciple(ctx, project, string(domain), id)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					continue
				}
				return nil, nil, err
			}
			if p.Status != api.StatusPromotedPrinciple {
				continue // defensive: only promoted principles influence runtime behavior
			}
			out = append(out, *p)
		}
	}
	return out, declared, nil
}

// forbiddenAliasIndex returns, for a domain, a map from forbidden-move id to the
// action_type aliases that should also match it. Aliases are declared as a
// comma-separated `action_aliases` field on the forbidden-move catalog entry in the
// domain pack (forbidden moves have no store table — they live only in the pack, so
// the kernel resolves them through the registry). Returns nil when the registry or
// the domain pack is absent; callers then fall back to exact id/target matching.
func (s *Service) forbiddenAliasIndex(domain api.DomainRef) map[string][]string {
	if s.registry == nil {
		return nil
	}
	d, ok := s.registry.Lookup(string(domain))
	if !ok {
		return nil
	}
	out := map[string][]string{}
	for _, fm := range d.Catalogs().ForbiddenMoves {
		raw := fm.Fields["action_aliases"]
		if raw == "" {
			continue
		}
		var aliases []string
		for _, a := range strings.Split(raw, ",") {
			if a = strings.TrimSpace(a); a != "" {
				aliases = append(aliases, a)
			}
		}
		if len(aliases) > 0 {
			out[fm.ID] = aliases
		}
	}
	return out
}

// forbiddenRefMatches reports whether a proposed action_type/target matches a
// forbidden-move ref — either by exact id/target equality (the original contract)
// or via one of the ref's declared aliases. A pure function so matching is unit-
// testable without a store or registry.
func forbiddenRefMatches(ref api.ForbiddenMoveRef, actionType, target string, aliasIdx map[string][]string) bool {
	id := string(ref)
	if id == actionType || (target != "" && id == target) {
		return true
	}
	for _, a := range aliasIdx[id] {
		if a == actionType || (target != "" && a == target) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ResolveGovernedContext returns the governed-memory bundle for a goal/condition
// set. It answers "what applies?" — it does not decide if an action is allowed.
func (s *Service) ResolveGovernedContext(ctx context.Context, req *api.ResolveGovernedContextRequest) (*api.ResolveGovernedContextResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	proj, dom := req.Project, string(req.Domain)

	principles, _, err := s.applicablePromotedPrinciples(ctx, proj, req.Domain, req.Conditions)
	if err != nil {
		return nil, fmt.Errorf("resolve governed context: %w", err)
	}
	bundle := api.GovernedContext{ApplicablePrinciples: principles}

	for _, c := range req.Conditions {
		if cond, err := s.store.GetCondition(ctx, proj, dom, string(c)); err == nil {
			bundle.MatchedConditions = append(bundle.MatchedConditions, *cond)
		}
	}

	reqEv, fm, auth := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range principles {
		for _, r := range p.RequiredEvidence {
			reqEv[string(r)] = true
		}
		for _, r := range p.ForbiddenMoves {
			fm[string(r)] = true
		}
		for _, r := range p.Authorities {
			auth[string(r)] = true
		}
	}
	// Required-evidence and forbidden-move catalogs do not exist yet (PR-5); the
	// refs are surfaced first-class as ID-only objects so the bundle stays
	// projection-ready without inventing catalog rows.
	for _, r := range sortedKeys(reqEv) {
		bundle.RequiredEvidence = append(bundle.RequiredEvidence, api.RequiredEvidence{ID: r, Project: proj, Domain: req.Domain})
	}
	for _, r := range sortedKeys(fm) {
		bundle.ForbiddenMoves = append(bundle.ForbiddenMoves, api.ForbiddenMove{ID: r, Project: proj, Domain: req.Domain})
	}
	if len(auth) > 0 {
		unresolved, err := s.store.ResolveAuthorityRefs(ctx, proj, dom, sortedKeys(auth))
		if err != nil {
			return nil, fmt.Errorf("resolve governed context: authorities: %w", err)
		}
		sort.Strings(unresolved)
		for _, r := range unresolved {
			bundle.UnresolvedAuthority = append(bundle.UnresolvedAuthority, api.Authority{ID: r, Project: proj, Domain: req.Domain})
		}
	}

	for _, p := range principles {
		contras, err := s.store.ListContradictionsForTarget(ctx, proj, dom, p.ID)
		if err != nil {
			return nil, fmt.Errorf("resolve governed context: contradictions: %w", err)
		}
		for _, c := range contras {
			if c.Resolution == "" || c.Resolution == "open" {
				bundle.KnownContradictions = append(bundle.KnownContradictions, c)
			}
		}
	}

	// Prior outcomes for a "similar theme" — best-effort, keyed by condition ref.
	for _, c := range req.Conditions {
		if outs, err := s.store.ListOutcomesByTheme(ctx, proj, dom, string(c)); err == nil {
			bundle.PriorOutcomes = append(bundle.PriorOutcomes, outs...)
		}
	}

	var actions []string
	for _, p := range principles {
		if p.RecommendedAction != "" {
			actions = append(actions, p.RecommendedAction)
		}
	}
	bundle.RecommendedBehavior = strings.Join(actions, " | ")
	bundle.Confidence = classifyConfidence(principles, bundle.UnresolvedAuthority, bundle.KnownContradictions)

	return &api.ResolveGovernedContextResponse{Context: bundle}, nil
}

func classifyConfidence(principles []api.Principle, unresolvedAuth []api.Authority, openContras []api.Contradiction) string {
	if len(principles) == 0 {
		return "none"
	}
	if len(unresolvedAuth) > 0 || len(openContras) > 0 {
		return "degraded"
	}
	return "high"
}

// CheckAction evaluates a proposed action against promoted principles and returns
// a single verdict, persisting an audit row for every call.
func (s *Service) CheckAction(ctx context.Context, req *api.CheckActionRequest) (*api.CheckActionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.ActionType == "" {
		return nil, fmt.Errorf("action_type is required")
	}
	proj, dom := req.Project, string(req.Domain)

	principles, declared, err := s.applicablePromotedPrinciples(ctx, proj, req.Domain, req.CurrentConditions)
	if err != nil {
		return nil, fmt.Errorf("check action: %w", err)
	}

	now := time.Now().Unix()
	ac := api.ActionCheck{
		ID: newID(), Project: proj, Domain: req.Domain, ActionType: req.ActionType, Target: req.Target,
		Conditions: req.CurrentConditions, AgentID: req.AgentID, CreatedAt: now,
	}
	for _, p := range principles {
		ac.CheckedAgainstPrinciples = append(ac.CheckedAgainstPrinciples, p.ID)
	}

	// 2. Forbidden moves: a ref matches when it equals the proposed action_type or
	//    target, OR when the action_type/target matches one of the forbidden move's
	//    declared action_aliases. Aliases let a naturally-named action (e.g.
	//    "replace_binary_in_place") match a forbidden-move id (forbidden.cluster.*)
	//    so the gate has reach without the caller guessing the canonical id. Aliases
	//    are resolved from the domain pack via the registry; if the pack is absent we
	//    fall back to exact id/target match (back-compatible).
	aliasIdx := s.forbiddenAliasIndex(req.Domain)
	forbiddenHit := map[string]bool{}
	for _, p := range principles {
		for _, fmRef := range p.ForbiddenMoves {
			if forbiddenRefMatches(fmRef, req.ActionType, req.Target, aliasIdx) {
				ac.ForbiddenMatched = append(ac.ForbiddenMatched, fmRef)
				ac.ViolatedPrinciples = appendUnique(ac.ViolatedPrinciples, p.ID)
				forbiddenHit[p.ID] = true
			}
		}
	}

	// Engaged principles drive the situational gates (evidence, authority, risk/
	// approval). A principle is engaged when the caller DECLARED one of its
	// conditions, OR its forbidden move matched this action. A principle reached
	// ONLY via the condition.always sentinel whose forbidden move did NOT match is
	// in scope for the forbidden check above but is NOT engaged here — otherwise a
	// single always-applicable high-risk rule (e.g. never-hot-swap) would force
	// needs_human_approval / missing-evidence on EVERY action in the domain, which
	// is exactly the over-blocking the sentinel must avoid. Forbidden matching stays
	// over ALL applicable principles so the universal rule still blocks its action.
	var engaged []api.Principle
	for _, p := range principles {
		if declared[p.ID] || forbiddenHit[p.ID] {
			engaged = append(engaged, p)
		}
	}

	// 3. Required evidence satisfaction — two provenance lanes kept DISTINCT
	//    (evidence.provenance_trust_levels). Authoritative evidence is a recorded
	//    Evidence row whose Satisfies covers the ref. Self-asserted evidence is a
	//    bare ref the caller passed in ProvidedEvidenceRefs with no backing row.
	//    CheckAction never runs a probe, so it cannot upgrade a self-assertion to
	//    authoritative — and it must not silently treat the two as equal, which
	//    would let an "allowed" verdict rest on an unverified claim with the trust
	//    provenance erased from the audit trail.
	recorded := map[string]bool{} // satisfied by an authoritative recorded evidence row
	for _, p := range engaged {
		evs, err := s.store.ListEvidenceForTarget(ctx, proj, dom, p.ID)
		if err != nil {
			return nil, fmt.Errorf("check action: evidence: %w", err)
		}
		for _, e := range evs {
			for _, sref := range e.Satisfies {
				recorded[string(sref)] = true
			}
		}
	}
	asserted := map[string]bool{} // satisfied only by caller self-assertion
	for _, r := range req.ProvidedEvidenceRefs {
		if !recorded[r] {
			asserted[r] = true
		}
	}
	var selfAsserted []string
	for _, p := range engaged {
		for _, r := range p.RequiredEvidence {
			switch {
			case recorded[string(r)]:
				// authoritative — satisfied
			case asserted[string(r)]:
				selfAsserted = appendUnique(selfAsserted, string(r))
			default:
				ac.MissingEvidence = appendUniqueRef(ac.MissingEvidence, r)
			}
		}
	}
	// Preserve the trust provenance on the audit row: a verdict resting on
	// self-asserted evidence is weaker than one backed by a recorded row, and the
	// distinction must survive into the audit trail rather than be collapsed.
	if len(selfAsserted) > 0 {
		sort.Strings(selfAsserted)
		if ac.Metadata == nil {
			ac.Metadata = map[string]string{}
		}
		ac.Metadata["self_asserted_evidence"] = strings.Join(selfAsserted, ",")
	}

	// 4. Authority resolvability.
	authSet := map[string]bool{}
	for _, p := range engaged {
		for _, a := range p.Authorities {
			authSet[string(a)] = true
		}
	}
	if len(authSet) > 0 {
		unresolved, err := s.store.ResolveAuthorityRefs(ctx, proj, dom, sortedKeys(authSet))
		if err != nil {
			return nil, fmt.Errorf("check action: authorities: %w", err)
		}
		for _, u := range unresolved {
			ac.UnresolvedAuthority = append(ac.UnresolvedAuthority, api.AuthorityRef(u))
		}
	}

	// 5. Risk / approval. Only ENGAGED principles can demand human approval — an
	// always-applicable high-risk rule that the action does not implicate must not
	// force approval on every action.
	highRisk := false
	for _, p := range engaged {
		if isHighRisk(p.RiskLevel) {
			highRisk = true
		}
	}

	// Verdict precedence: forbidden → evidence → authority → approval → allowed.
	switch {
	case len(ac.ForbiddenMatched) > 0:
		ac.Status, ac.Allowed = "blocked", false
		ac.RecommendedSteps = []string{"do not perform: matches a forbidden move on a promoted principle"}
	case len(ac.MissingEvidence) > 0:
		ac.Status = "needs_evidence"
		ac.RecommendedSteps = []string{"gather required evidence: " + strings.Join(refStrings(ac.MissingEvidence), ", ")}
	case len(ac.UnresolvedAuthority) > 0:
		ac.Status = "needs_authority"
		ac.RecommendedSteps = []string{"resolve governing authority: " + strings.Join(refStrings(ac.UnresolvedAuthority), ", ")}
	case highRisk && req.HumanApproval == "":
		ac.Status = "needs_human_approval"
		ac.RecommendedSteps = []string{"obtain explicit human approval before proceeding (high/irreversible risk)"}
	default:
		ac.Status, ac.Allowed = "allowed", true
		if len(selfAsserted) > 0 {
			ac.RecommendedSteps = append(ac.RecommendedSteps,
				"allowed on SELF-ASSERTED evidence ("+strings.Join(selfAsserted, ", ")+
					"): not backed by a recorded authoritative evidence row — verify before relying on this verdict for an irreversible action")
		}
	}

	// Coverage provenance: was this verdict actually governed by a principle with
	// REACH over THIS action (engaged: condition-declared or forbidden-matched), or
	// default-allowed for lack of one? Counting only engaged principles (not merely
	// sentinel-applicable ones) keeps `governed` honest: an unrelated always-applicable
	// rule being in scope is not governance of this action.
	ac.Governed = len(engaged) > 0
	if ac.Status == "allowed" && !ac.Governed {
		ac.Explanation = fmt.Sprintf("allowed: no applicable promoted principle for %q (ungoverned default-allow)", req.ActionType)
	} else {
		ac.Explanation = fmt.Sprintf("checked %q against %d engaged promoted principle(s): %s",
			req.ActionType, len(engaged), ac.Status)
	}

	// Persist every verdict — the audit row is part of the contract.
	if err := s.store.RecordActionCheck(ctx, &ac); err != nil {
		return nil, fmt.Errorf("check action: persist: %w", err)
	}
	// Best-effort coverage accounting — a counter failure must never fail the
	// verdict; the gate's correctness does not depend on the metric.
	_ = s.store.IncrementCoverage(ctx, proj, dom, ac.Governed)
	return &api.CheckActionResponse{Result: ac}, nil
}

// GetGovernanceCoverage reports how many CheckActions were governed (an applicable
// promoted principle was evaluated) vs ungoverned (default-allow) for a
// project/domain, so the gate's reach can be measured rather than assumed. It is
// read-only and never mutates governance.
func (s *Service) GetGovernanceCoverage(ctx context.Context, req *api.GetGovernanceCoverageRequest) (*api.GetGovernanceCoverageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	governed, ungoverned, err := s.store.GetCoverage(ctx, req.Project, string(req.Domain))
	if err != nil {
		return nil, fmt.Errorf("get governance coverage: %w", err)
	}
	total := governed + ungoverned
	var ratio float64
	if total > 0 {
		ratio = float64(governed) / float64(total)
	}
	return &api.GetGovernanceCoverageResponse{Coverage: api.GovernanceCoverage{
		Project:    req.Project,
		Domain:     string(req.Domain),
		Total:      total,
		Governed:   governed,
		Ungoverned: ungoverned,
		Ratio:      ratio,
	}}, nil
}

// RecordOutcome records what happened after an action/check. It records facts
// only — it never promotes or revokes anything.
func (s *Service) RecordOutcome(ctx context.Context, req *api.RecordOutcomeRequest) (*api.RecordOutcomeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	o := req.Outcome
	if err := requireScope(o.Project, o.Domain); err != nil {
		return nil, err
	}
	if o.ID == "" {
		o.ID = newID()
	}
	if o.CreatedAt == 0 {
		o.CreatedAt = time.Now().Unix()
	}
	if err := s.store.RecordOutcome(ctx, &o); err != nil {
		return nil, fmt.Errorf("record outcome: %w", err)
	}
	return &api.RecordOutcomeResponse{OutcomeID: o.ID}, nil
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func appendUniqueRef(s []api.RequiredEvidenceRef, v api.RequiredEvidenceRef) []api.RequiredEvidenceRef {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
