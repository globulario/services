package rules

import (
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
)

func TestClusterServicesDrift_NoDrift(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*clustercontrollerpb.NodeHealth{
			"node-1": {NodeId: "node-1", DesiredServicesHash: "abc123", AppliedServicesHash: "abc123"},
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for matching hashes, got %d", len(findings))
	}
}

func TestClusterServicesDrift_HashMismatch(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*clustercontrollerpb.NodeHealth{
			"node-1": {NodeId: "node-1", DesiredServicesHash: "abc123", AppliedServicesHash: "def456"},
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for hash mismatch, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "cluster.services.drift" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
	if f.EntityRef != "node-1" {
		t.Errorf("wrong entity_ref: %s", f.EntityRef)
	}
	kv := f.Evidence[0].KeyValues
	if kv["desired_hash"] != "abc123" || kv["applied_hash"] != "def456" {
		t.Errorf("wrong hashes in evidence: %v", kv)
	}
}

func TestClusterServicesDrift_EmptyDesiredHash(t *testing.T) {
	// If desired hash is empty (controller has no desired state), skip.
	snap := &collector.Snapshot{
		NodeHealths: map[string]*clustercontrollerpb.NodeHealth{
			"node-1": {NodeId: "node-1", DesiredServicesHash: "", AppliedServicesHash: "def456"},
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when desired hash is empty, got %d", len(findings))
	}
}
