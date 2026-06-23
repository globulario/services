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
// under any of the given conditions PLUS the always-applicable sentinel. The index
// holds only promoted mappings; the status guard is defensive against a stale index
// entry.
func (s *Service) applicablePromotedPrinciples(ctx context.Context, project string, domain api.DomainRef, conditions []api.ConditionRef) ([]api.Principle, error) {
	seen := map[string]bool{}
	var out []api.Principle
	// Always include the unconditional sentinel so universal hard-rules fire without
	// the caller declaring a condition. Principle-id dedup (seen) keeps a principle
	// indexed under both the sentinel and a specific condition from double-counting.
	condSet := append([]api.ConditionRef{AlwaysConditionRef}, conditions...)
	for _, c := range condSet {
		ids, err := s.store.ListPrincipleIDsByCondition(ctx, project, string(domain), string(c))
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			p, err := s.store.GetPrinciple(ctx, project, string(domain), id)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					continue
				}
				return nil, err
			}
			if p.Status != api.StatusPromotedPrinciple {
				continue // defensive: only promoted principles influence runtime behavior
			}
			out = append(out, *p)
		}
	}
	return out, nil
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

	principles, err := s.applicablePromotedPrinciples(ctx, proj, req.Domain, req.Conditions)
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

	principles, err := s.applicablePromotedPrinciples(ctx, proj, req.Domain, req.CurrentConditions)
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
	for _, p := range principles {
		for _, fmRef := range p.ForbiddenMoves {
			if forbiddenRefMatches(fmRef, req.ActionType, req.Target, aliasIdx) {
				ac.ForbiddenMatched = append(ac.ForbiddenMatched, fmRef)
				ac.ViolatedPrinciples = appendUnique(ac.ViolatedPrinciples, p.ID)
			}
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
	for _, p := range principles {
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
	for _, p := range principles {
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
	for _, p := range principles {
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

	// 5. Risk / approval.
	highRisk := false
	for _, p := range principles {
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

	// Coverage provenance: was this verdict actually governed (did any applicable
	// promoted principle apply), or default-allowed for lack of one? Without this,
	// "allowed" hides whether the gate had any reach over the action.
	ac.Governed = len(principles) > 0
	if ac.Status == "allowed" && !ac.Governed {
		ac.Explanation = fmt.Sprintf("allowed: no applicable promoted principle for %q (ungoverned default-allow)", req.ActionType)
	} else {
		ac.Explanation = fmt.Sprintf("checked %q against %d promoted principle(s): %s",
			req.ActionType, len(principles), ac.Status)
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
