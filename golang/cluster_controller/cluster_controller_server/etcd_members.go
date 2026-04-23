package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdJoinTimeout is the maximum time between MemberAdd and the new member
// becoming healthy. If exceeded, the member is rolled back (removed).
// The 1→2 expansion is especially dangerous because etcd immediately requires
// 2/2 for quorum, so this must be tight.
const etcdJoinTimeout = 2 * time.Minute

// nodeHasEtcdUnit returns true if the node reports a globular-etcd.service
// unit file (any state — active, inactive, or failed).
func nodeHasEtcdUnit(node *nodeState) bool {
	if node == nil {
		return false
	}
	for _, u := range node.Units {
		if u.Name == "globular-etcd.service" {
			return true
		}
	}
	return false
}

// nodeHasEtcdRunning returns true if the node reports globular-etcd.service
// as "active" in its unit list.
func nodeHasEtcdRunning(node *nodeState) bool {
	if node == nil {
		return false
	}
	for _, u := range node.Units {
		if u.Name == "globular-etcd.service" && u.State == "active" {
			return true
		}
	}
	return false
}

// nodeIsPreparedForEtcdJoin checks all preconditions for calling MemberAdd:
//   - node has an etcd profile
//   - etcd package is installed (unit file exists)
//   - node has a routable IP (not empty, not localhost)
//   - node is not already in the live member list
//   - node is not already in an in-progress join (member_added or started phase)
func nodeIsPreparedForEtcdJoin(node *nodeState, existingPeerURLs map[string]bool) bool {
	if node == nil {
		return false
	}
	// Must have etcd profile.
	if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForEtcd) {
		return false
	}
	// Must have the unit file installed (package present on disk).
	if !nodeHasEtcdUnit(node) {
		return false
	}
	// Must have a routable IP.
	ip := nodeRoutableIP(node)
	if ip == "" {
		return false
	}
	// Must not already be a member.
	peerURL := fmt.Sprintf("https://%s:2380", ip)
	if existingPeerURLs[peerURL] {
		return false
	}
	// Must not be mid-join already.
	switch node.EtcdJoinPhase {
	case EtcdJoinMemberAdded, EtcdJoinStarted:
		return false
	}
	// Must be in the etcd_joining bootstrap phase (or workload_ready for
	// backward compatibility with legacy/bootstrap nodes).
	if node.BootstrapPhase != BootstrapNone &&
		node.BootstrapPhase != BootstrapEtcdJoining &&
		node.BootstrapPhase != BootstrapWorkloadReady {
		return false
	}
	return true
}

// nodeRoutableIP returns the preferred non-loopback IP for the node, or "".
// It prefers wired interfaces (eth*, eno*, enp*, ens*, enx*) over WiFi
// for stable cluster addressing. The node-agent already sorts IPs wired-first,
// so this returns the first routable IP from the pre-sorted list.
func nodeRoutableIP(node *nodeState) string {
	if node == nil || len(node.Identity.Ips) == 0 {
		return ""
	}
	for _, ip := range node.Identity.Ips {
		if ip != "" && ip != "127.0.0.1" && ip != "::1" {
			return ip
		}
	}
	return ""
}

// nodeAnyIPIsEtcdMember checks if ANY of the node's IPs matches an existing
// etcd member peer URL. This prevents phase oscillation on multi-IP nodes
// where the etcd join used a different IP than nodeRoutableIP returns.
func nodeAnyIPIsEtcdMember(node *nodeState, existingURLs map[string]bool) bool {
	if node == nil {
		return false
	}
	for _, ip := range node.Identity.Ips {
		if ip == "" || ip == "127.0.0.1" || ip == "::1" {
			continue
		}
		if existingURLs[fmt.Sprintf("https://%s:2380", ip)] {
			return true
		}
	}
	return false
}

// etcdMemberManager handles automatic etcd cluster membership changes.
// It drives the etcd join state machine: nodes transition through
// prepared → member_added → started → verified, with rollback on failure.
type etcdMemberManager struct {
	client *clientv3.Client
}

type etcdMembershipManager interface {
	snapshotEtcdMembers(ctx context.Context) (*etcdMemberState, error)
	reconcileEtcdJoinPhases(ctx context.Context, nodes []*nodeState) (dirty bool)
	removeStaleMembers(ctx context.Context, desiredEtcdNodes []memberNode) error
}

func newEtcdMemberManager(client *clientv3.Client) *etcdMemberManager {
	return &etcdMemberManager{client: client}
}

// snapshotEtcdMembers queries the live etcd cluster and returns the current
// member state. This is used by renderEtcdConfig to set initial-cluster-state.
func (m *etcdMemberManager) snapshotEtcdMembers(ctx context.Context) (*etcdMemberState, error) {
	if m == nil || m.client == nil {
		return &etcdMemberState{Bootstrapped: false}, nil
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}

	state := &etcdMemberState{
		Bootstrapped:   len(resp.Members) > 0,
		MemberPeerURLs: make(map[string]string, len(resp.Members)),
	}
	for _, member := range resp.Members {
		name := member.Name
		if name == "" {
			// Unstarted member (added but not yet started) — use first peer URL.
			if len(member.PeerURLs) > 0 {
				name = member.PeerURLs[0]
			}
			continue
		}
		if len(member.PeerURLs) > 0 {
			state.MemberPeerURLs[name] = member.PeerURLs[0]
		}
	}
	return state, nil
}

