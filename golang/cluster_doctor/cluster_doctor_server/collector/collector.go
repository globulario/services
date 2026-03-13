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

// GetSnapshot returns a cached or freshly fetched Snapshot.
func (c *Collector) GetSnapshot(ctx context.Context) (*Snapshot, error) {
	cached, waiter := c.cache.get()
	if cached != nil {
		return cached, nil
	}
	if waiter != nil {
		// Another goroutine is already fetching — wait for it.
		select {
		case snap := <-waiter:
			return snap, nil
		case <-ctx.Done():
			return nil, ctx.Err()
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
	return snap, err
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

	return snap, nil
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

			// GetPlanStatusV1
			psCtx, cancel2 := context.WithTimeout(ctx, c.cfg.NodeTimeout)
			defer cancel2()
			psResp, err := agentClient.GetPlanStatusV1(psCtx, &node_agentpb.GetPlanStatusV1Request{})
			if err != nil {
				snap.addError("node_agent@"+nid, "GetPlanStatusV1", err)
			} else {
				snap.mu.Lock()
				snap.PlanStatuses[nid] = psResp.GetStatus()
				snap.mu.Unlock()
				snap.addSource("node_agent.GetPlanStatusV1@" + nid)
			}

			// GetNodePlanV1 (from controller, using node id)
			planCtx, cancel3 := context.WithTimeout(ctx, c.cfg.NodeTimeout)
			defer cancel3()
			planResp, err := c.controllerClient.GetNodePlanV1(planCtx, &cluster_controllerpb.GetNodePlanV1Request{
				NodeId: nid,
			})
			if err != nil {
				snap.addError("cluster_controller@"+nid, "GetNodePlanV1", err)
			} else if planResp.GetPlan() != nil {
				snap.mu.Lock()
				snap.NodePlans[nid] = planResp.GetPlan()
				snap.mu.Unlock()
				snap.addSource("cluster_controller.GetNodePlanV1@" + nid)
			}
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

	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(agentClientTLSCreds()))
	if err != nil {
		return nil, fmt.Errorf("dial node agent %s: %w", endpoint, err)
	}
	c.agentConns[endpoint] = conn
	return node_agentpb.NewNodeAgentServiceClient(conn), nil
}

// agentClientTLSCreds returns gRPC transport credentials for dialling node agents.
func agentClientTLSCreds() credentials.TransportCredentials {
	caFile := config.GetTLSFile("", "", "ca.crt")
	if caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				return credentials.NewTLS(&tls.Config{RootCAs: pool})
			}
		}
	}
	return credentials.NewTLS(&tls.Config{})
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
