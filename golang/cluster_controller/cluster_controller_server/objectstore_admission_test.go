package main

import (
	"context"
	"errors"
	"testing"

	configpkg "github.com/globulario/services/golang/config"
)

// candidateLoader returns an injectable DiskCandidate loader for tests.
// Pass loadErr != nil to simulate etcd failures.
func candidateLoader(
	byNodeID map[string][]*configpkg.DiskCandidate,
	loadErr error,
) func(ctx context.Context, nodeID string) ([]*configpkg.DiskCandidate, error) {
	return func(_ context.Context, nodeID string) ([]*configpkg.DiskCandidate, error) {
		if loadErr != nil {
			return nil, loadErr
		}
		return byNodeID[nodeID], nil
	}
}

// ── ValidateTopologyProposal ──────────────────────────────────────────────────

func TestValidateTopologyProposal_Valid(t *testing.T) {
	p := &configpkg.TopologyProposal{
		Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		NodePaths:     map[string]string{"10.0.0.1": "/data", "10.0.0.2": "/data", "10.0.0.3": "/data"},
		DrivesPerNode: 1,
	}
	errs := ValidateTopologyProposal(p, nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid proposal, got: %v", errs)
	}
}

func TestValidateTopologyProposal_InvalidIP(t *testing.T) {
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"not-an-ip"},
		NodePaths: map[string]string{"not-an-ip": "/data"},
	}
	errs := ValidateTopologyProposal(p, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid IP, got none")
	}
}

func TestValidateTopologyProposal_MissingPath(t *testing.T) {
	p := &configpkg.TopologyProposal{
		Nodes:         []string{"10.0.0.1", "10.0.0.2"},
		NodePaths:     map[string]string{"10.0.0.1": "/data"}, // node-2 path missing
		DrivesPerNode: 1,
	}
	errs := ValidateTopologyProposal(p, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for missing node path, got none")
	}
}

func TestValidateTopologyProposal_NonAbsolutePath(t *testing.T) {
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "relative/path"},
	}
	errs := ValidateTopologyProposal(p, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for non-absolute path, got none")
	}
}

func TestValidateTopologyProposal_EmptyNodes(t *testing.T) {
	p := &configpkg.TopologyProposal{Nodes: nil}
	errs := ValidateTopologyProposal(p, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for empty nodes list, got none")
	}
}

func TestValidateTopologyProposal_AdmittedPathMismatch(t *testing.T) {
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data/new"},
	}
	// Admitted records have /data/old but proposal asks for /data/new → reject.
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data/old": {NodeIP: "10.0.0.1", Path: "/data/old"}},
	}
	errs := ValidateTopologyProposal(p, admitted)
	if len(errs) == 0 {
		t.Fatal("expected error when proposal path differs from admitted path, got none")
	}
}

func TestValidateTopologyProposal_MultipleAdmittedDisksPerNode(t *testing.T) {
	// Two admitted disks on the same node — second is used in proposal.
	p := &configpkg.TopologyProposal{
		Nodes:         []string{"10.0.0.1", "10.0.0.2"},
		NodePaths:     map[string]string{"10.0.0.1": "/data/disk2", "10.0.0.2": "/data"},
		DrivesPerNode: 1,
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {
			"/data/disk1": {NodeIP: "10.0.0.1", Path: "/data/disk1"},
			"/data/disk2": {NodeIP: "10.0.0.1", Path: "/data/disk2"},
		},
		"10.0.0.2": {"/data": {NodeIP: "10.0.0.2", Path: "/data"}},
	}
	errs := ValidateTopologyProposal(p, admitted)
	if len(errs) != 0 {
		t.Fatalf("expected no errors with multiple admitted disks per node, got: %v", errs)
	}
}

func TestValidateTopologyProposal_AdmittedNodePresentButWrongPath(t *testing.T) {
	// Node has admission records but NOT for the path requested in the proposal.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data/unregistered"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data/registered": {NodeIP: "10.0.0.1", Path: "/data/registered"}},
	}
	errs := ValidateTopologyProposal(p, admitted)
	if len(errs) == 0 {
		t.Fatal("expected error when requested path not in admitted set, got none")
	}
}

// ── ComputeTopologyDestructiveness ────────────────────────────────────────────

