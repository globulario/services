package interceptors

import (
	"testing"
	"time"
)

// TestPermCacheGrantTTLIsBounded is the ratchet for
// meta.authorization_check_is_a_snapshot_not_a_promise. The permission-decision
// cache is per-service and is NOT invalidated when a binding is revoked, so the
// GRANT TTL is the upper bound on how long a revoked permission keeps being
// honored across the mesh. It was 15 minutes before 2026-06-09; this gate keeps
// it within the documented bound so the revocation window cannot silently
// regress. Raising the window requires deliberately raising permGrantTTLMaxBound
// (a reviewable change) — or, preferably, implementing cross-service cache
// invalidation so a longer TTL is safe.
func TestPermCacheGrantTTLIsBounded(t *testing.T) {
	// Read into vars so the comparison is not constant-folded away.
	grant := permGrantTTL
	bound := permGrantTTLMaxBound

	if grant > bound {
		t.Errorf("perm-cache GRANT TTL %s exceeds the documented bound %s — a revoked permission "+
			"would keep being honored for %s across the mesh (this cache has no cross-service "+
			"invalidation). Keep the revocation window short, or implement invalidation and raise "+
			"permGrantTTLMaxBound deliberately. See meta.authorization_check_is_a_snapshot_not_a_promise.",
			grant, bound, grant)
	}
	// The fix is real: the window must be far below the pre-fix 15m.
	if grant >= 15*time.Minute {
		t.Errorf("perm-cache GRANT TTL %s is back at/above the pre-fix 15m revocation window", grant)
	}
}