// existingPeerURLSet returns the set of peer URLs currently in the live etcd cluster.
func (m *etcdMemberManager) existingPeerURLSet(ctx context.Context) (map[string]bool, error) {
	if m == nil || m.client == nil {
		return nil, nil
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}
	urls := make(map[string]bool, len(resp.Members))
	for _, member := range resp.Members {
		for _, purl := range member.PeerURLs {
			urls[purl] = true
		}
	}
	return urls, nil
}

// memberAdd calls etcd MemberAdd for the given peer URL.
// Returns the new member's ID (for rollback) and any error.
func (m *etcdMemberManager) memberAdd(ctx context.Context, peerURL string) (uint64, error) {
	if m == nil || m.client == nil {
		return 0, fmt.Errorf("etcd client not available")
	}
	addCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := m.client.MemberAdd(addCtx, []string{peerURL})
	if err != nil {
		// If already added (race condition), treat as success.
		if strings.Contains(err.Error(), "already added") ||
			strings.Contains(err.Error(), "Peer URLs already exists") {
			log.Printf("etcd member-add: %s already registered, treating as success", peerURL)
			return 0, nil
		}
		return 0, fmt.Errorf("etcd member add %s: %w", peerURL, err)
	}
	if resp.Member != nil {
		return resp.Member.ID, nil
	}
	return 0, nil
}

// memberRemove calls etcd MemberRemove to roll back a failed join.
func (m *etcdMemberManager) memberRemove(ctx context.Context, memberID uint64) error {
	if m == nil || m.client == nil || memberID == 0 {
		return nil
	}
	rmCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := m.client.MemberRemove(rmCtx, memberID)
	if err != nil {
		return fmt.Errorf("etcd member remove id=%d: %w", memberID, err)
	}
	return nil
}

// reconcileEtcdJoinPhases drives the etcd join state machine for all nodes.
// It is called once per reconciliation cycle.
//
// The flow for a new node:
//  1. prepared: preconditions met → call MemberAdd → transition to member_added
//  2. member_added: config was rendered with live membership → wait for node agent
//     to start etcd → transition to started (detected via unit state "active")
//  3. started: etcd running → verify member is in member list with a name → verified
//  4. On timeout at any phase: call MemberRemove → transition to failed
//
// For 1→2 expansion (single-node cluster adding its first peer), the timeout
// is enforced strictly because quorum requires 2/2 members once MemberAdd is called.
func (m *etcdMemberManager) reconcileEtcdJoinPhases(ctx context.Context, nodes []*nodeState) (dirty bool) {
	if m == nil || m.client == nil {
		return false
	}

	existingURLs, err := m.existingPeerURLSet(ctx)
	if err != nil {
		log.Printf("etcd join: cannot list members: %v", err)
		return false
	}

	now := time.Now()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		// Only process nodes with etcd profiles.
		if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForEtcd) {
			continue
		}

		switch node.EtcdJoinPhase {

		case EtcdJoinNone, EtcdJoinFailed:
			// Observe-only: the join script on the node handles MemberAdd directly.
			// The controller just detects when a node has successfully joined the
			// etcd cluster and marks it as verified. This avoids dangerous
			// controller-initiated MemberAdd calls, especially during 1→2 expansion
			// where quorum requires 2/2 members immediately.
			if nodeAnyIPIsEtcdMember(node, existingURLs) {
				node.EtcdJoinPhase = EtcdJoinVerified
				node.EtcdJoinError = ""
				dirty = true
				log.Printf("etcd join: node %s (%s) detected as existing etcd member, marking verified", node.NodeID, node.Identity.Hostname)
				continue
			}
			// Node is not yet an etcd member — waiting for the join script to
			// run MemberAdd + start etcd. Nothing to do here.

		case EtcdJoinMemberAdded:
			// Waiting for the node agent to start etcd with the rendered config.
			if nodeHasEtcdRunning(node) {
				node.EtcdJoinPhase = EtcdJoinStarted
				dirty = true
				log.Printf("etcd join: node %s etcd service started, transitioning to started", node.NodeID)
				continue
			}
			// Check timeout.
			if now.Sub(node.EtcdJoinStartedAt) > etcdJoinTimeout {
				log.Printf("etcd join: node %s timed out in member_added phase after %v, rolling back", node.NodeID, now.Sub(node.EtcdJoinStartedAt))
				m.rollbackJoin(ctx, node, "timeout waiting for etcd service to start")
				dirty = true
			}

		case EtcdJoinStarted:
			// etcd is running — verify the member appears in the live list with a name.
			// Check ALL node IPs to handle multi-IP nodes (wired + WiFi).
			healthy := false
			for _, ip := range node.Identity.Ips {
				if ip == "" || ip == "127.0.0.1" || ip == "::1" {
					continue
				}
				if m.memberIsHealthy(ctx, fmt.Sprintf("https://%s:2380", ip)) {
					healthy = true
					break
				}
			}
			if healthy {
				node.EtcdJoinPhase = EtcdJoinVerified
				node.EtcdJoinError = ""
				node.EtcdMemberID = 0 // no longer needed for rollback
				dirty = true
				log.Printf("etcd join: node %s verified as healthy etcd member", node.NodeID)
				continue
			}
			// Check timeout.
			if now.Sub(node.EtcdJoinStartedAt) > etcdJoinTimeout {
				log.Printf("etcd join: node %s timed out in started phase after %v, rolling back", node.NodeID, now.Sub(node.EtcdJoinStartedAt))
				m.rollbackJoin(ctx, node, "timeout waiting for etcd member to become healthy")
				dirty = true
			}

		case EtcdJoinVerified:
			// Nothing to do — node is a healthy etcd member.
			// Detect if the member has disappeared (node removal).
			// Must check ALL IPs to avoid false resets on multi-IP nodes.
			if !nodeAnyIPIsEtcdMember(node, existingURLs) && !nodeHasEtcdRunning(node) {
				// Member disappeared — reset to allow re-join.
				node.EtcdJoinPhase = EtcdJoinNone
				node.EtcdJoinError = ""
				dirty = true
				log.Printf("etcd join: node %s member disappeared, resetting to none", node.NodeID)
			}
		}
	}

	return dirty
}

