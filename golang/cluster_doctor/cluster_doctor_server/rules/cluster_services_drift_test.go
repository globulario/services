package rules

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

func TestClusterServicesDrift_NoDrift(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
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
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
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

// W01: drift aging severity tests.

func TestClusterServicesDrift_UnknownAge_IsWarn(t *testing.T) {
	// No NodeDriftAge entry → unknown age → WARN.
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", DesiredServicesHash: "aaa", AppliedServicesHash: "bbb"},
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN for unknown age, got %v", findings[0].Severity)
	}
}

func TestClusterServicesDrift_ShortAge_IsWarn(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", DesiredServicesHash: "aaa", AppliedServicesHash: "bbb"},
		},
		NodeDriftAge: map[string]time.Duration{
			"n1": 90 * time.Second, // 1.5 min < 5 min threshold
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN for 1.5min drift, got %v", findings[0].Severity)
	}
}

func TestClusterServicesDrift_LongAge_IsError(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", DesiredServicesHash: "aaa", AppliedServicesHash: "bbb"},
		},
		NodeDriftAge: map[string]time.Duration{
			"n1": 8 * time.Minute, // > 5 min threshold
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR for 8min drift, got %v", findings[0].Severity)
	}
}

func TestClusterServicesDrift_AgeInEvidence(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", DesiredServicesHash: "aaa", AppliedServicesHash: "bbb"},
		},
		NodeDriftAge: map[string]time.Duration{
			"n1": 7 * time.Minute,
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	kv := findings[0].Evidence[0].KeyValues
	if kv["drift_age_seconds"] == "" || kv["drift_age_seconds"] == "0" {
		t.Errorf("expected drift_age_seconds in evidence, got %v", kv)
	}
}

func TestClusterServicesDrift_EmptyDesiredHash(t *testing.T) {
	// If desired hash is empty (controller has no desired state), skip.
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"node-1": {NodeId: "node-1", DesiredServicesHash: "", AppliedServicesHash: "def456"},
		},
	}
	findings := (clusterServicesDrift{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when desired hash is empty, got %d", len(findings))
	}
}
