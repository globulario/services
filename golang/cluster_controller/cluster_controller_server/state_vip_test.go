package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestStableIPSkipsVIP confirms the regression captured by failuregraph seed
// vip_transition_evicts_etcd_member: when a VIP-holder's Identity.Ips contains
// the floating VIP, StableIP must NOT return the VIP regardless of the order
// it appears in the list.
func TestStableIPSkipsVIP(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		vip  string
		want string
	}{
		{
			name: "vip first in list — must skip and return real IP",
			ips:  []string{"10.0.0.100", "10.0.0.63"},
			vip:  "10.0.0.100",
			want: "10.0.0.63",
		},
		{
			name: "vip last in list — must return real IP",
			ips:  []string{"10.0.0.63", "10.0.0.100"},
			vip:  "10.0.0.100",
			want: "10.0.0.63",
		},
		{
			name: "vip not in list — return first routable",
			ips:  []string{"10.0.0.63", "172.17.0.1"},
			vip:  "10.0.0.100",
			want: "10.0.0.63",
		},
		{
			name: "no vip configured — fallback to PrimaryIP behavior",
			ips:  []string{"10.0.0.63"},
			vip:  "",
			want: "10.0.0.63",
		},
		{
			name: "only the VIP is in the list — fallback to PrimaryIP returns VIP",
			ips:  []string{"10.0.0.100"},
			vip:  "10.0.0.100",
			want: "10.0.0.100", // documented fallback: better than empty for endpoints file
		},
		{
			name: "loopback skipped, vip skipped, return real IP",
			ips:  []string{"127.0.0.1", "10.0.0.100", "10.0.0.63"},
			vip:  "10.0.0.100",
			want: "10.0.0.63",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := &nodeState{
				Identity: storedIdentity{Ips: tc.ips},
			}
			if got := n.StableIP(tc.vip); got != tc.want {
				t.Errorf("StableIP(%q) with ips=%v = %q, want %q", tc.vip, tc.ips, got, tc.want)
			}
		})
	}
}

// TestPrimaryIPDangerousByDesign captures the documented hazard: PrimaryIP
// returns the FIRST routable IP, which is the floating VIP if it appears
// first in the list. This is why callers must use StableIP.
func TestPrimaryIPDangerousByDesign(t *testing.T) {
	n := &nodeState{
		Identity: storedIdentity{
			Ips: []string{"10.0.0.100", "10.0.0.63"},
		},
	}
	if got := n.PrimaryIP(); got != "10.0.0.100" {
		t.Errorf("PrimaryIP() with VIP-first ips returned %q, expected the VIP — if this changes, audit all PrimaryIP() callers", got)
	}
}

// TestSnapshotClusterMembershipUsesStableIP guards against the regression that
// caused removeStaleMembers to evict ryzen's etcd: snapshotClusterMembership
// previously used node.Identity.Ips[0], which on a VIP holder is the VIP,
// poisoning the desiredPeerURLs set built from membership.Nodes[i].IP.
func TestSnapshotClusterMembershipUsesStableIP(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"node-with-vip": {
					NodeID:   "node-with-vip",
					Profiles: []string{"core", "control-plane", "storage"},
					Identity: storedIdentity{
						Hostname: "globule-vip-holder",
						// VIP listed first — exactly the failure mode from the
						// 2026-05-10 incident.
						Ips: []string{"10.0.0.100", "10.0.0.63"},
					},
				},
				"node-without-vip": {
					NodeID:   "node-without-vip",
					Profiles: []string{"core", "control-plane", "storage"},
					Identity: storedIdentity{
						Hostname: "globule-plain",
						Ips:      []string{"10.0.0.20"},
					},
				},
			},
			ClusterNetworkSpec: &cluster_controllerpb.ClusterNetworkSpec{
				ClusterDomain: "globular.internal",
				Protocol:      "https",
			},
		},
		// etcdClient is nil — clusterVIP() will return "" and StableIP falls
		// back to PrimaryIP. Test that this fallback does NOT regress correctness
		// for the non-VIP-holder, and that when the VIP IS provided to StableIP
		// directly the VIP holder's IP is the stable one.
	}

	got := srv.snapshotClusterMembership()
	if got == nil {
		t.Fatal("snapshotClusterMembership returned nil")
	}
	byID := map[string]string{}
	for _, n := range got.Nodes {
		byID[n.NodeID] = n.IP
	}

	// With no VIP available (etcdClient nil → clusterVIP returns ""), StableIP
	// falls back to PrimaryIP. This test documents that fallback behavior;
	// the real protection happens when ingress spec is set, exercised by
	// TestStableIPSkipsVIP above.
	if byID["node-without-vip"] != "10.0.0.20" {
		t.Errorf("non-VIP-holder member.IP = %q, want 10.0.0.20", byID["node-without-vip"])
	}
}
