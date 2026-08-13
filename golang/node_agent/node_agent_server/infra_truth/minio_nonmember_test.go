package infra_truth

import "testing"

// The MinIO pool topology is a contract for pool MEMBERS. A node outside
// ExpectedPeers has its globular-minio unit deliberately held stopped by the
// node-agent topology gate, so it can neither form an isolated store nor
// reformat a pool drive — the two hazards attestMinioTopologyMatchesDesired
// exists to catch (fm:minio.wrong_topology_appears_healthy).
//
// Comparing a non-member's leftover rendered config against a pool it is not in
// produced CRITICAL findings on healthy nodes: migrating this cluster
// standalone -> distributed (3 nodes x 2 drives) immediately raised
// minio.topology_matches_desired and minio.not_stalled on both compute nodes,
// which are not storage-profile nodes and never render a pool config.

func desiredPool(self string, peers []string, mode string, drives int) *MinioDesired {
	return &MinioDesired{
		InfraDesiredState: InfraDesiredState{
			ExpectedListenAddresses: []string{self},
			ExpectedPeers:           peers,
		},
		Mode:          mode,
		DrivesPerNode: drives,
	}
}

func TestNonMemberIsNotJudgedAgainstPoolTopology(t *testing.T) {
	// node-5 (10.10.0.15) is a compute node: not in the 3-node storage pool.
	desired := desiredPool("10.10.0.15", []string{"10.10.0.11", "10.10.0.12", "10.10.0.13"},
		MinioModeDistributed, 2)
	rendered := &MinioRenderedConfig{Present: true, Mode: MinioModeStandalone, VolumeCount: 1}

	if v := attestMinioTopologyMatchesDesired(desired, rendered); v != nil {
		t.Fatalf("non-member flagged: %s — its unit is held stopped, so it cannot "+
			"diverge from a pool it is not in", v.GetMessage())
	}
}

// The guard must NOT weaken the check for real members: a member rendered
// standalone while the pool is distributed is the split-brain/reformat hazard.
func TestPoolMemberStandaloneRenderingStillCritical(t *testing.T) {
	desired := desiredPool("10.10.0.11", []string{"10.10.0.11", "10.10.0.12", "10.10.0.13"},
		MinioModeDistributed, 2)
	rendered := &MinioRenderedConfig{Present: true, Mode: MinioModeStandalone, VolumeCount: 1}

	v := attestMinioTopologyMatchesDesired(desired, rendered)
	if v == nil {
		t.Fatal("pool member rendered standalone was NOT flagged — this is the " +
			"split-brain case that can silently reformat drives")
	}
}

// A member with the wrong drive count is still a reformat risk.
func TestPoolMemberDriveCountMismatchStillCritical(t *testing.T) {
	desired := desiredPool("10.10.0.12", []string{"10.10.0.11", "10.10.0.12", "10.10.0.13"},
		MinioModeDistributed, 2)
	rendered := &MinioRenderedConfig{Present: true, Mode: MinioModeDistributed, VolumeCount: 3}

	if v := attestMinioTopologyMatchesDesired(desired, rendered); v == nil {
		t.Fatal("member drive-count mismatch was not flagged")
	}
}

// Missing evidence must not grant a waiver: with no published pool the check
// stays in force.
func TestUnknownPoolKeepsTheCheckInForce(t *testing.T) {
	desired := desiredPool("10.10.0.11", nil, MinioModeDistributed, 2)
	if !minioSelfIsPoolMember(desired) {
		t.Fatal("an empty pool must not be read as 'this node is a non-member' — " +
			"that would waive the check on missing evidence")
	}
}
