package main

// placement_generation.go — D1c step 1a: controller-owned per-node
// placement-intent generation.
//
// PlacementGeneration versions the EFFECTIVE authorized-placement set for a node
// so that a stale grant snapshot can never authorize an install after the
// controller has advanced placement (the replay fence completed in D1c 1c). The
// invariant copied from the objectstore IntentGeneration pattern is not merely
// the field shape: a generation is authoritative only when the CONTROLLER owns
// and advances it, and consumers learn the current value through a channel
// INDEPENDENT of the artifact being validated (the node-agent sync path, 1b).
//
// Rules (contract §10 D1c-1a):
//   - New nodes are initialized to 1 at creation (established placement).
//   - Legacy nodes load as 0 = unestablished; a 0 generation can NEVER authorize
//     explicit grants.
//   - Bumped ONLY on a real, set-level placement change (idempotent re-applies do
//     not bump), atomically with the profile write (same nodeState, same persist).
//   - EVERY profile-mutation entry point routes through
//     applyNodePlacementProfilesLocked — no handler increments independently.

// applyNodePlacementProfilesLocked is the SINGLE owner mutation for a node's
// placement profiles. It normalizes rawProfiles and, only when the effective
// (order/dup-independent) profile set actually changes, sets the new profiles
// and atomically bumps PlacementGeneration. Idempotent re-applies (same set, any
// order) leave both Profiles and PlacementGeneration untouched. The caller MUST
// hold srv.lock. Returns whether the effective placement changed (and thus
// whether the generation advanced) — the coupling is absolute: a changed
// placement always advances the generation, and the generation never advances
// without a placement change.
func applyNodePlacementProfilesLocked(node *nodeState, rawProfiles []string) bool {
	if node == nil {
		return false
	}
	normalized := normalizeProfiles(rawProfiles)
	if profilesSameSet(node.Profiles, normalized) {
		return false // effective placement unchanged — no bump, no write
	}
	node.Profiles = normalized
	node.PlacementGeneration++
	if node.PlacementGeneration == 0 {
		// Never land on the "unestablished" sentinel after a real mutation. A
		// uint64 wrap is astronomically unlikely, but the invariant is absolute:
		// a node whose placement just changed is, by definition, established.
		node.PlacementGeneration = 1
	}
	return true
}

// placementFreshnessEstablished reports whether the node has an established
// placement generation. A zero generation (legacy state loaded before the field
// existed, or a node whose placement was never established) can NEVER authorize
// explicit grants — the node-agent fails closed to profile-only (contract
// §10 D1c-1c).
func placementFreshnessEstablished(node *nodeState) bool {
	return node != nil && node.PlacementGeneration != 0
}
