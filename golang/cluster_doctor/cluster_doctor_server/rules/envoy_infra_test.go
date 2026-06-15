package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func snapWithEnvoyProbe(nid string, probe *cluster_controllerpb.InfraProbeResult) *collector.Snapshot {
	return &collector.Snapshot{
		Nodes: nodeListOne(nid),
		InfraProbes: map[string]*node_agentpb.GetInfraProbeResponse{
			nid: {Results: []*cluster_controllerpb.InfraProbeResult{probe}},
		},
	}
}

func servingEnvoyProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:    "envoy",
		Installed:    true,
		DaemonActive: true,
		Healthy:      true,
		ConfigValid:  true,
		Runtime: map[string]string{
			"admin_reachable": "true", "ready": "true",
			"cds_update_success": "4", "lds_update_attempt": "4",
			"lds_update_success": "4", "active_listeners": "2", "active_clusters": "3",
		},
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State: cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY, StateLabel: "member_ready",
		},
	}
}

func TestEnvoyInfraConfigValid_AllGood(t *testing.T) {
	snap := snapWithEnvoyProbe("node-1", servingEnvoyProbe())
	if f := (envoyInfraConfigValid{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(f), f)
	}
}

func TestEnvoyInfraConfigValid_MissingLDS(t *testing.T) {
	p := servingEnvoyProbe()
	p.ConfigValid = false
	p.Violations = []*cluster_controllerpb.InfraViolation{
		{Id: "envoy.config_valid", Severity: "CRITICAL", Message: "no lds_config — listeners never load", Evidence: "lds_config=absent", Remediation: "fix gateway"},
	}
	snap := snapWithEnvoyProbe("node-1", p)
	f := (envoyInfraConfigValid{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL finding, got %+v", f)
	}
}

func TestEnvoyInfraLDSProgress_WedgeDetected(t *testing.T) {
	p := servingEnvoyProbe()
	p.Healthy = false
	p.Runtime = map[string]string{"admin_reachable": "true", "cds_update_success": "4", "lds_update_attempt": "0"}
	snap := snapWithEnvoyProbe("node-1", p)
	f := (envoyInfraLDSProgress{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Fatalf("expected 1 CRITICAL wedge finding, got %+v", f)
	}
	if f[0].InvariantID != "envoy.lds_progress_required_for_http_mesh_readiness" {
		t.Errorf("expected the finding to pin the LDS readiness invariant, got %q", f[0].InvariantID)
	}
}

func TestEnvoyInfraLDSProgress_NotFiredWhenHealthy(t *testing.T) {
	snap := snapWithEnvoyProbe("node-1", servingEnvoyProbe())
	if f := (envoyInfraLDSProgress{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected no wedge finding when LDS has progressed, got %+v", f)
	}
}

func TestEnvoyInfraLDSProgress_NotFiredDuringColdInit(t *testing.T) {
	p := servingEnvoyProbe()
	p.Healthy = false
	p.Runtime = map[string]string{"admin_reachable": "true", "cds_update_success": "0", "lds_update_attempt": "0"}
	snap := snapWithEnvoyProbe("node-1", p)
	if f := (envoyInfraLDSProgress{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("cold init (cds=0) must not be classified as a wedge, got %+v", f)
	}
}

func TestEnvoyInfraListenersActive_NoneActive(t *testing.T) {
	p := servingEnvoyProbe()
	p.Healthy = false
	p.Runtime = map[string]string{"admin_reachable": "true", "cds_update_success": "4", "lds_update_attempt": "4", "lds_update_rejected": "2", "active_listeners": "0"}
	snap := snapWithEnvoyProbe("node-1", p)
	f := (envoyInfraListenersActive{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding for no active listeners, got %+v", f)
	}
}

func TestEnvoyInfraProbeRequired_WhenInstalledButMissing(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: nodeListOne("node-1"),
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Units: []*node_agentpb.UnitStatus{{Name: "globular-envoy.service"}}},
		},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	f := (envoyInfraProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig())
	if len(f) != 1 || f[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("expected 1 ERROR finding (installed but no probe), got %+v", f)
	}
}

func TestEnvoyInfraProbeRequired_NotInstalledSilent(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:                       nodeListOne("node-1"),
		Inventories:                 map[string]*node_agentpb.Inventory{},
		InfraProbes:                 map[string]*node_agentpb.GetInfraProbeResponse{},
		InfraProbeCapabilityMissing: map[string]bool{},
	}
	if f := (envoyInfraProbeRequiredWhenInstalled{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Fatalf("expected silence when envoy not installed, got %+v", f)
	}
}
