package main

import (
	"context"
	"fmt"
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
// Supported components: "scylladb", "etcd", "minio", "envoy" (and "all", which
// returns every adapter's result). The handler is local-only and never depends on
// the controller — it observes this node's own infrastructure.
func (srv *NodeAgentServer) GetInfraProbe(ctx context.Context, req *node_agentpb.GetInfraProbeRequest) (*node_agentpb.GetInfraProbeResponse, error) {
	component := strings.ToLower(strings.TrimSpace(req.GetComponent()))
	if component == "" {
		component = infra_truth.ComponentAll
	}

	switch component {
	case infra_truth.ComponentScylla:
		return &node_agentpb.GetInfraProbeResponse{
			Results: []*cluster_controllerpb.InfraProbeResult{srv.scyllaInfraProbe(ctx, req.GetBypassCache())},
		}, nil
	case infra_truth.ComponentEtcd:
		return &node_agentpb.GetInfraProbeResponse{
			Results: []*cluster_controllerpb.InfraProbeResult{srv.etcdInfraProbe(ctx, req.GetBypassCache())},
		}, nil
	case infra_truth.ComponentMinio:
		return &node_agentpb.GetInfraProbeResponse{
			Results: []*cluster_controllerpb.InfraProbeResult{srv.minioInfraProbe(ctx, req.GetBypassCache())},
		}, nil
	case infra_truth.ComponentEnvoy:
		return &node_agentpb.GetInfraProbeResponse{
			Results: []*cluster_controllerpb.InfraProbeResult{srv.envoyInfraProbe(ctx, req.GetBypassCache())},
		}, nil
	case infra_truth.ComponentAll:
		return &node_agentpb.GetInfraProbeResponse{
			Results: []*cluster_controllerpb.InfraProbeResult{
				srv.scyllaInfraProbe(ctx, req.GetBypassCache()),
				srv.etcdInfraProbe(ctx, req.GetBypassCache()),
				srv.minioInfraProbe(ctx, req.GetBypassCache()),
				srv.envoyInfraProbe(ctx, req.GetBypassCache()),
			},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument,
			"unknown infra component %q (supported: scylladb, etcd, minio, envoy, all)", component)
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
	go emitBehavioralInfraProbe(context.Background(), srv.clusterID, res)
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

// etcdInfraProbe applies the cache policy: bypass_cache forces a fresh probe;
// otherwise a cache entry younger than infraCacheStaleAfter is served (stamped
// with its age), and anything older triggers a fresh probe.
func (srv *NodeAgentServer) etcdInfraProbe(ctx context.Context, bypassCache bool) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	if !bypassCache {
		if cached, at, ok := srv.infraProbeCache.Get(infra_truth.ComponentEtcd); ok {
			age := time.Since(at)
			if age <= infraCacheStaleAfter {
				cached.ProbeAgeSeconds = int64(age.Seconds())
				cached.ProbeStale = false
				return cached
			}
		}
	}
	return srv.refreshEtcdInfraProbe(ctx)
}

// refreshEtcdInfraProbe runs a fresh structured probe and stores it in the cache.
func (srv *NodeAgentServer) refreshEtcdInfraProbe(ctx context.Context) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	desired, derr := infra_truth.BuildEtcdDesiredState(srv.buildEtcdDesiredInputs())
	res := srv.etcdProber.ProbeStructured(ctx, desired, derr)
	srv.infraProbeCache.Put(infra_truth.ComponentEtcd, res, time.Now())
	go emitBehavioralInfraProbe(context.Background(), srv.clusterID, res)
	return res
}

// buildEtcdDesiredInputs gathers this node's view of etcd desired state from
// local identity and the controller-rendered etcd endpoints list. The endpoints
// file is both the membership truth and the initial-cluster source. The cluster
// token is the fixed bootstrap constant the installer and controller render
// (config.EtcdClusterToken) — it is bootstrap-immutable, never derived from the
// mutable cluster_id, so the desired token always matches the running member's.
func (srv *NodeAgentServer) buildEtcdDesiredInputs() infra_truth.EtcdDesiredInputs {
	peers := etcdPeerHostsFromConfig()
	token := config.EtcdClusterToken
	return infra_truth.EtcdDesiredInputs{
		NodeID:       srv.nodeID,
		ClusterID:    srv.clusterID,
		LocalIP:      nodeRoutableIP(),
		Peers:        peers,
		ClusterToken: token,
		Now:          time.Now().Unix(),
	}
}

// etcdPeerHostsFromConfig returns the cluster-facing hosts of every etcd member,
// parsed from the controller-rendered etcd endpoints (URLs like
// https://10.0.0.63:2379). Best effort; an empty result still lets desired build
// succeed with self only.
func etcdPeerHostsFromConfig() []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range config.GetEtcdEndpointsHostPorts() {
		h := etcdHost(raw)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// minioInfraProbe applies the cache policy: bypass_cache forces a fresh probe;
// otherwise a cache entry younger than infraCacheStaleAfter is served (stamped
// with its age), and anything older triggers a fresh probe.
func (srv *NodeAgentServer) minioInfraProbe(ctx context.Context, bypassCache bool) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	if !bypassCache {
		if cached, at, ok := srv.infraProbeCache.Get(infra_truth.ComponentMinio); ok {
			age := time.Since(at)
			if age <= infraCacheStaleAfter {
				cached.ProbeAgeSeconds = int64(age.Seconds())
				cached.ProbeStale = false
				return cached
			}
		}
	}
	return srv.refreshMinioInfraProbe(ctx)
}

// refreshMinioInfraProbe runs a fresh structured probe and stores it in the cache.
func (srv *NodeAgentServer) refreshMinioInfraProbe(ctx context.Context) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	desired, derr := infra_truth.BuildMinioDesiredState(srv.buildMinioDesiredInputs(ctx))
	res := srv.minioProber.ProbeStructured(ctx, desired, derr)
	srv.infraProbeCache.Put(infra_truth.ComponentMinio, res, time.Now())
	go emitBehavioralInfraProbe(context.Background(), srv.clusterID, res)
	return res
}

