// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.reconcile_etcd_endpoints
// @awareness file_role=etcd_endpoint_list_reconciler_observes_membership_publishes_endpoints_never_evicts
// @awareness implements=globular.platform:intent.etcd.is_source_of_truth
// @awareness implements=globular.platform:intent.controller.leader_election_gates_all_writes
// @awareness enforces=globular.platform:invariant.etcd.endpoint_reachability
// @awareness risk=critical
package main

// reconcile_etcd_endpoints.go — keeps the cluster's
// authoritative etcd endpoint list in sync with healthy
// members. Observes membership, publishes endpoints; MUST NOT
// auto-evict or call MemberRemove from this loop. Removal goes
// through node_removal_requests.go with an explicit, audited
// operator request — the keepalived/VIP eviction cascade is
// exactly the cost of letting endpoint reconciliation grow a
// removal branch.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

const (
	// etcdEndpointListKey is the etcd key that holds the authoritative
	// JSON array of etcd client endpoints for the cluster.
	// Node-agents may read this key to refresh their local endpoint file
	// after topology changes.
	etcdEndpointListKey = "/globular/system/etcd_endpoints"

	// etcdEndpointReconcileOutcomeKey holds the JSON outcome of the most
	// recent reconcile run. Used by the doctor and MCP tools for observability.
	etcdEndpointReconcileOutcomeKey = "/globular/system/etcd_endpoint_reconcile/last"

	// etcdEndpointReconcileInterval is how often the reconciler wakes up.
	etcdEndpointReconcileInterval = 3 * time.Minute

	// etcdEndpointQuorumMin is the minimum number of core-profile ready nodes
	// that must be visible before the reconciler will write. If fewer nodes
	// are known, the reconciler skips the write to avoid publishing a partial
	// endpoint list that could cause other nodes to lose quorum access.
	etcdEndpointQuorumMin = 3
)

// etcdEndpointReconcileOutcome is the JSON payload written to
// etcdEndpointReconcileOutcomeKey after each reconcile cycle.
type etcdEndpointReconcileOutcome struct {
	TimestampUnix    int64    `json:"timestamp_unix"`
	Outcome          string   `json:"outcome"`
	Reason           string   `json:"reason,omitempty"`
	DesiredEndpoints []string `json:"desired_endpoints,omitempty"`
	LiveMemberCount  int      `json:"live_member_count,omitempty"`
	StaleMembers     []string `json:"stale_members,omitempty"`
	Drift            bool     `json:"drift,omitempty"`
}

// memberSnapshot is a transport-neutral view of a single etcd cluster member.
type memberSnapshot struct {
	Name       string
	PeerURLs   []string
	ClientURLs []string
}

// etcdEndpointReconciler is a leader-only periodic reconciler that keeps the
// etcd endpoint list in etcd consistent with the live cluster membership.
//
// On each tick it:
//  1. Snapshots core-profile ready nodes from controller state (quorum guard).
//  2. Calls MemberList to get the live etcd membership.
//  3. Computes the desired client-endpoint set from core-node IPs.
//  4. If drift is detected, writes the corrected list to etcdEndpointListKey.
//  5. Logs stale members (in etcd but not in controller state) — no auto-remove.
//  6. Writes an outcome record to etcdEndpointReconcileOutcomeKey.
//
// Auto-removal of stale members is intentionally NOT performed here; that
// responsibility belongs to etcdMemberManager.removeStaleMembers which has
// the necessary safety checks.
type etcdEndpointReconciler struct {
	srv      *server
	interval time.Duration
	now      func() time.Time

	listMembers       func(ctx context.Context) ([]memberSnapshot, error)
	snapshotCoreNodes func() []string // returns routable IPs of core-profile READY nodes (quorum guard only)
	// knownCoreNodes returns routable IPs of every core-profile node the
	// controller knows, REGARDLESS of convergence status. Endpoint membership
	// must not depend on whether a node happens to be mid-convergence: a node
	// that is a live etcd member is reachable at its client URL whether its
	// status says "ready" or "converging".
	knownCoreNodes func() []string
	// readEndpointList returns the currently published endpoint list, and
	// whether the key exists. The reconciler writes only when the published
	// value differs from desired — without this it cannot tell "already
	// correct" from "needs correcting" and rewrites on every tick.
	readEndpointList func(ctx context.Context) (string, bool, error)
	writeToEtcd      func(ctx context.Context, key, value string) error
	writeOutcome     func(ctx context.Context, out etcdEndpointReconcileOutcome) error
}

