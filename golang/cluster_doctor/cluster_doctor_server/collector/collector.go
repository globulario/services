package collector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// CollectorConfig carries per-fetch settings.
type CollectorConfig struct {
	ListTimeout  time.Duration
	NodeTimeout  time.Duration
	Concurrency  int
	SnapshotTTL  time.Duration
}

// Collector gathers upstream state and maintains a SnapshotCache.
type Collector struct {
	cfg              CollectorConfig
	controllerClient cluster_controllerpb.ClusterControllerServiceClient
	workflowClient   workflowpb.WorkflowServiceClient
	clusterID        string
	cache            *SnapshotCache

	connMu     sync.Mutex
	agentConns map[string]*grpc.ClientConn // keyed by AgentEndpoint
}

func New(cfg CollectorConfig, cc cluster_controllerpb.ClusterControllerServiceClient) *Collector {
	return &Collector{
		cfg:              cfg,
		controllerClient: cc,
		cache:            NewSnapshotCache(cfg.SnapshotTTL),
		agentConns:       make(map[string]*grpc.ClientConn),
	}
}

// WithWorkflowClient attaches a workflow-service client so the collector can
// pull convergence telemetry (step outcomes, drift, run summaries) into the
// snapshot. Optional — if nil, telemetry-based invariants degrade gracefully.
func (c *Collector) WithWorkflowClient(wf workflowpb.WorkflowServiceClient, clusterID string) *Collector {
	c.workflowClient = wf
	c.clusterID = clusterID
	return c
}

// SnapshotResult carries the snapshot plus provenance telemetry the
// report layer needs to populate the freshness fields in ReportHeader.
// CacheHit is true when the cache already had a fresh entry and no
// upstream fetch was performed for this call. CacheTTL is the TTL the
// cache is currently configured with; callers surface it so operators
// know the maximum staleness a cached response can have.
type SnapshotResult struct {
	Snapshot *Snapshot
	CacheHit bool
	CacheTTL time.Duration
}

// GetSnapshot returns a cached or freshly fetched Snapshot. Kept for
// back-compat; new callers should prefer GetSnapshotWithFreshness,
// which also reports whether the response was a cache hit.
func (c *Collector) GetSnapshot(ctx context.Context) (*Snapshot, error) {
	res, err := c.GetSnapshotWithFreshness(ctx, false)
	if res.Snapshot != nil || err == nil {
		return res.Snapshot, err
	}
	return nil, err
}

// GetSnapshotWithFreshness is the freshness-aware fetch entry point.
// When forceFresh is true the cached snapshot is dropped before the
// fetch so the caller is guaranteed an authoritative read — useful
// right after running a remediation, or when opening an incident.
//
// The returned SnapshotResult always carries CacheTTL so callers can
// communicate the staleness window to operators without reaching into
// doctor's config. CacheHit is decided on THIS call's path, not on
// any concurrent caller that may be waiting behind a singleflight.
func (c *Collector) GetSnapshotWithFreshness(ctx context.Context, forceFresh bool) (SnapshotResult, error) {
	res := SnapshotResult{CacheTTL: c.cache.ttlFor()}
	if forceFresh {
		c.cache.invalidate()
	}
	cached, waiter := c.cache.get()
	if cached != nil {
		res.Snapshot = cached
		res.CacheHit = true
		return res, nil
	}
	if waiter != nil {
		// Another goroutine is already fetching — share its result.
		// From this caller's point of view an upstream fetch DID
		// happen (we did not return cached bytes without waiting),
		// so CacheHit stays false.
		select {
		case snap := <-waiter:
			res.Snapshot = snap
			return res, nil
		case <-ctx.Done():
			return res, ctx.Err()
		}
	}

	// We are the fetcher.
	snap, err := c.fetch(ctx)
	if err != nil {
		// On hard error still store a partial snapshot so callers get what we have.
		if snap == nil {
			snap = newSnapshot(uuid.New().String())
			snap.DataIncomplete = true
		}
	}
	c.cache.set(snap)
	res.Snapshot = snap
	return res, err
}

// fetch does the actual upstream calls.
func (c *Collector) fetch(ctx context.Context) (*Snapshot, error) {
	snap := newSnapshot(uuid.New().String())

	// ── 1. ListNodes ──────────────────────────────────────────────────────────
	listCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	nodesResp, err := c.controllerClient.ListNodes(listCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		snap.addError("cluster_controller", "ListNodes", err)
	} else {
		snap.Nodes = nodesResp.GetNodes()
		snap.addSource("cluster_controller.ListNodes")
	}

	// ── 2. GetClusterHealthV1 ─────────────────────────────────────────────────
	healthCtx, cancel2 := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel2()

	healthResp, err := c.controllerClient.GetClusterHealthV1(healthCtx, &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		snap.addError("cluster_controller", "GetClusterHealthV1", err)
	} else {
		for _, nh := range healthResp.GetNodes() {
			snap.NodeHealths[nh.GetNodeId()] = nh
		}
		snap.addSource("cluster_controller.GetClusterHealthV1")
	}

	// ── 3. Per-node calls (concurrent, capped) ────────────────────────────────
	if len(snap.Nodes) > 0 {
		c.fetchPerNode(ctx, snap)
	}

	// ── 4. Workflow convergence telemetry (optional) ─────────────────────────
	if c.workflowClient != nil && c.clusterID != "" {
		c.fetchWorkflowTelemetry(ctx, snap)
	}

	return snap, nil
}

