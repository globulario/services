package rules

import (
	"errors"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// helper: a snapshot with nodeCount registered nodes and the given
// desired-services hashes keyed by node id.
func snapWithDesired(nodeCount int, desired map[string]string) *collector.Snapshot {
	nodes := make([]*cluster_controllerpb.NodeRecord, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, &cluster_controllerpb.NodeRecord{})
	}
	healths := make(map[string]*cluster_controllerpb.NodeHealth, len(desired))
	for id, h := range desired {
		healths[id] = &cluster_controllerpb.NodeHealth{DesiredServicesHash: h}
	}
	return &collector.Snapshot{Nodes: nodes, NodeHealths: healths}
}

// The condition this rule exists for: a formed cluster where no node carries any
// desired state. cluster.services.drift is silent here because desired ==
// applied == empty, so this rule must not be.
func TestDesiredStateAbsent_AllNodesEmpty(t *testing.T) {
	snap := snapWithDesired(3, map[string]string{
		"n1": "",
		"n2": "",
		"n3": "services:none",
	})
	got := (desiredStateAbsent{}).Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("all nodes without desired state → 1 finding, got %d", len(got))
	}
	if got[0].EntityRef != "cluster" {
		t.Errorf("finding is cluster-scoped, got EntityRef=%q", got[0].EntityRef)
	}
	if got[0].InvariantID != "cluster.desired_state.absent" {
		t.Errorf("unexpected InvariantID %q", got[0].InvariantID)
	}
}

// One node carrying desired state means Layer 2 exists. A per-node gap is
// placement intent (a compute node may legitimately be assigned nothing), and
// the desired-vs-applied question belongs to cluster.services.drift from there.
func TestDesiredStateAbsent_SomeNodesHaveDesired(t *testing.T) {
	snap := snapWithDesired(3, map[string]string{
		"n1": "services:abc123",
		"n2": "",
		"n3": "",
	})
	if got := (desiredStateAbsent{}).Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("Layer 2 present on any node → no finding, got %d", len(got))
	}
}

// Absence of data must never become a finding. A failed GetClusterHealthV1
// leaves NodeHealths empty, which means "controller unreachable", not "no
// desired state" — reporting an absent Layer 2 on that basis would be the exact
// not-found-where-vs-does-not-exist mistake this rule guards against.
func TestDesiredStateAbsent_HealthSourceErrored(t *testing.T) {
	snap := snapWithDesired(3, map[string]string{"n1": "", "n2": ""})
	snap.DataErrors = []collector.DataError{{
		Service: "cluster_controller",
		RPC:     "GetClusterHealthV1",
		Err:     errors.New("connection refused"),
	}}
	if got := (desiredStateAbsent{}).Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("errored health source → silent, got %d findings", len(got))
	}
}

func TestDesiredStateAbsent_ListNodesErrored(t *testing.T) {
	snap := snapWithDesired(3, map[string]string{"n1": ""})
	snap.DataErrors = []collector.DataError{{
		Service: "cluster_controller",
		RPC:     "ListNodes",
		Err:     errors.New("deadline exceeded"),
	}}
	if got := (desiredStateAbsent{}).Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("errored node source → silent, got %d findings", len(got))
	}
}

// An empty snapshot is no data, not a broken cluster.
func TestDesiredStateAbsent_NoHealthsReported(t *testing.T) {
	snap := snapWithDesired(3, map[string]string{})
	if got := (desiredStateAbsent{}).Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("no healths reported → silent, got %d findings", len(got))
	}
}

// A cluster with no registered nodes has not been bootstrapped; having no
// desired state is correct there, not a violation.
func TestDesiredStateAbsent_NoNodesRegistered(t *testing.T) {
	snap := snapWithDesired(0, map[string]string{"n1": ""})
	if got := (desiredStateAbsent{}).Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("unbootstrapped cluster → silent, got %d findings", len(got))
	}
}
