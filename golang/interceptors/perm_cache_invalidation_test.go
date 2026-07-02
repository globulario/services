package interceptors

import (
	"testing"
	"time"
)

// TestFlushPermCache_SurgicalAndComplete is the ratchet for the cross-service
// invalidation path of meta.authorization_check_is_a_snapshot_not_a_promise.
// On an RBAC change the interceptor flushes its permission-decision cache so a
// revoked binding stops being honored before the TTL expires. The flush must be
// (1) COMPLETE — no permCacheEntry survives, so no stale grant lingers — and
// (2) SURGICAL — the shared cache also holds stream-auth markers, client conns,
// and index ints, which must NOT be dropped.
func TestFlushPermCache_SurgicalAndComplete(t *testing.T) {
	// Seed permission-decision entries (these MUST be flushed).
	putPermCache("perm:a", true, time.Minute)
	putPermCache("perm:b", false, time.Minute)
	// Seed role-binding entries (these MUST also be flushed). A stale empty
	// binding can keep denying a newly granted service principal after Day-0
	// seeds RBAC.
	putCachedRoleBinding("globule-ryzen", []string{})
	// Seed non-permission entries sharing the same cache (these MUST survive).
	cache.Store("stream-uuid-1", struct{}{})
	cache.Store("idx-key", 7)

	flushPermCache()

	if _, ok := getPermCache("perm:a"); ok {
		t.Error("perm entry 'perm:a' survived flush — a stale grant could outlive a revocation")
	}
	if _, ok := getPermCache("perm:b"); ok {
		t.Error("perm entry 'perm:b' survived flush")
	}
	if _, ok := getCachedRoleBinding("globule-ryzen"); ok {
		t.Error("role binding entry survived flush — a stale empty binding could keep denying workflow writes")
	}
	if _, ok := cache.Load("stream-uuid-1"); !ok {
		t.Error("flush dropped a stream-auth marker — flush must be surgical (perm entries only)")
	}
	if _, ok := cache.Load("idx-key"); !ok {
		t.Error("flush dropped a non-permission index entry — flush must be surgical")
	}

	// Cleanup shared state for other tests.
	cache.Delete("stream-uuid-1")
	cache.Delete("idx-key")
}