// buildMinioDesiredInputs gathers this node's view of MinIO desired state from
// local identity and the controller-published ObjectStoreDesiredState (the single
// authority for MinIO topology — mode, pool nodes, drives-per-node). A missing or
// unreadable desired state still lets desired build succeed with self only; the
// attestation then reflects the (degraded) topology truthfully.
func (srv *NodeAgentServer) buildMinioDesiredInputs(ctx context.Context) infra_truth.MinioDesiredInputs {
	in := infra_truth.MinioDesiredInputs{
		NodeID:    srv.nodeID,
		ClusterID: srv.clusterID,
		LocalIP:   nodeRoutableIP(),
		Now:       time.Now().Unix(),
	}
	if state, err := config.LoadObjectStoreDesiredState(ctx); err == nil && state != nil {
		in.Mode = string(state.Mode)
		in.Nodes = state.Nodes
		in.DrivesPerNode = state.DrivesPerNode
		in.SourceVersion = fmt.Sprintf("%d", state.Generation)
	}
	return in
}

// envoyInfraProbe applies the cache policy: bypass_cache forces a fresh probe;
// otherwise a cache entry younger than infraCacheStaleAfter is served (stamped
// with its age), and anything older triggers a fresh probe.
func (srv *NodeAgentServer) envoyInfraProbe(ctx context.Context, bypassCache bool) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	if !bypassCache {
		if cached, at, ok := srv.infraProbeCache.Get(infra_truth.ComponentEnvoy); ok {
			age := time.Since(at)
			if age <= infraCacheStaleAfter {
				cached.ProbeAgeSeconds = int64(age.Seconds())
				cached.ProbeStale = false
				return cached
			}
		}
	}
	return srv.refreshEnvoyInfraProbe(ctx)
}

// refreshEnvoyInfraProbe runs a fresh structured probe and stores it in the cache.
func (srv *NodeAgentServer) refreshEnvoyInfraProbe(ctx context.Context) *cluster_controllerpb.InfraProbeResult {
	srv.ensureInfraTruth()

	desired, derr := infra_truth.BuildEnvoyDesiredState(srv.buildEnvoyDesiredInputs())
	res := srv.envoyProber.ProbeStructured(ctx, desired, derr)
	srv.infraProbeCache.Put(infra_truth.ComponentEnvoy, res, time.Now())
	go emitBehavioralInfraProbe(context.Background(), srv.clusterID, res)
	return res
}

// buildEnvoyDesiredInputs gathers this node's identity for the Envoy desired
// state. Envoy is a per-node data plane (not clustered) so there is no membership
// to read — only local identity and provenance.
func (srv *NodeAgentServer) buildEnvoyDesiredInputs() infra_truth.EnvoyDesiredInputs {
	return infra_truth.EnvoyDesiredInputs{
		NodeID:    srv.nodeID,
		ClusterID: srv.clusterID,
		LocalIP:   nodeRoutableIP(),
		Now:       time.Now().Unix(),
	}
}

// startInfraProbeRefresher launches the background goroutine that keeps the probe
// cache warm. It runs an initial probe immediately, then on infraRefreshInterval.
// This is what lets the heartbeat attach probe data WITHOUT ever running a slow
// native-API call inline on the heartbeat path.
func (srv *NodeAgentServer) startInfraProbeRefresher(ctx context.Context) {
	srv.ensureInfraTruth()
	refreshAll := func(parent context.Context) {
		rc, cancel := context.WithTimeout(parent, 5*time.Second)
		srv.refreshScyllaInfraProbe(rc)
		cancel()
		rc, cancel = context.WithTimeout(parent, 5*time.Second)
		srv.refreshEtcdInfraProbe(rc)
		cancel()
		rc, cancel = context.WithTimeout(parent, 5*time.Second)
		srv.refreshMinioInfraProbe(rc)
		cancel()
		rc, cancel = context.WithTimeout(parent, 5*time.Second)
		srv.refreshEnvoyInfraProbe(rc)
		cancel()
	}
	go func() {
		// Bounded first probe so a hung daemon can't delay startup.
		refreshAll(ctx)

		ticker := time.NewTicker(infraRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshAll(ctx)
			}
		}
	}()
}
