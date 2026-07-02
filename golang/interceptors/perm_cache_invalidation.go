// @awareness namespace=globular.platform
// @awareness component=platform.interceptors.perm_cache_invalidation
// @awareness file_role=cross_service_permission_cache_invalidation_watcher
// @awareness implements=globular.platform:invariant.meta.authorization_check_is_a_snapshot_not_a_promise
// @awareness risk=high
//
// Cross-service invalidation for the interceptor permission-decision cache.
//
// The perm cache (ServerInterceptors.go) is per-service and TTL-bounded
// (permGrantTTL). Without invalidation, a revoked binding keeps being honored
// until the cached grant expires — see
// meta.authorization_check_is_a_snapshot_not_a_promise. This watcher closes that
// window: the RBAC service bumps PermCacheGenerationKey in etcd on every
// permission/binding mutation; every service's interceptor watches that key and
// flushes its perm cache on change, so a revocation takes effect mesh-wide in
// ~one etcd round-trip instead of waiting out the TTL. The TTL remains the
// backstop if the watch is down (degrade, don't fail). Flush is coarse (all perm
// entries) — over-invalidation only causes a re-check, never a stale allow.
package interceptors

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// PermCacheGenerationKey is bumped by the RBAC service on any permission or
// role-binding mutation. Its value is opaque; only the fact that it CHANGED
// matters. Interceptors watch it and flush the perm cache on change.
const PermCacheGenerationKey = "/globular/system/rbac/generation"

var permWatchOnce sync.Once

// ensurePermCacheWatcher lazily starts the invalidation watcher the first time
// the perm cache is written, so only services that actually cache permission
// decisions pay for the watch (mirrors the acc config-watcher lifecycle).
func ensurePermCacheWatcher() {
	permWatchOnce.Do(func() {
		go runPermCacheWatcher()
	})
}

// flushPermCache drops every authorization cache entry derived from RBAC state.
// It is surgical for the shared permission cache: the cache also holds
// stream-auth markers, client conns, and index ints, so it deletes ONLY values
// of type permCacheEntry. Role bindings live in their own cache and must be
// cleared too; otherwise a bootstrap-time empty binding can keep denying a newly
// seeded service principal until roleBindingTTL expires.
func flushPermCache() {
	n := 0
	cache.Range(func(k, v any) bool {
		if _, ok := v.(permCacheEntry); ok {
			cache.Delete(k)
			n++
		}
		return true
	})
	roleN := 0
	roleBindingCache.Range(func(k, _ any) bool {
		roleBindingCache.Delete(k)
		roleN++
		return true
	})
	if n > 0 || roleN > 0 {
		slog.Info("interceptors: flushed authorization caches on RBAC change",
			"permission_entries", n,
			"role_binding_entries", roleN)
	}
}

// runPermCacheWatcher reconnects the watch with capped backoff, mirroring the
// acc config watcher. A dropped watch degrades to TTL-bounded staleness, never
// a hard failure.
func runPermCacheWatcher() {
	backoff := 5 * time.Second
	const maxBackoff = 5 * time.Minute
	for {
		if err := watchPermCacheGeneration(); err != nil {
			slog.Warn("interceptors: perm-cache invalidation watcher stopped, will retry",
				"err", err, "backoff", backoff)
		}
		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// watchPermCacheGeneration watches PermCacheGenerationKey and flushes the perm
// cache on every change event. Returns on watch close/error so the caller can
// reconnect.
func watchPermCacheGeneration() error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	wctx, wcancel := context.WithCancel(context.Background())
	defer wcancel()

	wch := cli.Watch(wctx, PermCacheGenerationKey)
	for {
		wr, ok := <-wch
		if !ok {
			return clientv3.ErrNoAvailableEndpoints
		}
		if wr.Err() != nil {
			return wr.Err()
		}
		// Any event on the key means a permission/binding changed somewhere;
		// flush so the next check re-validates against RBAC.
		if len(wr.Events) > 0 {
			flushPermCache()
		}
	}
}