func newEtcdEndpointReconciler(srv *server) *etcdEndpointReconciler {
	r := &etcdEndpointReconciler{
		srv:      srv,
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.listMembers = func(ctx context.Context) ([]memberSnapshot, error) {
		if srv.etcdClient == nil {
			return nil, fmt.Errorf("etcd client not initialised")
		}
		tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		resp, err := srv.etcdClient.MemberList(tctx)
		if err != nil {
			return nil, err
		}
		out := make([]memberSnapshot, 0, len(resp.Members))
		for _, m := range resp.Members {
			out = append(out, memberSnapshot{
				Name:       m.Name,
				PeerURLs:   m.PeerURLs,
				ClientURLs: m.ClientURLs,
			})
		}
		return out, nil
	}
	r.snapshotCoreNodes = func() []string {
		srv.lock("etcd-endpoint-reconciler:snapshot")
		defer srv.unlock()
		var ips []string
		for _, n := range srv.state.Nodes {
			if n == nil {
				continue
			}
			if n.Status != "ready" && n.Status != "admitted" {
				continue
			}
			hasCoreProfile := false
			for _, p := range n.Profiles {
				if p == "core" {
					hasCoreProfile = true
					break
				}
			}
			if !hasCoreProfile {
				continue
			}
			ip := nodeRoutableIP(n)
			if ip != "" {
				ips = append(ips, ip)
			}
		}
		return ips
	}
	r.knownCoreNodes = func() []string {
		srv.lock("etcd-endpoint-reconciler:known")
		defer srv.unlock()
		var ips []string
		for _, n := range srv.state.Nodes {
			if n == nil {
				continue
			}
			hasCoreProfile := false
			for _, p := range n.Profiles {
				if p == "core" {
					hasCoreProfile = true
					break
				}
			}
			if !hasCoreProfile {
				continue
			}
			// Deliberately NOT filtered on n.Status — see knownCoreNodes doc.
			if ip := nodeRoutableIP(n); ip != "" {
				ips = append(ips, ip)
			}
		}
		return ips
	}
	r.readEndpointList = func(ctx context.Context) (string, bool, error) {
		if srv.etcdClient == nil {
			return "", false, fmt.Errorf("etcd client not initialised")
		}
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		resp, err := srv.etcdClient.Get(rctx, etcdEndpointListKey)
		if err != nil {
			return "", false, err
		}
		if len(resp.Kvs) == 0 {
			return "", false, nil
		}
		return string(resp.Kvs[0].Value), true, nil
	}
	r.writeToEtcd = func(ctx context.Context, key, value string) error {
		if srv.etcdClient == nil {
			return fmt.Errorf("etcd client not initialised")
		}
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := srv.etcdClient.Put(wctx, key, value)
		return err
	}
	r.writeOutcome = r.defaultWriteOutcome
	return r
}

func (r *etcdEndpointReconciler) defaultWriteOutcome(ctx context.Context, out etcdEndpointReconcileOutcome) error {
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return r.writeToEtcd(ctx, etcdEndpointReconcileOutcomeKey, string(b))
}

// Start runs the reconcile loop until ctx is cancelled.
// Must be called in its own goroutine (via safeGo).
func (r *etcdEndpointReconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !r.srv.isLeader() {
				continue
			}
			r.reconcileOnce(ctx)
		}
	}
}