func TestComputeTopologyDestructiveness_NilCurrent_MultiNode_Destructive(t *testing.T) {
	proposal := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1", "10.0.0.2"},
		NodePaths: map[string]string{"10.0.0.1": "/data", "10.0.0.2": "/data"},
	}
	isDestructive, reasons := ComputeTopologyDestructiveness(proposal, nil)
	if !isDestructive {
		t.Fatal("expected destructive for first multi-node topology, got false")
	}
	if len(reasons) == 0 {
		t.Fatal("expected reasons for destructive transition")
	}
}

func TestComputeTopologyDestructiveness_NilCurrent_SingleNode_NotDestructive(t *testing.T) {
	proposal := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	isDestructive, _ := ComputeTopologyDestructiveness(proposal, nil)
	if isDestructive {
		t.Fatal("expected non-destructive for first single-node topology")
	}
}

func TestComputeTopologyDestructiveness_StandaloneToDistributed_Destructive(t *testing.T) {
	current := &configpkg.ObjectStoreDesiredState{
		Mode:       configpkg.ObjectStoreModeStandalone,
		Generation: 1,
	}
	proposal := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1", "10.0.0.2"},
		NodePaths: map[string]string{"10.0.0.1": "/data", "10.0.0.2": "/data"},
	}
	isDestructive, reasons := ComputeTopologyDestructiveness(proposal, current)
	if !isDestructive {
		t.Fatal("expected destructive for standalone→distributed transition")
	}
	if len(reasons) == 0 {
		t.Fatal("expected reason for standalone→distributed")
	}
}

func TestComputeTopologyDestructiveness_PathChange_Destructive(t *testing.T) {
	current := &configpkg.ObjectStoreDesiredState{
		Mode:      configpkg.ObjectStoreModeDistributed,
		NodePaths: map[string]string{"10.0.0.1": "/data/old"},
		Generation: 2,
	}
	proposal := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data/new"},
	}
	isDestructive, _ := ComputeTopologyDestructiveness(proposal, current)
	if !isDestructive {
		t.Fatal("expected destructive for node path change")
	}
}

func TestComputeTopologyDestructiveness_SameDistributed_NotDestructive(t *testing.T) {
	vols := map[string]string{"10.0.0.1": "/data", "10.0.0.2": "/data", "10.0.0.3": "/data"}
	current := &configpkg.ObjectStoreDesiredState{
		Mode:          configpkg.ObjectStoreModeDistributed,
		Generation:    3,
		Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		DrivesPerNode: 1,
		NodePaths:     vols,
		VolumesHash:   configpkg.ComputeVolumesHash(vols),
	}
	// Re-apply the exact same proposal (idempotent — should be non-destructive
	// unless topology changed, which it hasn't here).
	proposal := &configpkg.TopologyProposal{
		Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		NodePaths:     vols,
		DrivesPerNode: 1,
	}
	isDestructive, _ := ComputeTopologyDestructiveness(proposal, current)
	if isDestructive {
		t.Fatal("expected non-destructive for identical re-apply")
	}
}

// ── validateAdmissionsAgainstCandidates ───────────────────────────────────────

// ── Item 2: fail-closed candidate validation ──────────────────────────────────

func TestValidateAdmissions_CandidateLoadFailure_Rejected(t *testing.T) {
	// etcd transient: loader returns error → apply must be rejected (fail closed).
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {NodeID: "node1", NodeIP: "10.0.0.1", Path: "/data"}},
	}
	loader := candidateLoader(nil, errors.New("etcd unavailable"))
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error when candidate load fails (fail-closed), got none")
	}
}

func TestValidateAdmissions_MissingCandidate_Rejected(t *testing.T) {
	// Disk candidates exist for the node but not for this mount path.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {NodeID: "node1", NodeIP: "10.0.0.1", Path: "/data"}},
	}
	// Loader returns candidates for a *different* path — /data not present.
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/other"}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error when mount path not in candidates, got none")
	}
}

func TestValidateAdmissions_EmptyCandidateList_Rejected(t *testing.T) {
	// Node returns empty candidate list — disk was removed.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {NodeID: "node1", NodeIP: "10.0.0.1", Path: "/data"}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{"node1": {}}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error when candidate list is empty (disk removed), got none")
	}
}

// ── Item 3: physical disk identity checks ─────────────────────────────────────

func TestValidateAdmissions_StaleStableID_Rejected(t *testing.T) {
	// Admitted disk has StableID=A but live candidate has StableID=B → disk was replaced.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {
			NodeID: "node1", NodeIP: "10.0.0.1", Path: "/data",
			StableID: "partuuid-original",
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/data", StableID: "partuuid-replaced", Eligible: true}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error for StableID mismatch (disk replaced), got none")
	}
}

