package config

import (
	"context"
	"sync"

	"github.com/globulario/services/golang/internal/depcache"
)

var (
	svcCacheOnce sync.Once
	svcCache     *depcache.Cache[string, []map[string]interface{}]
)

// getServiceCache returns the package-level service config cache, creating it
// on first call. The cache wraps fetchServicesFromEtcd with PolicyHotConfig
// (5s TTL, 60s stale-if-error) so reconcile storms do not hammer etcd with a
// full /globular/services/ prefix scan on every tick.
func getServiceCache() *depcache.Cache[string, []map[string]interface{}] {
	svcCacheOnce.Do(func() {
		svcCache = depcache.New(
			depcache.PolicyHotConfig,
			func(ctx context.Context, _ string) ([]map[string]interface{}, error) {
				return fetchServicesFromEtcd(ctx)
			},
			deepCopyServiceList,
		)
	})
	return svcCache
}

// StartServiceConfigWatcher starts an etcd watch on /globular/services/ that
// immediately invalidates the service config cache on any PUT or DELETE, so
// writes are visible in the next Get without waiting for the 5s TTL to expire.
//
// Should be called once by long-running services (controller, node-agent) that
// need sub-TTL freshness. The watcher runs until ctx is cancelled.
func StartServiceConfigWatcher(ctx context.Context) error {
	c, err := etcdClient()
	if err != nil {
		return err
	}
	cache := getServiceCache()
	w := depcache.NewWatchInvalidator(
		c,
		etcdPrefix,
		func(_ string) { cache.InvalidateAll() },
		cache.InvalidateAll,
	)
	w.Start(ctx)
	return nil
}

// deepCopyServiceList returns an independent copy of src so that callers
// cannot mutate the maps stored in the cache.
//
// Top-level map values are copied shallowly. This is sufficient because service
// config values are strings, numbers, and booleans — no nested mutable types.
func deepCopyServiceList(src []map[string]interface{}) []map[string]interface{} {
	if src == nil {
		return nil
	}
	out := make([]map[string]interface{}, len(src))
	for i, m := range src {
		cp := make(map[string]interface{}, len(m))
		for k, v := range m {
			cp[k] = v
		}
		out[i] = cp
	}
	return out
}
