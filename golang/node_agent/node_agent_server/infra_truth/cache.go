package infra_truth

import (
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/protobuf/proto"
)

// InfraProbeCache holds the most recent probe result per component. The
// heartbeat reads from this cache instead of running slow probes inline; a
// background goroutine refreshes it on a timer.
//
// Critical property (enforces fm.industry.stale_cache_served_as_live_state): a
// cached result is NEVER handed out as if it were live. Snapshot() stamps every
// returned result with probe_stale / probe_age_seconds so the consumer (and the
// controller that receives the heartbeat) can tell a fresh probe from a stale
// cache entry.
type InfraProbeCache struct {
	mu        sync.RWMutex
	results   map[string]*cluster_controllerpb.InfraProbeResult
	updatedAt map[string]time.Time
}

// NewInfraProbeCache returns an empty cache.
func NewInfraProbeCache() *InfraProbeCache {
	return &InfraProbeCache{
		results:   make(map[string]*cluster_controllerpb.InfraProbeResult),
		updatedAt: make(map[string]time.Time),
	}
}

// Put stores a fresh probe result for a component.
func (c *InfraProbeCache) Put(component string, r *cluster_controllerpb.InfraProbeResult, at time.Time) {
	if r == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[component] = proto.Clone(r).(*cluster_controllerpb.InfraProbeResult)
	c.updatedAt[component] = at
}

// Get returns a clone of the cached result for a component and when it was
// stored. ok is false when nothing is cached.
func (c *InfraProbeCache) Get(component string) (*cluster_controllerpb.InfraProbeResult, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.results[component]
	if !ok {
		return nil, time.Time{}, false
	}
	return proto.Clone(r).(*cluster_controllerpb.InfraProbeResult), c.updatedAt[component], true
}

// Snapshot returns clones of every cached result, each stamped with its current
// staleness relative to now. A result older than staleAfter has probe_stale=true.
// This is the method the heartbeat uses — it can never accidentally present a
// stale cache entry as live truth.
func (c *InfraProbeCache) Snapshot(now time.Time, staleAfter time.Duration) []*cluster_controllerpb.InfraProbeResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]*cluster_controllerpb.InfraProbeResult, 0, len(c.results))
	for comp, r := range c.results {
		clone := proto.Clone(r).(*cluster_controllerpb.InfraProbeResult)
		age := now.Sub(c.updatedAt[comp])
		if age < 0 {
			age = 0
		}
		clone.ProbeAgeSeconds = int64(age.Seconds())
		clone.ProbeStale = staleAfter > 0 && age > staleAfter
		out = append(out, clone)
	}
	return out
}
