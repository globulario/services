package config

import (
	"context"
	"sync"

	"github.com/globulario/services/golang/internal/depcache"
)

var (
	hostCacheOnce sync.Once
	hostCache     *depcache.Cache[string, []string]
)

// getHostCache returns the package-level cluster host list cache, creating it
// on first call. The cache wraps LoadClusterHostListFromEtcd with
// PolicyStableHosts (30s TTL, 120s stale-if-error).
//
// Host lists (Scylla, MinIO, DNS) are written once during cluster bootstrap and
// only change when nodes join or leave — a 30s stale window is always safe.
// The 120s stale-if-error window shields all 18+ callers from etcd blips during
// Scylla reconnect storms or rolling restarts.
func getHostCache() *depcache.Cache[string, []string] {
	hostCacheOnce.Do(func() {
		hostCache = depcache.New(
			depcache.PolicyStableHosts,
			func(ctx context.Context, etcdKey string) ([]string, error) {
				return loadClusterHostListFromEtcd(ctx, etcdKey)
			},
			copyStringSlice,
		)
	})
	return hostCache
}

// StartClusterHostsWatcher starts an etcd watch on /globular/cluster/ that
// invalidates the host list cache when any host list changes. Long-running
// services can call this once to get sub-TTL invalidation.
func StartClusterHostsWatcher(ctx context.Context) error {
	c, err := GetEtcdClient()
	if err != nil {
		return err
	}
	cache := getHostCache()
	w := depcache.NewWatchInvalidator(
		c,
		"/globular/cluster/",
		func(_ string) { cache.InvalidateAll() },
		cache.InvalidateAll,
	)
	w.Start(ctx)
	return nil
}

// copyStringSlice returns an independent copy of s.
// Strings are immutable in Go so a shallow slice copy is a full deep copy.
func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}