func (r *etcdEndpointReconciler) reconcileOnce(ctx context.Context) {
	coreIPs := r.snapshotCoreNodes()
	if len(coreIPs) < etcdEndpointQuorumMin {
		log.Printf("etcd-endpoint-reconciler: only %d core node(s) ready (need %d) — skipping write",
			len(coreIPs), etcdEndpointQuorumMin)
		return
	}

	members, err := r.listMembers(ctx)
	if err != nil {
		log.Printf("etcd-endpoint-reconciler: MemberList error: %v", err)
		_ = r.writeOutcome(ctx, etcdEndpointReconcileOutcome{
			TimestampUnix: r.now().Unix(),
			Outcome:       "error",
			Reason:        fmt.Sprintf("MemberList failed: %v", err),
		})
		return
	}

	// Endpoint membership comes from etcd's OWN member list, filtered to nodes
	// this controller knows carry the core profile. It must NOT be derived from
	// node convergence status.
	//
	// It was: desired = computeDesiredEndpoints(coreIPs) where coreIPs held only
	// nodes with Status ready/admitted, while drift was measured against the live
	// member list. Those are two different authorities, so they disagreed
	// whenever any core node was mid-convergence, and the disagreement could not
	// be resolved by writing — every tick re-detected drift and rewrote the key.
	//
	// Observed 2026-08-10 on a converging 5-node cluster: `live=5` on every pass
	// while the published list was rewritten to a rotating 3-member subset —
	// [.12 .14 .15], then [.11 .14 .15], then [.13 .14 .15] — each one omitting
	// two members that were live in etcd, and each labelling those live members
	// "stale". Publishing an endpoint list that omits reachable members is a
	// direct hazard to invariant:etcd.endpoint_reachability (a service starting
	// in that window can be handed endpoints it cannot reach), and deriving
	// membership from anything but etcd forks the truth
	// (intent:etcd.is_source_of_truth).
	knownIPs := coreIPs
	if r.knownCoreNodes != nil {
		if all := r.knownCoreNodes(); len(all) > 0 {
			knownIPs = all
		}
	}
	desired := computeDesiredEndpoints(knownIPs)
	stale := detectStaleEtcdMembers(members, knownIPs)

	// Drift is what the PUBLISHED list says, when we can read it: that is the
	// value other services actually consume, so it decides both ways — an
	// already-correct list is left alone, and a wrong one is corrected even
	// while live membership happens to agree with desired. The membership
	// comparison is only the fallback for when the read fails.
	drift := detectEtcdEndpointDrift(members, desired)
	if r.readEndpointList != nil {
		if current, found, err := r.readEndpointList(ctx); err != nil {
			log.Printf("etcd-endpoint-reconciler: read published endpoint list: %v — falling back to membership comparison", err)
		} else {
			drift = !found || !publishedEndpointsMatch(current, desired)
		}
	}

	if len(stale) > 0 {
		log.Printf("etcd-endpoint-reconciler: stale members (no auto-remove): %v", stale)
	}

	outcome := "ok"
	if len(stale) > 0 {
		outcome = "stale_member_detected"
	}

	if !drift {
		_ = r.writeOutcome(ctx, etcdEndpointReconcileOutcome{
			TimestampUnix:    r.now().Unix(),
			Outcome:          outcome,
			DesiredEndpoints: desired,
			LiveMemberCount:  len(members),
			StaleMembers:     stale,
			Drift:            false,
		})
		return
	}

	b, err := json.Marshal(desired)
	if err != nil {
		log.Printf("etcd-endpoint-reconciler: failed to marshal endpoint list: %v", err)
		_ = r.writeOutcome(ctx, etcdEndpointReconcileOutcome{
			TimestampUnix:    r.now().Unix(),
			Outcome:          "error",
			Reason:           fmt.Sprintf("marshal failed: %v", err),
			DesiredEndpoints: desired,
			LiveMemberCount:  len(members),
			StaleMembers:     stale,
			Drift:            true,
		})
		return
	}
	value := string(b)
	if err := r.writeToEtcd(ctx, etcdEndpointListKey, value); err != nil {
		log.Printf("etcd-endpoint-reconciler: failed to write endpoint list: %v", err)
		_ = r.writeOutcome(ctx, etcdEndpointReconcileOutcome{
			TimestampUnix:    r.now().Unix(),
			Outcome:          "error",
			Reason:           fmt.Sprintf("write failed: %v", err),
			DesiredEndpoints: desired,
			LiveMemberCount:  len(members),
			StaleMembers:     stale,
			Drift:            true,
		})
		return
	}

	log.Printf("etcd-endpoint-reconciler: corrected endpoint list (live=%d desired=%v stale=%v)",
		len(members), desired, stale)
	_ = r.writeOutcome(ctx, etcdEndpointReconcileOutcome{
		TimestampUnix:    r.now().Unix(),
		Outcome:          "drift_corrected",
		DesiredEndpoints: desired,
		LiveMemberCount:  len(members),
		StaleMembers:     stale,
		Drift:            true,
	})
}

