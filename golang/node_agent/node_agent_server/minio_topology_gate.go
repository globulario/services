// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.minio_topology_gate
// @awareness file_role=node_local_minio_membership_admission_check_against_controller_published_state
// @awareness enforces=globular.platform:invariant.objectstore.topology_contract
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=critical
package main

// minio_topology_gate.go — the node-agent's read-only check for
// "is this node currently admitted in the MinIO pool the
// controller has published." Returns false for empty/nil state
// without inferring membership — the
// objectstore.local_membership_inference failure mode is exactly
// what happens when a node-agent guesses membership from local
// state instead of from the controller-published
// ObjectStoreDesiredState.
//
// Pure function; no I/O. Adding any side effect — "auto-add
// missing IP", "auto-remove stale entry" — moves authority into
// the node-agent and violates the
// controller-is-sole-authority-for-MinIO-topology contract
// enforced by objectstore_admission.go on the controller side.

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
)

// nodeIPInPool reports whether nodeIP is an admitted member of the MinIO pool
// described by state. Returns false for nil state, empty node list, or empty IP.
//
// Pure function — no I/O. Safe to call from tests without a live cluster.
// Preserved for legacy clusters where AuthorizedMembers is nil.
func nodeIPInPool(nodeIP string, state *config.ObjectStoreDesiredState) bool {
	if state == nil || nodeIP == "" {
		return false
	}
	for _, ip := range state.Nodes {
		if ip == nodeIP {
			return true
		}
	}
	return false
}

// nodeIsTopologyMember is the Phase E.2 node-agent topology gate.
// It returns (true, "") when this node may render active MinIO topology config
// and start globular-minio.service. It returns (false, reason) otherwise.
//
// v2 mode (state.AuthorizedMembers non-nil):
//   - The node must appear in AuthorizedMembers by NodeID.
//   - The entry must have Admitted=true.
//   - The entry's IntentGeneration must match state.Generation.
//
// legacy mode (state.AuthorizedMembers nil):
//   - Falls back to nodeIPInPool (IP-based check) for backward compat.
//
// Phase E.2 invariants enforced here:
//   - "node not listed in approved objectstore topology"  → member missing or empty list
//   - "objectstore generation mismatch; topology not applied" → IntentGeneration drift
//   - "blocked: <reason>"  → Admitted=false (controller set this from lifecycle/intent)
//
// Pure function — no I/O. Safe to call from unit tests without a live cluster.
//
func nodeIsTopologyMember(nodeID, nodeIP string, state *config.ObjectStoreDesiredState) (bool, string) {
	if state == nil {
		return false, "objectstore desired state not available"
	}

	// v2 mode: use explicit NodeID-based authorization list.
	if state.AuthorizedMembers != nil {
		if nodeID == "" {
			return false, "node not listed in approved objectstore topology"
		}
		for _, m := range state.AuthorizedMembers {
			if m.NodeID != nodeID {
				continue
			}
			// Found the node. Check admission and generation.
			if !m.Admitted {
				reason := m.BlockedReason
				if reason == "" {
					reason = "node not yet admitted to objectstore topology"
				}
				return false, fmt.Sprintf("blocked: %s", reason)
			}
			if m.IntentGeneration != uint64(state.Generation) {
				return false, fmt.Sprintf("objectstore generation mismatch; topology not applied (node_gen=%d desired_gen=%d)",
					m.IntentGeneration, state.Generation)
			}
			return true, ""
		}
		// NodeID not found in the authorized list.
		return false, "node not listed in approved objectstore topology"
	}

	// legacy mode: fall back to IP-in-pool check.
	if nodeIPInPool(nodeIP, state) {
		return true, ""
	}
	return false, "node not listed in approved objectstore topology"
}

// enforceMinioHeld stops globular-minio.service if it is currently active and
// this node is not in ObjectStoreDesiredState.Nodes.
//
// Topology contract: MinIO may only run on nodes that are explicitly admitted
// into the pool. A storage-profile node that has the MinIO package installed
// but is not in the pool MUST NOT run MinIO — doing so creates a standalone
// split-brain where each node has an isolated data store.
//
// Safety guarantees:
//   - No data is wiped (only the service is stopped).
//   - No config files are modified.
//   - Stopping is idempotent — if the service is already stopped, this is a no-op.
//   - If systemctl fails, the error is logged but the reconcile loop continues.
//
func (srv *NodeAgentServer) enforceMinioHeld(ctx context.Context, nodeIP string, desiredGen int64) {
	checkCtx, checkCancel := context.WithTimeout(ctx, 3*time.Second)
	defer checkCancel()

	active := exec.CommandContext(checkCtx, "systemctl", "is-active", "--quiet", "globular-minio.service").Run() == nil
	if !active {
		// Already stopped — nothing to do.
		return
	}

	// Service is active on a non-member node. Stop it immediately.
	log.Printf("minio-topology-gate: node %s (ip=%s) is NOT in ObjectStoreDesiredState.Nodes (gen=%d) — "+
		"stopping globular-minio.service to enforce topology contract (held_not_in_topology)",
		srv.nodeID, nodeIP, desiredGen)

	stopCtx, stopCancel := context.WithTimeout(ctx, 15*time.Second)
	defer stopCancel()
	if err := supervisor.Stop(stopCtx, "globular-minio.service"); err != nil {
		log.Printf("minio-topology-gate: WARNING: failed to stop globular-minio.service on non-member node %s (ip=%s): %v",
			srv.nodeID, nodeIP, err)
		return
	}
	log.Printf("minio-topology-gate: stopped globular-minio.service on non-member node %s (ip=%s) — "+
		"state=held_not_in_topology; MinIO will start only after apply-topology admits this node",
		srv.nodeID, nodeIP)
}
