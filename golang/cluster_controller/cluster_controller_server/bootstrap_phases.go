package main

import (
	"log"
	"time"
)

// bootstrapPhaseTimeout is the maximum time a node may spend in any single
// bootstrap phase before being marked as failed.
const bootstrapPhaseTimeout = 5 * time.Minute

// eventEmitter is the subset of server used by the bootstrap state machine.
type eventEmitter interface {
	emitClusterEvent(eventType string, data map[string]interface{})
}

// reconcileBootstrapPhases drives the bootstrap state machine for all nodes
// that have not yet reached workload_ready. It is called once per reconcile cycle.
//
// Phase transitions:
//
//	admitted        → infra_preparing  (immediate)
//	infra_preparing → etcd_joining     (etcd unit file present, or skip if no etcd profile)
//	etcd_joining    → etcd_ready       (EtcdJoinPhase == verified)
//	etcd_ready      → xds_ready        (globular-xds.service active, or skip)
//	xds_ready       → envoy_ready      (globular-envoy.service active, or skip)
//	envoy_ready     → workload_ready   (immediate)
//
// Phases are skipped when the node's profiles don't include the relevant service.
// Returns true if any node state was modified.
func reconcileBootstrapPhases(nodes []*nodeState, emitter eventEmitter) (dirty bool) {
	now := time.Now()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		// Skip nodes that are fully done or legacy.
		if node.BootstrapPhase == BootstrapNone || node.BootstrapPhase == BootstrapWorkloadReady {
			continue
		}
		// Skip failed nodes (require manual reset).
		if node.BootstrapPhase == BootstrapFailed {
			continue
		}

		oldPhase := node.BootstrapPhase

		switch node.BootstrapPhase {
		case BootstrapAdmitted:
			// Immediate transition to infra_preparing.
			node.BootstrapPhase = BootstrapInfraPreparing
			node.BootstrapStartedAt = now
			node.BootstrapError = ""
			dirty = true

		case BootstrapInfraPreparing:
			// Wait for infrastructure packages to be installed.
			// Signal: etcd unit file present (for etcd-profiled nodes),
			// or skip etcd phases entirely if node has no etcd profile.
			if nodeHasEtcdProfile(node) {
				if nodeHasEtcdUnit(node) {
					node.BootstrapPhase = BootstrapEtcdJoining
					node.BootstrapStartedAt = now
					dirty = true
				} else if phaseTimedOut(node, now) {
					failBootstrap(node, "timeout waiting for etcd package installation")
					dirty = true
				}
			} else {
				// No etcd profile — skip etcd phases, advance to post-etcd.
				node.BootstrapPhase = advancePastEtcd(node, now)
				node.BootstrapStartedAt = now
				dirty = true
			}

		case BootstrapEtcdJoining:
			// Wait for the etcd join state machine to reach verified.
			// The etcd state machine (reconcileEtcdJoinPhases) runs separately.
			if node.EtcdJoinPhase == EtcdJoinVerified {
				node.BootstrapPhase = BootstrapEtcdReady
				node.BootstrapStartedAt = now
				dirty = true
			} else if node.EtcdJoinPhase == EtcdJoinFailed {
				failBootstrap(node, "etcd join failed: "+node.EtcdJoinError)
				dirty = true
			} else if phaseTimedOut(node, now) {
				failBootstrap(node, "timeout waiting for etcd join")
				dirty = true
			}

		case BootstrapEtcdReady:
			// etcd is verified. Wait for xDS to come up.
			if nodeHasXdsProfile(node) {
				if nodeHasUnitActive(node, "globular-xds.service") {
					node.BootstrapPhase = BootstrapXdsReady
					node.BootstrapStartedAt = now
					dirty = true
				} else if phaseTimedOut(node, now) {
					failBootstrap(node, "timeout waiting for xDS service")
					dirty = true
				}
			} else {
				// No xds profile — skip to envoy or workload.
				node.BootstrapPhase = advancePastXds(node, now)
				node.BootstrapStartedAt = now
				dirty = true
			}

		case BootstrapXdsReady:
			// xDS is active. Wait for Envoy.
			if nodeHasEnvoyProfile(node) {
				if nodeHasUnitActive(node, "globular-envoy.service") {
					node.BootstrapPhase = BootstrapEnvoyReady
					node.BootstrapStartedAt = now
					dirty = true
				} else if phaseTimedOut(node, now) {
					failBootstrap(node, "timeout waiting for Envoy service")
					dirty = true
				}
			} else {
				// No envoy/gateway profile — skip to storage or workload.
				node.BootstrapPhase = advancePastEnvoy(node)
				node.BootstrapStartedAt = now
				dirty = true
			}

		case BootstrapEnvoyReady:
			// Envoy is active — check if node hosts storage services.
			if nodeNeedsStorageJoin(node) {
				node.BootstrapPhase = BootstrapStorageJoining
				node.BootstrapStartedAt = now
				node.BootstrapError = ""
				dirty = true
			} else {
				node.BootstrapPhase = BootstrapWorkloadReady
				node.BootstrapStartedAt = now
				node.BootstrapError = ""
				dirty = true
			}

		case BootstrapStorageJoining:
			// Verify storage services are active and healthy.
			// MinIO: globular-minio.service must be active.
			// ScyllaDB: ScyllaJoinPhase must be verified (gossip ring joined).
			allReady := true
			var waiting string

			if nodeHasMinioProfile(node) {
				if node.MinioJoinPhase != MinioJoinVerified {
					allReady = false
					waiting = "globular-minio.service (join phase: " + string(node.MinioJoinPhase) + ")"
				}
			}
			if nodeHasScyllaProfile(node) {
				if node.ScyllaJoinPhase != ScyllaJoinVerified {
					allReady = false
					waiting = "scylla-server.service (join phase: " + string(node.ScyllaJoinPhase) + ")"
				}
			}

			if allReady {
				node.BootstrapPhase = BootstrapWorkloadReady
				node.BootstrapStartedAt = now
				node.BootstrapError = ""
				dirty = true
			} else if phaseTimedOut(node, now) {
				failBootstrap(node, "timeout waiting for storage service: "+waiting)
				dirty = true
			}
		}

		// Emit event on phase transition.
		if node.BootstrapPhase != oldPhase && emitter != nil {
			log.Printf("bootstrap: node %s (%s) phase %s → %s",
				node.NodeID, node.Identity.Hostname, oldPhase, node.BootstrapPhase)
			emitter.emitClusterEvent("node.bootstrap_phase_changed", map[string]interface{}{
				"severity":       "INFO",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"from_phase":     string(oldPhase),
				"to_phase":       string(node.BootstrapPhase),
				"correlation_id": "bootstrap:" + node.NodeID,
			})
		}
	}

	return dirty
}

