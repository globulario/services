package main

import (
	"context"
	"fmt"
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

func (srv *server) probeInfraHealthForNode(ctx context.Context, nodeID, endpoint, probeName string) bool {
	if endpoint == "" {
		return false
	}

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
	if err != nil {
		log.Printf("infra-probe: failed to connect to %s for probe %s: %v", endpoint, probeName, err)
		return false
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.RunWorkflow(probeCtx, &node_agentpb.RunWorkflowRequest{
		WorkflowName: probeName,
	})
	if err != nil {
		log.Printf("infra-probe: %s on %s failed: %v", probeName, endpoint, err)
		return false
	}

	healthy := resp.GetStatus() == "SUCCEEDED"
	if !healthy {
		log.Printf("infra-probe: %s on %s returned status=%s error=%s",
			probeName, endpoint, resp.GetStatus(), resp.GetError())
	}
	return healthy
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

// infraUnitForComponent maps an infra_unhealthy drift item's "component"
// (as emitted by reconcileScanDrift's probeEtcdHealth/probeScyllaHealth/
// probeMinioHealth checks) to the systemd unit ControlService controls.
// Mirrors detectUnits' baseline unit names (server.go).
func infraUnitForComponent(component string) string {
	switch component {
	case "etcd":
		return "globular-etcd.service"
	case "scylladb":
		return "scylla-server.service"
	case "minio":
		return "globular-minio.service"
	default:
		return ""
	}
}

// restartInfraUnit restarts a verified-member infra unit (etcd/scylladb/minio)
// that reconcileScanDrift observed unhealthy, via node-agent's ControlService
// RPC — the same allowlisted, supervisor-backed path `globular node control`
// and cluster-doctor's remediate command already use (HARD RULE #6: mutating
// systemd actions go through node-agent's allowlist, never raw systemctl from
// the controller). ControlService itself gates globular-minio.service
// start/restart behind ObjectStoreDesiredState pool membership, so this
// cannot fight an in-flight topology transition — node-agent refuses the
// action on a non-member rather than the controller having to know.
//
// Root cause this closes: infra_unhealthy drift was detected and surfaced by
// cluster-doctor but its reconcile remediation was a logged no-op ("for now,
// just log"), so a cleanly-stopped infra service (e.g. an operator's
// `systemctl stop`, or any other non-crash stop that systemd's
// Restart=on-failure does not cover) stayed down indefinitely, cascading into
// CRITICAL objectstore.endpoint_unreachable / installed_state_runtime_mismatch
// findings cluster-wide until a human ran `doctor remediate` by hand.
func (srv *server) restartInfraUnit(ctx context.Context, nodeID, endpoint, component string) (ok bool, message string) {
	unit := infraUnitForComponent(component)
	if unit == "" {
		return false, fmt.Sprintf("no known systemd unit for infra component %q", component)
	}
	if endpoint == "" {
		return false, "no agent endpoint for node " + nodeID
	}
	restartCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
	if err != nil {
		return false, fmt.Sprintf("dial node agent %s (%s): %v", nodeID, endpoint, err)
	}
	defer conn.Close()
	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.ControlService(restartCtx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: "restart",
	})
	if err != nil {
		return false, fmt.Sprintf("ControlService restart %s on %s: %v", unit, nodeID, err)
	}
	log.Printf("reconcile-workflow: infra_unhealthy restart %s on %s: ok=%v state=%s message=%s",
		unit, nodeID, resp.GetOk(), resp.GetState(), resp.GetMessage())
	return resp.GetOk(), resp.GetMessage()
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
