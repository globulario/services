package main

import (
	"log"
	"time"
)

// minioJoinTimeout is the maximum time for a MinIO node to join the pool
// and become healthy.
const minioJoinTimeout = 5 * time.Minute

// nodeHasMinioUnit returns true if the node reports globular-minio.service
// unit file (any state).
func nodeHasMinioUnit(node *nodeState) bool {
	if node == nil {
		return false
	}
	for _, u := range node.Units {
		if u.Name == "globular-minio.service" {
			return true
		}
	}
	return false
}

// nodeHasMinioRunning returns true if globular-minio.service is "active".
func nodeHasMinioRunning(node *nodeState) bool {
	if node == nil {
		return false
	}
	for _, u := range node.Units {
		if u.Name == "globular-minio.service" && u.State == "active" {
			return true
		}
	}
	return false
}

// nodeIsPreparedForMinioJoin checks all preconditions:
//   - node has a storage/core/compute profile (runs MinIO)
//   - globular-minio.service unit exists
//   - node has a routable IP
//   - node is not mid-join
//   - node is in correct bootstrap phase
func nodeIsPreparedForMinioJoin(node *nodeState) bool {
	if node == nil {
		return false
	}
	if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForMinio) {
		return false
	}
	if !nodeHasMinioUnit(node) {
		return false
	}
	ip := nodeRoutableIP(node)
	if ip == "" {
		return false
	}
	switch node.MinioJoinPhase {
	case MinioJoinPoolUpdated, MinioJoinStarted:
		return false
	}
	if node.BootstrapPhase != BootstrapNone &&
		node.BootstrapPhase != BootstrapStorageJoining &&
		node.BootstrapPhase != BootstrapWorkloadReady {
		return false
	}
	return true
}

// minioPoolManager drives MinIO pool expansion.
// MinIO erasure sets are fixed at creation — expansion appends new nodes
// to the ordered pool list and restarts all nodes with the updated config.
type minioPoolManager struct{}

func newMinioPoolManager() *minioPoolManager {
	return &minioPoolManager{}
}

// reconcileMinioJoinPhases drives the MinIO join state machine.
//
// The flow:
//  1. prepared: preconditions met
//  2. pool_updated: node IP appended to MinioPoolNodes (triggers config re-render for all nodes)
//  3. started: globular-minio.service active
//  4. verified: service healthy (active for >30s)
func (m *minioPoolManager) reconcileMinioJoinPhases(nodes []*nodeState, state *controllerState) (dirty bool) {
	now := time.Now()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForMinio) {
			continue
		}

		switch node.MinioJoinPhase {
		case MinioJoinNone, MinioJoinFailed:
			if !nodeIsPreparedForMinioJoin(node) {
				continue
			}
			ip := nodeRoutableIP(node)

			// Check if already in the pool list.
			if ipInPool(ip, state.MinioPoolNodes) {
				// Already in pool — fast-forward based on service state.
				if nodeHasMinioRunning(node) {
					node.MinioJoinPhase = MinioJoinVerified
					node.MinioJoinError = ""
				} else {
					node.MinioJoinPhase = MinioJoinPoolUpdated
					node.MinioJoinStartedAt = now
					node.MinioJoinError = ""
				}
				dirty = true
				continue
			}

			log.Printf("minio pool: node %s (%s) is prepared, marking for pool join",
				node.NodeID, node.Identity.Hostname)
			node.MinioJoinPhase = MinioJoinPrepared
			node.MinioJoinStartedAt = now
			node.MinioJoinError = ""
			dirty = true

		case MinioJoinPrepared:
			// Append node IP to the ordered pool list.
			ip := nodeRoutableIP(node)
			if ip == "" {
				continue
			}
			if !ipInPool(ip, state.MinioPoolNodes) {
				state.MinioPoolNodes = append(state.MinioPoolNodes, ip)
				log.Printf("minio pool: appended %s to pool (total %d nodes)",
					ip, len(state.MinioPoolNodes))
			}
			node.MinioJoinPhase = MinioJoinPoolUpdated
			dirty = true
			// Note: the next reconcile cycle will re-render configs for ALL
			// MinIO nodes (the pool list changed → config hash changes →
			// restart triggered by restartActionsForChangedConfigs).

		case MinioJoinPoolUpdated:
			// Wait for globular-minio.service to start.
			if nodeHasMinioRunning(node) {
				node.MinioJoinPhase = MinioJoinStarted
				node.MinioJoinStartedAt = now
				dirty = true
				log.Printf("minio pool: node %s minio started", node.NodeID)
				continue
			}
			if now.Sub(node.MinioJoinStartedAt) > minioJoinTimeout {
				log.Printf("minio pool: node %s timed out waiting for minio to start", node.NodeID)
				node.MinioJoinPhase = MinioJoinFailed
				node.MinioJoinError = "timeout waiting for globular-minio.service to start"
				dirty = true
			}

		case MinioJoinStarted:
			// MinIO is running — verify it's healthy.
			// Heuristic: active for >30s means erasure set formed.
			elapsed := now.Sub(node.MinioJoinStartedAt)
			if elapsed > 30*time.Second {
				node.MinioJoinPhase = MinioJoinVerified
				node.MinioJoinError = ""
				dirty = true
				log.Printf("minio pool: node %s verified healthy", node.NodeID)
				continue
			}
			if now.Sub(node.MinioJoinStartedAt) > minioJoinTimeout {
				node.MinioJoinPhase = MinioJoinFailed
				node.MinioJoinError = "timeout waiting for MinIO health verification"
				dirty = true
			}

		case MinioJoinVerified:
			// Detect if MinIO stopped.
			if !nodeHasMinioRunning(node) {
				node.MinioJoinPhase = MinioJoinNone
				node.MinioJoinError = ""
				dirty = true
				log.Printf("minio pool: node %s minio stopped, resetting", node.NodeID)
			}
		}
	}

	return dirty
}

// ipInPool checks if an IP is already in the pool list.
func ipInPool(ip string, pool []string) bool {
	for _, p := range pool {
		if p == ip {
			return true
		}
	}
	return false
}
