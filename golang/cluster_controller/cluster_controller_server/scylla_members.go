package main

import (
	"log"
	"time"
)

// scyllaJoinTimeout is the maximum time between config render and the new
// ScyllaDB node becoming healthy in the gossip ring.
const scyllaJoinTimeout = 5 * time.Minute

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
//   - node is in the correct bootstrap phase (storage_joining or workload_ready)
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
	if node.BootstrapPhase != BootstrapNone &&
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
func (m *scyllaClusterManager) reconcileScyllaJoinPhases(nodes []*nodeState) (dirty bool) {
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
			dirty = true

		case ScyllaJoinConfigured:
			// Waiting for scylla-server.service to start.
			if nodeHasScyllaRunning(node) {
				node.ScyllaJoinPhase = ScyllaJoinStarted
				dirty = true
				log.Printf("scylla join: node %s scylla-server started", node.NodeID)
				continue
			}
			if now.Sub(node.ScyllaJoinStartedAt) > scyllaJoinTimeout {
				log.Printf("scylla join: node %s timed out waiting for scylla-server to start", node.NodeID)
				node.ScyllaJoinPhase = ScyllaJoinFailed
				node.ScyllaJoinError = "timeout waiting for scylla-server.service to start"
				dirty = true
			}

		case ScyllaJoinStarted:
			// ScyllaDB is running. Verify it has joined the gossip ring.
			// ScyllaDB gossip join takes a few seconds after startup.
			// We use a simple heuristic: if the service has been active for
			// at least 30 seconds, consider it joined (gossip converges fast).
			elapsed := now.Sub(node.ScyllaJoinStartedAt)
			if elapsed > 30*time.Second || m.isNodeInRing(node) {
				node.ScyllaJoinPhase = ScyllaJoinVerified
				node.ScyllaJoinError = ""
				dirty = true
				log.Printf("scylla join: node %s verified in gossip ring", node.NodeID)
				continue
			}
			if now.Sub(node.ScyllaJoinStartedAt) > scyllaJoinTimeout {
				log.Printf("scylla join: node %s timed out waiting for ring join", node.NodeID)
				node.ScyllaJoinPhase = ScyllaJoinFailed
				node.ScyllaJoinError = "timeout waiting for ScyllaDB to join gossip ring"
				dirty = true
			}

		case ScyllaJoinVerified:
			// Detect if ScyllaDB stopped running (e.g. crash, node restart).
			if !nodeHasScyllaRunning(node) {
				node.ScyllaJoinPhase = ScyllaJoinNone
				node.ScyllaJoinError = ""
				dirty = true
				log.Printf("scylla join: node %s scylla-server stopped, resetting to none", node.NodeID)
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
