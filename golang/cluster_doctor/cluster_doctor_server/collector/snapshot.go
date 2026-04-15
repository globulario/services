package collector

import (
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/workflowpb"
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

	Nodes       []*cluster_controllerpb.NodeRecord
	NodeHealths map[string]*cluster_controllerpb.NodeHealth // keyed by NodeId
	Inventories map[string]*node_agentpb.Inventory          // keyed by NodeId

	// Per-node subsystem health, populated from the
	// GetSubsystemHealth RPC. Consumed by the "subsystem.stuck"
	// invariant family in rules/ to surface stuck/failed goroutines.
	// Keyed by NodeId.
	SubsystemHealth map[string]*node_agentpb.GetSubsystemHealthResponse

	// Per-node certificate status, populated from the
	// GetCertificateStatus RPC. Consumed by the "security.certs.*"
	// invariant family in rules/ to surface expiring certs and
	// SAN coverage gaps as doctor findings. Keyed by NodeId.
	CertificateStatus map[string]*node_agentpb.GetCertificateStatusResponse

	// Per-node artifact integrity reports, populated from the
	// VerifyPackageIntegrity RPC. Consumed by the
	// "artifact.*" invariant family in rules/ to surface
	// cache / installed-digest mismatches as doctor findings.
	// Keyed by NodeId. Missing entries mean the collector could
	// not obtain a report (dial failure, unimplemented on that
	// node's running binary, etc).
	IntegrityReports map[string]*IntegrityReport

	// Workflow convergence telemetry — see WI17/WI18.
	StepOutcomes      []*workflowpb.WorkflowStepOutcome
	WorkflowSummaries []*workflowpb.WorkflowRunSummary
	DriftUnresolved   []*workflowpb.DriftUnresolved
	BlockedRuns       []*workflowpb.WorkflowRun // MC-4: runs paused for operator approval

	// Prometheus-derived control-plane signals (optional)
	PromMetrics map[string]float64 // small, fixed key set
	PromTS      time.Time          // scrape timestamp

	mu sync.Mutex
}

// IntegrityReport is the internal representation of the JSON report returned
// by node_agent's VerifyPackageIntegrity RPC. It mirrors the schema produced
// by the `package.verify_integrity` action in node_agent/internal/actions.
//
// Unmarshalled from report_json verbatim — keep field tags in sync with the
// action's integrityReport type.
type IntegrityReport struct {
	NodeID     string              `json:"node_id"`
	Checked    int                 `json:"checked"`
	Findings   []IntegrityFinding  `json:"findings"`
	Errors     []string            `json:"errors,omitempty"`
	Invariants map[string]int      `json:"invariants"`
}

// IntegrityFinding is a single artifact-integrity violation.
type IntegrityFinding struct {
	Invariant string            `json:"invariant"`
	Severity  string            `json:"severity"`
	Package   string            `json:"package"`
	Kind      string            `json:"kind"`
	Summary   string            `json:"summary"`
	Evidence  map[string]string `json:"evidence,omitempty"`
}

func newSnapshot(id string) *Snapshot {
	return &Snapshot{
		SnapshotID:       id,
		GeneratedAt:      time.Now(),
		NodeHealths:      make(map[string]*cluster_controllerpb.NodeHealth),
		Inventories:      make(map[string]*node_agentpb.Inventory),
		SubsystemHealth:   make(map[string]*node_agentpb.GetSubsystemHealthResponse),
		CertificateStatus: make(map[string]*node_agentpb.GetCertificateStatusResponse),
		IntegrityReports:  make(map[string]*IntegrityReport),
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

// invalidate drops the cached snapshot so the next get() forces a
// fresh fetch. Used to implement FreshnessMode.FRESHNESS_FRESH — the
// caller asks for authoritative state, so we throw the cache away
// before the collector runs its fetch cycle.
func (c *SnapshotCache) invalidate() {
	c.mu.Lock()
	c.snapshot = nil
	c.fetchedAt = time.Time{}
	c.mu.Unlock()
}

// ttlFor returns the TTL the cache was configured with, so render
// layers can expose it to callers without reaching into unexported
// fields.
func (c *SnapshotCache) ttlFor() time.Duration { return c.ttl }

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
