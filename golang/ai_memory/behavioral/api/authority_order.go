package api

// authority_order.go supplies the canonical trust ordering for
// ObservationAuthorityLevel. It is a generic primitive: the kernel knows that
// TRUTH_PLANE outranks INTERPRETATION, but never which floor a given
// requirement demands — that is domain policy.
//
// WHY THIS EXISTS AS A FUNCTION
//
// The levels MUST NOT be compared lexically. Alphabetically they sort
//
//	DERIVED_EVIDENCE < DIAGNOSTIC_CLAIM < EVENT_STREAM < INTERPRETATION < TRUTH_PLANE
//
// which is close to the reverse of their actual trust order. A string compare
// would let an agent's INTERPRETATION outrank DERIVED_EVIDENCE and satisfy a
// requirement demanding authoritative verification — a self-asserted result
// standing in for an owner-verified one. Explicit ranks make that impossible.

// authorityRank is the trust ordering, lowest to highest. Unspecified is
// deliberately absent: it has no rank, and AuthorityRank reports that.
var authorityRank = map[ObservationAuthorityLevel]int{
	ObservationAuthorityInterpretation: 1, // an agent's reading of something
	ObservationAuthorityEventStream:    2, // something happened, per an emitter
	ObservationAuthorityDiagnostic:     3, // a diagnosing service's claim
	ObservationAuthorityDerived:        4, // computed from observed state
	ObservationAuthorityTruthPlane:     5, // the owner of the truth said so
}

// AuthorityRank returns the trust rank of a level and whether it is known.
//
// ok=false for both the unspecified level and any value not in the model.
// Callers must fail closed on ok=false: an authority the kernel cannot rank is
// not "lowest but valid", it is unrankable, and treating it as the floor would
// let an unrecognised or future level silently satisfy a requirement.
func AuthorityRank(l ObservationAuthorityLevel) (int, bool) {
	r, ok := authorityRank[l]
	return r, ok
}

// AuthorityAtLeast reports whether have meets or exceeds floor.
//
// Fails closed in every ambiguous case:
//   - an unrankable `have` never satisfies any floor
//   - an unrankable `floor` is never satisfiable, because a requirement whose
//     own floor cannot be interpreted must not be waived by default
//
// A floor of Unspecified is therefore NOT "no floor". Express "any authority is
// acceptable" by omitting the check, not by leaving the floor blank — a blank
// floor is far more likely to be an unfinished catalog entry than a deliberate
// decision to accept anything.
func AuthorityAtLeast(have, floor ObservationAuthorityLevel) bool {
	hr, ok := AuthorityRank(have)
	if !ok {
		return false
	}
	fr, ok := AuthorityRank(floor)
	if !ok {
		return false
	}
	return hr >= fr
}
