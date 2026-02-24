package collector

import (
	"sync"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	planpb "github.com/globulario/services/golang/plan/planpb"
)

// DataError records a failed upstream RPC call.
type DataError struct {
	Service string
	RPC     string
	Err     error
}

// Snapshot holds a point-in-time view of cluster state gathered from upstream services.
type Snapshot struct {
	SnapshotID     string
	GeneratedAt    time.Time
	DataSources    []string
	DataIncomplete bool
	DataErrors     []DataError

	Nodes       []*clustercontrollerpb.NodeRecord
	NodeHealths map[string]*clustercontrollerpb.NodeHealth // keyed by NodeId
	Inventories map[string]*nodeagentpb.Inventory          // keyed by NodeId
	PlanStatuses map[string]*planpb.NodePlanStatus         // keyed by NodeId
	NodePlans   map[string]*planpb.NodePlan                // keyed by NodeId

	mu sync.Mutex
}

func newSnapshot(id string) *Snapshot {
	return &Snapshot{
		SnapshotID:   id,
		GeneratedAt:  time.Now(),
		NodeHealths:  make(map[string]*clustercontrollerpb.NodeHealth),
		Inventories:  make(map[string]*nodeagentpb.Inventory),
		PlanStatuses: make(map[string]*planpb.NodePlanStatus),
		NodePlans:    make(map[string]*planpb.NodePlan),
	}
}

func (s *Snapshot) addSource(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DataSources = append(s.DataSources, name)
}

func (s *Snapshot) addError(service, rpc string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DataErrors = append(s.DataErrors, DataError{Service: service, RPC: rpc, Err: err})
	s.DataIncomplete = true
}

// ─── SnapshotCache ────────────────────────────────────────────────────────────

// SnapshotCache caches the most recent Snapshot for a configurable TTL.
// Concurrent callers during a fetch share a single in-flight request (singleflight).
type SnapshotCache struct {
	mu        sync.Mutex
	snapshot  *Snapshot
	fetchedAt time.Time
	ttl       time.Duration

	// singleflight fields
	inflight bool
	waiters  []chan *Snapshot
}

func NewSnapshotCache(ttl time.Duration) *SnapshotCache {
	return &SnapshotCache{ttl: ttl}
}

// get returns the cached snapshot if still fresh, along with a done channel that the
// caller must close after a fresh fetch (nil chan means cache hit — no fetch needed).
func (c *SnapshotCache) get() (*Snapshot, chan *Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.snapshot != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.snapshot, nil
	}

	if c.inflight {
		ch := make(chan *Snapshot, 1)
		c.waiters = append(c.waiters, ch)
		return nil, ch
	}

	c.inflight = true
	return nil, nil
}

// set stores a freshly fetched snapshot and notifies waiters.
func (c *SnapshotCache) set(snap *Snapshot) {
	c.mu.Lock()
	waiters := c.waiters
	c.snapshot = snap
	c.fetchedAt = time.Now()
	c.inflight = false
	c.waiters = nil
	c.mu.Unlock()

	for _, ch := range waiters {
		ch <- snap
	}
}
