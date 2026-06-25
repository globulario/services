// @awareness namespace=globular.platform
// @awareness component=platform_rbac.perm_cache_invalidation
// @awareness file_role=cross_instance_rbac_permission_cache_invalidation_watcher
// @awareness implements=globular.platform:invariant.meta.authorization_check_is_a_snapshot_not_a_promise
// @awareness risk=high
//
// Cross-INSTANCE invalidation for the RBAC server's own permission cache.
//
// srv.cache is a per-instance, in-memory BigCache mirror of ScyllaDB permission
// data (server.go documents it as the one allowed Scylla cache, kept on a short
// 30s TTL "so permission changes propagate across replicas"). On a mutation the
// handling instance does a LOCAL srv.cache.RemoveItem(path) and bumps the etcd
// generation key. The bump is consumed by every service's INTERCEPTOR perm-
// decision cache (interceptors/perm_cache_invalidation.go) — but NOT by peer rbac
// instances' srv.cache, so a peer keeps serving the stale permission entry until
// its TTL expires (up to 30s of decisions on stale permissions after a write
// handled elsewhere).
//
// This watcher closes that window using the SAME generation signal — no new write
// path is added: each rbac instance watches permCacheGenerationKey and flushes its
// own srv.cache on change, so a mutation handled by ANY instance invalidates the
// permission cache on ALL instances in ~one etcd round-trip. The 30s TTL remains
// the backstop if the watch is down (degrade, don't fail). Flush is coarse (whole
// cache) because the generation value is opaque; over-invalidation only forces a
// re-read from the authoritative Scylla store, never a stale allow
// (meta.authorization_check_is_a_snapshot_not_a_promise — deny/allow is always
// re-evaluated from source on the next check).
package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// flushPermissionCache drops every entry from this instance's permission cache so
// the next GetResourcePermissions/ValidateAccess re-reads from the authoritative
// store. Safe to call at any time; a concurrent read just repopulates with a fresh
// (re-validated) entry. Errors are logged and absorbed — invalidation must never
// fail an RPC.
func (srv *server) flushPermissionCache() {
	if srv.cache == nil {
		return
	}
	if err := srv.cache.Clear(); err != nil {
		slog.Warn("rbac: failed to flush permission cache on RBAC change", "err", err)
		return
	}
	slog.Info("rbac: flushed permission cache on cross-instance RBAC change")
}

// runPermCacheInvalidationWatcher reconnects the generation-key watch with capped
// backoff until ctx is cancelled. A dropped watch degrades to TTL-bounded
// staleness, never a hard failure. Mirrors the interceptor and acc watchers.
func (srv *server) runPermCacheInvalidationWatcher(ctx context.Context) {
	backoff := 5 * time.Second
	const maxBackoff = 5 * time.Minute
	for {
		if err := srv.watchPermCacheGenerationOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("rbac: perm-cache invalidation watcher stopped, will retry",
				"err", err, "backoff", backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// watchPermCacheGenerationOnce watches permCacheGenerationKey until the watch
// closes or ctx is cancelled, flushing srv.cache on every change event. Returns
// on watch close/error so the caller can reconnect.
func (srv *server) watchPermCacheGenerationOnce(ctx context.Context) error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()

	wch := cli.Watch(wctx, permCacheGenerationKey)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case wr, ok := <-wch:
			if !ok {
				return clientv3.ErrNoAvailableEndpoints
			}
			if wr.Err() != nil {
				return wr.Err()
			}
			// Any event means a permission/binding changed somewhere; flush so the
			// next check re-reads from Scylla rather than serving a stale entry.
			if len(wr.Events) > 0 {
				srv.flushPermissionCache()
			}
		}
	}
}
