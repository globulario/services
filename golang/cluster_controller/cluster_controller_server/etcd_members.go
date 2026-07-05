// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.etcd_members
// @awareness file_role=etcd_member_add_remove_with_quorum_safe_rollback_and_bounded_join_timeout
// @awareness implements=globular.platform:intent.etcd.is_source_of_truth
// @awareness implements=globular.platform:intent.infrastructure.etcd.quorum_backed_config_authority
// @awareness enforces=globular.platform:invariant.founding.quorum.three_nodes_required
// @awareness risk=critical
package main

// etcd_members.go — the quorum-critical path. Two non-negotiable
// properties:
//
//  1. 1→2 expansion: etcd immediately requires 2/2 for quorum
//     after MemberAdd. If the new member fails to come online
//     within etcdJoinTimeout (2 min), MemberRemove must roll back
//     so the cluster does not deadlock on a quorum it cannot
//     reach. Lengthening that timeout to "fix" slow nodes
//     re-introduces the deadlock window — fix the slow node, not
//     the timeout.
//
//  2. Removal: only ever in response to an explicit operator
//     removal request (see node_removal_requests.go) — NEVER from
//     heartbeat staleness or doctor findings. Even an obviously
//     dead member must wait for the audited removal record. The
//     keepalived/VIP eviction cascade (StableIP vs PrimaryIP) is
//     exactly the class of bug auto-eviction would re-introduce.

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdJoinTimeout is the maximum time between MemberAdd and the new member
// becoming healthy. If exceeded, the member is rolled back (removed).
// The 1→2 expansion is especially dangerous because etcd immediately requires
// 2/2 for quorum, so this must be tight.
const etcdJoinTimeout = 2 * time.Minute

// etcdStuckJoinThreshold is how long a node must remain in BootstrapEtcdJoining
// without joining before being classified as rejoin_required. Must be larger
// than bootstrapPhaseTimeout (5 min) to avoid false positives, but short enough
// to surface the problem quickly.
const etcdStuckJoinThreshold = 10 * time.Minute

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

func nodeAllRoutableIPs(node *nodeState) []string {
	if node == nil {
		return nil
	}
	out := make([]string, 0, len(node.Identity.Ips))
	seen := make(map[string]struct{}, len(node.Identity.Ips))
	for _, ip := range node.Identity.Ips {
		if ip == "" || ip == "127.0.0.1" || ip == "::1" {
			continue
		}
		// Skip container/VM bridge ranges (172.16-31.x, 192.168.122.x Docker/libvirt).
		// These are local-only interfaces that will never reach remote cluster peers.
		if isContainerBridgeIP(ip) {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	return out
}

// isContainerBridgeIP returns true for IPs in bridge ranges used by Docker,
// libvirt, and similar container runtimes that are not reachable cluster-wide.
func isContainerBridgeIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	// 172.16.0.0/12 — Docker default bridge range
	_, bridge, _ := net.ParseCIDR("172.16.0.0/12")
	// 192.168.122.0/24 — libvirt default NAT network
	_, libvirt, _ := net.ParseCIDR("192.168.122.0/24")
	return bridge.Contains(parsed) || libvirt.Contains(parsed)
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

// resolveEtcdLeaderNode maps a reported etcd leader member id to a node id and
// reports whether the leader was CONFIDENTLY identified. It is the fail-closed
// gate for the "never auto-rejoin the etcd leader" safety check: leaderKnown is
// false — and the caller MUST refuse all destructive auto-rejoins this cycle —
// when the leader id is 0 (no leader elected / quorum lost) or the reported
// leader is not a current member with a peer URL. A leaderKnown=true with an
// empty leaderNodeID is legitimate and safe: the leader is a real member that is
// simply not in the candidate `nodes` set, so it will not be wiped.
//
// memberPeerURLs maps each current member id to its first peer URL (members with
// no peer URL are intentionally absent, which fails closed for that leader).
func resolveEtcdLeaderNode(leaderID uint64, memberPeerURLs map[uint64]string, nodes []*nodeState) (leaderNodeID string, leaderKnown bool) {
	if leaderID == 0 {
		return "", false
	}
	peerURL, ok := memberPeerURLs[leaderID]
	if !ok {
		return "", false // reported leader is not a current member we can map
	}
	leaderKnown = true
	urlSet := map[string]bool{peerURL: true}
	for _, n := range nodes {
		if n != nil && nodeAnyIPIsEtcdMember(n, urlSet) {
			leaderNodeID = n.NodeID
		}
	}
	return leaderNodeID, leaderKnown
}

// etcdClientAPI is the subset of *clientv3.Client that the etcd member manager
// uses. Extracting it lets the join/promotion FSM (learner-first work) be
// unit-tested against a fake etcd — quorum-mutating logic must be provable
// without a live cluster. *clientv3.Client satisfies this directly (methods are
// promoted from its embedded Cluster/KV/Maintenance interfaces).
type etcdClientAPI interface {
	Endpoints() []string
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	MemberList(ctx context.Context) (*clientv3.MemberListResponse, error)
	MemberAdd(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)
	MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)
	MemberRemove(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error)
	MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*clientv3.MemberUpdateResponse, error)
	MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error)
	Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error)
}

