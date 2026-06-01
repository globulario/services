// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=node_ip_identity_matching_rule
// @awareness implements=globular.platform:intent.identity.stable_node_identity_not_floating_vip
// @awareness risk=high
package rules

// node_ip_matching.go — VIP-contamination-safe node lookup helpers.
//
// Why this file exists
// --------------------
// `NodeIdentity.AdvertiseIp` is a single-string field that the controller
// does not currently populate (verified live 2026-05-10 — empty on every
// node in a 5-node cluster). Code that does
//
//     for _, ip := range desired.Nodes {
//         for _, n := range snap.Nodes {
//             if n.GetIdentity().GetAdvertiseIp() == ip { ... }
//         }
//     }
//
// fails silently: every comparison is `"" == "10.0.0.x"` → false, every
// pool node is classified as "not found", every downstream rule emits a
// false-positive CRITICAL (write_quorum_lost active_drives=0,
// unapproved_path on every admitted disk, …).
//
// Even if AdvertiseIp WERE populated, on a keepalived MASTER it would
// likely report the floating VIP rather than the stable interface IP —
// the same VIP-contamination class the cluster_controller already fixed
// in commit ead2bb94 (nodeAnyIPInPool for MinIO pool admission).
//
// The canonical, contamination-safe pattern is to iterate
// `n.GetIdentity().GetIps()` (the multi-value list) and accept any match
// against the target IP. This file centralises that pattern so the five
// rule sites in this package use it consistently.

import (
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// nodeIDByPoolIP returns a map from each pool IP to the nodeID of the
// node that advertises that IP, by iterating every node's full IP list.
// A node that advertises multiple IPs (e.g. a keepalived VIP holder
// reporting both the VIP and its stable interface IPs) is matched
// against whichever pool IP appears in its list.
//
// Pool IPs that don't match any node remain absent from the map.
// Callers must treat "missing key" as "not found" rather than as a
// pre-populated zero value.
func nodeIDByPoolIP(nodes []*cluster_controllerpb.NodeRecord, poolIPs []string) map[string]string {
	want := make(map[string]bool, len(poolIPs))
	for _, ip := range poolIPs {
		if ip != "" {
			want[ip] = true
		}
	}
	out := make(map[string]string, len(poolIPs))
	for _, n := range nodes {
		nid := n.GetNodeId()
		if nid == "" {
			continue
		}
		for _, ip := range n.GetIdentity().GetIps() {
			if want[ip] {
				// First match wins. If multiple nodes claim the same IP
				// (shouldn't happen in a healthy cluster — that's split
				// brain), the first node in snap.Nodes is preferred.
				if _, already := out[ip]; !already {
					out[ip] = nid
				}
			}
		}
	}
	return out
}

// nodeHasIP reports whether the given node's Identity.Ips contains the
// target IP. Use this when you need a per-node membership test rather
// than a full pool-wide map.
func nodeHasIP(n *cluster_controllerpb.NodeRecord, target string) bool {
	if n == nil || target == "" {
		return false
	}
	for _, ip := range n.GetIdentity().GetIps() {
		if ip == target {
			return true
		}
	}
	return false
}