func TestValidateAdmissions_DeviceChanged_Rejected(t *testing.T) {
	// Block device changed behind the same mount path — different disk mounted there.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {
			NodeID: "node1", NodeIP: "10.0.0.1", Path: "/data",
			Device: "/dev/sda1",
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/data", Device: "/dev/sdb1", Eligible: true}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error for device change, got none")
	}
}

func TestValidateAdmissions_SizeShrunkenSignificantly_Rejected(t *testing.T) {
	// Disk size shrank >20% — smaller disk silently substituted.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {
			NodeID:               "node1",
			NodeIP:               "10.0.0.1",
			Path:                 "/data",
			SizeBytesAtAdmission: 1_000_000_000, // 1 GiB
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/data", SizeBytes: 500_000_000, Eligible: true}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error for >20%% size shrink, got none")
	}
}

func TestValidateAdmissions_SizeGrownSignificantly_Rejected(t *testing.T) {
	// Disk size grew >20% — suggests a completely different (larger) disk.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {
			NodeID:               "node1",
			NodeIP:               "10.0.0.1",
			Path:                 "/data",
			SizeBytesAtAdmission: 1_000_000_000,
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/data", SizeBytes: 5_000_000_000, Eligible: true}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error for >20%% size growth, got none")
	}
}

func TestValidateAdmissions_SizeChangeSmall_Passes(t *testing.T) {
	// <20% size delta (filesystem overhead reporting variation) — must pass.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/data"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {
			NodeID:               "node1",
			NodeIP:               "10.0.0.1",
			Path:                 "/data",
			SizeBytesAtAdmission: 1_000_000_000,
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/data", SizeBytes: 1_050_000_000, Eligible: true}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for small size change, got: %v", errs)
	}
}

func TestValidateAdmissions_MatchingIdentity_Passes(t *testing.T) {
	// All identity fields match exactly — should pass with no errors.
	p := &configpkg.TopologyProposal{
		Nodes:         []string{"10.0.0.1", "10.0.0.2"},
		NodePaths:     map[string]string{"10.0.0.1": "/data", "10.0.0.2": "/data"},
		DrivesPerNode: 1,
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/data": {
			NodeID: "node1", NodeIP: "10.0.0.1", Path: "/data",
			StableID: "uuid-node1", Device: "/dev/sda1", SizeBytesAtAdmission: 500_000_000_000,
		}},
		"10.0.0.2": {"/data": {
			NodeID: "node2", NodeIP: "10.0.0.2", Path: "/data",
			StableID: "uuid-node2", Device: "/dev/sdb1", SizeBytesAtAdmission: 500_000_000_000,
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{NodeID: "node1", MountPath: "/data", StableID: "uuid-node1", Device: "/dev/sda1", SizeBytes: 500_000_000_000, Eligible: true}},
		"node2": {{NodeID: "node2", MountPath: "/data", StableID: "uuid-node2", Device: "/dev/sdb1", SizeBytes: 500_000_000_000, Eligible: true}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for matching identity, got: %v", errs)
	}
}

func TestValidateAdmissions_DiskReplacement_BehindSameMountPath(t *testing.T) {
	// Classic silent replacement: StableID, Device, AND SizeBytes all changed.
	// Any one of those is sufficient to reject; all three changing confirms it.
	p := &configpkg.TopologyProposal{
		Nodes:     []string{"10.0.0.1"},
		NodePaths: map[string]string{"10.0.0.1": "/mnt/storage"},
	}
	admitted := map[string]map[string]*configpkg.AdmittedDisk{
		"10.0.0.1": {"/mnt/storage": {
			NodeID: "node1", NodeIP: "10.0.0.1", Path: "/mnt/storage",
			StableID: "original-partuuid", Device: "/dev/nvme0n1p1",
			SizeBytesAtAdmission: 2_000_000_000_000, // 2 TiB
		}},
	}
	loader := candidateLoader(map[string][]*configpkg.DiskCandidate{
		"node1": {{
			NodeID:    "node1",
			MountPath: "/mnt/storage",
			StableID:  "replacement-partuuid",  // different
			Device:    "/dev/sda1",              // different
			SizeBytes: 500_000_000_000,          // 500 GiB — >20% smaller
			Eligible:  true,
		}},
	}, nil)
	errs := validateAdmissionsAgainstCandidates(context.Background(), p, admitted, loader)
	if len(errs) == 0 {
		t.Fatal("expected error for full disk replacement behind same mount path, got none")
	}
}
