// perm_cache_invalidation_test.go: regression for cross-instance permission-cache
// invalidation. The watcher reacts to the etcd generation key (integration-level,
// mirrors the untested-by-unit interceptor watcher); this test locks the flush
// CONTRACT it relies on — flushPermissionCache drops every entry so a peer
// instance cannot keep serving permissions cached before a mutation elsewhere.
package main

import (
	"testing"
)

// TestFlushPermissionCache_ClearsAllEntries proves the flush invoked on every
// generation-key change empties the per-instance permission cache, so the next
// GetResourcePermissions/ValidateAccess re-reads from the authoritative store
// rather than serving an entry cached before a cross-instance mutation.
func TestFlushPermissionCache_ClearsAllEntries(t *testing.T) {
	srv := newTestServer(t)

	for _, p := range []string{"/a/b", "/a/c", "/d"} {
		if err := srv.cache.SetItem(p, []byte("perms")); err != nil {
			t.Fatalf("seed cache item %q: %v", p, err)
		}
	}
	// Sanity: an entry is served before the flush.
	if v, err := srv.cache.GetItem("/a/b"); err != nil || len(v) == 0 {
		t.Fatalf("expected /a/b to be cached before flush (v=%q err=%v)", v, err)
	}

	srv.flushPermissionCache()

	for _, p := range []string{"/a/b", "/a/c", "/d"} {
		if v, err := srv.cache.GetItem(p); err == nil && len(v) > 0 {
			t.Errorf("after flush, %q must not be served from cache; got %q", p, v)
		}
	}
}

// TestFlushPermissionCache_NilCacheSafe guards the degrade path: flushing must
// never panic, even if the cache was never opened (invalidation is best-effort
// and must never fail an RPC).
func TestFlushPermissionCache_NilCacheSafe(t *testing.T) {
	srv := &server{}
	srv.flushPermissionCache() // must not panic
}
