package rules

// objectstore_admission.go — doctor invariants for disk admission and topology safety.
//
// Invariants:
//   objectstore.minio.standalone_splitbrain    — CRITICAL: ≥2 nodes in cluster, all running standalone
//   objectstore.minio.unapproved_path          — CRITICAL: MinIO running on a path not in admitted disks
//   objectstore.minio.quorum_shape             — WARN/CRITICAL: pool below minimum node/drive count
//   objectstore.minio.existing_data_guard      — CRITICAL: destructive apply would wipe non-MinIO data
//
// objectstore.minio.fingerprint_divergence is already implemented in objectstore_topology.go
// and is listed here for reference. The invariants below are complementary.

import (
	"fmt"
	"strings"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/config"
)

// ─── objectstore.minio.standalone_splitbrain ──────────────────────────────────
//
// CRITICAL when the cluster has ≥2 nodes with MinIO running, but the desired
// topology is standalone (or missing). Each node runs an isolated MinIO with
// no data sharing — objects written to one node are not visible on others.
// This is a split-brain condition that silently loses writes.

type objectstoreMinioStandaloneSplitbrain struct{}

func (objectstoreMinioStandaloneSplitbrain) ID() string {
	return "objectstore.minio.standalone_splitbrain"
}
func (objectstoreMinioStandaloneSplitbrain) Category() string { return "objectstore" }
func (objectstoreMinioStandaloneSplitbrain) Scope() string    { return "cluster" }

