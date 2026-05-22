package render

// drift_report_keepalived_test.go — Regression for the keepalived
// drift-item waiver. The rules package already suppresses the
// node.systemd.units_running finding when ingress is explicitly
// disabled (see node_units_running_ingress_test.go); without the
// matching waiver here the drift report still surfaced
// "keepalived.service inactive (desired active)" on every Day-0
// cluster — defeating the cluster-doctor's ingress gating.

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestDriftReport_KeepalivedSilentWhenIngressDisabled(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "keepalived.service", State: "inactive"},
		}}},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"disabled","explicit_disabled":true}`,
	}
	report := DriftReport(snap, "", "test", Freshness{})
	for _, item := range report.GetItems() {
		if item.GetEntityRef() == "keepalived.service" {
			t.Errorf("keepalived drift must be suppressed when ingress disabled, got: %+v", item)
		}
	}
}

func TestDriftReport_KeepalivedFiresWhenIngressActive(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "keepalived.service", State: "inactive"},
		}}},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"vip","explicit_disabled":false,"vip":"10.0.0.100"}`,
	}
	report := DriftReport(snap, "", "test", Freshness{})
	hit := false
	for _, item := range report.GetItems() {
		if item.GetEntityRef() == "keepalived.service" && item.GetCategory() == cluster_doctorpb.DriftCategory_UNIT_STOPPED {
			hit = true
		}
	}
	if !hit {
		t.Fatal("keepalived inactive with ingress active must surface a drift item")
	}
}

func TestDriftReport_OtherUnitsStillFire(t *testing.T) {
	// Narrow waiver: any other inactive unit (e.g. minio) must still
	// produce a drift item even when ingress is disabled.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "keepalived.service", State: "inactive"},
			{Name: "globular-minio.service", State: "inactive"},
		}}},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"disabled","explicit_disabled":true}`,
	}
	report := DriftReport(snap, "", "test", Freshness{})
	sawMinio := false
	for _, item := range report.GetItems() {
		if item.GetEntityRef() == "keepalived.service" {
			t.Errorf("keepalived must be suppressed; got %+v", item)
		}
		if item.GetEntityRef() == "globular-minio.service" {
			sawMinio = true
		}
	}
	if !sawMinio {
		t.Fatal("globular-minio.service inactive must still surface a drift item when ingress disabled")
	}
}
