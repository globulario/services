package rules

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func nodeListOne(nid string) []*cluster_controllerpb.NodeRecord {
	return []*cluster_controllerpb.NodeRecord{{NodeId: nid}}
}

func snapWithScyllaProbe(nid string, probe *cluster_controllerpb.InfraProbeResult) *collector.Snapshot {
	return &collector.Snapshot{
		Nodes: nodeListOne(nid),
		InfraProbes: map[string]*node_agentpb.GetInfraProbeResponse{
			nid: {Results: []*cluster_controllerpb.InfraProbeResult{probe}},
		},
	}
}

func healthyScyllaProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:     "scylladb",
		Installed:     true,
		DaemonActive:  true,
		Healthy:       true,
		ConfigValid:   true,
		ExpectedPeers: []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		ObservedPeers: []string{"10.0.0.8", "10.0.0.20"},
		PeersMatch:    true,
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:      cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY,
			StateLabel: "member_ready",
		},
	}
}

func TestScyllaConfigValid_AllGood(t *testing.T) {
	snap := snapWithScyllaProbe("node-1", healthyScyllaProbe())
	if f := (scyllaConfigValid{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected 0 findings for valid config, got %d: %+v", len(f), f)
	}
}

func TestScyllaConfigValid_LocalhostDetected(t *testing.T) {
	p := healthyScyllaProbe()
	p.ConfigValid = false
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "scylla.config_valid", Severity: "ERROR", Message: "cluster_name is empty", Evidence: "cluster_name=\"\""},
	}
	snap := snapWithScyllaProbe("node-1", p)
	f := (scyllaConfigValid{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding, got %+v", f)
	}
}

func TestScyllaLoopbackForbidden_RPC(t *testing.T) {
	p := healthyScyllaProbe()
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "scylla.loopback_forbidden", Severity: "CRITICAL", Message: "rpc_address is a loopback address (127.0.0.1)", Evidence: "rpc_address=127.0.0.1", Remediation: "fix renderer"},
	}
	snap := snapWithScyllaProbe("node-1", p)
	f := (scyllaLoopbackForbidden{}).Evaluate(snap, testConfig())
	if len(f) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(f))
	}
	if f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", f[0].Severity)
	}
	if f[0].InvariantID != "scylla.loopback_forbidden" {
		t.Errorf("invariant=%s", f[0].InvariantID)
	}
}

func TestScyllaPeersMatch_MissingPeer(t *testing.T) {
	p := healthyScyllaProbe()
	p.PeersMatch = false
	p.ObservedPeers = []string{"10.0.0.8"} // missing 10.0.0.20
	snap := snapWithScyllaProbe("node-1", p)
	f := (scyllaPeersMatchExpected{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding for missing peer, got %+v", f)
	}
}

func TestScyllaJoinStalled_Detection(t *testing.T) {
	p := healthyScyllaProbe()
	p.Healthy = false
	p.Lifecycle = &cluster_controllerpb.InfraLifecycleObservation{
		State:          cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
		StateLabel:     "stalled",
		BlockingReason: "daemon is active but listen_address is a loopback address (127.0.0.1)",
	}
	snap := snapWithScyllaProbe("node-1", p)
	f := (scyllaJoinNotStalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding for stalled join, got %+v", f)
	}
}

func TestScyllaProbeRequired_WhenInstalledButMissing(t *testing.T) {
	// ScyllaDB installed (inventory shows the unit) but no probe data at all.
	snap := &collector.Snapshot{
		Nodes: nodeListOne("node-1"),
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Units: []*node_agentpb.UnitStatus{{Name: "scylla-server.service"}}},
		},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	f := (scyllaProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding (installed but no probe), got %+v", f)
	}
}

func TestScyllaProbeRequired_CapabilityMissingIsWarn(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: nodeListOne("node-1"),
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Units: []*node_agentpb.UnitStatus{{Name: "scylla-server.service"}}},
		},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{"node-1": true},
	}
	f := (scyllaProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("expected 1 WARN finding (old binary), got %+v", f)
	}
}

// TestScyllaProbeRequired_HarvestErrorIsUnknownWarn reproduces the live
// 2026-06-22 incident: a single context-canceled infra-probe harvest made the
// doctor emit ERROR / INVARIANT_FAIL "ScyllaDB produced no infra probe" against
// a fully healthy node. When the no-probe cause is a collector error, the
// verdict must be INDETERMINATE (WARN + INVARIANT_UNKNOWN + CheckError), not a
// failure — "could not observe" is not "observed failure"
// (meta.harvest_and_yield_are_distinct_availability_dimensions). It must still
// emit a finding (never go silent: degraded_is_explicit_not_hidden).
func TestScyllaProbeRequired_HarvestErrorIsUnknownWarn(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: nodeListOne("node-1"),
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Units: []*node_agentpb.UnitStatus{{Name: "scylla-server.service"}}},
		},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
		// The harvest's GetInfraProbe sub-fetch errored (dial / context canceled).
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "node_agent@node-1", RPC: "GetInfraProbe", Err: context.Canceled},
		},
	}
	f := (scyllaProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 {
		t.Fatalf("expected exactly 1 finding (never silent under reduced harvest), got %d: %+v", len(f), f)
	}
	if f[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("harvest-error no-probe must be WARN, got %v", f[0].Severity)
	}
	if f[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("harvest-error no-probe must be INVARIANT_UNKNOWN, got %v", f[0].InvariantStatus)
	}
	if f[0].CheckError == "" {
		t.Error("harvest-error no-probe must set CheckError so aggregators do not count it as FAIL")
	}
}

func TestScyllaNoProbeData_NotInstalledSilent(t *testing.T) {
	// No ScyllaDB anywhere → silence is correct.
	snap := &collector.Snapshot{
		Nodes:                       nodeListOne("node-1"),
		Inventories:                 map[string]*node_agentpb.Inventory{},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	if f := (scyllaProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected silence when scylla not installed, got %+v", f)
	}
}
