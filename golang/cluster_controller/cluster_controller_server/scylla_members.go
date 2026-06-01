package main

import (
	"context"
	"log"
	"time"
)

// scyllaJoinTimeout is the maximum time between config render and
// scylla-server.service becoming active. Used in the ScyllaJoinConfigured
// case where we're waiting for systemd to start the unit. 5 minutes covers
// slow apt installs and disk-heavy first starts.
const scyllaJoinTimeout = 5 * time.Minute

// scyllaRaftRestartTimeout is the maximum time scylla-server can be active
// without joining the gossip ring before the first restart fires. Set lower
// than scyllaJoinTimeout because at this stage the symptom is specific
// (Raft group 0 join hung, process alive but silent). A clean Scylla join
// takes 60–90s on healthy hardware; 2 min gives ample slack without leaving
// the operator waiting 10+ min on the documented v1.x raft hang.
const scyllaRaftRestartTimeout = 2 * time.Minute

// nodeHasScyllaUnit returns true if the node reports a scylla-server.service
// unit file (any state).
func nodeHasScyllaUnit(node *nodeState) bool {
	if node == nil {
		return false
	}
	for _, u := range node.Units {
		if u.Name == "scylla-server.service" {
			return true
		}
	}
	return false
}

// nodeHasScyllaRunning returns true if the node reports scylla-server.service
// as "active" in its unit list.
func nodeHasScyllaRunning(node *nodeState) bool {
	if node == nil {
		return false
	}
	for _, u := range node.Units {
		if u.Name == "scylla-server.service" && u.State == "active" {
			return true
		}
	}
	return false
}

// nodeIsPreparedForScyllaJoin checks all preconditions for rendering scylla config:
//   - node has a scylla/database profile
//   - scylla-server.service unit file exists (package installed)
//   - node has a routable IP
//   - node is not mid-join (configured or started phase)
//   - node is in the correct bootstrap phase (awareness_ready, storage_joining, or workload_ready)
//
// awareness_ready is included so ScyllaDB join can start while the awareness
// bundle is still being fetched (or its 5-minute timeout is running). By the
// time the node advances to storage_joining, ScyllaDB will already be in
// progress or verified — eliminating the 5-minute delay on Day-1.
func nodeIsPreparedForScyllaJoin(node *nodeState) bool {
	if node == nil {
		return false
	}
	if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForScyllaDB) {
		return false
	}
	if !nodeHasScyllaUnit(node) {
		return false
	}
	ip := nodeRoutableIP(node)
	if ip == "" {
		return false
	}
	// Must not be mid-join.
	switch node.ScyllaJoinPhase {
	case ScyllaJoinConfigured, ScyllaJoinStarted:
		return false
	}
	// Must be in the correct bootstrap phase.
	// awareness_ready is included: ScyllaDB joining is independent of the
	// awareness bundle and can run in parallel with the bundle fetch wait.
	if node.BootstrapPhase != BootstrapNone &&
		node.BootstrapPhase != BootstrapAwarenessReady &&
		node.BootstrapPhase != BootstrapStorageJoining &&
		node.BootstrapPhase != BootstrapWorkloadReady {
		return false
	}
	return true
}

// scyllaClusterManager drives ScyllaDB cluster join for new nodes.
// Unlike etcd (which requires explicit MemberAdd), ScyllaDB uses gossip —
// the controller just needs to render correct config with seed nodes and
// verify the node joined the ring.
type scyllaClusterManager struct {
	// seedChecker is a function that checks if a given IP is in the ScyllaDB
	// gossip ring. In production, this queries system.peers via CQL.
	// For testing, it can be replaced with a mock.
	seedChecker func(seedIP, checkIP string) bool

	// probeNodeHealth is a function that probes ScyllaDB health on a node
	// agent endpoint via gRPC. Set by the server at startup.
	// Returns true if the probe reports the node is healthy.
	probeNodeHealth func(ctx context.Context, endpoint string) bool

	// restartService restarts a systemd unit on a node via the node-agent
	// ControlService RPC. Used to unstick ScyllaDB when it's stuck in
	// "join cluster" Raft state (CQL never comes up without a restart).
	restartService func(ctx context.Context, endpoint, unit string) error

	// wipeScyllaData stops ScyllaDB, wipes /var/lib/scylla/data, and restarts.
	// Used as escalation when restart alone doesn't unstick the Raft join —
	// stale Raft group state from a failed first boot prevents re-joining.
	wipeScyllaData func(ctx context.Context, endpoint string) error
}

func newScyllaClusterManager() *scyllaClusterManager {
	return &scyllaClusterManager{
		seedChecker: defaultScyllaSeedChecker,
	}
}

