package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func snapWithEtcdProbe(nid string, probe *cluster_controllerpb.InfraProbeResult) *collector.Snapshot {
	return &collector.Snapshot{
		Nodes: nodeListOne(nid),
		InfraProbes: map[string]*node_agentpb.GetInfraProbeResponse{
			nid: {Results: []*cluster_controllerpb.InfraProbeResult{probe}},
		},
	}
}

func healthyEtcdProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:     "etcd",
		Installed:     true,
		DaemonActive:  true,
		Healthy:       true,
		ConfigValid:   true,
		ExpectedPeers: []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		ObservedPeers: []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		PeersMatch:    true,
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:      cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY,
			StateLabel: "member_ready",
		},
	}
}

func TestEtcdInfraConfigValid_AllGood(t *testing.T) {
	snap := snapWithEtcdProbe("node-1", healthyEtcdProbe())
	if f := (etcdInfraConfigValid{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected 0 findings for valid config, got %d: %+v", len(f), f)
	}
}

func TestEtcdInfraLoopbackForbidden(t *testing.T) {
	p := healthyEtcdProbe()
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "etcd.loopback_forbidden", Severity: "CRITICAL", Message: "listen-peer-urls advertises a loopback address (127.0.0.1)", Evidence: "listen-peer-urls=https://127.0.0.1:2380", Remediation: "fix renderer"},
	}
	snap := snapWithEtcdProbe("node-1", p)
	f := (etcdInfraLoopbackForbidden{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding, got %+v", f)
	}
	if f[0].InvariantID != "etcd.loopback_forbidden" {
		t.Errorf("invariant=%s", f[0].InvariantID)
	}
}

func TestEtcdInfraConfigValid_SelfOnlyInitialCluster(t *testing.T) {
	p := healthyEtcdProbe()
	p.ConfigValid = false
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "etcd.config_valid", Severity: "ERROR", Message: "initial-cluster contains only this node", Evidence: "initial-cluster=globule-ryzen"},
	}
	snap := snapWithEtcdProbe("node-1", p)
	f := (etcdInfraConfigValid{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding, got %+v", f)
	}
}

func TestEtcdInfraPeersMatch_MissingMember(t *testing.T) {
	p := healthyEtcdProbe()
	p.PeersMatch = false
	p.ObservedPeers = []string{"10.0.0.63", "10.0.0.8"} // missing 10.0.0.20
	snap := snapWithEtcdProbe("node-1", p)
	f := (etcdInfraPeersMatchExpected{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding for missing member, got %+v", f)
	}
}

func TestEtcdInfraJoinStalled_Corrupt(t *testing.T) {
	p := healthyEtcdProbe()
	p.Healthy = false
	p.Lifecycle = &cluster_controllerpb.InfraLifecycleObservation{
		State:          cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
		StateLabel:     "stalled",
		BlockingReason: "etcd raised a CORRUPT alarm — this member's data is damaged",
	}
	snap := snapWithEtcdProbe("node-1", p)
	f := (etcdInfraJoinNotStalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding for stalled member, got %+v", f)
	}
}

func TestEtcdInfraProbeRequired_WhenInstalledButMissing(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: nodeListOne("node-1"),
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Units: []*node_agentpb.UnitStatus{{Name: "globular-etcd.service"}}},
		},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	f := (etcdInfraProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding (installed but no probe), got %+v", f)
	}
}

func TestEtcdInfraProbeRequired_NotInstalledSilent(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:                       nodeListOne("node-1"),
		Inventories:                 map[string]*node_agentpb.Inventory{},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	if f := (etcdInfraProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected silence when etcd not installed, got %+v", f)
	}
}
