package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func nodeRecord(id, ip string) *cluster_controllerpb.NodeRecord {
	return &cluster_controllerpb.NodeRecord{
		NodeId:   id,
		Identity: &cluster_controllerpb.NodeIdentity{AdvertiseIp: ip},
	}
}

func localCandidate(nodeID, mountPath, stableID string) *config.DiskCandidate {
	return &config.DiskCandidate{
		NodeID:    nodeID,
		Device:    "/dev/sdb1",
		MountPath: mountPath,
		FSType:    "ext4",
		StableID:  stableID,
		SizeBytes: 100 * 1024 * 1024 * 1024,
		Eligible:  true,
	}
}

func nfsCandidate(nodeID, mountPath, nfsSource string) *config.DiskCandidate {
	return &config.DiskCandidate{
		NodeID:         nodeID,
		Device:         nfsSource,
		MountPath:      mountPath,
		FSType:         "nfs4",
		MountSource:    nfsSource,
		IsNetworkMount: true,
		SizeBytes:      100 * 1024 * 1024 * 1024,
		Eligible:       true,
	}
}

func distributedDesired(nodes []string, paths map[string]string, drivesPerNode int) *config.ObjectStoreDesiredState {
	return &config.ObjectStoreDesiredState{
		Mode:          config.ObjectStoreModeDistributed,
		Generation:    1,
		Nodes:         nodes,
		NodePaths:     paths,
		DrivesPerNode: drivesPerNode,
	}
}

// ── objectstore.duplicate_physical_path ──────────────────────────────────────

// Scenario 1: ryzen uses NFS mount of dell's disk → same NFS source → CRITICAL.
func TestDuplicatePhysicalPath_SameNFSSource_CRITICAL(t *testing.T) {
	const (
		ryzID = "ryzen-node"
		dellID = "dell-node"
		nucID  = "nuc-node"
	)
	// ryzen: /mnt/dell-media served via NFS from dell (10.0.0.20:/mnt/data)
	// dell:  /mnt/data/data is local
	// nuc:   /mnt/data/data is local
	nfsSource := "10.0.0.20:/mnt/data"

	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord(ryzID, "10.0.0.63"),
			nodeRecord(dellID, "10.0.0.20"),
			nodeRecord(nucID, "10.0.0.8"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.63", "10.0.0.20", "10.0.0.8"},
			map[string]string{
				"10.0.0.63": "/mnt/dell-media/data",
				"10.0.0.20": "/mnt/data/data",
				"10.0.0.8":  "/mnt/data/data",
			},
			1,
		),
		DiskCandidates: map[string][]*config.DiskCandidate{
			ryzID:  {nfsCandidate(ryzID, "/mnt/dell-media", nfsSource)},
			dellID: {localCandidate(dellID, "/mnt/data", "uuid-dell-data")},
			nucID:  {localCandidate(nucID, "/mnt/data", "uuid-nuc-data")},
		},
	}

	findings := objectstoreDuplicatePhysicalPath{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for NFS overlap, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
	if findings[0].InvariantID != "objectstore.duplicate_physical_path" {
		t.Errorf("unexpected invariant ID: %s", findings[0].InvariantID)
	}
}

