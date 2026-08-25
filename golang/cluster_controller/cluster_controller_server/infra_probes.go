package main

import (
	"context"
	"log"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// probeInfraHealth calls a named probe workflow on the node agent at the given
// endpoint via gRPC. The controller CANNOT use os/exec (security constraint),
// so all probes are delegated to node agents.
//
// Returns true if the probe reports Status == "SUCCEEDED".
func (srv *server) probeInfraHealth(ctx context.Context, endpoint, probeName string) bool {
	return srv.probeInfraHealthForNode(ctx, "", endpoint, probeName)
}

// infraProbeResult separates the two things a failed probe used to mean.
//
// Every infra health probe is asked THROUGH the node agent: the controller dials
// the agent and has it run probe-<component>-health locally. So when the agent is
// unreachable — down, restarting, mid-upgrade, partitioned — the controller
// learns nothing about the component. Collapsing that into "unhealthy" reports an
// absence of signal as a definite negative, which is how a perfectly healthy etcd
// gets an infra_unhealthy drift raised against it
// (invariant:diagnostics.must_measure_reality, and the sibling rule
// invariant:deadline_exceeded_must_not_drive_definitive_node_state — a timeout
// must not be the sole evidence for a definitive verdict about a remote node).
type infraProbeResult int

const (
	// infraProbeHealthy — the agent answered and the component is healthy.
	infraProbeHealthy infraProbeResult = iota
	// infraProbeUnhealthy — the agent answered and the component is NOT healthy.
	// This is the only result that justifies raising drift or remediating.
	infraProbeUnhealthy
	// infraProbeUnreachable — the agent could not be asked. Says nothing about the
	// component. Node reachability is the heartbeat path's job, not this one's.
	infraProbeUnreachable
)

func (r infraProbeResult) String() string {
	switch r {
	case infraProbeHealthy:
		return "healthy"
	case infraProbeUnhealthy:
		return "unhealthy"
	default:
		return "unreachable"
	}
}

// probeInfraHealthForNode keeps the boolean contract for callers that only care
// whether the component is provably healthy. Callers that must not act on an
// unreachable agent use probeInfraResultForNode instead.
func (srv *server) probeInfraHealthForNode(ctx context.Context, nodeID, endpoint, probeName string) bool {
	return srv.probeInfraResultForNode(ctx, nodeID, endpoint, probeName) == infraProbeHealthy
}

func (srv *server) probeInfraResultForNode(ctx context.Context, nodeID, endpoint, probeName string) infraProbeResult {
	if endpoint == "" {
		return infraProbeUnreachable
	}

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
	if err != nil {
		log.Printf("infra-probe: failed to connect to %s for probe %s: %v — component state UNKNOWN, not unhealthy", endpoint, probeName, err)
		return infraProbeUnreachable
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.RunWorkflow(probeCtx, &node_agentpb.RunWorkflowRequest{
		WorkflowName: probeName,
	})
	if err != nil {
		// The agent did not answer. We did not learn that the component is down;
		// we learned that we could not ask.
		log.Printf("infra-probe: %s on %s failed: %v — component state UNKNOWN, not unhealthy", probeName, endpoint, err)
		return infraProbeUnreachable
	}

	if resp.GetStatus() == "SUCCEEDED" {
		return infraProbeHealthy
	}
	log.Printf("infra-probe: %s on %s returned status=%s error=%s",
		probeName, endpoint, resp.GetStatus(), resp.GetError())
	return infraProbeUnhealthy
}

// probeInfraComponentResult probes one component by catalog name and reports the
// three-way result. Drift raising and remediation resolution both go through it
// so they cannot disagree about what a silent agent means.
func (srv *server) probeInfraComponentResult(ctx context.Context, nodeID, endpoint, component string) infraProbeResult {
	switch component {
	case "etcd":
		return srv.probeInfraResultForNode(ctx, nodeID, endpoint, "probe-etcd-health")
	case "scylladb":
		return srv.probeInfraResultForNode(ctx, nodeID, endpoint, "probe-scylla-health")
	case "minio":
		return srv.probeInfraResultForNode(ctx, nodeID, endpoint, "probe-minio-health")
	default:
		return infraProbeUnreachable
	}
}

// probeScyllaHealth probes ScyllaDB health on the given node agent endpoint.
func (srv *server) probeScyllaHealth(ctx context.Context, endpoint string) bool {
	return srv.probeInfraHealth(ctx, endpoint, "probe-scylla-health")
}

// probeEtcdHealth probes etcd health on the given node agent endpoint.
func (srv *server) probeEtcdHealth(ctx context.Context, endpoint string) bool {
	return srv.probeInfraHealth(ctx, endpoint, "probe-etcd-health")
}

// probeMinioHealth probes MinIO health on the given node agent endpoint.
func (srv *server) probeMinioHealth(ctx context.Context, endpoint string) bool {
	return srv.probeInfraHealth(ctx, endpoint, "probe-minio-health")
}

// dispatchEtcdWipeAndRejoin sends the "wipe-etcd-and-rejoin" workflow to every
// node in EtcdJoinRejoinInProgress that has a reachable node agent. The node
// agent stops globular-etcd, wipes /var/lib/globular/etcd/member, and restarts
// globular-etcd so it joins the cluster with the fresh MemberAdd config.
func (srv *server) dispatchEtcdWipeAndRejoin(ctx context.Context, nodes []*nodeState) {
	for _, node := range nodes {
		if node == nil || node.EtcdJoinPhase != EtcdJoinRejoinInProgress {
			continue
		}
		endpoint := node.AgentEndpoint
		if endpoint == "" {
			continue
		}
		wCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		go func(ep, nodeID, hostname string) {
			defer cancel()
			conn, _, err := srv.dialNodeAgentForNode(nodeID, ep)
			if err != nil {
				log.Printf("etcd auto-rejoin: cannot dial agent %s (%s): %v", nodeID, hostname, err)
				return
			}
			defer conn.Close()
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.RunWorkflow(wCtx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: "wipe-etcd-and-rejoin",
			})
			if err != nil {
				log.Printf("etcd auto-rejoin: wipe-etcd-and-rejoin on %s (%s) RPC error: %v", nodeID, hostname, err)
				return
			}
			log.Printf("etcd auto-rejoin: wipe-etcd-and-rejoin on %s (%s) status=%s error=%s",
				nodeID, hostname, resp.GetStatus(), resp.GetError())
		}(endpoint, node.NodeID, node.Identity.Hostname)
	}
}

// dispatchWebrootSync triggers webroot-sync on all gateway nodes.
// Best-effort: failures are logged but don't block reconciliation.
func (srv *server) dispatchWebrootSync(ctx context.Context) {
	srv.lock("webroot-sync")
	nodes := srv.gatewayNodes()
	srv.unlock()

	for _, node := range nodes {
		if node.AgentEndpoint == "" {
			continue
		}
		go func(ep, hostname string) {
			syncCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			ok := srv.probeInfraHealth(syncCtx, ep, "webroot-sync")
			if !ok {
				log.Printf("webroot-sync: %s failed", hostname)
			}
		}(node.AgentEndpoint, node.Identity.Hostname)
	}
}

// gatewayNodes returns nodes that have the gateway profile.
func (srv *server) gatewayNodes() []*nodeState {
	var out []*nodeState
	if srv.state == nil {
		return out
	}
	for _, node := range srv.state.Nodes {
		if node == nil || node.Status == "unreachable" {
			continue
		}
		for _, p := range node.Profiles {
			if strings.EqualFold(p, "gateway") {
				out = append(out, node)
				break
			}
		}
	}
	return out
}

// isActiveInfraMember returns true if the node is an active member of the
// infrastructure cluster identified by pkgName. Active members must NOT be
// reinstalled or disrupted by the release pipeline — doing so would cause
// data loss or cluster instability.
func isActiveInfraMember(node *nodeState, pkgName string) bool {
	if node == nil {
		return false
	}

	name := strings.ToLower(pkgName)

	switch {
	case name == "scylladb":
		// ScyllaJoinConfigured means config was rendered but service hasn't started.
		// It does NOT mean the node is an active ring member — don't block installation.
		// Note: scylla-manager and scylla-manager-agent are auxiliary services, NOT ring
		// members — they must NOT be caught by this check.
		switch node.ScyllaJoinPhase {
		case ScyllaJoinVerified, ScyllaJoinStarted:
			return true
		}

	case name == "etcd":
		switch node.EtcdJoinPhase {
		case EtcdJoinVerified, EtcdJoinStarted:
			return true
		}

	case name == "minio":
		switch node.MinioJoinPhase {
		case MinioJoinVerified, MinioJoinStarted:
			return true
		}
	}

	return false
}
