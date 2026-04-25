package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// TestDoctorFailsOnDNSMinioEndpoint verifies that objectstoreEndpointDNSWildcard
// fires a CRITICAL finding when the desired state endpoint is a DNS hostname.
// DNS wildcards resolve round-robin to all cluster nodes — most with empty MinIO
// instances — causing silent object-not-found errors.
func TestDoctorFailsOnDNSMinioEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{"wildcard hostname", "minio.globular.internal:9000"},
		{"plain hostname", "minio-primary:9000"},
		{"hostname without port", "minio.globular.internal"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := &collector.Snapshot{
				ObjectStoreDesired: &config.ObjectStoreDesiredState{
					Endpoint: tc.endpoint,
					Mode:     config.ObjectStoreModeStandalone,
				},
			}
			findings := objectstoreEndpointDNSWildcard{}.Evaluate(snap, Config{})
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding for DNS endpoint %q, got %d", tc.endpoint, len(findings))
			}
			f := findings[0]
			if f.InvariantID != "objectstore.endpoint_dns_wildcard" {
				t.Errorf("wrong invariant_id: %s", f.InvariantID)
			}
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
				t.Errorf("expected CRITICAL severity, got %v", f.Severity)
			}
			if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
				t.Errorf("expected INVARIANT_FAIL status, got %v", f.InvariantStatus)
			}
		})
	}
}

// TestDoctorOKOnIPMinioEndpoint verifies that objectstoreEndpointDNSWildcard
// does NOT fire when the endpoint is a bare IP:port.
func TestDoctorOKOnIPMinioEndpoint(t *testing.T) {
	cases := []string{"10.0.0.63:9000", "10.0.0.100:9000", "192.168.1.5:9000"}
	for _, endpoint := range cases {
		snap := &collector.Snapshot{
			ObjectStoreDesired: &config.ObjectStoreDesiredState{
				Endpoint: endpoint,
				Mode:     config.ObjectStoreModeStandalone,
			},
		}
		findings := objectstoreEndpointDNSWildcard{}.Evaluate(snap, Config{})
		if len(findings) != 0 {
			t.Errorf("endpoint %q: expected 0 findings for bare IP, got %d: %+v", endpoint, len(findings), findings)
		}
	}
}

// TestDoctorFailsOnStandaloneInMultiNodeCluster verifies that
// objectstoreStandaloneInCluster fires a WARN finding when MinIO is configured
// as standalone but the cluster has more than one node.
// Standalone mode means only one node holds data — all other nodes see an empty bucket.
func TestDoctorFailsOnStandaloneInMultiNodeCluster(t *testing.T) {
	cases := []struct {
		name      string
		nodeCount int
		wantFire  bool
	}{
		{"single node — OK", 1, false},
		{"two nodes — fires", 2, true},
		{"three nodes — fires", 3, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := make([]*cluster_controllerpb.NodeRecord, tc.nodeCount)
			for i := range nodes {
				nodes[i] = &cluster_controllerpb.NodeRecord{NodeId: "node"}
			}
			snap := &collector.Snapshot{
				ObjectStoreDesired: &config.ObjectStoreDesiredState{
					Endpoint: "10.0.0.63:9000",
					Mode:     config.ObjectStoreModeStandalone,
				},
				Nodes: nodes,
			}
			findings := objectstoreStandaloneInCluster{}.Evaluate(snap, Config{})
			if tc.wantFire && len(findings) == 0 {
				t.Fatalf("expected finding for standalone mode with %d nodes, got none", tc.nodeCount)
			}
			if !tc.wantFire && len(findings) != 0 {
				t.Fatalf("expected no finding for standalone with %d node(s), got %d", tc.nodeCount, len(findings))
			}
			if tc.wantFire {
				f := findings[0]
				if f.InvariantID != "objectstore.standalone_in_cluster" {
					t.Errorf("wrong invariant_id: %s", f.InvariantID)
				}
				if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
					t.Errorf("expected INVARIANT_FAIL, got %v", f.InvariantStatus)
				}
			}
		})
	}
}

// TestDoctorDistributedModeOK verifies that objectstoreStandaloneInCluster
// does NOT fire when the objectstore is in distributed mode.
func TestDoctorDistributedModeOK(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Endpoint: "10.0.0.63:9000",
			Mode:     config.ObjectStoreModeDistributed,
		},
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node1"}, {NodeId: "node2"}, {NodeId: "node3"},
		},
	}
	findings := objectstoreStandaloneInCluster{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("distributed mode with 3 nodes should produce 0 findings, got %d", len(findings))
	}
}

// TestDoctorNoDesiredState_NoNodes verifies that objectstoreNoDesiredState
// does NOT fire when there are no nodes (fresh single-node install).
func TestDoctorNoDesiredState_NoNodes(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: nil,
		Nodes:              nil,
	}
	findings := objectstoreNoDesiredState{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings with no nodes and no desired state, got %d", len(findings))
	}
}

// TestDoctorNoDesiredState_WithStorageNodes verifies that objectstoreNoDesiredState
// fires when storage nodes exist but no desired state has been published.
func TestDoctorNoDesiredState_WithStorageNodes(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: nil,
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node1", Profiles: []string{"core", "storage"}},
		},
	}
	findings := objectstoreNoDesiredState{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when storage nodes exist but desired state missing, got %d", len(findings))
	}
	if findings[0].InvariantID != "objectstore.no_desired_state" {
		t.Errorf("wrong invariant_id: %s", findings[0].InvariantID)
	}
}