// computeDesiredEndpoints returns a sorted list of etcd client endpoints
// (https://{ip}:2379) for the given core-node IPs.
// publishedEndpointsMatch reports whether the JSON currently stored at
// etcdEndpointListKey already holds exactly the desired endpoint set.
// Compared as a SET so a reordering never counts as drift.
func publishedEndpointsMatch(published string, desired []string) bool {
	var current []string
	if err := json.Unmarshal([]byte(published), &current); err != nil {
		return false
	}
	if len(current) != len(desired) {
		return false
	}
	set := make(map[string]bool, len(current))
	for _, ep := range current {
		set[ep] = true
	}
	for _, ep := range desired {
		if !set[ep] {
			return false
		}
	}
	return true
}

func computeDesiredEndpoints(coreIPs []string) []string {
	eps := make([]string, 0, len(coreIPs))
	for _, ip := range coreIPs {
		eps = append(eps, fmt.Sprintf("https://%s:2379", ip))
	}
	sort.Strings(eps)
	return eps
}

// detectStaleEtcdMembers returns the names of etcd members whose peer URL
// does not resolve to any known core-node IP. These represent members that
// left the cluster without a formal removal. The list is for observability
// only — the caller must not auto-remove based on this alone.
func detectStaleEtcdMembers(members []memberSnapshot, coreIPs []string) []string {
	ipSet := make(map[string]bool, len(coreIPs))
	for _, ip := range coreIPs {
		ipSet[ip] = true
	}
	var stale []string
	for _, m := range members {
		matched := false
		for _, peerURL := range m.PeerURLs {
			if ip := extractEtcdIPFromURL(peerURL); ip != "" && ipSet[ip] {
				matched = true
				break
			}
		}
		if !matched {
			label := m.Name
			if label == "" {
				label = "<unnamed>"
			}
			stale = append(stale, label)
		}
	}
	sort.Strings(stale)
	return stale
}

// detectEtcdEndpointDrift returns true when the set of IPs reachable via
// live member ClientURLs differs from the set in desired.
func detectEtcdEndpointDrift(members []memberSnapshot, desired []string) bool {
	desiredSet := make(map[string]bool, len(desired))
	for _, ep := range desired {
		if ip := extractEtcdIPFromURL(ep); ip != "" {
			desiredSet[ip] = true
		}
	}
	liveSet := make(map[string]bool, len(members))
	for _, m := range members {
		for _, cu := range m.ClientURLs {
			if ip := extractEtcdIPFromURL(cu); ip != "" {
				liveSet[ip] = true
			}
		}
	}
	if len(desiredSet) != len(liveSet) {
		return true
	}
	for ip := range desiredSet {
		if !liveSet[ip] {
			return true
		}
	}
	return false
}

// extractEtcdIPFromURL parses the host IP out of a URL such as
// "https://10.0.0.8:2379" or "https://10.0.0.8:2380".
func extractEtcdIPFromURL(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[:i]
	}
	return s
}
