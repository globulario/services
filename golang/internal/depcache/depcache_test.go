package depcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
)

// ── Cache tests ───────────────────────────────────────────────────────────────

func TestCacheTTLExpiry(t *testing.T) {
	var calls atomic.Int32
	c := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		n := calls.Add(1)
		return fmt.Sprintf("v%d", n), nil
	}, nil)

	v1, _ := c.Get(context.Background(), "k")
	if calls.Load() != 1 {
		t.Fatalf("expected 1 fetch call, got %d", calls.Load())
	}

	// Expire the entry by backdating its fetchedAt beyond TTL.
	c.mu.Lock()
	c.entries["k"].fetchedAt = time.Now().Add(-(c.policy.TTL + time.Second))
	c.mu.Unlock()

	v2, _ := c.Get(context.Background(), "k")
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetch calls after TTL expiry, got %d", calls.Load())
	}
	if v1 == v2 {
		t.Errorf("expected a fresh value after TTL expiry, but got same value %q", v1)
	}
}

func TestCacheNoStaleMode(t *testing.T) {
	var calls atomic.Int32
	c := New[string, string](PolicyNoStale, func(_ context.Context, _ string) (string, error) {
		n := calls.Add(1)
		return fmt.Sprintf("v%d", n), nil
	}, nil)

	v1, _ := c.Get(context.Background(), "k")
	v2, _ := c.Get(context.Background(), "k")
	v3, _ := c.Get(context.Background(), "k")

	if calls.Load() != 3 {
		t.Errorf("PolicyNoStale must call fetch on every Get; got %d calls", calls.Load())
	}
	if v1 == v2 || v2 == v3 {
		t.Errorf("expected distinct values per Get, got %q %q %q", v1, v2, v3)
	}
}

func TestCacheStaleIfError(t *testing.T) {
	var fetchErr error
	c := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		if fetchErr != nil {
			return "", fetchErr
		}
		return "value", nil
	}, nil)

	// Warm the cache.
	if _, err := c.Get(context.Background(), "k"); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}

	// Expire the TTL but stay inside the StaleIfError window.
	c.mu.Lock()
	c.entries["k"].fetchedAt = time.Now().Add(-(PolicyHotConfig.TTL + time.Second))
	c.mu.Unlock()

	// Now fetch fails — stale value should be served.
	fetchErr = errors.New("etcd unavailable")
	v, err := c.Get(context.Background(), "k")
	if err != nil {
		t.Errorf("expected stale value on error, got: %v", err)
	}
	if v != "value" {
		t.Errorf("expected stale %q, got %q", "value", v)
	}
}

