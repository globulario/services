package main

import (
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/globulario/services/golang/config"
)

// nodeIPInPool reports whether nodeIP is an admitted member of the MinIO pool
// described by state. Returns false for nil state, empty node list, or empty IP.
//
// Pure function — no I/O. Safe to call from tests without a live cluster.
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
	if err := exec.CommandContext(stopCtx, "systemctl", "stop", "globular-minio.service").Run(); err != nil {
		log.Printf("minio-topology-gate: WARNING: failed to stop globular-minio.service on non-member node %s (ip=%s): %v",
			srv.nodeID, nodeIP, err)
		return
	}
	log.Printf("minio-topology-gate: stopped globular-minio.service on non-member node %s (ip=%s) — "+
		"state=held_not_in_topology; MinIO will start only after apply-topology admits this node",
		srv.nodeID, nodeIP)
}
