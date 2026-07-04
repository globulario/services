package main

import (
	"context"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
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
//
// Phase E-lite: if the node carries an explicit ObjectStoreIntent with
// Member=false, it is excluded immediately regardless of profile. The
// authoritative membership gate (DesiredObjectStoreMembers) is checked
// in reconcileMinioJoinPhases before this function is called.
func nodeIsPreparedForMinioJoin(node *nodeState) bool {
	if node == nil {
		return false
	}
	// Explicit controller exclusion takes precedence over profile.
	if node.ObjectStoreIntent != nil && !node.ObjectStoreIntent.Member {
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
type minioPoolManager struct {
	// probeMinioHealth returns true only when the node's MinIO reports SUBSTRATE
	// TRUTH — write quorum AND read quorum — via the node-agent probe. It is nil
	// when no probe is wired, in which case a node is held PROVISIONAL rather than
	// falsely marked Verified from elapsed time
	// (forbidden_fix:heuristic_signal_marks_substrate_verified).
	probeMinioHealth func(ctx context.Context, endpoint string) bool
}

func newMinioPoolManager() *minioPoolManager {
	return &minioPoolManager{}
}

// minioVerificationUnavailable is the evidence recorded on a node held PROVISIONAL
// because no substrate-truth probe is available to verify pool health.
const minioVerificationUnavailable = "substrate verification unavailable — provisional, not verified"

// markMinioNonMember sets the node to MinioJoinNonMember idempotently and
// reports whether the phase changed.
//
// A non-member node has MinIO correctly held inactive. Recording NonMember —
// rather than leaving the node at the empty MinioJoinNone — is what lets the
// bootstrap storage-join gate (bootstrap_phases.go) SKIP the minio runtime
// check instead of blocking forever. Leaving a confirmed non-member silently
// at MinioJoinNone is a no-op that wedges bootstrap (meta.silence_is_not_valid
// _for_unexpected): the gate only advances on Verified OR NonMember.
func markMinioNonMember(node *nodeState) bool {
	if node.MinioJoinPhase != MinioJoinNonMember {
		node.MinioJoinPhase = MinioJoinNonMember
		return true
	}
	return false
}

// reconcileMinioJoinPhases drives the MinIO join state machine.
//
// Topology contract:
//   - The pool manager may only auto-create MinioPoolNodes when the pool is
//     completely empty (Day-0 bootstrap of the first node).
//   - Once a pool exists, ObjectStoreDesiredState.Nodes is owned by the
//     topology contract: additions require an explicit apply-topology call.
//     A Day-1 storage-profile node that is not yet in MinioPoolNodes is
//     resolved to MinioJoinNonMember (non-blocking) until apply-topology adds
//     it — never left at the empty MinioJoinNone, which would wedge bootstrap.
//
// State flow (for nodes already admitted into the pool):
//  1. prepared: preconditions met
//  2. pool_updated: node IP appended to MinioPoolNodes (bootstrap only)
//  3. started: globular-minio.service active
//  4. verified: pool healthy — write + read quorum via substrate-truth probe
func (m *minioPoolManager) reconcileMinioJoinPhases(ctx context.Context, nodes []*nodeState, state *controllerState) (dirty bool) {
	now := time.Now()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		// Phase E-lite membership gate: when DesiredObjectStoreMembers is non-nil
		// (v2 mode), only explicitly listed nodes are eligible. Legacy clusters with
		// nil desired list fall back to the profile check for backward compat.
		memberStatus := objectStoreMembershipStatus(node, state.DesiredObjectStoreMembers)
		switch memberStatus {
		case "not_listed", "intent_not_member":
			// Explicit-authority invariant (incident 9598b8f7): a node that is
			// not in the desired member list — or explicitly excluded — must NOT
			// be touched here. Inferring/writing membership for such nodes is the
			// non_member cycling failure mode. Leave the phase untouched.
			continue
		case "explicit_desired_state":
			// v2 mode: explicit authorization — skip profile check
		case "legacy_profile_derived":
			// legacy mode: apply profilesForMinio filter as before
			if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForMinio) {
				continue
			}
		}

		switch node.MinioJoinPhase {
		case MinioJoinNone, MinioJoinFailed, MinioJoinNonMember:
			// Topology contract: once a pool exists, a node that is not in it is
			// a held non-member — a Day-1 node awaiting apply-topology admission,
			// or any node before the pool is expanded to it. MinIO is correctly
			// held inactive on such nodes, so mark them NonMember (non-blocking)
			// here, BEFORE the prepared check below.
			//
			// This must precede nodeIsPreparedForMinioJoin: a held node may not
			// have started globular-minio.service yet (the unit may not even be
			// reported), so the prepared check would otherwise strand it at the
			// empty MinioJoinNone and wedge bootstrap at storage_joining forever.
			// Use ANY of the node's IPs — on a VIP holder (keepalived MASTER)
			// nodeRoutableIP returns the floating VIP, never the pool IP.
			if len(state.MinioPoolNodes) > 0 && !nodeAnyIPInPool(node, state.MinioPoolNodes) {
				if markMinioNonMember(node) {
					dirty = true
				}
				continue
			}

			if !nodeIsPreparedForMinioJoin(node) {
				continue
			}

			// Already in the pool list — fast-forward based on service state.
			if nodeAnyIPInPool(node, state.MinioPoolNodes) {
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

			// Reaching here means the pool is empty (the non-empty/not-in-pool
			// case was resolved to NonMember above). Allow auto-pool-creation
			// only during Day-0 bootstrap of the first node.
			log.Printf("minio pool: node %s (%s) is prepared, marking for pool join (Day-0 bootstrap)",
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
				// Safety guard: if another node was appended first (or this node
				// entered MinioJoinPrepared from persisted state before this code
				// was deployed), the pool is now non-empty. Reset to None — the
				// topology contract gate above will hold the node correctly.
				if len(state.MinioPoolNodes) > 0 {
					node.MinioJoinPhase = MinioJoinNone
					dirty = true
					continue
				}
				state.MinioPoolNodes = append(state.MinioPoolNodes, ip)
				state.ObjectStoreGeneration++
				log.Printf("minio pool: appended %s to pool (total %d nodes, gen=%d)",
					ip, len(state.MinioPoolNodes), state.ObjectStoreGeneration)
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
			// MinIO is running — verify pool health. Elapsed time and unit-active
			// state prove only that the process STARTED; they must never prove the
			// erasure set is formed and serving
			// (forbidden_fix:heuristic_signal_marks_substrate_verified). Only a
			// passing substrate-truth probe (write + read quorum) may mark Verified.
			elapsed := now.Sub(node.MinioJoinStartedAt)
			minWaitMet := elapsed > 30*time.Second

			// No substrate-truth probe wired → hold PROVISIONAL: stay started, never
			// Verified, never counted toward durability, never failed merely for
			// lack of a probe. Production always wires the probe (main.go).
			if m.probeMinioHealth == nil {
				if minWaitMet && node.MinioJoinError != minioVerificationUnavailable {
					node.MinioJoinError = minioVerificationUnavailable
					dirty = true
					log.Printf("minio pool: node %s PROVISIONAL — no substrate-truth probe available; not marking verified", node.NodeID)
				}
				continue
			}

			if minWaitMet && node.AgentEndpoint != "" && m.probeMinioHealth(ctx, node.AgentEndpoint) {
				node.MinioJoinPhase = MinioJoinVerified
				node.MinioJoinError = ""
				dirty = true
				log.Printf("minio pool: node %s verified healthy (write+read quorum)", node.NodeID)
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

// resetHeldNonMembersInPool resets MinioJoinPhase from MinioJoinNonMember to
// MinioJoinNone for every node whose IP is in pool, returning the NodeIDs reset.
//
// It unblocks the standalone→distributed grow path. A node held at NonMember
// (because it was not yet in a non-empty pool) is excluded from the published
// objectstore contract by filterEligiblePoolIPsLocked. Once an operator-approved,
// transition-gated apply-topology adds it to MinioPoolNodes, it must be reset to
// None so it re-enters the contract and reconcileMinioJoinPhases can drive it to
// Verified. Without this reset the node deadlocks: excluded from the contract →
// node-agent holds globular-minio.service → nodeHasMinioUnit stays false → never
// prepared → never advances → stays NonMember → excluded.
//
// Scope is deliberately tight:
//   - ONLY nodes whose IP is in pool are reset. Held non-members OUTSIDE the pool
//     keep their NonMember phase, preserving the
//     bootstrap.held_minio_nonmember_stranded_at_none fix (a non-member stranded at
//     None wedges bootstrap storage_joining).
//   - ONLY the NonMember phase is reset; None/Prepared/PoolUpdated/Started/Verified
//     are left untouched.
//
// This is explicit operator authority via the admission/transition record — not the
// forbidden objectstore.local_membership_inference.
func resetHeldNonMembersInPool(nodes map[string]*nodeState, pool []string) []string {
	var reset []string
	for _, node := range nodes {
		if node == nil || node.MinioJoinPhase != MinioJoinNonMember {
			continue
		}
		if nodeAnyIPInPool(node, pool) {
			node.MinioJoinPhase = MinioJoinNone
			reset = append(reset, node.NodeID)
		}
	}
	return reset
}

// nodeAnyIPInPool checks if ANY of the node's IPs is already in the MinIO
// pool list. Mirrors nodeAnyIPIsEtcdMember — a VIP-holding node reports
// multiple IPs (VIP + stable interface IPs) and nodeRoutableIP may return
// the VIP, which never matches the stable IP recorded in MinioPoolNodes.
func nodeAnyIPInPool(node *nodeState, pool []string) bool {
	if node == nil {
		return false
	}
	for _, ip := range node.Identity.Ips {
		if ip == "" || config.IsLoopbackEndpoint(ip) {
			continue
		}
		if ipInPool(ip, pool) {
			return true
		}
	}
	return false
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
