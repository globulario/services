package main

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
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

	if resp.GetStatus() == node_agentpb.ProbeStatusSucceeded {
		return infraProbeHealthy
	}

	// The agent answered that it could not settle the question. That is a
	// reduced harvest, not a verdict, and it belongs in the same bucket as a
	// silent agent: we did not learn the component is down, we learned we
	// could not find out.
	//
	// Treating it as UNHEALTHY is failure.an_unreachable_node_agent_is_reported_as_an_unhealthy_infra:
	// in the 1.2.360 soak run a node agent whose own etcd client had lost its
	// client certificate reported its perfectly healthy local etcd as FAILED,
	// the controller raised infra_unhealthy on that member, and remediation
	// ground against it for 55 reconcile cycles without progress — because no
	// remediation of etcd can fix an agent's certificate.
	if resp.GetStatus() == node_agentpb.ProbeStatusUnknown {
		log.Printf("infra-probe: %s on %s returned status=UNKNOWN detail=%s — component state UNKNOWN, not unhealthy",
			probeName, endpoint, resp.GetError())
		return infraProbeUnreachable
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
// etcdRejoinDispatchCooldown bounds how often the same node may be sent the
// destructive wipe-and-rejoin workflow.
//
// Without it the dispatch below fires on EVERY reconcile tick for as long as the
// node sits in RejoinInProgress — and the phase only leaves that state once the
// node is BOTH a ring member and running, which is strictly after a wipe has
// succeeded. A wipe that was working would therefore be re-issued on top of
// itself every tick, re-wiping an etcd that had just rejoined: the shape of
// objectstore.minio.transition_wipe_loop, where a destructive step re-runs each
// cycle and the thing it creates is never allowed to become stable.
//
// This did not bite before only because the RPC was cancelled immediately (see
// below) and never actually ran, so the guard lands together with the fix that
// makes the loop reachable.
const etcdRejoinDispatchCooldown = 3 * time.Minute

// etcdRejoinConfigInput is the RunWorkflow input key under which the live ring's
// membership travels to the node agent, as the value of etcd.yaml's
// initial-cluster. The node agent refuses the wipe without it — see
// runWipeEtcdAndRejoin, which also explains why this is the membership rather
// than a whole rendered file.
const etcdRejoinConfigInput = "etcd_initial_cluster"

// etcdInitialClusterFromRenderedConfig pulls the initial-cluster VALUE out of a
// rendered etcd.yaml. It matches "initial-cluster:" exactly, never the
// -state or -token keys that share its prefix, and returns "" for a config that
// does not carry one — which the caller treats as "nothing lawful to send".
func etcdInitialClusterFromRenderedConfig(rendered string) string {
	for _, line := range strings.Split(rendered, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "initial-cluster:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "initial-cluster:"))
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
		return value
	}
	return ""
}

// etcdRejoinHoldLogged rate-limits the "holding the wipe" line to once per node
// per etcdRejoinDispatchCooldown.
var etcdRejoinHoldLogged sync.Map // nodeID -> time.Time

func logEtcdRejoinHold(nodeID, hostname string) {
	now := time.Now()
	if last, ok := etcdRejoinHoldLogged.Load(nodeID); ok {
		if at, isTime := last.(time.Time); isTime && now.Sub(at) < etcdRejoinDispatchCooldown {
			return
		}
	}
	etcdRejoinHoldLogged.Store(nodeID, now)
	log.Printf("etcd auto-rejoin: %s (%s) — holding the wipe: no initial-cluster could be rendered "+
		"(node not yet in the live ring, or no routable IP). A wipe without current membership "+
		"leaves etcd unable to start at all.", nodeID, hostname)
}

func (srv *server) dispatchEtcdWipeAndRejoin(ctx context.Context, nodes []*nodeState) {
	for _, node := range nodes {
		if node == nil || node.EtcdJoinPhase != EtcdJoinRejoinInProgress {
			// Not repairing — forget any past dispatch, so a LATER repair episode
			// for this node is not suppressed by a stale timestamp.
			if node != nil {
				srv.etcdRejoinDispatch.Delete(node.NodeID)
			}
			continue
		}
		endpoint := node.AgentEndpoint
		if endpoint == "" {
			continue
		}
		if last, ok := srv.etcdRejoinDispatch.Load(node.NodeID); ok {
			if at, isTime := last.(time.Time); isTime && time.Since(at) < etcdRejoinDispatchCooldown {
				continue // a wipe is in flight or was just issued
			}
		}

		// The membership the node must come back with travels WITH the request.
		//
		// The wipe makes this node an uninitialized etcd member, and an
		// uninitialized member takes its whole membership from initial-cluster in
		// /var/lib/globular/config/etcd.yaml. That file is written by Day-0 and by
		// the join script and is refreshed by nothing afterwards — dispatchPlan,
		// which used to carry renderEtcdConfig's output to the node, is a no-op
		// stub. So the file on disk is a snapshot of the ring as it was when this
		// node last joined, and etcd refuses to start against a ring of a
		// different size: "error validating peerURLs ...: member count is
		// unequal". The node then crash-loops forever while this repair reports
		// SUCCEEDED every cooldown.
		//
		// renderEtcdConfig renders initial-cluster from the LIVE RING (it returns
		// !ok for a node that is not in it), which is the same set etcd validates
		// against. Taking the membership from that render — rather than computing
		// it a second way here — keeps ONE authority for "who is in this ring",
		// and closes the gap between MemberAdd and the restart without reviving a
		// general config-push path.
		initialCluster := ""
		if rendered := srv.renderedConfigForNode(node); rendered != nil {
			initialCluster = etcdInitialClusterFromRenderedConfig(rendered[etcdConfigPath])
		}
		if initialCluster == "" {
			// No lawful membership to send — do NOT stamp the cooldown, so the
			// dispatch happens on the first pass that can render one (typically
			// the tick right after MemberAdd lands in the ring). Wiping a node we
			// cannot configure is the unrecoverable case.
			//
			// This branch is re-entered on EVERY reconcile tick for as long as the
			// node cannot be rendered, so the line is rate-limited: a guard against
			// an unrecoverable repair must not become a log storm of its own.
			logEtcdRejoinHold(node.NodeID, node.Identity.Hostname)
			continue
		}
		srv.etcdRejoinDispatch.Store(node.NodeID, time.Now())

		// Detach from the caller's context.
		//
		// ctx belongs to the reconcile workflow ACTION
		// (engine.reconcileAdvanceInfraJoins); the engine cancels it as soon as
		// that handler returns, which this function does immediately after
		// launching the goroutine below. Every RunWorkflow call therefore died
		// with "rpc error: code = Canceled desc = context canceled" before the
		// node agent could act — measured on the 5-node simulation, 1.2.358,
		// 2026-09-03: node-5 got its MemberAdd, sat in the ring as an *unstarted*
		// member, and the dispatch failed with that error on every tick while
		// etcd_healthy_endpoints stayed at 4.
		//
		// The leader context is the lifetime this work belongs to. It is the
		// idiom the neighbouring detached repair already uses (the Scylla ring
		// cleanup in removeStaleNodesLocked), and it is cancelled on loss of
		// leadership — exactly when a repair should stop.
		wCtx, cancel := context.WithTimeout(srv.getLeaderCtx(), 3*time.Minute)
		go func(ep, nodeID, hostname, cfg string) {
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
				Inputs:       map[string]string{etcdRejoinConfigInput: cfg},
			})
			if err != nil {
				log.Printf("etcd auto-rejoin: wipe-etcd-and-rejoin on %s (%s) RPC error: %v", nodeID, hostname, err)
				return
			}
			log.Printf("etcd auto-rejoin: wipe-etcd-and-rejoin on %s (%s) status=%s error=%s",
				nodeID, hostname, resp.GetStatus(), resp.GetError())
		}(endpoint, node.NodeID, node.Identity.Hostname, initialCluster)
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