// defaultScyllaSeedChecker is the production implementation that checks if a
// node is in the ScyllaDB ring. Since the controller doesn't have a CQL driver,
// we rely on the unit being active as a proxy. The full verification (system.peers
// query) would require a CQL dependency. For now, "active" + "has been active
// for >30 seconds" is the heuristic.
func defaultScyllaSeedChecker(seedIP, checkIP string) bool {
	// The controller doesn't have a CQL client. We verify membership
	// through the unit state reported by the node agent (via heartbeat).
	// A more thorough check would query system.peers, but that requires
	// adding a gocql dependency to the controller.
	return false // always fall back to unit-state-based verification
}

// reconcileScyllaJoinPhases drives the ScyllaDB join state machine for all nodes.
//
// The flow:
//  1. prepared: preconditions met, scylla.yaml will be rendered by renderScyllaConfig
//  2. configured: config dispatched via rendered configs in plan
//  3. started: scylla-server.service is active
//  4. verified: node has been active long enough to have joined gossip ring
//
// ScyllaDB uses gossip, so there's no explicit "add to cluster" API call.
// The config rendering (via renderScyllaConfig in service_config.go) provides
// the seed list. The node starts, contacts seeds, and auto-joins.
func (m *scyllaClusterManager) reconcileScyllaJoinPhases(ctx context.Context, nodes []*nodeState) (dirty bool) {
	now := time.Now()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForScyllaDB) {
			continue
		}

		switch node.ScyllaJoinPhase {
		case ScyllaJoinNone, ScyllaJoinFailed:
			if !nodeIsPreparedForScyllaJoin(node) {
				continue
			}
			// Config will be rendered by renderScyllaConfig in the plan's
			// rendered configs. Mark as configured so we start tracking.
			log.Printf("scylla join: node %s (%s) is prepared, marking configured",
				node.NodeID, node.Identity.Hostname)
			node.ScyllaJoinPhase = ScyllaJoinConfigured
			node.ScyllaJoinStartedAt = now
			node.ScyllaJoinError = ""
			node.ScyllaJoinRestarts = 0
			dirty = true

		case ScyllaJoinConfigured:
			// Waiting for scylla-server.service to start.
			if nodeHasScyllaRunning(node) {
				node.ScyllaJoinPhase = ScyllaJoinStarted
				// Clear replace_address once bootstrap succeeded — it must not
				// persist into subsequent restarts, which would re-trigger a
				// replace of an already-healthy node.
				if node.ScyllaReplaceAddress != "" {
					log.Printf("scylla join: node %s bootstrap succeeded with replace_address; clearing replace_address", node.NodeID)
					node.ScyllaReplaceAddress = ""
				}
				dirty = true
				log.Printf("scylla join: node %s scylla-server started", node.NodeID)
				continue
			}
			if now.Sub(node.ScyllaJoinStartedAt) > scyllaJoinTimeout {
				ip := nodeRoutableIP(node)
				if node.ScyllaJoinRestarts == 0 && ip != "" && node.ScyllaReplaceAddress == "" {
					// First timeout: scylla-server never started. The most common
					// cause is the node's IP still being DN in gossip (it was
					// cleaned without decommissioning). Retry once with
					// replace_address_first_boot so ScyllaDB can claim the DN slot
					// instead of refusing to bootstrap.
					node.ScyllaReplaceAddress = ip
					node.ScyllaJoinPhase = ScyllaJoinNone // will re-enter via prepared
					node.ScyllaJoinRestarts = 1
					node.ScyllaJoinStartedAt = now
					node.ScyllaJoinError = "retrying with replace_address_first_boot (IP may still be DN in gossip ring)"
					dirty = true
					log.Printf("scylla join: node %s timed out waiting for scylla-server — retrying with replace_address_first_boot=%s", node.NodeID, ip)
				} else {
					log.Printf("scylla join: node %s timed out waiting for scylla-server to start", node.NodeID)
					node.ScyllaJoinPhase = ScyllaJoinFailed
					node.ScyllaJoinError = "timeout waiting for scylla-server.service to start"
					dirty = true
				}
			}

		case ScyllaJoinStarted:
			// ScyllaDB is running. Verify it has joined the gossip ring.
			// We require a minimum 30s wait to avoid flapping, then use a
			// real probe if available, falling back to the time-based heuristic.
			elapsed := now.Sub(node.ScyllaJoinStartedAt)
			minWaitMet := elapsed > 30*time.Second

			probeOK := false
			if minWaitMet && m.probeNodeHealth != nil && node.AgentEndpoint != "" {
				probeOK = m.probeNodeHealth(ctx, node.AgentEndpoint)
			}

			if minWaitMet && (probeOK || m.isNodeInRing(node)) {
				node.ScyllaJoinPhase = ScyllaJoinVerified
				node.ScyllaWasEverVerified = true
				node.ScyllaJoinError = ""
				dirty = true
				log.Printf("scylla join: node %s verified in gossip ring (probe=%v)", node.NodeID, probeOK)
				continue
			}
			// Fallback: if no probe is available, use elapsed-only heuristic.
			if minWaitMet && m.probeNodeHealth == nil {
				node.ScyllaJoinPhase = ScyllaJoinVerified
				node.ScyllaWasEverVerified = true
				node.ScyllaJoinError = ""
				dirty = true
				log.Printf("scylla join: node %s verified in gossip ring (heuristic)", node.NodeID)
				continue
			}
			if now.Sub(node.ScyllaJoinStartedAt) > scyllaRaftRestartTimeout {
				if node.AgentEndpoint == "" {
					log.Printf("scylla join: node %s timed out waiting for ring join (no agent endpoint)", node.NodeID)
					node.ScyllaJoinPhase = ScyllaJoinFailed
					node.ScyllaJoinError = "timeout waiting for ScyllaDB to join gossip ring"
					dirty = true
				} else if node.ScyllaJoinRestarts == 0 && m.restartService != nil {
					// First timeout: simple restart to unstick Raft join.
					log.Printf("scylla join: node %s timed out — restarting scylla-server (attempt 1)", node.NodeID)
					if err := m.restartService(ctx, node.AgentEndpoint, "scylla-server.service"); err != nil {
						log.Printf("scylla join: node %s restart failed: %v", node.NodeID, err)
					}
					node.ScyllaJoinRestarts = 1
					node.ScyllaJoinStartedAt = now
					node.ScyllaJoinError = "restarted scylla-server (Raft join stuck)"
					dirty = true
				} else if node.ScyllaJoinRestarts >= 1 && m.wipeScyllaData != nil && !node.ScyllaWasEverVerified {
					// Second timeout: restart didn't help. The stale Raft group
					// state from a failed first boot prevents re-joining. Wipe
					// data and restart for a clean Raft bootstrap.
					// SAFETY: only wipe nodes that have never successfully joined.
					// Existing cluster members regressing through probe failure must
					// never be wiped — that destroys data on healthy nodes.
					log.Printf("scylla join: node %s timed out again — wiping stale Raft data and restarting (attempt %d)",
						node.NodeID, node.ScyllaJoinRestarts+1)
					if err := m.wipeScyllaData(ctx, node.AgentEndpoint); err != nil {
						log.Printf("scylla join: node %s wipe+restart failed: %v", node.NodeID, err)
					}
					node.ScyllaJoinRestarts++
					node.ScyllaJoinStartedAt = now
					node.ScyllaJoinError = "wiped stale Raft data and restarted scylla-server"
					dirty = true
				} else {
					log.Printf("scylla join: node %s timed out waiting for ring join", node.NodeID)
					node.ScyllaJoinPhase = ScyllaJoinFailed
					node.ScyllaJoinError = "timeout waiting for ScyllaDB to join gossip ring"
					dirty = true
				}
			}

		case ScyllaJoinVerified:
			// Detect if ScyllaDB stopped running (e.g. crash, node restart).
			if !nodeHasScyllaRunning(node) {
				node.ScyllaJoinPhase = ScyllaJoinNone
				node.ScyllaJoinError = ""
				dirty = true
				log.Printf("scylla join: node %s scylla-server stopped, resetting to none", node.NodeID)
				continue
			}
			// Re-probe to detect regression (e.g. node fell out of ring).
			if m.probeNodeHealth != nil && node.AgentEndpoint != "" {
				if !m.probeNodeHealth(ctx, node.AgentEndpoint) {
					log.Printf("scylla join: node %s probe regression detected, resetting to started", node.NodeID)
					node.ScyllaJoinPhase = ScyllaJoinStarted
					node.ScyllaJoinStartedAt = now
					// Reset restarts so the restart/wipe pipeline starts fresh.
					// This node was previously verified — ScyllaWasEverVerified
					// will gate the wipe and prevent data loss.
					node.ScyllaJoinRestarts = 0
					node.ScyllaJoinError = "probe regression: node no longer healthy"
					dirty = true
				}
			}
		}
	}

	return dirty
}

// isNodeInRing checks if the node's ScyllaDB instance has joined the gossip ring.
// Uses the seedChecker function which can be overridden for testing.
func (m *scyllaClusterManager) isNodeInRing(node *nodeState) bool {
	if m.seedChecker == nil {
		return false
	}
	ip := nodeRoutableIP(node)
	if ip == "" {
		return false
	}
	// Try to check against any seed node that's already verified.
	// For now, we return false and rely on the time-based heuristic.
	return m.seedChecker("", ip)
}