// etcdMemberManager handles automatic etcd cluster membership changes.
// It drives the etcd join state machine: nodes transition through
// prepared → member_added → started → verified, with rollback on failure.
type etcdMemberManager struct {
	client etcdClientAPI
}

type etcdMembershipManager interface {
	snapshotEtcdMembers(ctx context.Context) (*etcdMemberState, error)
	reconcileEtcdJoinPhases(ctx context.Context, nodes []*nodeState) (dirty bool)
	reconcileEtcdAutoRejoin(ctx context.Context, nodes []*nodeState) (dirty bool)
	removeStaleMembers(ctx context.Context, desiredEtcdNodes []memberNode) error
	reconcileLearnerPromotion(ctx context.Context, targetVoters int) (dirty bool)
	voterClientEndpoints(ctx context.Context) ([]string, error)
}

func newEtcdMemberManager(client *clientv3.Client) *etcdMemberManager {
	m := &etcdMemberManager{}
	// Preserve a nil interface (not a typed-nil *clientv3.Client) so the
	// existing `m.client == nil` guards keep working when there is no client.
	if client != nil {
		m.client = client
	}
	return m
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

// namedMemberPeerURLSet returns the set of peer URLs for named (started) etcd
// members only. Unlike existingPeerURLSet, it excludes ghost members — unnamed
// entries added by MemberAdd that have not yet started etcd and joined raft.
func (m *etcdMemberManager) namedMemberPeerURLSet(ctx context.Context) (map[string]bool, error) {
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
		if member.Name == "" {
			continue // skip unnamed ghost members
		}
		for _, purl := range member.PeerURLs {
			urls[purl] = true
		}
	}
	return urls, nil
}

// classifyStuckEtcdJoin returns true if a node appears permanently stuck in
// etcd_joining: it has been in BootstrapEtcdJoining for longer than
// etcdStuckJoinThreshold, is not a named etcd member, and etcd is not running.
//
// namedURLs must contain only named (started) members so that ghost members
// (unnamed entries from a prior MemberAdd) are also detected as stuck.
// Use namedMemberPeerURLSet, NOT existingPeerURLSet.
func classifyStuckEtcdJoin(node *nodeState, namedURLs map[string]bool, now time.Time) bool {
	if node == nil {
		return false
	}
	switch node.EtcdJoinPhase {
	case EtcdJoinNone, EtcdJoinFailed:
		// only classify from these base states
	default:
		return false
	}
	if node.BootstrapPhase != BootstrapEtcdJoining {
		return false
	}
	if node.BootstrapStartedAt.IsZero() {
		return false
	}
	if now.Sub(node.BootstrapStartedAt) < etcdStuckJoinThreshold {
		return false
	}
	if nodeAnyIPIsEtcdMember(node, namedURLs) {
		return false // already a healthy named member
	}
	return !nodeHasEtcdRunning(node)
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

// etcdMemberRole is the live role of a named etcd member, keyed by peer URL.
type etcdMemberRole struct {
	id        uint64
	isLearner bool
}

// namedMemberRoles returns peerURL -> role for every NAMED etcd member (unnamed
// ghost members are skipped). This is the live-membership truth used to project a
// node's join phase (task#17 e): a learner is NOT a voter and must never be
// reported as verified.
func (m *etcdMemberManager) namedMemberRoles(ctx context.Context) (map[string]etcdMemberRole, error) {
	if m == nil || m.client == nil {
		return nil, nil
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}
	roles := make(map[string]etcdMemberRole, len(resp.Members))
	for _, member := range resp.Members {
		if member.Name == "" {
			continue // skip unnamed ghost members
		}
		for _, purl := range member.PeerURLs {
			roles[purl] = etcdMemberRole{id: member.ID, isLearner: member.IsLearner}
		}
	}
	return roles, nil
}

// voterClientEndpoints returns the client endpoints (https://IP:2379) of all
// NON-learner (voting) etcd members, from live MemberList truth. It deliberately
// EXCLUDES learners — a learner refuses client RPCs ("rpc not supported for
// learner"), so a joining node whose own local member is a learner must use this
// authoritative voter list as its desired-state authority endpoint(s) rather than
// depending on its local learner. Returns an error (never a silently-empty list)
// when membership cannot be read, so callers can stay degraded/pending with a
// precise reason instead of guessing endpoints.
func (m *etcdMemberManager) voterClientEndpoints(ctx context.Context) ([]string, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("etcd member manager not initialised")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}
	seen := map[string]bool{}
	eps := make([]string, 0, len(resp.Members))
	for _, member := range resp.Members {
		if member.Name == "" || member.IsLearner {
			continue // skip unnamed ghost members and non-voting learners
		}
		for _, curl := range member.ClientURLs {
			curl = strings.TrimSpace(curl)
			if curl == "" || seen[curl] {
				continue
			}
			seen[curl] = true
			eps = append(eps, curl)
		}
	}
	sort.Strings(eps)
	return eps, nil
}

