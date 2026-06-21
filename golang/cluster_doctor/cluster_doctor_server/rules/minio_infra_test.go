package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func snapWithMinioProbe(nid string, probe *cluster_controllerpb.InfraProbeResult) *collector.Snapshot {
	return &collector.Snapshot{
		Nodes: nodeListOne(nid),
		InfraProbes: map[string]*node_agentpb.GetInfraProbeResponse{
			nid: {Results: []*cluster_controllerpb.InfraProbeResult{probe}},
		},
	}
}

func healthyMinioProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:     "minio",
		Installed:     true,
		DaemonActive:  true,
		Healthy:       true,
		ConfigValid:   true,
		ExpectedPeers: []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		Runtime:       map[string]string{"live": "true", "write_quorum": "true", "read_quorum": "true"},
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:      cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY,
			StateLabel: "member_ready",
		},
	}
}

func TestMinioInfraConfigValid_AllGood(t *testing.T) {
	snap := snapWithMinioProbe("node-1", healthyMinioProbe())
	if f := (minioInfraConfigValid{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(f), f)
	}
}

func TestMinioInfraTopologyMatchesDesired_SplitBrain(t *testing.T) {
	p := healthyMinioProbe()
	p.ConfigValid = false
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "minio.topology_matches_desired", Severity: "CRITICAL", Message: "desired distributed but rendered standalone", Evidence: "desired_mode=distributed rendered_mode=standalone", Remediation: "fix owner"},
	}
	snap := snapWithMinioProbe("node-1", p)
	f := (minioInfraTopologyMatchesDesired{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding, got %+v", f)
	}
}

func TestMinioInfraLoopbackForbidden(t *testing.T) {
	p := healthyMinioProbe()
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "minio.loopback_forbidden", Severity: "CRITICAL", Message: "volume advertises 127.0.0.1", Evidence: "volume=https://127.0.0.1:9000/data", Remediation: "fix owner"},
	}
	snap := snapWithMinioProbe("node-1", p)
	f := (minioInfraLoopbackForbidden{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding, got %+v", f)
	}
}

func TestMinioInfraWriteQuorum_Lost(t *testing.T) {
	p := healthyMinioProbe()
	p.Healthy = false
	p.Runtime = map[string]string{"live": "true", "write_quorum": "false", "read_quorum": "false"}
	p.Lifecycle = &cluster_controllerpb.InfraLifecycleObservation{
		State: cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED, StateLabel: "degraded",
		BlockingReason: "MinIO is live but the pool has lost write quorum",
	}
	snap := snapWithMinioProbe("node-1", p)
	f := (minioInfraWriteQuorum{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding for write-quorum loss, got %+v", f)
	}
}

func TestMinioInfraWriteQuorum_NotFiredWhenHealthy(t *testing.T) {
	snap := snapWithMinioProbe("node-1", healthyMinioProbe())
	if f := (minioInfraWriteQuorum{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected no write-quorum finding for a healthy member, got %+v", f)
	}
}

func TestMinioInfraNotStalled_Detection(t *testing.T) {
	p := healthyMinioProbe()
	p.Healthy = false
	p.Lifecycle = &cluster_controllerpb.InfraLifecycleObservation{
		State: cluster_controllerpb.InfraLifecycleState_INFRA_STALLED, StateLabel: "stalled",
		BlockingReason: "daemon is active but this node would form an isolated single-node store",
	}
	snap := snapWithMinioProbe("node-1", p)
	f := (minioInfraNotStalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding for stalled member, got %+v", f)
	}
}

func TestMinioInfraProbeRequired_WhenInstalledButMissing(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: nodeListOne("node-1"),
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Units: []*node_agentpb.UnitStatus{{Name: "globular-minio.service"}}},
		},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	f := (minioInfraProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding (installed but no probe), got %+v", f)
	}
}

func TestMinioInfraProbeRequired_NotInstalledSilent(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:                       nodeListOne("node-1"),
		Inventories:                 map[string]*node_agentpb.Inventory{},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	if f := (minioInfraProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected silence when minio not installed, got %+v", f)
	}
}