// Scenario 2: same FS UUID on two nodes → CRITICAL.
func TestDuplicatePhysicalPath_SameStableID_CRITICAL(t *testing.T) {
	const (
		nodeA = "node-a"
		nodeB = "node-b"
	)
	sharedUUID := "aaaa-bbbb-cccc"

	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord(nodeA, "10.0.0.1"),
			nodeRecord(nodeB, "10.0.0.2"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2"},
			map[string]string{"10.0.0.1": "/mnt/data", "10.0.0.2": "/mnt/data"},
			1,
		),
		DiskCandidates: map[string][]*config.DiskCandidate{
			nodeA: {localCandidate(nodeA, "/mnt/data", sharedUUID)},
			nodeB: {localCandidate(nodeB, "/mnt/data", sharedUUID)},
		},
	}

	findings := objectstoreDuplicatePhysicalPath{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for shared UUID, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

// Scenario 3: all nodes use distinct local disks → no finding.
func TestDuplicatePhysicalPath_AllDistinct_OK(t *testing.T) {
	const (
		nodeA = "node-a"
		nodeB = "node-b"
		nodeC = "node-c"
	)
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord(nodeA, "10.0.0.1"),
			nodeRecord(nodeB, "10.0.0.2"),
			nodeRecord(nodeC, "10.0.0.3"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			map[string]string{
				"10.0.0.1": "/mnt/data",
				"10.0.0.2": "/mnt/data",
				"10.0.0.3": "/mnt/data",
			},
			1,
		),
		DiskCandidates: map[string][]*config.DiskCandidate{
			nodeA: {localCandidate(nodeA, "/mnt/data", "uuid-a")},
			nodeB: {localCandidate(nodeB, "/mnt/data", "uuid-b")},
			nodeC: {localCandidate(nodeC, "/mnt/data", "uuid-c")},
		},
	}

	findings := objectstoreDuplicatePhysicalPath{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for distinct local disks, got %d: %v", len(findings), findings[0].Summary)
	}
}

// ── objectstore.network_mount_used ───────────────────────────────────────────

// Scenario 4: one pool node uses NFS → WARN.
func TestNetworkMountUsed_NFSPath_WARN(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord("node-a", "10.0.0.1"),
			nodeRecord("node-b", "10.0.0.2"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2"},
			map[string]string{
				"10.0.0.1": "/mnt/nfs-share/data",
				"10.0.0.2": "/mnt/local/data",
			},
			1,
		),
		DiskCandidates: map[string][]*config.DiskCandidate{
			"node-a": {nfsCandidate("node-a", "/mnt/nfs-share", "192.168.1.10:/exports/data")},
			"node-b": {localCandidate("node-b", "/mnt/local", "uuid-local")},
		},
	}

	findings := objectstoreNetworkMountUsed{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for NFS path, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN, got %v", findings[0].Severity)
	}
}

// All local block → no network mount warning.
func TestNetworkMountUsed_AllLocal_OK(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord("node-a", "10.0.0.1"),
			nodeRecord("node-b", "10.0.0.2"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2"},
			map[string]string{"10.0.0.1": "/mnt/data", "10.0.0.2": "/mnt/data"},
			1,
		),
		DiskCandidates: map[string][]*config.DiskCandidate{
			"node-a": {localCandidate("node-a", "/mnt/data", "uuid-a")},
			"node-b": {localCandidate("node-b", "/mnt/data", "uuid-b")},
		},
	}
	findings := objectstoreNetworkMountUsed{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for all-local pool, got %d", len(findings))
	}
}

// ── objectstore.zero_write_fault_tolerance ───────────────────────────────────

// Scenario 5: 3 nodes × 1 drive = 3 total (EC:1) → WARN.
func TestZeroWriteFaultTolerance_3x1_WARN(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			nil,
			1,
		),
	}
	findings := objectstoreZeroWriteFaultTolerance{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 WARN for 3-drive EC:1, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN, got %v", findings[0].Severity)
	}
}

// 4 nodes × 1 drive = 4 total (EC:2) → no zero_write_fault_tolerance warning.
func TestZeroWriteFaultTolerance_4x1_OK(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4"},
			nil,
			1,
		),
	}
	findings := objectstoreZeroWriteFaultTolerance{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for 4-drive EC:2, got %d", len(findings))
	}
}

// ── objectstore.write_quorum_lost ─────────────────────────────────────────────