// fetchWorkflowTelemetry pulls bounded convergence signals from the workflow
// service: per-step outcomes, drift items still unresolved, and per-workflow
// summaries. These drive the workflow.* invariants.
func (c *Collector) fetchWorkflowTelemetry(ctx context.Context, snap *Snapshot) {
	wfCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	if stepsResp, err := c.workflowClient.ListStepOutcomes(wfCtx, &workflowpb.ListStepOutcomesRequest{
		ClusterId: c.clusterID,
	}); err != nil {
		snap.addError("workflow", "ListStepOutcomes", err)
	} else {
		snap.StepOutcomes = stepsResp.GetOutcomes()
		snap.addSource("workflow.ListStepOutcomes")
	}

	if summariesResp, err := c.workflowClient.ListWorkflowSummaries(wfCtx, &workflowpb.ListWorkflowSummariesRequest{
		ClusterId: c.clusterID,
	}); err != nil {
		snap.addError("workflow", "ListWorkflowSummaries", err)
	} else {
		snap.WorkflowSummaries = summariesResp.GetSummaries()
		snap.addSource("workflow.ListWorkflowSummaries")
	}

	if driftResp, err := c.workflowClient.ListDriftUnresolved(wfCtx, &workflowpb.ListDriftUnresolvedRequest{
		ClusterId: c.clusterID,
	}); err != nil {
		snap.addError("workflow", "ListDriftUnresolved", err)
	} else {
		snap.DriftUnresolved = driftResp.GetItems()
		snap.addSource("workflow.ListDriftUnresolved")
	}
}

func (c *Collector) fetchPerNode(ctx context.Context, snap *Snapshot) {
	sem := make(chan struct{}, c.cfg.Concurrency)
	var wg sync.WaitGroup

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		endpoint := node.GetAgentEndpoint()
		if endpoint == "" {
			snap.addError("node_agent@"+nodeID, "dial", fmt.Errorf("node %s has no agent_endpoint", nodeID))
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(nid, ep string) {
			defer wg.Done()
			defer func() { <-sem }()

			agentClient, err := c.agentClient(ep)
			if err != nil {
				snap.addError("node_agent@"+nid, "dial", err)
				return
			}

			// GetInventory
			invCtx, cancel := context.WithTimeout(ctx, c.cfg.NodeTimeout)
			defer cancel()
			invResp, err := agentClient.GetInventory(invCtx, &node_agentpb.GetInventoryRequest{})
			if err != nil {
				snap.addError("node_agent@"+nid, "GetInventory", err)
			} else {
				snap.mu.Lock()
				snap.Inventories[nid] = invResp.GetInventory()
				snap.mu.Unlock()
				snap.addSource("node_agent.GetInventory@" + nid)
			}

			// Plan collection removed — plan system deleted.
		}(nodeID, endpoint)
	}

	wg.Wait()
}

// agentClient returns a cached or new NodeAgent gRPC client for the given endpoint.
func (c *Collector) agentClient(endpoint string) (node_agentpb.NodeAgentServiceClient, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if conn, ok := c.agentConns[endpoint]; ok {
		return node_agentpb.NewNodeAgentServiceClient(conn), nil
	}

	dt := config.ResolveDialTarget(endpoint)
	conn, err := grpc.NewClient(dt.Address, grpc.WithTransportCredentials(agentClientTLSCreds(dt.ServerName)))
	if err != nil {
		return nil, fmt.Errorf("dial node agent %s: %w", dt.Address, err)
	}
	c.agentConns[endpoint] = conn
	return node_agentpb.NewNodeAgentServiceClient(conn), nil
}

// agentClientTLSCreds returns gRPC transport credentials for dialling node agents.
func agentClientTLSCreds(serverName string) credentials.TransportCredentials {
	tlsCfg := &tls.Config{ServerName: serverName}
	caFile := config.GetTLSFile("", "", "ca.crt")
	if caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				tlsCfg.RootCAs = pool
			}
		}
	}
	return credentials.NewTLS(tlsCfg)
}

// Close releases all agent connections.
func (c *Collector) Close() {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	for _, conn := range c.agentConns {
		conn.Close()
	}
	c.agentConns = make(map[string]*grpc.ClientConn)
}
