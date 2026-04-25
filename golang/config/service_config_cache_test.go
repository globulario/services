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

// resetSvcCacheWith replaces the package-level service config cache singleton
// with a new one using policy p and fetch function fetchFn.
// The caller must call restoreSvcCache via t.Cleanup.
func resetSvcCacheWith(
	p depcache.Policy,
	fetchFn func(ctx context.Context, _ string) ([]map[string]interface{}, error),
) {
	svcCache = depcache.New(p, fetchFn, deepCopyServiceList)
	svcCacheOnce = sync.Once{}
	svcCacheOnce.Do(func() {}) // mark done so getServiceCache returns svcCache as-is
}

// resetSvcCache is a convenience wrapper using the production PolicyHotConfig.
func resetSvcCache(fetchFn func(ctx context.Context, _ string) ([]map[string]interface{}, error)) {
	resetSvcCacheWith(depcache.PolicyHotConfig, fetchFn)
}

func restoreSvcCache() {
	svcCache = nil
	svcCacheOnce = sync.Once{}
}

// TestServiceConfigCacheHit verifies that a second GetServicesConfigurations
// call within the TTL window is served from cache without calling etcd again.
func TestServiceConfigCacheHit(t *testing.T) {
	var calls atomic.Int32
	resetSvcCache(func(_ context.Context, _ string) ([]map[string]interface{}, error) {
		calls.Add(1)
		return []map[string]interface{}{{"Id": "svc1", "Name": "test"}}, nil
	})
	t.Cleanup(restoreSvcCache)

	got1, err := GetServicesConfigurations()
	if err != nil || len(got1) != 1 {
		t.Fatalf("first call: (%v, %v)", got1, err)
	}
	got2, err := GetServicesConfigurations()
	if err != nil || len(got2) != 1 {
		t.Fatalf("second call: (%v, %v)", got2, err)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 fetch within TTL, got %d", calls.Load())
	}
}

// TestServiceConfigCacheInvalidatedOnWrite verifies that after a write-path
// invalidation (simulating SaveServiceConfiguration / DeleteServiceConfiguration),
// the next GetServicesConfigurations re-fetches.
func TestServiceConfigCacheInvalidatedOnWrite(t *testing.T) {
	var calls atomic.Int32
	resetSvcCache(func(_ context.Context, _ string) ([]map[string]interface{}, error) {
		calls.Add(1)
		return []map[string]interface{}{{"Id": "svc1"}}, nil
	})
	t.Cleanup(restoreSvcCache)

	if _, err := GetServicesConfigurations(); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call to warm, got %d", calls.Load())
	}

	// Simulate what SaveServiceConfiguration / DeleteServiceConfiguration do.
	getServiceCache().InvalidateAll()

	if _, err := GetServicesConfigurations(); err != nil {
		t.Fatalf("after invalidate: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetch calls after invalidation, got %d", calls.Load())
	}
}

// TestServiceConfigCacheWatchInvalidation verifies that the onEvent callback
// (called by WatchInvalidator for every /globular/services/ change) causes the
// next read to re-fetch rather than returning the TTL-cached value.
func TestServiceConfigCacheWatchInvalidation(t *testing.T) {
	var calls atomic.Int32
	resetSvcCache(func(_ context.Context, _ string) ([]map[string]interface{}, error) {
		n := calls.Add(1)
		return []map[string]interface{}{{"Id": "svc", "Seq": n}}, nil
	})
	t.Cleanup(restoreSvcCache)

	if _, err := GetServicesConfigurations(); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 fetch, got %d", calls.Load())
	}

	// Simulate what WatchInvalidator.onEvent does on a key change.
	getServiceCache().InvalidateAll()

	if _, err := GetServicesConfigurations(); err != nil {
		t.Fatalf("after watch event: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetches after watch-driven invalidation, got %d", calls.Load())
	}
}

// TestServiceConfigCacheStaleOnEtcdError verifies that when etcd becomes
// unavailable, GetServicesConfigurations returns the last successful result
// rather than an error (within the StaleIfError window).
//
// We use TTL=0 so every Get calls fetch directly — no TTL backdating needed
// to trigger the stale-if-error path. The stale-if-error behaviour is identical
// regardless of TTL; TTL only affects the fast path.
func TestServiceConfigCacheStaleOnEtcdError(t *testing.T) {
	var failFetch atomic.Bool
	// Policy: no positive caching (TTL=0), but serve stale for 30s on error.
	resetSvcCacheWith(
		depcache.Policy{TTL: 0, StaleIfError: 30 * time.Second},
		func(_ context.Context, _ string) ([]map[string]interface{}, error) {
			if failFetch.Load() {
				return nil, errors.New("etcd unavailable")
			}
			return []map[string]interface{}{{"Id": "svc1", "Name": "stable"}}, nil
		},
	)
	t.Cleanup(restoreSvcCache)

	// First Get: fetch succeeds; entry stored for stale-if-error use.
	got, err := GetServicesConfigurations()
	if err != nil || len(got) != 1 || got[0]["Id"] != "svc1" {
		t.Fatalf("warm: (%v, %v)", got, err)
	}

	// Now etcd goes down.
	failFetch.Store(true)

	// TTL=0 means fetch is called again; it fails. StaleIfError serves prior result.
	got, err = GetServicesConfigurations()
	if err != nil {
		t.Errorf("expected stale result when etcd is down, got error: %v", err)
	}
	if len(got) == 0 || got[0]["Id"] != "svc1" {
		t.Errorf("expected stale svc1, got %v", got)
	}
}

// TestServiceConfigCacheDeepCopy verifies that mutating the map returned by
// GetServicesConfigurations does not corrupt the cached entry.
func TestServiceConfigCacheDeepCopy(t *testing.T) {
	resetSvcCache(func(_ context.Context, _ string) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"Id": "svc1", "Name": "original"}}, nil
	})
	t.Cleanup(restoreSvcCache)

	got1, _ := GetServicesConfigurations()
	got1[0]["Name"] = "mutated" // mutate the returned copy

	// Within TTL — cache must return "original", not "mutated".
	got2, _ := GetServicesConfigurations()
	if got2[0]["Name"] != "original" {
		t.Errorf("deepCopy must protect stored entry; got Name=%q after caller mutation", got2[0]["Name"])
	}
}
