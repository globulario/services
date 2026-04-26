package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ── objectstore.minio.standalone_splitbrain ───────────────────────────────────

func TestStandaloneSplitbrain_OnlyOneNode_OK(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-1"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": minioActiveInventory(),
		},
	}
	findings := objectstoreMinioStandaloneSplitbrain{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for single-node cluster, got %d", len(findings))
	}
}

func TestStandaloneSplitbrain_TwoNodesStandalone_CRITICAL(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-1"},
			{NodeId: "node-2"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": minioActiveInventory(),
			"node-2": minioActiveInventory(),
		},
		// No desired state — both running standalone.
	}
	findings := objectstoreMinioStandaloneSplitbrain{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

func TestStandaloneSplitbrain_DistributedApplied_OK(t *testing.T) {
	desired := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeDistributed,
		Generation: 2,
	}
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-1"},
			{NodeId: "node-2"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": minioActiveInventory(),
			"node-2": minioActiveInventory(),
		},
		ObjectStoreDesired:           desired,
		ObjectStoreAppliedGeneration: 2, // applied = desired → OK
	}
	findings := objectstoreMinioStandaloneSplitbrain{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings when distributed mode applied, got %d", len(findings))
	}
}

func TestStandaloneSplitbrain_DistributedDesiredNotYetApplied_CRITICAL(t *testing.T) {
	desired := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeDistributed,
		Generation: 5,
	}
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-1"},
			{NodeId: "node-2"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": minioActiveInventory(),
			"node-2": minioActiveInventory(),
		},
		ObjectStoreDesired:           desired,
		ObjectStoreAppliedGeneration: 3, // behind desired
	}
	findings := objectstoreMinioStandaloneSplitbrain{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when distributed desired but not yet applied, got %d", len(findings))
	}
}

// ── objectstore.minio.unapproved_path ─────────────────────────────────────────

func TestUnapprovedPath_NoDesiredState_OK(t *testing.T) {
	snap := &collector.Snapshot{}
	findings := objectstoreMinioUnapprovedPath{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings with no desired state, got %d", len(findings))
	}
}

func TestUnapprovedPath_AllPathsApproved_OK(t *testing.T) {
	desired := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeDistributed,
		Generation: 1,
		NodePaths:  map[string]string{"10.0.0.1": "/data"},
	}
	admitted := []*config.AdmittedDisk{
		{NodeID: "node-1", NodeIP: "10.0.0.1", Path: "/data"},
	}
	snap := &collector.Snapshot{
		ObjectStoreDesired: desired,
		AdmittedDisks:      admitted,
	}
	findings := objectstoreMinioUnapprovedPath{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all paths admitted, got %d", len(findings))
	}
}

func TestUnapprovedPath_PathNotAdmitted_CRITICAL(t *testing.T) {
	desired := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeDistributed,
		Generation: 1,
		NodePaths:  map[string]string{"10.0.0.1": "/data"},
	}
	// No admitted disks at all.
	snap := &collector.Snapshot{
		ObjectStoreDesired: desired,
		AdmittedDisks:      nil,
	}
	findings := objectstoreMinioUnapprovedPath{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unapproved path, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

func TestUnapprovedPath_WrongPath_CRITICAL(t *testing.T) {
	desired := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeDistributed,
		Generation: 1,
		NodePaths:  map[string]string{"10.0.0.1": "/data/minio"},
	}
	admitted := []*config.AdmittedDisk{
		{NodeID: "node-1", NodeIP: "10.0.0.1", Path: "/data"},
	}
	snap := &collector.Snapshot{
		ObjectStoreDesired: desired,
		AdmittedDisks:      admitted,
	}
	findings := objectstoreMinioUnapprovedPath{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when desired path differs from admitted path, got %d", len(findings))
	}
}

// ── objectstore.minio.quorum_shape ────────────────────────────────────────────

func TestQuorumShape_StandaloneMode_OK(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Mode: config.ObjectStoreModeStandalone,
		},
	}
	findings := objectstoreMinioQuorumShape{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings in standalone mode, got %d", len(findings))
	}
}

func TestQuorumShape_DistributedOneNode_CRITICAL(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Mode:          config.ObjectStoreModeDistributed,
			Nodes:         []string{"10.0.0.1"},
			DrivesPerNode: 1,
		},
	}
	findings := objectstoreMinioQuorumShape{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for 1-node distributed, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

func TestQuorumShape_TwoNodesTwoDrivesTotal_WARN(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Mode:          config.ObjectStoreModeDistributed,
			Nodes:         []string{"10.0.0.1", "10.0.0.2"},
			DrivesPerNode: 1, // 2 nodes × 1 drive = 2 total drives < 4
		},
	}
	findings := objectstoreMinioQuorumShape{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for 2 total drives, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN, got %v", findings[0].Severity)
	}
}

func TestQuorumShape_FourDrives_OK(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Mode:          config.ObjectStoreModeDistributed,
			Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4"},
			DrivesPerNode: 1, // 4 × 1 = 4 drives
		},
	}
	findings := objectstoreMinioQuorumShape{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for 4-node/1-drive pool, got %d", len(findings))
	}
}

// ── objectstore.minio.existing_data_guard ────────────────────────────────────

func TestExistingDataGuard_NoAdmittedDisks_OK(t *testing.T) {
	snap := &collector.Snapshot{}
	findings := objectstoreMinioExistingDataGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings with no admitted disks, got %d", len(findings))
	}
}

func TestExistingDataGuard_ExistingDataForceAcknowledged_OK(t *testing.T) {
	snap := &collector.Snapshot{
		AdmittedDisks: []*config.AdmittedDisk{
			{NodeID: "node-1", Path: "/data", ForceExistingData: true},
		},
		DiskCandidates: map[string][]*config.DiskCandidate{
			"node-1": {
				{MountPath: "/data", HasExistingData: true, HasMinioSys: false},
			},
		},
	}
	findings := objectstoreMinioExistingDataGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings when force_existing_data acknowledged, got %d", len(findings))
	}
}

func TestExistingDataGuard_ExistingDataNotAcknowledged_CRITICAL(t *testing.T) {
	snap := &collector.Snapshot{
		AdmittedDisks: []*config.AdmittedDisk{
			{NodeID: "node-1", Path: "/data", ForceExistingData: false},
		},
		DiskCandidates: map[string][]*config.DiskCandidate{
			"node-1": {
				{MountPath: "/data", HasExistingData: true, HasMinioSys: false},
			},
		},
	}
	findings := objectstoreMinioExistingDataGuard{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unacknowledged existing data, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

func TestExistingDataGuard_MinioSysPresent_OK(t *testing.T) {
	// A disk that has .minio.sys is a prior MinIO deployment — not a guard concern.
	snap := &collector.Snapshot{
		AdmittedDisks: []*config.AdmittedDisk{
			{NodeID: "node-1", Path: "/data", ForceExistingData: false},
		},
		DiskCandidates: map[string][]*config.DiskCandidate{
			"node-1": {
				{MountPath: "/data", HasExistingData: true, HasMinioSys: true},
			},
		},
	}
	findings := objectstoreMinioExistingDataGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings when disk has .minio.sys (prior deployment), got %d", len(findings))
	}
}
