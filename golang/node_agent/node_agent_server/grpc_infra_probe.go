package main

import (
	"context"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// infraCacheStaleAfter is the freshness window for the on-demand RPC: a cached
// probe older than this is refreshed before being served. The heartbeat uses the
// same value to stamp probe_stale.
const infraCacheStaleAfter = 2 * time.Minute

// infraRefreshInterval is how often the background goroutine refreshes the probe
// cache so the heartbeat always has a recent (and explicitly-aged) snapshot.
const infraRefreshInterval = 30 * time.Second

// GetInfraProbe returns the infrastructure truth-plane probe for a component.
// Phase 1 supports "scylladb" (and "all", which resolves to scylladb only). The
// handler is local-only and never depends on the controller — it observes this
// node's own infrastructure.
func (srv *NodeAgentServer) GetInfraProbe(ctx context.Context, req *node_agentpb.GetInfraProbeRequest) (*node_agentpb.GetInfraProbeResponse, error) {
	component := strings.ToLower(strings.TrimSpace(req.GetComponent()))
	if component == "" {
		component = infra_truth.ComponentAll
	}

	switch component {
	case infra_truth.ComponentScylla, infra_truth.ComponentAll:
		res := srv.scyllaInfraProbe(ctx, req.GetBypassCache())
		return &node_agentpb.GetInfraProbeResponse{
			Results: []*cluster_controllerpb.InfraProbeResult{res},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument,
			"unknown infra component %q (Phase 1 supports: scylladb, all)", component)
	}
}

// scyllaInfraProbe applies the cache policy: bypass_cache forces a fresh probe;
// otherwise a cache entry younger than infraCacheStaleAfter is served (stamped
// with its age), and anything older triggers a fresh probe.
func (srv *NodeAgentServer) scyllaInfraProbe(ctx context.Context, bypassCache bool) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	if !bypassCache {
		if cached, at, ok := srv.infraProbeCache.Get(infra_truth.ComponentScylla); ok {
			age := time.Since(at)
			if age <= infraCacheStaleAfter {
				cached.ProbeAgeSeconds = int64(age.Seconds())
				cached.ProbeStale = false
				return cached
			}
		}
	}
	return srv.refreshScyllaInfraProbe(ctx)
}

// refreshScyllaInfraProbe runs a fresh structured probe and stores it in the
// cache. It is the single place that builds desired state + probes + caches, so
// the RPC handler and the background refresher share identical behavior.
func (srv *NodeAgentServer) refreshScyllaInfraProbe(ctx context.Context) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	desired, derr := infra_truth.BuildScyllaDesiredState(srv.buildScyllaDesiredInputs())
	res := srv.scyllaProber.ProbeStructured(ctx, desired, derr)
	srv.infraProbeCache.Put(infra_truth.ComponentScylla, res, time.Now())
	return res
}

// buildScyllaDesiredInputs gathers this node's view of ScyllaDB desired state
// from local identity and etcd cluster membership. In Globular the scylla hosts
// list is both the seed list and the expected-peer list. The cluster name is not
// wired to an authoritative source in Phase 1 — provenance records that, and
// attestation still enforces a non-empty rendered cluster_name.
func (srv *NodeAgentServer) buildScyllaDesiredInputs() infra_truth.ScyllaDesiredInputs {
	hosts, _ := config.GetScyllaHosts() // best effort; empty => desired build still succeeds with self only
	return infra_truth.ScyllaDesiredInputs{
		NodeID:    srv.nodeID,
		ClusterID: srv.clusterID,
		LocalIP:   nodeRoutableIP(),
		Peers:     hosts,
		Seeds:     hosts,
		Now:       time.Now().Unix(),
	}
}

// startInfraProbeRefresher launches the background goroutine that keeps the probe
// cache warm. It runs an initial probe immediately, then on infraRefreshInterval.
// This is what lets the heartbeat attach probe data WITHOUT ever running a slow
// CQL/REST/nodetool call inline on the heartbeat path.
func (srv *NodeAgentServer) startInfraProbeRefresher(ctx context.Context) {
	srv.ensureInfraTruth()
	go func() {
		// Bounded first probe so a hung daemon can't delay startup.
		first, cancel := context.WithTimeout(ctx, 5*time.Second)
		srv.refreshScyllaInfraProbe(first)
		cancel()

		ticker := time.NewTicker(infraRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rc, cancel := context.WithTimeout(ctx, 5*time.Second)
				srv.refreshScyllaInfraProbe(rc)
				cancel()
			}
		}
	}()
}