// TestPolicyNoStale_NoPositiveCache_NoStaleOnError verifies the stricter
// semantics: TTL=0 + StaleIfError=0 means errors are always returned directly,
// even when a prior successful result is stored.
//
// This is the required policy for install and release resolver paths. A YANKED
// artifact must never be returned as PUBLISHED from a stale cache entry.
func TestPolicyNoStale_NoPositiveCache_NoStaleOnError(t *testing.T) {
	var fetchErr error
	var calls atomic.Int32
	c := New[string, string](PolicyNoStale, func(_ context.Context, _ string) (string, error) {
		calls.Add(1)
		if fetchErr != nil {
			return "", fetchErr
		}
		return "published-artifact", nil
	}, nil)

	// First Get: fetch succeeds, result stored internally.
	v, err := c.Get(context.Background(), "k")
	if err != nil || v != "published-artifact" {
		t.Fatalf("first Get: unexpected (%q, %v)", v, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 fetch call, got %d", calls.Load())
	}

	// Simulate artifact being yanked — fetch now returns an error.
	fetchErr = errors.New("artifact lifecycle: YANKED")

	// Second Get: PolicyNoStale must NOT serve the prior "published-artifact" value.
	_, err = c.Get(context.Background(), "k")
	if err == nil {
		t.Error("PolicyNoStale must return the fetch error, not a stale value")
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetch calls, got %d", calls.Load())
	}
}

// TestPolicyZeroTTLWithStaleOnError verifies Policy{TTL:0, StaleIfError:30s}:
// fetch is called on every Get (no positive caching), but a prior successful
// result may be served when fetch fails.
//
// NOTE: this policy must NOT be used for install or release resolver paths.
// It is documented here to show the intended semantics for non-install paths
// that want read-fresh-or-serve-stale behaviour (e.g. host-list reads).
func TestPolicyZeroTTLWithStaleOnError(t *testing.T) {
	p := Policy{TTL: 0, StaleIfError: 30 * time.Second}
	var fetchErr error
	var calls atomic.Int32

	c := New[string, string](p, func(_ context.Context, _ string) (string, error) {
		calls.Add(1)
		if fetchErr != nil {
			return "", fetchErr
		}
		return "v", nil
	}, nil)

	// First Get: fetch succeeds; value stored for stale-if-error use.
	v, err := c.Get(context.Background(), "k")
	if err != nil || v != "v" {
		t.Fatalf("first Get: (%q, %v)", v, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", calls.Load())
	}

	// Second Get: TTL=0, so fetch is called again — no fast path.
	v, err = c.Get(context.Background(), "k")
	if err != nil || v != "v" {
		t.Fatalf("second Get: (%q, %v)", v, err)
	}
	if calls.Load() != 2 {
		t.Errorf("TTL=0 must call fetch on every Get; got %d calls after 2 Gets", calls.Load())
	}

	// Third Get: fetch fails — stale value served from prior successful result.
	fetchErr = errors.New("transient error")
	v, err = c.Get(context.Background(), "k")
	if err != nil {
		t.Errorf("expected stale value on error, got: %v", err)
	}
	if v != "v" {
		t.Errorf("expected stale %q, got %q", "v", v)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 fetch calls, got %d", calls.Load())
	}
}

func TestCacheBypassFlag(t *testing.T) {
	var calls atomic.Int32
	c := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		calls.Add(1)
		return "v", nil
	}, nil)
	c.bypass = true

	for i := 0; i < 5; i++ {
		if _, err := c.Get(context.Background(), "k"); err != nil {
			t.Fatalf("Get %d failed: %v", i, err)
		}
	}
	if calls.Load() != 5 {
		t.Errorf("bypass must call fetch on every Get; got %d calls", calls.Load())
	}

	// Bypass must not store anything in the cache.
	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	if n != 0 {
		t.Errorf("bypass must not store entries; got %d", n)
	}
}

// TestCacheDeepCopyMutation verifies that mutating the value returned by Get
// does not corrupt the entry stored in the cache.
func TestCacheDeepCopyMutation(t *testing.T) {
	c := New[string, map[string]string](
		PolicyHotConfig,
		func(_ context.Context, _ string) (map[string]string, error) {
			return map[string]string{"x": "original"}, nil
		},
		func(m map[string]string) map[string]string {
			cp := make(map[string]string, len(m))
			for k, v := range m {
				cp[k] = v
			}
			return cp
		},
	)

	v1, _ := c.Get(context.Background(), "k")
	v1["x"] = "mutated" // mutate the returned copy

	// Within TTL — should return from cache, but the cached entry must be "original".
	v2, _ := c.Get(context.Background(), "k")
	if v2["x"] != "original" {
		t.Errorf("copyFn must protect stored entry; got %q after caller mutation", v2["x"])
	}
}

func TestCacheSingleflightCoalesce(t *testing.T) {
	var calls atomic.Int32
	c := New[string, string](PolicyNoStale, func(_ context.Context, _ string) (string, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond) // hold the fetch open so goroutines pile up
		return "v", nil
	}, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Get(context.Background(), "k"); err != nil {
				t.Errorf("Get failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Singleflight coalesces concurrent in-flight fetches.
	// With 10 goroutines all starting before the 20ms fetch completes, most
	// should share a single call. Allow a small number for scheduling jitter.
	if calls.Load() > 3 {
		t.Errorf("singleflight should coalesce concurrent Gets; got %d fetch calls", calls.Load())
	}
}

func TestCacheInvalidate(t *testing.T) {
	var calls atomic.Int32
	c := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		n := calls.Add(1)
		return fmt.Sprintf("v%d", n), nil
	}, nil)

	v1, _ := c.Get(context.Background(), "k") // calls=1, stores "v1"
	c.Invalidate("k")
	v2, _ := c.Get(context.Background(), "k") // calls=2, must fetch fresh

	if calls.Load() != 2 {
		t.Errorf("expected 2 fetch calls, got %d", calls.Load())
	}
	if v1 == v2 {
		t.Errorf("expected fresh value after Invalidate, got same value %q", v1)
	}
}

