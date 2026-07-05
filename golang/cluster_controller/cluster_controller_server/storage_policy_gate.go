package main

import (
	"context"
	"sync/atomic"
	"time"

	config "github.com/globulario/services/golang/config"
)

// storage_policy_gate.go — cluster storage-durability policy accessor.
//
// The cluster's declared StoragePolicy (config.EtcdKeyStoragePolicy) governs
// whether stateful substrates (ScyllaDB, MinIO) may materialize below the
// durable 3-node quorum. Quorum gates in this package read the policy-derived
// storage-node floor through the accessors here instead of the hardcoded 3 /
// MinQuorumNodes literal.
//
// Safety contract (mirrors config.LoadStoragePolicy):
//   - The policy is NEVER nil and NEVER degraded-by-accident: an absent key, an
//     etcd error, or a cold cache all resolve to the DURABLE default (floor 3).
//   - Degraded is only ever returned when an operator explicitly declared it.
//
// A package-level TTL cache (not a server-struct field) matches the existing
// singleton pattern for cluster-wide config (config.GetEtcdClient) and keeps a
// no-I/O path available to callers that hold srv.lock.

const storagePolicyCacheTTL = 5 * time.Second

var (
	storagePolicyCache    atomic.Pointer[config.StoragePolicy]
	storagePolicyLoadedAt atomic.Int64 // unix nanos of last successful load
)

// loadStoragePolicyCached returns the current cluster storage policy, refreshing
// from etcd at most once per TTL. On any failure it falls back to the last
// cached value, then to the durable default — it never returns nil and never
// silently degrades (awareness:runtime_evidence_must_be_fresh,
// inventory.missing_means_uncertain).
func loadStoragePolicyCached(ctx context.Context) *config.StoragePolicy {
	now := time.Now().UnixNano()
	cached := storagePolicyCache.Load()
	if cached != nil && now-storagePolicyLoadedAt.Load() < int64(storagePolicyCacheTTL) {
		return cached
	}
	p, err := config.LoadStoragePolicy(ctx)
	if err != nil || p == nil {
		if cached != nil {
			return cached
		}
		return config.DefaultStoragePolicy()
	}
	storagePolicyCache.Store(p)
	storagePolicyLoadedAt.Store(now)
	return p
}

// storagePolicyCachedOrDurable returns the last cached policy without any etcd
// I/O — for callers holding srv.lock, where a blocking load is unsafe. Resolves
// to the durable default when the cache is cold (safe: never degraded by cold
// start).
func storagePolicyCachedOrDurable() *config.StoragePolicy {
	if p := storagePolicyCache.Load(); p != nil {
		return p
	}
	return config.DefaultStoragePolicy()
}

// effectiveMinStorageNodes is the policy-derived storage-node floor for
// materializing stateful substrates. Durable / undeclared → 3; declared degraded
// → 2 or 1. Use this at gate sites that already have a context and may do I/O.
func effectiveMinStorageNodes(ctx context.Context) int {
	return loadStoragePolicyCached(ctx).MinStorageNodes()
}

// cachedMinStorageNodes is the no-I/O variant of effectiveMinStorageNodes for
// lock-holding callers. Conservative on a cold cache (durable floor 3).
func cachedMinStorageNodes() int {
	return storagePolicyCachedOrDurable().MinStorageNodes()
}
