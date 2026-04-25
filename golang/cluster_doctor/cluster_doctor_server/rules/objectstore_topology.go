package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// ─── objectstore.minio.topology_consistency ───────────────────────────────────
//
// Fires when the objectstore desired topology (distributed mode) has not yet
// been applied — the desired generation exceeds the applied generation, meaning
// the objectstore.minio.apply_topology_generation workflow has not completed.
//
// This can occur when:
//   - A new MinIO pool node was added (generation incremented) but MinIO has
//     not yet been restarted in distributed mode.
//   - The topology workflow failed and restart_in_progress was not cleared.
//   - Node agents have not yet rendered the new minio.env / distributed.conf.
//
// The invariant is PASS when:
//   - There is only one pool node (standalone is correct for single-node).
//   - OR the applied_generation >= desired.Generation (workflow applied).
//
// The invariant is WARN when:
//   - Multiple pool nodes but topology workflow has not applied the generation.
//   - This is not yet an ERROR because the workflow may be in-flight.
//
// The invariant is CRITICAL when the desired mode is distributed but the applied
// generation has been behind for an extended period (detected via restart_in_progress
// being absent — meaning the workflow never even started or stalled before acquiring
// the lock).

type objectstoreMinioTopologyConsistency struct{}

func (objectstoreMinioTopologyConsistency) ID() string {
	return "objectstore.minio.topology_consistency"
}
func (objectstoreMinioTopologyConsistency) Category() string { return "objectstore" }
func (objectstoreMinioTopologyConsistency) Scope() string    { return "cluster" }

func (objectstoreMinioTopologyConsistency) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil {
		return nil // no objectstore configured yet
	}

	// Standalone with 0 or 1 pool nodes is always correct.
	if len(desired.Nodes) <= 1 {
		return nil
	}

	appliedGen := snap.ObjectStoreAppliedGeneration
	desiredGen := desired.Generation

	if appliedGen >= desiredGen {
		return nil // topology is current
	}

	// Multiple nodes in pool but topology not yet applied.
	lag := desiredGen - appliedGen
	severity := cluster_doctorpb.Severity_SEVERITY_WARN
	summary := fmt.Sprintf(
		"MinIO topology generation mismatch: desired=%d applied=%d (lag=%d). "+
			"Pool has %d nodes but MinIO has not been restarted in distributed mode. "+
			"The objectstore.minio.apply_topology_generation workflow must complete.",
		desiredGen, appliedGen, lag, len(desired.Nodes))

	// Escalate to CRITICAL if the mode is still standalone but the pool has multiple nodes.
	// This means the workflow has never run OR the desired state was never written correctly.
	if desired.Mode == config.ObjectStoreModeStandalone && len(desired.Nodes) > 1 {
		severity = cluster_doctorpb.Severity_SEVERITY_CRITICAL
		summary = fmt.Sprintf(
			"MinIO is in standalone mode but pool has %d nodes (desired=%v). "+
				"Data is stored on a single node only — distributed restart required. "+
				"Run: globular workflow start objectstore.minio.apply_topology_generation",
			len(desired.Nodes), desired.Nodes)
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.topology_consistency", "cluster", fmt.Sprintf("gen-%d", desiredGen)),
		InvariantID: "objectstore.minio.topology_consistency",
		Severity:    severity,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary:     summary,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "objectstore_desired_state", map[string]string{
				"mode":            string(desired.Mode),
				"desired_gen":     fmt.Sprintf("%d", desiredGen),
				"applied_gen":     fmt.Sprintf("%d", appliedGen),
				"pool_nodes":      strings.Join(desired.Nodes, ","),
				"volumes_hash":    desired.VolumesHash,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Check topology workflow status",
				"globular workflow status objectstore.minio.apply_topology_generation"),
			step(2, "Start topology workflow if not running",
				"globular workflow start objectstore.minio.apply_topology_generation"),
			step(3, "Verify applied_generation matches desired",
				"globular config get /globular/objectstore/applied_generation"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
