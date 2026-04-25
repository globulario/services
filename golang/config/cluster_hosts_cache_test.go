package config

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/globulario/services/golang/internal/depcache"
)

func resetHostCache(fetchFn func(ctx context.Context, key string) ([]string, error)) {
	hostCache = depcache.New(depcache.PolicyStableHosts, fetchFn, copyStringSlice)
	hostCacheOnce = sync.Once{}
	hostCacheOnce.Do(func() {})
}

func resetHostCacheWith(p depcache.Policy, fetchFn func(ctx context.Context, key string) ([]string, error)) {
	hostCache = depcache.New(p, fetchFn, copyStringSlice)
	hostCacheOnce = sync.Once{}
	hostCacheOnce.Do(func() {})
}

func restoreHostCache() {
	hostCache = nil
	hostCacheOnce = sync.Once{}
}

// TestScyllaHostsCacheHit verifies that repeated calls to GetScyllaHosts within
// the TTL window are served from cache without hitting etcd again.
func TestScyllaHostsCacheHit(t *testing.T) {
	var calls atomic.Int32
	resetHostCache(func(_ context.Context, key string) ([]string, error) {
		calls.Add(1)
		return []string{"10.0.0.1", "10.0.0.2"}, nil
	})
	t.Cleanup(restoreHostCache)

	h1, err := GetScyllaHosts()
	if err != nil || len(h1) != 2 {
		t.Fatalf("first call: (%v, %v)", h1, err)
	}
	h2, err := GetScyllaHosts()
	if err != nil || len(h2) != 2 {
		t.Fatalf("second call: (%v, %v)", h2, err)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 etcd fetch within TTL, got %d", calls.Load())
	}
}

// TestMinioHostsCacheHit verifies GetMinioHosts is also cached.
func TestMinioHostsCacheHit(t *testing.T) {
	var calls atomic.Int32
	resetHostCache(func(_ context.Context, key string) ([]string, error) {
		calls.Add(1)
		return []string{"10.0.0.3"}, nil
	})
	t.Cleanup(restoreHostCache)

	if _, err := GetMinioHosts(); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := GetMinioHosts(); err != nil {
		t.Fatalf("second: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 fetch for MinIO hosts, got %d", calls.Load())
	}
}

// TestScyllaHostsCacheStaleOnEtcdDown verifies that when etcd becomes unavailable,
// GetScyllaHosts returns the last known host list rather than an error (within
// the StaleIfError window). This prevents Scylla reconnect storms from also
// spiking etcd reads.
//
// Uses TTL=0 so every Get calls fetch; stale-if-error is tested cleanly without
// backdating entries.
func TestScyllaHostsCacheStaleOnEtcdDown(t *testing.T) {
	var etcdDown atomic.Bool
	resetHostCacheWith(
		depcache.Policy{TTL: 0, StaleIfError: 120 * time.Second},
		func(_ context.Context, key string) ([]string, error) {
			if etcdDown.Load() {
				return nil, errors.New("etcd unavailable")
			}
			return []string{"10.0.0.1", "10.0.0.2"}, nil
		},
	)
	t.Cleanup(restoreHostCache)

	// Warm the cache.
	got, err := GetScyllaHosts()
	if err != nil || len(got) != 2 {
		t.Fatalf("warm: (%v, %v)", got, err)
	}

	// etcd goes down.
	etcdDown.Store(true)

	// TTL=0 triggers a fresh fetch that fails. Stale hosts should be returned.
	got, err = GetScyllaHosts()
	if err != nil {
		t.Errorf("expected stale hosts when etcd is down, got error: %v", err)
	}
	if len(got) != 2 || got[0] != "10.0.0.1" {
		t.Errorf("expected stale [10.0.0.1 10.0.0.2], got %v", got)
	}
}

// TestHostCacheInvalidatedOnSave verifies that SaveClusterHostList invalidates
// the per-key cache entry so the next read re-fetches the updated list.
func TestHostCacheInvalidatedOnSave(t *testing.T) {
	var calls atomic.Int32
	resetHostCache(func(_ context.Context, key string) ([]string, error) {
		calls.Add(1)
		return []string{"10.0.0.1"}, nil
	})
	t.Cleanup(restoreHostCache)

	// Warm.
	if _, err := GetScyllaHosts(); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 fetch, got %d", calls.Load())
	}

	// Simulate the invalidation that SaveClusterHostList does on success.
	getHostCache().Invalidate(EtcdKeyClusterScyllaHosts)

	// Next read must re-fetch.
	if _, err := GetScyllaHosts(); err != nil {
		t.Fatalf("after invalidate: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetches after invalidation, got %d", calls.Load())
	}
}

// TestHostCacheDeepCopy verifies that mutating the slice returned by
// GetScyllaHosts does not corrupt the cached entry.
func TestHostCacheDeepCopy(t *testing.T) {
	resetHostCache(func(_ context.Context, _ string) ([]string, error) {
		return []string{"10.0.0.1", "10.0.0.2"}, nil
	})
	t.Cleanup(restoreHostCache)

	h1, _ := GetScyllaHosts()
	h1[0] = "mutated" // mutate the returned copy

	// Within TTL — cache must return the original value.
	h2, _ := GetScyllaHosts()
	if h2[0] != "10.0.0.1" {
		t.Errorf("copyFn must protect stored entry; got %q after caller mutation", h2[0])
	}
}