// Scenario 6: MinIO applied, 2 of 3 nodes down → write quorum lost (2 active drives < 2 quorum).
// Wait — with 3 drives, write_quorum = 3 - 1 = 2. With 1 active node (1 drive), 1 < 2 → CRITICAL.
func TestWriteQuorumLost_OneNodeActive_CRITICAL(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord("node-a", "10.0.0.1"),
			nodeRecord("node-b", "10.0.0.2"),
			nodeRecord("node-c", "10.0.0.3"),
		},
		Inventories: map[string]*node_agentpb.Inventory{
			// Only node-a has minio active.
			"node-a": minioActiveInventory(),
			"node-b": minioInactiveInventory("failed"),
			"node-c": minioInactiveInventory("failed"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			nil,
			1,
		),
		ObjectStoreAppliedGeneration: 1, // topology has been applied
	}

	findings := objectstoreWriteQuorumLost{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 CRITICAL finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

// All 3 nodes active → no quorum loss.
func TestWriteQuorumLost_AllActive_OK(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord("node-a", "10.0.0.1"),
			nodeRecord("node-b", "10.0.0.2"),
			nodeRecord("node-c", "10.0.0.3"),
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-a": minioActiveInventory(),
			"node-b": minioActiveInventory(),
			"node-c": minioActiveInventory(),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			nil,
			1,
		),
		ObjectStoreAppliedGeneration: 1,
	}
	findings := objectstoreWriteQuorumLost{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all nodes active, got %d", len(findings))
	}
}

// ── objectstore.format_heal_deadlock ─────────────────────────────────────────

// Scenario 7: all 3 nodes down + all have .minio.sys → heal deadlock CRITICAL.
func TestFormatHealDeadlock_AllDownWithSys_CRITICAL(t *testing.T) {
	const (
		nodeA = "node-a"
		nodeB = "node-b"
		nodeC = "node-c"
	)
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord(nodeA, "10.0.0.1"),
			nodeRecord(nodeB, "10.0.0.2"),
			nodeRecord(nodeC, "10.0.0.3"),
		},
		Inventories: map[string]*node_agentpb.Inventory{
			nodeA: minioInactiveInventory("failed"),
			nodeB: minioInactiveInventory("failed"),
			nodeC: minioInactiveInventory("failed"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			map[string]string{
				"10.0.0.1": "/mnt/data",
				"10.0.0.2": "/mnt/data",
				"10.0.0.3": "/mnt/data",
			},
			1,
		),
		ObjectStoreAppliedGeneration: 1,
		DiskCandidates: map[string][]*config.DiskCandidate{
			nodeA: {{NodeID: nodeA, MountPath: "/mnt/data", HasMinioSys: true}},
			nodeB: {{NodeID: nodeB, MountPath: "/mnt/data", HasMinioSys: true}},
			nodeC: {{NodeID: nodeC, MountPath: "/mnt/data", HasMinioSys: true}},
		},
	}

	findings := objectstoreFormatHealDeadlock{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 CRITICAL finding for heal deadlock, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

// Scenario 8: all nodes down but none have .minio.sys (fresh cluster, no deadlock).
func TestFormatHealDeadlock_AllDownNoSys_OK(t *testing.T) {
	const (
		nodeA = "node-a"
		nodeB = "node-b"
		nodeC = "node-c"
	)
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			nodeRecord(nodeA, "10.0.0.1"),
			nodeRecord(nodeB, "10.0.0.2"),
			nodeRecord(nodeC, "10.0.0.3"),
		},
		Inventories: map[string]*node_agentpb.Inventory{
			nodeA: minioInactiveInventory("failed"),
			nodeB: minioInactiveInventory("failed"),
			nodeC: minioInactiveInventory("failed"),
		},
		ObjectStoreDesired: distributedDesired(
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			map[string]string{
				"10.0.0.1": "/mnt/data",
				"10.0.0.2": "/mnt/data",
				"10.0.0.3": "/mnt/data",
			},
			1,
		),
		ObjectStoreAppliedGeneration: 1,
		DiskCandidates: map[string][]*config.DiskCandidate{
			nodeA: {{NodeID: nodeA, MountPath: "/mnt/data", HasMinioSys: false}},
			nodeB: {{NodeID: nodeB, MountPath: "/mnt/data", HasMinioSys: false}},
			nodeC: {{NodeID: nodeC, MountPath: "/mnt/data", HasMinioSys: false}},
		},
	}

	findings := objectstoreFormatHealDeadlock{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings when .minio.sys absent (fresh cluster), got %d", len(findings))
	}
}

