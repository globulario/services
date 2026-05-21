package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// helper: build a snapshot with a single node carrying the given IPs.
func snapshotWithIPs(ips []string, etcdPhase string) *collector.Snapshot {
	identity := &cluster_controllerpb.NodeIdentity{
		Hostname: "globule-test",
		Ips:      ips,
	}
	meta := map[string]string{}
	if etcdPhase != "" {
		meta["etcd_join_phase"] = etcdPhase
	}
	node := &cluster_controllerpb.NodeRecord{
		NodeId:   "test-node-uuid",
		Identity: identity,
		Profiles: []string{"core", "control-plane", "storage"},
		Metadata: meta,
	}
	return &collector.Snapshot{Nodes: []*cluster_controllerpb.NodeRecord{node}}
}

// Wired primary + docker0 secondary: advisory only (INVARIANT_PASS).
// This is the most common "fake alarm" on dev clusters and we must NOT
// open an OPEN incident for it.
func TestNodeMultiIP_DockerBridgeAdvisoryWhenPrimaryIsWired(t *testing.T) {
	snap := snapshotWithIPs([]string{"10.0.0.63", "172.17.0.1"}, "verified")
	findings := (nodeMultiIP{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected one multi_ip finding (advisory)")
	}
	f := findings[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Errorf("docker0 + wired primary must be INVARIANT_PASS (advisory); got %v", f.InvariantStatus)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Errorf("severity must stay INFO for bridge-only secondaries; got %v", f.Severity)
	}
}

// All known container-runtime / VM bridge ranges should produce
// advisory findings.
func TestNodeMultiIP_KnownBridgeRangesAreAllAdvisory(t *testing.T) {
	cases := []string{
		"172.17.0.1",     // docker0
		"172.18.0.1",     // docker custom
		"10.42.0.1",      // k3s
		"10.88.0.1",      // podman
		"192.168.122.1",  // libvirt
		"192.168.49.2",   // minikube
		"169.254.169.254",
	}
	for _, bridge := range cases {
		t.Run(bridge, func(t *testing.T) {
			snap := snapshotWithIPs([]string{"10.0.0.63", bridge}, "verified")
			findings := (nodeMultiIP{}).Evaluate(snap, testConfig())
			if len(findings) == 0 {
				t.Fatalf("expected finding for primary + %s", bridge)
			}
			if findings[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
				t.Errorf("bridge IP %s must be advisory PASS; got %v", bridge, findings[0].InvariantStatus)
			}
		})
	}
}

// Two real wired/WiFi IPs (neither is a bridge): elevate to WARN +
// INVARIANT_FAIL so the operator notices. Etcd peer-URL ambiguity
// stays a real risk.
func TestNodeMultiIP_TwoRealNICsStaysFailWhenEtcdActive(t *testing.T) {
	snap := snapshotWithIPs([]string{"10.0.0.63", "192.168.1.42"}, "verified")
	findings := (nodeMultiIP{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	f := findings[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("two real NICs on an etcd node must be INVARIANT_FAIL; got %v", f.InvariantStatus)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("two real NICs on etcd-verified node must elevate to WARN; got %v", f.Severity)
	}
}

// Mixed secondaries (one bridge, one real): not "all bridges", so the
// WARN path runs. Conservative — any real second NIC reopens the
// operator-attention case.
func TestNodeMultiIP_MixedSecondariesElevatesToWarn(t *testing.T) {
	snap := snapshotWithIPs([]string{"10.0.0.63", "172.17.0.1", "192.168.1.42"}, "verified")
	findings := (nodeMultiIP{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if findings[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("mixed bridge + real NIC must stay INVARIANT_FAIL; got %v", findings[0].InvariantStatus)
	}
}

func TestIsBridgeIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"172.17.0.1", true},
		{"172.18.5.1", true},
		{"10.42.0.1", true},
		{"10.88.0.0", true},
		{"169.254.169.254", true},
		{"192.168.122.1", true},
		{"192.168.49.2", true},

		{"10.0.0.63", false}, // typical wired
		{"192.168.1.42", false},
		{"172.16.0.1", false}, // outside docker ranges
		{"", false},
		{"not-an-ip", false},
	}
	for _, c := range cases {
		if got := isBridgeIP(c.ip); got != c.want {
			t.Errorf("isBridgeIP(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}