func TestCacheInvalidateAll(t *testing.T) {
	var calls atomic.Int32
	c := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		calls.Add(1)
		return "v", nil
	}, nil)

	c.Get(context.Background(), "k1")
	c.Get(context.Background(), "k2")
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}

	c.InvalidateAll()

	c.Get(context.Background(), "k1") // must re-fetch
	c.Get(context.Background(), "k2") // must re-fetch
	if calls.Load() != 4 {
		t.Errorf("expected 4 calls after InvalidateAll, got %d", calls.Load())
	}
}

// ── WatchInvalidator tests ───────────────────────────────────────────────────

func TestWatchInvalidation(t *testing.T) {
	ch := make(chan clientv3.WatchResponse, 4)

	var calls atomic.Int32
	cache := New[string, string](PolicyHotConfig, func(_ context.Context, _ string) (string, error) {
		n := calls.Add(1)
		return fmt.Sprintf("v%d", n), nil
	}, nil)

	// Warm the cache.
	cache.Get(context.Background(), "mykey")
	if calls.Load() != 1 {
		t.Fatalf("expected 1 initial fetch, got %d", calls.Load())
	}

	w := &WatchInvalidator{
		prefix: "/test/",
		onEvent: func(etcdKey string) {
			// Map the etcd key to the cache key and invalidate.
			cache.Invalidate("mykey")
		},
		onFail:  cache.InvalidateAll,
		watchFn: func(_ context.Context, _ string, _ ...clientv3.OpOption) clientv3.WatchChan { return ch },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	// Send a watch event.
	ch <- clientv3.WatchResponse{
		Events: []*clientv3.Event{
			{Kv: &mvccpb.KeyValue{Key: []byte("/test/mykey")}},
		},
	}
	time.Sleep(30 * time.Millisecond)

	// After invalidation, next Get must re-fetch.
	cache.Get(context.Background(), "mykey")
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetch calls after watch-driven invalidation, got %d", calls.Load())
	}
}

func TestWatchFailureInvalidatesAll(t *testing.T) {
	ch := make(chan clientv3.WatchResponse) // will be closed to simulate failure

	cache := New[string, string](PolicyHotConfig, func(_ context.Context, key string) (string, error) {
		return "v-" + key, nil
	}, nil)

	// Warm two keys.
	cache.Get(context.Background(), "k1")
	cache.Get(context.Background(), "k2")
	cache.mu.RLock()
	before := len(cache.entries)
	cache.mu.RUnlock()
	if before != 2 {
		t.Fatalf("expected 2 cached entries, got %d", before)
	}

	var invalidateAllCalled atomic.Bool
	w := &WatchInvalidator{
		prefix:  "/test/",
		onEvent: nil,
		onFail: func() {
			invalidateAllCalled.Store(true)
			cache.InvalidateAll()
		},
		watchFn:        func(_ context.Context, _ string, _ ...clientv3.OpOption) clientv3.WatchChan { return ch },
		initialBackoff: 10 * time.Second, // prevent restart during test
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	// Close the watch channel to simulate an unexpected failure.
	close(ch)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if !invalidateAllCalled.Load() {
		t.Error("onFail (InvalidateAll) must be called when watch channel closes")
	}

	cache.mu.RLock()
	after := len(cache.entries)
	cache.mu.RUnlock()
	if after != 0 {
		t.Errorf("cache must be empty after InvalidateAll; got %d entries", after)
	}
}

func TestWatchRestartAfterFailure(t *testing.T) {
	var watchFnCalls atomic.Int32
	blockCh := make(chan clientv3.WatchResponse) // never closed — blocks the second watch

	w := &WatchInvalidator{
		prefix:  "/test/",
		onEvent: nil,
		onFail:  func() {},
		watchFn: func(_ context.Context, _ string, _ ...clientv3.OpOption) clientv3.WatchChan {
			n := watchFnCalls.Add(1)
			if n == 1 {
				// First watch: close immediately to trigger restart.
				failCh := make(chan clientv3.WatchResponse)
				close(failCh)
				return failCh
			}
			return blockCh
		},
		initialBackoff: 10 * time.Millisecond, // fast restart for tests
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	// Wait up to 2s for the watch to restart.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if watchFnCalls.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if watchFnCalls.Load() < 2 {
		t.Errorf("watch must restart after failure; watchFn called %d time(s)", watchFnCalls.Load())
	}
}