func (objectstoreMinioStandaloneSplitbrain) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired

	// Count nodes that have globular-minio.service active.
	var activeNodes []string
	for _, n := range snap.Nodes {
		nodeID := n.GetNodeId()
		if minioServiceState(snap, nodeID) == "active" {
			activeNodes = append(activeNodes, nodeID)
		}
	}

	// Not a split-brain if only one or zero nodes have MinIO running.
	if len(activeNodes) < 2 {
		return nil
	}

	// If desired mode is already distributed and generation is applied, OK.
	if desired != nil &&
		desired.Mode == config.ObjectStoreModeDistributed &&
		snap.ObjectStoreAppliedGeneration >= desired.Generation {
		return nil
	}

	// Multiple nodes running MinIO but desired state is standalone (or absent).
	desiredMode := "none (no desired state published)"
	if desired != nil {
		desiredMode = string(desired.Mode)
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.standalone_splitbrain", "cluster", fmt.Sprintf("nodes-%d", len(activeNodes))),
		InvariantID: "objectstore.minio.standalone_splitbrain",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO standalone split-brain: %d nodes running MinIO but desired mode is %q. "+
				"Each node has an isolated data store — objects are NOT shared across nodes. "+
				"Run topology plan and apply to form a distributed MinIO pool. "+
				"Nodes: %v",
			len(activeNodes), desiredMode, activeNodes),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "minio_service_state+desired_mode", map[string]string{
				"active_nodes":  strings.Join(activeNodes, ","),
				"desired_mode":  desiredMode,
				"applied_gen":   fmt.Sprintf("%d", snap.ObjectStoreAppliedGeneration),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Admit disks on each node", "globular objectstore disk scan"),
			step(2, "Approve disks", "globular objectstore disk approve --node <id> --path <path> --node-ip <ip>"),
			step(3, "Plan topology", "globular objectstore topology plan"),
			step(4, "Apply topology (if destructive add --i-understand-data-reset)",
				"globular objectstore topology apply --proposal <id>"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.minio.unapproved_path ────────────────────────────────────────
//
// CRITICAL when MinIO is running on a data path that is NOT present in the
// admitted disks in etcd. This means the path was either:
//   - Set manually (bypassing the admission workflow)
//   - Left from a previous standalone deployment that was never re-admitted
//   - The result of an un-tracked path migration
//
// An unapproved path is a topology integrity violation: the operator has not
// explicitly consented to MinIO using this disk.

type objectstoreMinioUnapprovedPath struct{}

func (objectstoreMinioUnapprovedPath) ID() string { return "objectstore.minio.unapproved_path" }
func (objectstoreMinioUnapprovedPath) Category() string { return "objectstore" }
func (objectstoreMinioUnapprovedPath) Scope() string    { return "cluster" }

func (objectstoreMinioUnapprovedPath) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || len(desired.NodePaths) == 0 {
		return nil
	}

	admitted := snap.AdmittedDisks
	if len(admitted) == 0 {
		// No admitted disks recorded — the admission workflow was never used.
		// Only fire if distributed mode is active (standalone might predate admission).
		if desired.Mode != config.ObjectStoreModeDistributed {
			return nil
		}
	}

	// Build admitted set: nodeIP → set of approved paths.
	admittedByIP := make(map[string]map[string]bool)
	for _, ad := range admitted {
		if admittedByIP[ad.NodeIP] == nil {
			admittedByIP[ad.NodeIP] = make(map[string]bool)
		}
		admittedByIP[ad.NodeIP][ad.Path] = true
	}

	var unapproved []string
	for ip, path := range desired.NodePaths {
		paths, ok := admittedByIP[ip]
		if !ok || !paths[path] {
			unapproved = append(unapproved, fmt.Sprintf("%s:%s", ip, path))
		}
	}

	if len(unapproved) == 0 {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.unapproved_path", "cluster", strings.Join(unapproved, ",")),
		InvariantID: "objectstore.minio.unapproved_path",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO running on unapproved disk path(s): %v. "+
				"These paths were not admitted via 'globular objectstore disk approve'. "+
				"Admit the paths explicitly or re-plan the topology.",
			unapproved),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "objectstore_desired+admitted_disks", map[string]string{
				"unapproved_paths": strings.Join(unapproved, "; "),
				"desired_mode":     string(desired.Mode),
				"desired_gen":      fmt.Sprintf("%d", desired.Generation),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Approve the current paths", "globular objectstore disk approve --node <id> --path <path> --node-ip <ip>"),
			step(2, "Re-plan and re-apply", "globular objectstore topology plan && globular objectstore topology apply --proposal <id>"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.minio.quorum_shape ───────────────────────────────────────────
//
// WARN when pool has ≥2 nodes but fewer than the minimum recommended for
// full erasure coding (4 drives total for EC:2+2).
// CRITICAL when pool has exactly 1 node but distributed mode is desired.

type objectstoreMinioQuorumShape struct{}

func (objectstoreMinioQuorumShape) ID() string { return "objectstore.minio.quorum_shape" }
func (objectstoreMinioQuorumShape) Category() string { return "objectstore" }
func (objectstoreMinioQuorumShape) Scope() string    { return "cluster" }

func (objectstoreMinioQuorumShape) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeDistributed {
		return nil
	}

	nodeCount := len(desired.Nodes)
	drivesPerNode := desired.DrivesPerNode
	if drivesPerNode < 1 {
		drivesPerNode = 1
	}
	totalDrives := nodeCount * drivesPerNode

	// CRITICAL: distributed desired but only 1 node.
	if nodeCount < 2 {
		return []Finding{{
			FindingID:   FindingID("objectstore.minio.quorum_shape", "cluster", "nodes-1"),
			InvariantID: "objectstore.minio.quorum_shape",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "objectstore",
			EntityRef:   "cluster",
			Summary: "MinIO desired mode is distributed but pool has only 1 node. " +
				"Distributed MinIO requires ≥2 nodes. Add at least one more storage node.",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		}}
	}

	// WARN: fewer than 4 total drives — erasure coding is suboptimal.
	if totalDrives < 4 {
		return []Finding{{
			FindingID: FindingID("objectstore.minio.quorum_shape", "cluster",
				fmt.Sprintf("drives-%d", totalDrives)),
			InvariantID: "objectstore.minio.quorum_shape",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "objectstore",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf(
				"MinIO pool has %d total drives (%d nodes × %d drives). "+
					"Full erasure coding (EC:2+2) requires ≥4 drives total. "+
					"Add nodes or drives to improve redundancy.",
				totalDrives, nodeCount, drivesPerNode),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "objectstore_desired", map[string]string{
					"nodes":          fmt.Sprintf("%d", nodeCount),
					"drives_per_node": fmt.Sprintf("%d", drivesPerNode),
					"total_drives":   fmt.Sprintf("%d", totalDrives),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Add storage nodes to reach ≥4 total drives",
					"globular cluster join --profiles core,storage"),
				step(2, "Or add drives per node with multi-drive admission",
					"globular objectstore disk approve --node <id> --path <path> --drives 2"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		}}
	}

	return nil
}

// ─── objectstore.minio.existing_data_guard ────────────────────────────────────
//
// CRITICAL when the current topology proposal (if any) would wipe a path that
// contains non-MinIO data but was admitted without --force-existing-data.
// This guards against accidental data loss in the apply workflow.

type objectstoreMinioExistingDataGuard struct{}

func (objectstoreMinioExistingDataGuard) ID() string {
	return "objectstore.minio.existing_data_guard"
}
func (objectstoreMinioExistingDataGuard) Category() string { return "objectstore" }
func (objectstoreMinioExistingDataGuard) Scope() string    { return "cluster" }

func (objectstoreMinioExistingDataGuard) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Check disk candidates for admitted paths that have existing non-MinIO data
	// but were not admitted with ForceExistingData.
	if len(snap.AdmittedDisks) == 0 {
		return nil
	}

	// Build candidate lookup: nodeID:path → HasExistingData
	type candidateKey struct{ nodeID, path string }
	existingData := make(map[candidateKey]bool)
	for nodeID, candidates := range snap.DiskCandidates {
		for _, dc := range candidates {
			if dc.HasExistingData && !dc.HasMinioSys {
				existingData[candidateKey{nodeID, dc.MountPath}] = true
			}
		}
	}

	var risky []string
	for _, ad := range snap.AdmittedDisks {
		if ad.ForceExistingData {
			continue // operator explicitly acknowledged this
		}
		if existingData[candidateKey{ad.NodeID, ad.Path}] {
			risky = append(risky, fmt.Sprintf("%s:%s", ad.NodeID, ad.Path))
		}
	}

	if len(risky) == 0 {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.existing_data_guard", "cluster", strings.Join(risky, ",")),
		InvariantID: "objectstore.minio.existing_data_guard",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"Admitted disk path(s) contain existing non-MinIO data that would be lost "+
				"if the topology workflow wipes .minio.sys: %v. "+
				"Re-admit with --force-existing-data to explicitly acknowledge data loss, "+
				"or choose a different path.",
			risky),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "admitted_disks+disk_candidates", map[string]string{
				"risky_paths": strings.Join(risky, "; "),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Re-admit with explicit data-loss acknowledgement",
				"globular objectstore disk approve --node <id> --path <path> --node-ip <ip> --force-existing-data"),
			step(2, "Or choose a different (empty) path and re-admit",
				"globular objectstore disk reject --node <id> --path <current>"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
