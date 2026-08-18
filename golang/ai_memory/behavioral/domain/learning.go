package domain

// learning.go defines the GENERIC learning-template boundary.
//
// Before this, the only autonomous learner was cluster-doctor's
// draftForInvariant table: a hand-maintained map from two invariant ids to
// fully-authored governance drafts. Anything absent from that table produced no
// candidate, so most cluster findings, every programming behavior, and cases
// like a Scylla data wipe could accumulate evidence forever without ever
// becoming reviewable.
//
// The table was not careless — its comment explains the real constraint it was
// protecting: every ref in a draft must resolve in the pack's catalogs, and a
// DERIVED id would silently stop resolving the moment a naming convention
// shifted. The failure would be invisible, because an empty review queue reads
// exactly like a healthy cluster.
//
// This file keeps that constraint and removes the duplication, by deriving
// templates from the catalogs THEMSELVES rather than from a parallel copy of
// them. A template's refs are not constructed, guessed, or interpolated — they
// are the seed principle's own refs, which resolve by construction because the
// pack authored and validated them.
//
// The authority split this preserves:
//
//	the kernel        knows a pattern repeated
//	the domain pack   knows what that pattern MEANS
//	a human           decides whether it becomes a rule
//
// The kernel must never author the middle one. A generic learner that invented
// canonical refs would be manufacturing domain semantics it has no standing to
// hold — and it would do so in the one place nobody looks, since a wrong draft
// and a right draft are equally silent until someone reviews them.

import "sort"

// LearningObservation is what the kernel can honestly say about a repeated
// pattern: which theme it accumulated under, what action was attempted, and
// which conditions were declared at the time. It carries no domain meaning.
type LearningObservation struct {
	Theme      string
	ActionType string
	Conditions []string
}

// LearningSource is an OPTIONAL domain-pack capability. A pack implements it
// only when catalog-derived matching is not expressive enough for its domain;
// packs that do not implement it still learn, through MatchCandidateTemplate.
//
// It returns a seed the pack already owns. It deliberately cannot return an
// arbitrary draft: a pack proposing refs outside its own catalogs would
// reintroduce exactly the unresolvable-ref failure this boundary exists to
// prevent.
type LearningSource interface {
	Domain
	// CandidateTemplate returns the id of the seed principle this pack considers
	// the template for an observation, and whether it has one.
	CandidateTemplate(obs LearningObservation) (seedID string, ok bool)
}

// MatchCandidateTemplate returns the pack's own seed principle that best
// matches an observation, or ok=false when none does.
//
// Matching is by CONDITION OVERLAP first, then by forbidden-move alias against
// the attempted action. Both are properties the pack already declared; nothing
// is inferred from naming shape, string similarity, or the theme text, because
// each of those would let a rename silently change which rule gets proposed.
//
// Ambiguity yields NO template. When two seeds match an observation equally
// well, the pack has not said which one governs it, and picking either would
// ask a human to approve a scope nobody established. No hint is honest; a
// confidently wrong hint is not. This is the same safe direction the table it
// replaces chose when it produced nothing for an unlisted invariant.
func MatchCandidateTemplate(cats Catalogs, obs LearningObservation) (PrincipleSeed, bool) {
	conds := newStringSet(obs.Conditions)

	type scored struct {
		seed  PrincipleSeed
		score int
	}
	var best []scored
	topScore := 0

	for _, seed := range cats.Principles {
		score := 0
		for _, c := range seed.AppliesWhen {
			if conds.has(c) {
				score++
			}
		}
		// An attempted action naming one of the seed's forbidden moves is a
		// direct hit: the pack has already said this move is not allowed under
		// this rule.
		if obs.ActionType != "" {
			for _, fm := range seed.ForbiddenMoves {
				if forbiddenMoveCoversAction(cats, fm, obs.ActionType) {
					score += 2
					break
				}
			}
		}
		if score == 0 {
			continue
		}
		switch {
		case score > topScore:
			topScore, best = score, []scored{{seed, score}}
		case score == topScore:
			best = append(best, scored{seed, score})
		}
	}

	if len(best) != 1 {
		// Zero matches, or a tie the pack never broke.
		return PrincipleSeed{}, false
	}
	return best[0].seed, true
}

// forbiddenMoveCoversAction reports whether a forbidden-move ref names the
// attempted action. It matches on the catalog entry's declared action alias,
// never on substring resemblance between a ref and an action type — two
// unrelated ids sharing a prefix is a naming coincidence, not a governance
// statement.
//
// No pack declares an alias today, so this path is currently inert for
// cluster_operator: its forbidden moves carry reason/recommended_behavior/
// safe_next_step but no action key. That is stated rather than hidden, because
// a scoring branch that silently always contributes zero looks like coverage it
// does not provide. #249 gap 4's scylla.data.wipe rule is the first expected
// user; until a pack declares an alias, matching is by condition alone.
func forbiddenMoveCoversAction(cats Catalogs, moveRef, actionType string) bool {
	for _, fm := range cats.ForbiddenMoves {
		if fm.ID != moveRef {
			continue
		}
		for _, key := range []string{"action", "action_type", "action_alias"} {
			if fm.Fields[key] == actionType {
				return true
			}
		}
		return false
	}
	return false
}

// CandidateTemplateFor resolves the template for an observation, preferring a
// pack's explicit LearningSource over catalog matching.
//
// An explicit seed id that does not resolve in the pack's own catalogs is
// REFUSED rather than fallen back on. A pack naming a seed it does not have is
// a defect in the pack, and quietly substituting a catalog match would hide it
// behind a plausible-looking candidate.
func CandidateTemplateFor(d Domain, obs LearningObservation) (PrincipleSeed, bool) {
	cats := d.Catalogs()
	if ls, ok := d.(LearningSource); ok {
		if seedID, has := ls.CandidateTemplate(obs); has {
			for _, seed := range cats.Principles {
				if seed.ID == seedID {
					return seed, true
				}
			}
			return PrincipleSeed{}, false
		}
	}
	return MatchCandidateTemplate(cats, obs)
}

type stringSet map[string]struct{}

func newStringSet(in []string) stringSet {
	s := make(stringSet, len(in))
	for _, v := range in {
		s[v] = struct{}{}
	}
	return s
}

func (s stringSet) has(v string) bool {
	_, ok := s[v]
	return ok
}

// SeedIDs returns every seed principle id in a catalog set, sorted. Used by
// tests and diagnostics that need a stable enumeration.
func SeedIDs(cats Catalogs) []string {
	ids := make([]string, 0, len(cats.Principles))
	for _, p := range cats.Principles {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	return ids
}
