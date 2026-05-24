package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

func TestIngressSpecMissing_Day0Suppressed(t *testing.T) {
	inv := ingressSpecMissing{}
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		CriticalKeyPresent: map[string]bool{
			"/globular/system/config": false,
			"/globular/nodes/":        false,
			"/globular/resources/":    false,
		},
		IngressSpecPresent: false,
	}

	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Fatalf("expected ingress.spec_missing suppressed in day-0 bootstrap, got %d findings", len(got))
	}
}

func TestIngressSpecMissing_PostBootstrapStillFails(t *testing.T) {
	inv := ingressSpecMissing{}
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}, {NodeId: "n2"}},
		CriticalKeyPresent: map[string]bool{
			"/globular/system/config": true,
			"/globular/nodes/":        true,
			"/globular/resources/":    true,
		},
		IngressSpecPresent: false,
	}

	if got := inv.Evaluate(snap, Config{}); len(got) != 1 {
		t.Fatalf("expected ingress.spec_missing finding post-bootstrap, got %d", len(got))
	}
}

func TestRepositoryKeyspaceRFPolicyViolation_FiresOnRepositoryOnly(t *testing.T) {
	inv := repositoryKeyspaceRFPolicyViolation{}
	snap := &collector.Snapshot{
		ScyllaSchemaGuardStatus: map[string]map[string]interface{}{
			"repository": {
				"violation":   true,
				"current_rf":  float64(1),
				"required_rf": float64(3),
				"last_error":  "",
			},
			"dns": {
				"violation":   true,
				"current_rf":  float64(1),
				"required_rf": float64(3),
				"last_error":  "",
			},
		},
	}

	got := inv.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected exactly one repository RF finding, got %d", len(got))
	}
	if got[0].InvariantID != "repository.keyspace.rf_policy_violation" {
		t.Fatalf("unexpected invariant id: %s", got[0].InvariantID)
	}
}