// rollbackJoin removes the etcd member and marks the node as failed.
func (m *etcdMemberManager) rollbackJoin(ctx context.Context, node *nodeState, reason string) {
	if node.EtcdMemberID != 0 {
		if err := m.memberRemove(ctx, node.EtcdMemberID); err != nil {
			log.Printf("etcd join: rollback MemberRemove for %s failed: %v", node.NodeID, err)
			node.EtcdJoinError = fmt.Sprintf("%s; rollback failed: %v", reason, err)
		} else {
			log.Printf("etcd join: rolled back member %s (memberID=%d)", node.NodeID, node.EtcdMemberID)
			node.EtcdJoinError = reason
		}
	} else {
		node.EtcdJoinError = reason
	}
	node.EtcdJoinPhase = EtcdJoinFailed
	node.EtcdMemberID = 0
}

// anyNodeMidJoin returns true if any node (other than excludeID) is in the
// member_added or started phase.
func (m *etcdMemberManager) anyNodeMidJoin(nodes []*nodeState, excludeID string) bool {
	for _, n := range nodes {
		if n == nil || n.NodeID == excludeID {
			continue
		}
		if n.EtcdJoinPhase == EtcdJoinMemberAdded || n.EtcdJoinPhase == EtcdJoinStarted {
			return true
		}
	}
	return false
}

// memberIsHealthy checks if a member with the given peer URL appears in the
// live member list with a name (indicating it has completed raft join).
func (m *etcdMemberManager) memberIsHealthy(ctx context.Context, peerURL string) bool {
	if m == nil || m.client == nil {
		return false
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return false
	}
	for _, member := range resp.Members {
		if member.Name == "" {
			continue // not yet started
		}
		for _, purl := range member.PeerURLs {
			if purl == peerURL {
				return true
			}
		}
	}
	return false
}

// removeStaleMembers removes etcd members whose peer URL doesn't match any
// desired etcd node. This handles node removal from the cluster.
// Skips members that are mid-join (unnamed) to avoid interfering with the join flow.
func (m *etcdMemberManager) removeStaleMembers(ctx context.Context, desiredEtcdNodes []memberNode) error {
	if m == nil || m.client == nil {
		return nil
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return fmt.Errorf("etcd member list: %w", err)
	}

	// Build set of desired peer URLs.
	desiredPeerURLs := make(map[string]bool, len(desiredEtcdNodes))
	for _, node := range desiredEtcdNodes {
		if node.IP != "" {
			desiredPeerURLs[fmt.Sprintf("https://%s:2380", node.IP)] = true
		}
	}

	for _, member := range resp.Members {
		if member.Name == "" {
			continue // unstarted member — might be mid-join, don't remove
		}
		isDesired := false
		for _, purl := range member.PeerURLs {
			if desiredPeerURLs[purl] {
				isDesired = true
				break
			}
		}
		if isDesired {
			continue
		}

		rmCtx, rmCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := m.client.MemberRemove(rmCtx, member.ID)
		rmCancel()
		if err != nil {
			log.Printf("etcd member-remove: failed to remove stale member %s (id=%d): %v", member.Name, member.ID, err)
			continue
		}
		log.Printf("etcd member-remove: removed stale member %s (id=%d, peer=%v)", member.Name, member.ID, member.PeerURLs)
	}

	return nil
}