// phaseTimedOut returns true if the node has been in its current bootstrap
// phase longer than bootstrapPhaseTimeout.
func phaseTimedOut(node *nodeState, now time.Time) bool {
	if node.BootstrapStartedAt.IsZero() {
		return false
	}
	return now.Sub(node.BootstrapStartedAt) > bootstrapPhaseTimeout
}

// failBootstrap marks a node as bootstrap_failed with the given reason.
func failBootstrap(node *nodeState, reason string) {
	log.Printf("bootstrap: node %s (%s) failed: %s", node.NodeID, node.Identity.Hostname, reason)
	node.BootstrapPhase = BootstrapFailed
	node.BootstrapError = reason
}

// advancePastEtcd returns the next bootstrap phase after skipping etcd.
// Depends on whether the node has xds/envoy profiles.
func advancePastEtcd(node *nodeState, now time.Time) BootstrapPhase {
	if nodeHasXdsProfile(node) {
		return BootstrapEtcdReady // will wait for xDS in next cycle
	}
	return advancePastXds(node, now)
}

// advancePastXds returns the next phase after skipping xDS.
func advancePastXds(node *nodeState, now time.Time) BootstrapPhase {
	if nodeHasEnvoyProfile(node) {
		return BootstrapXdsReady // will wait for envoy in next cycle
	}
	return advancePastEnvoy(node)
}

// advancePastEnvoy returns the next phase after skipping Envoy.
func advancePastEnvoy(node *nodeState) BootstrapPhase {
	if nodeNeedsStorageJoin(node) {
		return BootstrapStorageJoining
	}
	return BootstrapWorkloadReady
}

// --- Profile and unit helpers ---

// nodeHasEtcdProfile returns true if the node has a profile that runs etcd.
func nodeHasEtcdProfile(node *nodeState) bool {
	return nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForEtcd)
}

// nodeHasXdsProfile returns true if the node has a profile that runs xDS.
func nodeHasXdsProfile(node *nodeState) bool {
	return nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForXDS)
}

// nodeHasEnvoyProfile returns true if the node has a gateway profile (runs Envoy).
func nodeHasEnvoyProfile(node *nodeState) bool {
	for _, p := range node.Profiles {
		if p == "gateway" {
			return true
		}
	}
	return false
}

// nodeHasUnitActive returns true if the node reports the given unit as "active".
func nodeHasUnitActive(node *nodeState, unitName string) bool {
	for _, u := range node.Units {
		if u.Name == unitName && u.State == "active" {
			return true
		}
	}
	return false
}

// profilesForMinio lists the profiles that run MinIO.
// Defined in service_config.go: core, compute, storage.
var profilesForStorage = []string{"core", "compute", "storage"}

// profilesForScylla lists the profiles that run ScyllaDB.
var profilesForScylla = []string{"scylla", "database"}

// nodeHasMinioProfile returns true if the node has a profile that runs MinIO.
func nodeHasMinioProfile(node *nodeState) bool {
	return nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForStorage)
}

// nodeHasScyllaProfile returns true if the node has a scylla or database profile.
func nodeHasScyllaProfile(node *nodeState) bool {
	return nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForScylla)
}

// nodeNeedsStorageJoin returns true if the node hosts storage services
// (MinIO or ScyllaDB) that need explicit join verification.
func nodeNeedsStorageJoin(node *nodeState) bool {
	return nodeHasMinioProfile(node) || nodeHasScyllaProfile(node)
}
