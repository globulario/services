// Package depcache provides a generic read-through cache with singleflight
// coalescing for shielding control-plane dependencies (etcd, ScyllaDB, MinIO)
// from thundering-herd reads during reconcile storms.
//
// Policy contract:
//
//   TTL > 0     — cache successful results; return without calling fetch until TTL elapses.
//   TTL == 0    — no positive-result caching; fetch is called on every Get.
//               Successful results are still stored so stale-if-error can serve them on failure.
//   StaleIfError > 0 — on fetch failure, serve a prior result if it was stored within that window.
//   StaleIfError == 0 — return fetch errors directly; no stale value served.
//
// Install and release resolver paths MUST use PolicyNoStale (TTL=0, StaleIfError=0).
// Returning a stale lifecycle state (e.g. YANKED artifact as PUBLISHED) from those
// paths is a correctness bug that can cause wrong software to be installed.
package depcache

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Policy defines caching and stale-serving behavior for a key class.
type Policy struct {
	TTL          time.Duration
	StaleIfError time.Duration
}

// Named policies — always use these; never construct Policy literals in call sites.
var (
	// PolicyHotConfig: service config for display, routing, health (CLI, MCP, health checks).
	PolicyHotConfig = Policy{TTL: 5 * time.Second, StaleIfError: 60 * time.Second}

	// PolicyStableHosts: host lists (Scylla, etcd peers) that change rarely.
	PolicyStableHosts = Policy{TTL: 30 * time.Second, StaleIfError: 120 * time.Second}

	// PolicyRepositoryResolver: install/release resolver path.
	// TTL=0 + StaleIfError=0 — never cache, never serve stale.
	// Use for any path that may dispatch an install or act on artifact lifecycle state.
	PolicyRepositoryResolver = Policy{}

	// PolicyRepositoryListView: repository listing for display (MCP, CLI) only.
	// Stale results are acceptable for display; DO NOT use for install dispatch.
	PolicyRepositoryListView = Policy{TTL: 10 * time.Second, StaleIfError: 30 * time.Second}

	// PolicyMinioSentinel: MinIO availability probe (metadata only).
	// Shields repeated probes. Does NOT imply artifact blobs are readable.
	PolicyMinioSentinel = Policy{TTL: 30 * time.Second, StaleIfError: 60 * time.Second}

	// PolicyMinioStat: MinIO object stat calls (metadata only).
	// Shields repeated stat probes. Does NOT imply blob transfer will succeed.
	PolicyMinioStat = Policy{TTL: 10 * time.Second, StaleIfError: 30 * time.Second}

	// PolicyNoStale is identical to PolicyRepositoryResolver.
	// Use wherever serving cached data would be incorrect.
	PolicyNoStale = Policy{}
)

// Invalidatable is implemented by any cache that supports bulk invalidation.
// Used by WatchInvalidator to clear stale entries after a watch failure.
type Invalidatable interface {
	InvalidateAll()
}

type entry[V any] struct {
	value     V
	fetchedAt time.Time
}

// Cache is a generic read-through cache with singleflight coalescing.
// Safe for concurrent use.
//
// K must be comparable. If V contains maps, slices, or pointer fields,
// copyFn must be non-nil to prevent callers from mutating the stored entry.
// Pass nil only for immutable V (string, int, plain value structs).
type Cache[K comparable, V any] struct {
	policy Policy
	fetch  func(ctx context.Context, key K) (V, error)
	copyFn func(V) V
	bypass bool // set from GLOBULAR_DISABLE_DEPCACHE or in tests

	mu      sync.RWMutex
	entries map[K]*entry[V]
	sf      singleflight.Group
}

// New creates a Cache with the given policy and fetch function.
//
// copyFn is called on every returned value to prevent mutation of the stored entry.
// Pass nil only if V is immutable.
//
// If GLOBULAR_DISABLE_DEPCACHE=true, all Gets call fetch directly;
// nothing is stored or returned from cache.
func New[K comparable, V any](
	policy Policy,
	fetch func(ctx context.Context, key K) (V, error),
	copyFn func(V) V,
) *Cache[K, V] {
	return &Cache[K, V]{
		policy:  policy,
		fetch:   fetch,
		copyFn:  copyFn,
		bypass:  os.Getenv("GLOBULAR_DISABLE_DEPCACHE") == "true",
		entries: make(map[K]*entry[V]),
	}
}

// Get returns the value for key, obeying the cache policy.
//
// Bypass: always calls fetch; nothing stored or returned from cache.
// TTL > 0 and fresh entry exists: returned without calling fetch.
// TTL == 0: always calls fetch; may serve prior value on error if StaleIfError > 0.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, error) {
	if c.bypass {
		return c.fetch(ctx, key)
	}

	// Fast path: fresh positive-cache entry (TTL > 0 only).
	if c.policy.TTL > 0 {
		if v, ok := c.loadWithin(key, c.policy.TTL); ok {
			return v, nil
		}
	}

	// Coalesce concurrent fetches for the same key.
	sfKey := fmt.Sprintf("%v", key)
	resCh := c.sf.DoChan(sfKey, func() (interface{}, error) {
		v, err := c.fetch(ctx, key)
		if err == nil {
			// Store on success for stale-if-error use, even when TTL=0.
			c.mu.Lock()
			c.entries[key] = &entry[V]{value: v, fetchedAt: time.Now()}
			c.mu.Unlock()
		}
		return v, err
	})

	select {
	case res := <-resCh:
		if res.Err == nil {
			return c.maybeCopy(res.Val.(V)), nil
		}
		// Fetch failed — serve stale if policy allows.
		if c.policy.StaleIfError > 0 {
			if v, ok := c.loadWithin(key, c.policy.StaleIfError); ok {
				return v, nil
			}
		}
		var zero V
		return zero, res.Err
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// Invalidate removes the cached entry for key. The next Get will call fetch.
func (c *Cache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// InvalidateAll clears all cached entries. The next Get for any key will call fetch.
func (c *Cache[K, V]) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[K]*entry[V])
	c.mu.Unlock()
}

// loadWithin returns the stored value if it exists and was fetched within maxAge.
func (c *Cache[K, V]) loadWithin(key K, maxAge time.Duration) (V, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Since(e.fetchedAt) >= maxAge {
		var zero V
		return zero, false
	}
	return c.maybeCopy(e.value), true
}

func (c *Cache[K, V]) maybeCopy(v V) V {
	if c.copyFn != nil {
		return c.copyFn(v)
	}
	return v
}