// matchNodeEtcdMemberRole returns the live role of the etcd member for this node,
// matched by any of the node's peer URLs (https://<ip>:2380), and whether a match
// was found.
func matchNodeEtcdMemberRole(node *nodeState, roles map[string]etcdMemberRole) (etcdMemberRole, bool) {
	for _, ip := range node.Identity.Ips {
		if ip == "" || ip == "127.0.0.1" || ip == "::1" {
			continue
		}
		if r, ok := roles[fmt.Sprintf("https://%s:2380", ip)]; ok {
			return r, true
		}
	}
	return etcdMemberRole{}, false
}

// projectEtcdRolePhase sets node.EtcdJoinPhase from the live member role and
// returns whether it changed anything (task#17 e). This is the single source of
// truth for the transitional topology:
//   - LEARNER -> EtcdJoinPromoting, and the member ID is PRESERVED (needed for
//     promotion tracking and safe removal of a failed pre-promotion learner).
//   - VOTER   -> EtcdJoinVerified (promoted, participating).
//
// It never fabricates verified from a learner, and never clears EtcdMemberID
// before verification is confirmed.
func projectEtcdRolePhase(node *nodeState, role etcdMemberRole) (changed bool) {
	if role.isLearner {
		if node.EtcdMemberID != role.id {
			node.EtcdMemberID = role.id
			changed = true
		}
		if node.EtcdJoinPhase != EtcdJoinPromoting {
			node.EtcdJoinPhase = EtcdJoinPromoting
			node.EtcdJoinError = ""
			changed = true
		}
		return changed
	}
	if node.EtcdJoinPhase != EtcdJoinVerified {
		node.EtcdJoinPhase = EtcdJoinVerified
		node.EtcdJoinError = ""
		changed = true
	}
	return changed
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

	// namedURLs excludes ghost members (unnamed). Used for stuck detection and
	// for the None→Verified shortcut, so that an unstarted ghost doesn't
	// prematurely mark a node as verified.
	namedURLs, err := m.namedMemberPeerURLSet(ctx)
	if err != nil {
		log.Printf("etcd join: cannot list named members (using all): %v", err)
		namedURLs = existingURLs
	}

	// Live member roles (learner vs voter) — the truth used to project a node's
	// join phase (task#17 e). On error, namedRoles is empty and the role-aware
	// projection is skipped this cycle (the phase is left unchanged, never faked).
	namedRoles, rerr := m.namedMemberRoles(ctx)
	if rerr != nil {
		log.Printf("etcd join: cannot list member roles (skipping role projection this cycle): %v", rerr)
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
			// The controller projects the node's phase from the LIVE member role
			// (task#17 e), never from script assumptions: a learner becomes
			// EtcdJoinPromoting (awaiting promotion), a voter becomes
			// EtcdJoinVerified. This avoids dangerous controller-initiated MemberAdd
			// and — critically — never reports a non-voting learner as verified.
			if role, ok := matchNodeEtcdMemberRole(node, namedRoles); ok {
				if projectEtcdRolePhase(node, role) {
					dirty = true
					log.Printf("etcd join: node %s (%s) projected from live member role (learner=%v) -> %s",
						node.NodeID, node.Identity.Hostname, role.isLearner, node.EtcdJoinPhase)
				}
				continue
			}
			// Check for permanently stuck join: WAL "removed from cluster" or ghost
			// member scenario. Transition to rejoin_required so the operator is
			// alerted. No destructive action is taken automatically.
			if classifyStuckEtcdJoin(node, namedURLs, now) {
				node.EtcdJoinPhase = EtcdJoinRejoinRequired
				node.EtcdJoinError = fmt.Sprintf(
					"stuck in etcd_joining for %v: etcd not running and not in member list; "+
						"run 'globular node repair-etcd --node %s --wipe-local-etcd'",
					now.Sub(node.BootstrapStartedAt).Round(time.Second),
					node.Identity.Hostname,
				)
				dirty = true
				log.Printf("etcd join: node %s (%s) classified as rejoin_required: %s",
					node.NodeID, node.Identity.Hostname, node.EtcdJoinError)
				continue
			}
			// Node is not yet an etcd member — waiting for the join script to
			// run MemberAdd + start etcd. Nothing to do here.

		case EtcdJoinRejoinRequired:
			// Operator repair needed. Detect if the node has manually recovered
			// (e.g., by running the gateway join script directly).
			if nodeAnyIPIsEtcdMember(node, namedURLs) {
				node.EtcdJoinPhase = EtcdJoinVerified
				node.EtcdJoinError = ""
				dirty = true
				log.Printf("etcd join: node %s (%s) recovered from rejoin_required, marking verified",
					node.NodeID, node.Identity.Hostname)
			}
			// Otherwise stay in rejoin_required; bootstrap_phases.go resets the
			// timeout clock so the node doesn't get failed while waiting for repair.

		case EtcdJoinRejoinInProgress:
			// A repair workflow is running — check for completion.
			if nodeAnyIPIsEtcdMember(node, namedURLs) && nodeHasEtcdRunning(node) {
				node.EtcdJoinPhase = EtcdJoinVerified
				node.EtcdJoinError = ""
				dirty = true
				log.Printf("etcd join: node %s (%s) rejoin completed, marking verified",
					node.NodeID, node.Identity.Hostname)
			}
			// rejoin_failed is set by the repair workflow handler, not here.

		case EtcdJoinRejoinFailed:
			// Terminal — stays until operator resets; bootstrap_phases.go reads
			// this to fail bootstrap so the node auto-retries from admitted.

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

		case EtcdJoinPromoting:
			// Node joined as a non-voting LEARNER and is awaiting promotion by
			// reconcileLearnerPromotion (Policy A′). Re-project from the live role:
			// once promoted to a voter it becomes verified; while still a learner it
			// stays promoting — a valid, stable state (1v+1l waits safely for a third
			// node), so there is NO timeout-to-failed here. If the member has
			// vanished pre-promotion, the phase is left for the removal/rollback
			// path (g); it is never faked to verified.
			if role, ok := matchNodeEtcdMemberRole(node, namedRoles); ok {
				if projectEtcdRolePhase(node, role) {
					dirty = true
					log.Printf("etcd join: node %s (%s) promoting -> %s (learner=%v)",
						node.NodeID, node.Identity.Hostname, node.EtcdJoinPhase, role.isLearner)
				}
			}

		case EtcdJoinVerified:
			// Nothing to do — node is a healthy etcd member.
			// Detect if the member has disappeared (node removal).
			// Must check ALL IPs to avoid false resets on multi-IP nodes.
			if !nodeAnyIPIsEtcdMember(node, existingURLs) && !nodeHasEtcdRunning(node) {
				// Cooldown: require 3 consecutive cycles of "missing + not running"
				// before triggering rejoin. This prevents false positives from
				// transient etcd restarts or brief network partitions.
				node.EtcdMissingCycles++
				dirty = true
				if node.EtcdMissingCycles < 3 {
					log.Printf("etcd join: node %s (%s) member missing, cycle %d/3 (cooldown)",
						node.NodeID, node.Identity.Hostname, node.EtcdMissingCycles)
					continue
				}

				// Member disappeared for 3+ consecutive cycles.
				// Count remaining healthy peers to decide the recovery path.
				healthyPeers := 0
				for _, n := range nodes {
					if n == nil || n.NodeID == node.NodeID {
						continue
					}
					if n.EtcdJoinPhase == EtcdJoinVerified && nodeHasEtcdRunning(n) {
						healthyPeers++
					}
				}
				if healthyPeers > 0 {
					// Other healthy members remain — safe to auto-rejoin.
					node.EtcdJoinPhase = EtcdJoinRejoinRequired
					node.EtcdJoinError = fmt.Sprintf("member disappeared from live cluster for %d consecutive cycles while etcd was not running; auto-rejoin triggered", node.EtcdMissingCycles)
				} else {
					// Sole surviving member or unknown state — reset to None
					// to allow the normal join flow without risking quorum loss.
					node.EtcdJoinPhase = EtcdJoinNone
					node.EtcdJoinError = ""
				}
				node.EtcdMissingCycles = 0
				log.Printf("etcd join: node %s (%s) member disappeared after %d cycles, transitioning to %s",
					node.NodeID, node.Identity.Hostname, 3, node.EtcdJoinPhase)
			} else if node.EtcdMissingCycles > 0 {
				// Node is back — reset the cooldown counter.
				node.EtcdMissingCycles = 0
				dirty = true
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

// etcdJoinsInProgressPrefix is the etcd key prefix the join script writes
// to before issuing `etcdctl member add`. Keys are leased with a short TTL
// so the lock self-cleans if the join crashes between member-add and the
// node-agent heartbeat that would have registered the node in /globular/nodes/.
//
// Schema: /globular/etcd_joins/<sanitized_hostname>  → JSON {peer_url, started_unix}
// Lease : 300 seconds (covers the longest healthy Day-1 join we've observed)
const etcdJoinsInProgressPrefix = "/globular/etcd_joins/"

// joinInProgressMembers returns the set of sanitized hostnames whose Day-1
// join is currently in flight, keyed by the same name etcd uses for
// member.Name. The set is best-effort: if etcd is unreachable or the key
// JSON is malformed, the function returns an empty set so removeStaleMembers
// behaves as before — fail-open on the read side.
//
// The lock exists to close a structural race: `etcdctl member add` runs in
// the join script BEFORE the joining node's node-agent has registered with
// the controller via heartbeat. Between those two events the controller's
// desired set does NOT contain the joining node, and removeStaleMembers
// would otherwise classify the freshly-added etcd member as stale and
// remove it (observed live 2026-05-14 on nuc).
func (m *etcdMemberManager) joinInProgressMembers(ctx context.Context) map[string]bool {
	if m == nil || m.client == nil {
		return map[string]bool{}
	}
	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := m.client.Get(tctx, etcdJoinsInProgressPrefix, clientv3.WithPrefix())
	if err != nil {
		// Fail-open. The race we're closing is narrow (~30s window per join);
		// leaving the door open on a brief etcd hiccup is safer than refusing
		// to reconcile stale members because the lock read flapped.
		log.Printf("removeStaleMembers: failed to read %s — proceeding without lock set: %v", etcdJoinsInProgressPrefix, err)
		return map[string]bool{}
	}
	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}
	return parseEtcdJoinLockKeys(keys)
}

// parseEtcdJoinLockKeys extracts the sanitized-hostname set from raw etcd
// keys under the join-lock prefix. Pure function so the schema can be
// unit-tested without a live etcd. Empty keys, whitespace-only suffixes,
// and entries outside the prefix are silently dropped — the controller
// never has standing to evict based on a malformed lock entry.
func parseEtcdJoinLockKeys(keys []string) map[string]bool {
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		if !strings.HasPrefix(k, etcdJoinsInProgressPrefix) {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(k, etcdJoinsInProgressPrefix))
		if name == "" {
			continue
		}
		out[name] = true
	}
	return out
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

	// In-progress joins are protected from eviction. The join script writes
	// /globular/etcd_joins/<sanitized_hostname> with a leased TTL before
	// running `etcdctl member add` and deletes the key once the node-agent
	// heartbeat has put the node into /globular/nodes/. Either path closes
	// the race; the TTL is the safety net.
	joinInProgress := m.joinInProgressMembers(ctx)

	// Build set of desired peer URLs and hostnames.
	// Hostname matching is a safety net: if a node's IP is temporarily absent
	// from controller state (e.g. after a restart before the heartbeat arrives),
	// we must not remove it from the etcd cluster.
	desiredPeerURLs := make(map[string]bool, len(desiredEtcdNodes))
	desiredHostnames := make(map[string]bool, len(desiredEtcdNodes))
	desiredPeerURLByHostname := make(map[string]string, len(desiredEtcdNodes))
	for _, node := range desiredEtcdNodes {
		if node.IP != "" {
			peerURL := fmt.Sprintf("https://%s:2380", node.IP)
			desiredPeerURLs[peerURL] = true
			if node.Hostname != "" {
				desiredPeerURLByHostname[sanitizeEtcdName(node.Hostname)] = peerURL
			}
		}
		if node.Hostname != "" {
			desiredHostnames[sanitizeEtcdName(node.Hostname)] = true
		}
	}

	// Collect stale removal candidates and count live started VOTERS first, so the
	// fail-safe guards below can refuse on empty/ambiguous desired authority or a
	// quorum-breaking batch. NEVER MemberRemove inline before the guards run
	// (sibling of etcd.auto_rejoin_leader_guard_fails_open — a guard that failed
	// OPEN on empty input and could wipe the cluster).
	//
	// Quorum is a VOTER concept: etcd learners do not vote and do not count toward
	// quorum (meta.limited_members_are_not_capacity). A started-but-unpromoted
	// learner has a Name, so counting it here would miscalibrate the removal
	// quorum floor exactly when learners exist — the safety brake would carry ghost
	// weight. Count only non-learner (voting) members toward startedVoters, and
	// track per-candidate learner status so removing a learner (always quorum-safe)
	// never counts against the voter-survivor floor.
	startedVoters := 0
	var staleCandidates []staleEtcdMember
	for _, member := range resp.Members {
		if member.Name == "" {
			continue // unstarted member — might be mid-join, don't remove
		}
		if !member.IsLearner {
			startedVoters++
		}
		if joinInProgress[member.Name] {
			// Day-1 join in flight for this hostname; don't evict.
			continue
		}
		if expectedPeerURL, ok := desiredPeerURLByHostname[member.Name]; ok && expectedPeerURL != "" {
			hasExpected := false
			for _, purl := range member.PeerURLs {
				if purl == expectedPeerURL {
					hasExpected = true
					break
				}
			}
			if !hasExpected {
				updCtx, updCancel := context.WithTimeout(ctx, 10*time.Second)
				_, updErr := m.client.MemberUpdate(updCtx, member.ID, []string{expectedPeerURL})
				updCancel()
				if updErr != nil {
					log.Printf("etcd member-update: failed to update member %s (id=%d) peerURLs from %v to %s: %v",
						member.Name, member.ID, member.PeerURLs, expectedPeerURL, updErr)
				} else {
					log.Printf("etcd member-update: updated member %s (id=%d) peerURLs from %v to %s",
						member.Name, member.ID, member.PeerURLs, expectedPeerURL)
				}
			}
			// Same logical node (hostname match): never treat as stale.
			continue
		}
		isDesired := false
		for _, purl := range member.PeerURLs {
			if desiredPeerURLs[purl] {
				isDesired = true
				break
			}
		}
		// Fallback: match by sanitized hostname in case IP is temporarily empty
		// in controller state but the node is still a legitimate cluster member.
		if !isDesired && desiredHostnames[member.Name] {
			isDesired = true
		}
		if isDesired {
			continue
		}

		staleCandidates = append(staleCandidates, staleEtcdMember{
			id:        member.ID,
			name:      member.Name,
			peerURLs:  append([]string(nil), member.PeerURLs...),
			isLearner: member.IsLearner,
		})
	}

	// Count how many removal candidates are voters — only these reduce the
	// voter-survivor count. Removing a learner is always quorum-safe.
	votingRemovalCandidates := 0
	for _, cand := range staleCandidates {
		if !cand.isLearner {
			votingRemovalCandidates++
		}
	}

	// Fail-safe guards: absence, emptiness, or ambiguity in desired etcd
	// membership must NEVER authorize member removal, and a removal batch must
	// never drop live VOTERS below quorum. Refuse the whole pass on either.
	if reason := etcdRemovalRefusalReason(
		len(desiredEtcdNodes), len(desiredPeerURLs), len(desiredHostnames),
		startedVoters, len(staleCandidates), votingRemovalCandidates,
	); reason != "" {
		log.Printf("etcd member-remove REFUSED (no members removed): %s", reason)
		return fmt.Errorf("etcd stale-member removal refused: %s", reason)
	}

	for _, cand := range staleCandidates {
		rmCtx, rmCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := m.client.MemberRemove(rmCtx, cand.id)
		rmCancel()
		if err != nil {
			log.Printf("etcd member-remove: failed to remove stale member %s (id=%d): %v", cand.name, cand.id, err)
			continue
		}
		log.Printf("etcd member-remove: removed stale member %s (id=%d, peer=%v)", cand.name, cand.id, cand.peerURLs)
	}

	return nil
}

// staleEtcdMember is a captured removal candidate — enough to MemberRemove and
// log, without holding a reference to the live MemberList response.
type staleEtcdMember struct {
	id        uint64
	name      string
	peerURLs  []string
	isLearner bool // learners never count toward the removal quorum floor
}

// etcdRemovalRefusalReason returns a non-empty reason when a stale etcd member
// removal batch MUST be refused. Absence, emptiness, or ambiguity in the desired
// etcd set — or a removal that would drop live started VOTERS below quorum —
// never authorizes MemberRemove. An empty return means the batch is safe.
//
// This is the fail-CLOSED counterpart to etcd.auto_rejoin_leader_guard_fails_open:
// an empty desired set is uncertainty (inventory.missing_means_uncertain), not a
// mandate to execute the cluster (delete_requires_explicit_intent_marker).
//
// Quorum is a VOTER concept (meta.limited_members_are_not_capacity): the guard
// counts only started voters and only voting removals. Learners neither raise the
// quorum floor nor, when removed, lower the voter-survivor count — so removing a
// stale learner is always safe, and a started learner never inflates the brake.
func etcdRemovalRefusalReason(desiredNodes, desiredPeerURLs, desiredHostnames, startedVoters, removalCandidates, votingRemovalCandidates int) string {
	if removalCandidates == 0 {
		return "" // nothing to remove — trivially safe
	}
	// Guard A: empty or no-usable-identity desired authority.
	if desiredNodes == 0 || (desiredPeerURLs == 0 && desiredHostnames == 0) {
		return fmt.Sprintf("desired etcd set is empty or has no usable identities "+
			"(nodes=%d peerURLs=%d hostnames=%d) — absence is not permission to remove members",
			desiredNodes, desiredPeerURLs, desiredHostnames)
	}
	// Guard B: voter quorum floor. etcd quorum is floor(V/2)+1 of the started
	// VOTERS; the voting survivors after removal must still form that quorum.
	// Removing learners does not reduce votingSurvivors, so it never trips this.
	quorum := startedVoters/2 + 1
	votingSurvivors := startedVoters - votingRemovalCandidates
	if votingSurvivors < quorum {
		return fmt.Sprintf("removing %d voting of %d started etcd voters would leave %d, "+
			"below quorum floor %d — refusing",
			votingRemovalCandidates, startedVoters, votingSurvivors, quorum)
	}
	return ""
}

// seedEtcdEndpointsFromState writes /var/lib/globular/config/etcd_endpoints
// from persisted cluster state when the file is absent. This lets the controller
// bootstrap its etcd client after a restart even when its local etcd is down
// (e.g. after being permanently removed from the cluster). Only remote endpoints
// are included so the controller connects to a reachable peer first.
func seedEtcdEndpointsFromState(state *controllerState, logger *slog.Logger) {
	const endpointsFile = "/var/lib/globular/config/etcd_endpoints"
	if _, err := os.Stat(endpointsFile); err == nil {
		return // file exists — controller rendered it
	}
	if state == nil {
		return
	}

	localIP, _ := config.GetRoutableIP()
	var eps []string
	for _, node := range state.Nodes {
		routableIPs := nodeAllRoutableIPs(node)
		ip := ""
		for _, candidate := range routableIPs {
			if candidate == localIP {
				continue
			}
			ip = candidate
			break
		}
		if ip == "" {
			continue // skip missing and local (possibly down) endpoint
		}
		// Only include nodes that have a core or control-plane profile —
		// those are guaranteed to run etcd.
		for _, p := range node.Profiles {
			if p == "core" || p == "control-plane" {
				eps = append(eps, fmt.Sprintf("https://%s:2379", ip))
				break
			}
		}
	}
	if len(eps) == 0 {
		return
	}

	if err := os.MkdirAll(filepath.Dir(endpointsFile), 0o755); err != nil {
		logger.Warn("etcd bootstrap: cannot create config dir", "error", err)
		return
	}
	content := strings.Join(eps, "\n") + "\n"
	if err := os.WriteFile(endpointsFile, []byte(content), 0o644); err != nil {
		logger.Warn("etcd bootstrap: failed to seed endpoints file", "error", err)
		return
	}
	logger.Info("etcd bootstrap: seeded endpoints from cluster state", "endpoints", eps)
}

// reconcileEtcdAutoRejoin automatically initiates the etcd rejoin workflow for
// nodes that are in EtcdJoinRejoinRequired and satisfy all safety preconditions.
// It calls MemberAdd and transitions the node to EtcdJoinRejoinInProgress.
// Called from reconcileAdvanceInfraJoins after reconcileEtcdJoinPhases.
func (m *etcdMemberManager) reconcileEtcdAutoRejoin(ctx context.Context, nodes []*nodeState) (dirty bool) {
	if m == nil || m.client == nil {
		return false
	}

	// Safety: identify the current etcd leader. NEVER auto-rejoin the leader —
	// wiping its data directory destroys cluster quorum with no recovery path.
	//
	// This identification FAILS CLOSED. If we cannot confidently determine the
	// leader — Status/MemberList error, no leader elected yet (Leader==0), an
	// empty endpoint list, or a reported leader that is not a known member — we
	// REFUSE all auto-rejoins this cycle and retry next reconcile. Proceeding with
	// an empty leaderNodeID would skip the "never wipe the leader" guard below and
	// could wipe the leader itself. Missing leader evidence is uncertainty, not
	// "there is no leader to protect" (intent.inventory.missing_means_uncertain,
	// node_recovery.fence_before_destructive_reseed).
	statusCtx, statusCancel := context.WithTimeout(ctx, 5*time.Second)
	defer statusCancel()

	eps := m.client.Endpoints()
	if len(eps) == 0 {
		log.Printf("etcd auto-rejoin: etcd client has no endpoints — refusing all auto-rejoins this cycle (fail-closed; will retry)")
		return false
	}
	resp, err := m.client.Status(statusCtx, eps[0])
	if err != nil {
		log.Printf("etcd auto-rejoin: cannot read etcd status to identify the leader (%v) — refusing all auto-rejoins this cycle (fail-closed; will retry)", err)
		return false
	}
	leaderMemberID := resp.Leader
	if leaderMemberID == 0 {
		log.Printf("etcd auto-rejoin: etcd reports no elected leader (election in progress or quorum lost) — refusing all auto-rejoins this cycle (fail-closed; will retry)")
		return false
	}
	membResp, err := m.client.MemberList(statusCtx)
	if err != nil {
		log.Printf("etcd auto-rejoin: cannot read etcd member list to map the leader (%v) — refusing all auto-rejoins this cycle (fail-closed; will retry)", err)
		return false
	}
	memberPeerURLs := make(map[uint64]string, len(membResp.Members))
	for _, mem := range membResp.Members {
		if len(mem.PeerURLs) > 0 {
			memberPeerURLs[mem.ID] = mem.PeerURLs[0]
		}
	}
	leaderNodeID, leaderKnown := resolveEtcdLeaderNode(leaderMemberID, memberPeerURLs, nodes)
	if !leaderKnown {
		log.Printf("etcd auto-rejoin: reported leader memberID=%d could not be confidently mapped to a current member — refusing all auto-rejoins this cycle (fail-closed; will retry)", leaderMemberID)
		return false
	}

	for _, node := range nodes {
		if node == nil || node.EtcdJoinPhase != EtcdJoinRejoinRequired {
			continue
		}
		// Never wipe the etcd leader.
		if leaderNodeID != "" && node.NodeID == leaderNodeID {
			log.Printf("etcd auto-rejoin: REFUSING to rejoin node %s (%s) — it is the current etcd leader",
				node.NodeID, node.Identity.Hostname)
			node.EtcdJoinPhase = EtcdJoinVerified
			node.EtcdJoinError = ""
			dirty = true
			continue
		}
		ip := nodeRoutableIP(node)
		if ip == "" {
			continue // can't re-add without an IP
		}
		checks := validateEtcdRejoinPreconditions(node, nodes)
		if !checks.Valid() {
			log.Printf("etcd auto-rejoin: node %s (%s) preconditions not met: %v",
				node.NodeID, node.Identity.Hostname, checks.Error)
			continue
		}
		peerURL := fmt.Sprintf("https://%s:2380", ip)
		memberID, err := m.memberAdd(ctx, peerURL)
		if err != nil {
			log.Printf("etcd auto-rejoin: MemberAdd for %s (%s) failed: %v",
				node.NodeID, node.Identity.Hostname, err)
			continue
		}
		node.EtcdJoinPhase = EtcdJoinRejoinInProgress
		node.EtcdMemberID = memberID
		node.EtcdJoinError = ""
		dirty = true
		log.Printf("etcd auto-rejoin: MemberAdd succeeded for %s (%s) peerURL=%s memberID=%d",
			node.NodeID, node.Identity.Hostname, peerURL, memberID)
	}
	return dirty
}
