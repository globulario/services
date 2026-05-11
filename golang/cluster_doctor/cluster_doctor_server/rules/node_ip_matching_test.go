package rules

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// VIP-contamination regression suite — see node_ip_matching.go for the
// failure-mode rationale. Every test in this file simulates the live
// 2026-05-10 cluster shape: AdvertiseIp empty on every node, the
// keepalived VIP holder reporting the VIP as one of several entries in
// Identity.Ips. Pre-fix, all five rule sites mis-classified pool nodes
// as "down" (false-positive CRITICAL); post-fix they resolve correctly.

func TestNodeIDByPoolIP_MatchesByAnyIP(t *testing.T) {
	// nuc holds the VIP — its Identity.Ips contains both the VIP and its
	// stable IP. The desired pool list contains the stable IP only.
	nodes := []*cluster_controllerpb.NodeRecord{
		nodeRecordWithIPs("ryzen-id", "10.0.0.63", "172.17.0.1"),
		nodeRecordWithIPs("nuc-id", "10.0.0.100", "10.0.0.8", "10.0.0.214"),
		nodeRecordWithIPs("dell-id", "10.0.0.20"),
	}
	got := nodeIDByPoolIP(nodes, []string{"10.0.0.63", "10.0.0.20", "10.0.0.8"})
	want := map[string]string{
		"10.0.0.63": "ryzen-id",
		"10.0.0.20": "dell-id",
		"10.0.0.8":  "nuc-id",
	}
	for ip, expected := range want {
		if got[ip] != expected {
			t.Errorf("nodeIDByPoolIP[%q] = %q, want %q (full map: %v)", ip, got[ip], expected, got)
		}
	}
}

func TestNodeIDByPoolIP_EmptyAdvertiseIpDoesNotPoison(t *testing.T) {
	// The exact failure shape we hit on 2026-05-10: AdvertiseIp empty,
	// only Identity.Ips populated. Pre-fix, comparison of "" against
	// "10.0.0.63" failed for every node. Post-fix, the multi-IP
	// iteration matches correctly.
	nodes := []*cluster_controllerpb.NodeRecord{
		{
			NodeId:   "n1",
			Identity: &cluster_controllerpb.NodeIdentity{Ips: []string{"10.0.0.63"}},
		},
	}
	got := nodeIDByPoolIP(nodes, []string{"10.0.0.63"})
	if got["10.0.0.63"] != "n1" {
		t.Fatalf("empty AdvertiseIp must not block lookup; got map=%v", got)
	}
}

func TestNodeIDByPoolIP_UnmatchedPoolIPIsAbsent(t *testing.T) {
	nodes := []*cluster_controllerpb.NodeRecord{
		nodeRecordWithIPs("n1", "10.0.0.1"),
	}
	got := nodeIDByPoolIP(nodes, []string{"10.0.0.99"})
	if _, ok := got["10.0.0.99"]; ok {
		t.Errorf("unmatched pool IP must be absent (not zero-valued), got %v", got)
	}
}

func TestNodeHasIP(t *testing.T) {
	n := nodeRecordWithIPs("nuc", "10.0.0.100", "10.0.0.8", "10.0.0.214")
	if !nodeHasIP(n, "10.0.0.8") {
		t.Error("nuc should match its stable IP")
	}
	if !nodeHasIP(n, "10.0.0.100") {
		t.Error("nuc should match its VIP (still one of its IPs)")
	}
	if nodeHasIP(n, "10.0.0.42") {
		t.Error("nuc should NOT match an unrelated IP")
	}
	if nodeHasIP(nil, "10.0.0.8") {
		t.Error("nil node must not match")
	}
	if nodeHasIP(n, "") {
		t.Error("empty target must not match")
	}
}

// End-to-end regression: write_quorum_lost must NOT fire false-positive
// when the cluster is healthy and AdvertiseIp is empty (the live cluster
// shape circa 2026-05-10).
func TestWriteQuorumLost_VIPHolder_NotMisclassifiedAsDown(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			// All three storage nodes report Ips only, not AdvertiseIp.
			// nuc is the VIP holder; its first IP is the floating VIP.
			nodeRecordWithIPs("ryzen", "10.0.0.63"),
			nodeRecordWithIPs("nuc", "10.0.0.100", "10.0.0.8"),
			nodeRecordWithIPs("dell", "10.0.0.20"),
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"ryzen": minioActiveInventory(),
			"nuc":   minioActiveInventory(),
			"dell":  minioActiveInventory(),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.63", "10.0.0.20", "10.0.0.8"},
			nil,
			1,
		),
		ObjectStoreAppliedGeneration: 1,
	}
	findings := objectstoreWriteQuorumLost{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings (all nodes active and correctly resolved), got %d:\n  first: %s",
			len(findings), findings[0].Summary)
	}
}

// Companion negative case: when nodes really ARE down, the rule still fires.
// Confirms the fix doesn't accidentally suppress real CRITICAL detection.
func TestWriteQuorumLost_VIPHolder_RealDowntimeStillFires(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecordWithIPs("ryzen", "10.0.0.63"),
			nodeRecordWithIPs("nuc", "10.0.0.100", "10.0.0.8"),
			nodeRecordWithIPs("dell", "10.0.0.20"),
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"ryzen": minioActiveInventory(),
			"nuc":   minioInactiveInventory("failed"),
			"dell":  minioInactiveInventory("failed"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.63", "10.0.0.20", "10.0.0.8"},
			nil,
			1,
		),
		ObjectStoreAppliedGeneration: 1,
	}
	findings := objectstoreWriteQuorumLost{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 CRITICAL when 2 of 3 nodes are down, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Summary, "active_drives=1") {
		t.Errorf("summary should report active_drives=1, got %q", findings[0].Summary)
	}
}
