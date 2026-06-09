package main

import (
	"context"
	"strconv"
	"time"

	"github.com/globulario/services/golang/config"
)

// permCacheGenerationKey mirrors interceptors.PermCacheGenerationKey. The RBAC
// service bumps it on every permission/binding mutation; every service's
// interceptor watches it and flushes its permission-decision cache on change,
// so a revoked binding takes effect mesh-wide in ~one etcd round-trip instead of
// waiting out the interceptor cache TTL. See
// meta.authorization_check_is_a_snapshot_not_a_promise.
//
// Best-effort BY DESIGN: this is cache invalidation, not an authorization
// decision, so a failed bump is absorbed (degrade to TTL-bounded staleness) and
// must never block or fail the mutation it follows — the mutation already
// succeeded, and the interceptor cache TTL is the safety backstop. This is the
// inverse of the auth path, where connection errors must surface.
const permCacheGenerationKey = "/globular/system/rbac/generation"

// bumpPermCacheGeneration writes a fresh value to the generation key. The value
// is opaque — only the change matters — so a monotonic wall-clock nanosecond is
// sufficient to guarantee the watched key changes on every mutation.
func (srv *server) bumpPermCacheGeneration() {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return // degrade to TTL; do not block the mutation
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = cli.Put(ctx, permCacheGenerationKey, strconv.FormatInt(time.Now().UnixNano(), 10))
}
