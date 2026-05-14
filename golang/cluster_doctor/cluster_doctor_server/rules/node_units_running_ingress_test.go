package rules

// node_units_running_ingress_test.go — Regression for the keepalived
// false-positive suppression in the systemd-units rule. The companion
// installed_state_runtime_mismatch rule has its own keepalived tests; this
// file pins the second emission path so both stay quiet on a healthy Day-0
// cluster and both stay loud when ingress is actually configured.

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestNodeUnitsRunning_KeepalivedSilentWhenIngressDisabled(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:              []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories:        map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{{Name: "keepalived.service", State: "inactive"}}}},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"disabled","explicit_disabled":true}`,
	}
	findings := (nodeUnitsRunning{}).Evaluate(snap, testConfig())
	for _, f := range findings {
		if f.InvariantID == "node.systemd.units_running" {
			t.Errorf("keepalived inactive must be silent when ingress disabled, got finding: %+v", f)
		}
	}
}

func TestNodeUnitsRunning_KeepalivedFiresWhenIngressActive(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:              []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories:        map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{{Name: "keepalived.service", State: "inactive"}}}},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"vip","explicit_disabled":false,"vip":"10.0.0.100"}`,
	}
	findings := (nodeUnitsRunning{}).Evaluate(snap, testConfig())
	hit := false
	for _, f := range findings {
		if f.InvariantID == "node.systemd.units_running" {
			hit = true
		}
	}
	if !hit {
		t.Fatal("keepalived inactive with ingress active must surface a finding")
	}
}

func TestNodeUnitsRunning_OtherUnitsStillFire(t *testing.T) {
	// The suppression must be narrow: any other inactive unit (e.g. minio)
	// remains a real problem even when ingress is disabled. This guards
	// against an over-broad future patch.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "keepalived.service", State: "inactive"},
			{Name: "globular-minio.service", State: "inactive"},
		}}},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"disabled","explicit_disabled":true}`,
	}
	findings := (nodeUnitsRunning{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("globular-minio.service inactive must still fire even when ingress disabled")
	}
	for _, f := range findings {
		if f.EntityRef == "n1/keepalived.service" {
			t.Errorf("keepalived must be suppressed; got %+v", f)
		}
	}
}
