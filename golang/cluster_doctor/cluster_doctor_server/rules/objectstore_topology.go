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
// WARN:     Pool has > 1 node but applied_generation < desired.Generation —
//           the topology workflow has not yet completed (may be in-flight).
//
// CRITICAL: Desired mode is distributed but applied_generation is still at
//           the standalone level (workflow has never run or was never triggered).

type objectstoreMinioTopologyConsistency struct{}

func (objectstoreMinioTopologyConsistency) ID() string {
	return "objectstore.minio.topology_consistency"
}
func (objectstoreMinioTopologyConsistency) Category() string { return "objectstore" }
func (objectstoreMinioTopologyConsistency) Scope() string    { return "cluster" }

func (objectstoreMinioTopologyConsistency) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil {
		return nil
	}
	// Single-node standalone is always correct — nothing to check.
	if len(desired.Nodes) <= 1 {
		return nil
	}

	appliedGen := snap.ObjectStoreAppliedGeneration
	desiredGen := desired.Generation
	if appliedGen >= desiredGen {
		return nil
	}

	lag := desiredGen - appliedGen
	severity := cluster_doctorpb.Severity_SEVERITY_WARN
	summary := fmt.Sprintf(
		"MinIO topology generation lag: desired=%d applied=%d (lag=%d). "+
			"Pool has %d nodes — the objectstore.minio.apply_topology_generation workflow has not completed.",
		desiredGen, appliedGen, lag, len(desired.Nodes))

	// CRITICAL when desired mode is distributed but applied_generation is 0
	// (workflow never ran) or when the desired mode has been explicitly set
	// distributed while applied_generation is still at standalone level.
	if desired.Mode == config.ObjectStoreModeDistributed && appliedGen == 0 {
		severity = cluster_doctorpb.Severity_SEVERITY_CRITICAL
		summary = fmt.Sprintf(
			"MinIO desired mode is distributed but workflow has never applied a generation "+
				"(applied=0, desired=%d, pool=%d nodes). "+
				"MinIO data is at risk — run the topology workflow immediately. "+
				"Run: globular workflow start objectstore.minio.apply_topology_generation",
			desiredGen, len(desired.Nodes))
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
				"mode":         string(desired.Mode),
				"desired_gen":  fmt.Sprintf("%d", desiredGen),
				"applied_gen":  fmt.Sprintf("%d", appliedGen),
				"pool_nodes":   strings.Join(desired.Nodes, ","),
				"volumes_hash": desired.VolumesHash,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Check topology workflow status",
				"globular workflow status objectstore.minio.apply_topology_generation"),
			step(2, "Start topology workflow if not running",
				"globular workflow start objectstore.minio.apply_topology_generation"),
			step(3, "Check all nodes rendered the topology",
				"globular objectstore topology status"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.minio.fingerprint_divergence ─────────────────────────────────
//
// CRITICAL when any pool node rendered a topology fingerprint that does not
// match the current desired-state fingerprint. This catches:
//   - A node that rendered a previous generation's config and never updated
//   - A node that was wiped and re-rendered standalone when distributed is desired
//   - Any partial apply where pool membership or drives_per_node differ
//
// A fingerprint divergence means MinIO nodes are not running the same topology.
// The objectstore.minio.apply_topology_generation workflow should NOT proceed
// if fingerprints diverge — the check_all_rendered step gates on this.

type objectstoreMinioFingerprintDivergence struct{}

func (objectstoreMinioFingerprintDivergence) ID() string {
	return "objectstore.minio.fingerprint_divergence"
}
func (objectstoreMinioFingerprintDivergence) Category() string { return "objectstore" }
func (objectstoreMinioFingerprintDivergence) Scope() string    { return "cluster" }

func (objectstoreMinioFingerprintDivergence) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || len(desired.Nodes) <= 1 {
		return nil
	}

	expectedFP := config.RenderStateFingerprint(desired)
	if expectedFP == "" {
		return nil
	}

	// Build IP → nodeID map from known nodes.
	ipToNodeID := make(map[string]string, len(snap.Nodes))
	for _, n := range snap.Nodes {
		for _, ip := range n.GetIdentity().GetIps() {
			ipToNodeID[ip] = n.GetNodeId()
		}
	}

	var diverged []string
	var missing []string
	for _, poolIP := range desired.Nodes {
		nodeID := ipToNodeID[poolIP]
		if nodeID == "" {
			// IP not yet mapped to a known node; skip rather than false-positive.
			continue
		}
		fp, ok := snap.NodeRenderedFingerprints[nodeID]
		if !ok || fp == "" {
			missing = append(missing, fmt.Sprintf("%s(%s):no_fingerprint", nodeID, poolIP))
			continue
		}
		if fp != expectedFP {
			diverged = append(diverged, fmt.Sprintf("%s(%s):got=%s", nodeID, poolIP, fp[:safeLen(fp, 8)]))
		}
	}

	if len(diverged) == 0 && len(missing) == 0 {
		return nil
	}

	parts := make([]string, 0, len(diverged)+len(missing))
	parts = append(parts, diverged...)
	parts = append(parts, missing...)
	summary := fmt.Sprintf(
		"MinIO topology fingerprint divergence: %d node(s) rendered a different or missing topology. "+
			"Expected fingerprint %s. Diverged: %v. "+
			"These nodes may be running standalone config while distributed is desired. "+
			"Wait for node-agents to re-render, then re-run the topology workflow.",
		len(parts), expectedFP[:safeLen(expectedFP, 16)], parts)

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.fingerprint_divergence", "cluster", expectedFP[:safeLen(expectedFP, 16)]),
		InvariantID: "objectstore.minio.fingerprint_divergence",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary:     summary,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "node_rendered_fingerprints", map[string]string{
				"expected_fingerprint": expectedFP,
				"diverged_nodes":       strings.Join(diverged, "; "),
				"missing_nodes":        strings.Join(missing, "; "),
				"pool_nodes":           strings.Join(desired.Nodes, ","),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Check per-node render status",
				"globular objectstore topology status"),
			step(2, "Wait for node-agents to render topology (checks etcd rendered_state_fingerprint)",
				"watch -n10 'globular objectstore topology status'"),
			step(3, "Re-run topology workflow once all fingerprints match",
				"globular workflow start objectstore.minio.apply_topology_generation"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.minio.post_apply_health ──────────────────────────────────────
//
// CRITICAL when applied_generation equals desired.Generation (the topology
// workflow completed and recorded success) but any pool node's MinIO service
// is no longer active in the inventory. This catches post-apply regressions:
// a service crash, a failed restart after node reboot, or a stale standalone
// config that survived.
//
// The workflow already verified health at apply time. This invariant detects
// regressions that occur AFTER the workflow succeeded.

type objectstoreMinioPostApplyHealth struct{}

func (objectstoreMinioPostApplyHealth) ID() string { return "objectstore.minio.post_apply_health" }
func (objectstoreMinioPostApplyHealth) Category() string { return "objectstore" }
func (objectstoreMinioPostApplyHealth) Scope() string    { return "cluster" }

func (objectstoreMinioPostApplyHealth) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || len(desired.Nodes) <= 1 {
		return nil
	}

	// Only fire when the workflow has already applied the generation.
	if snap.ObjectStoreAppliedGeneration < desired.Generation {
		return nil
	}

	// Build IP → nodeID map.
	ipToNodeID := make(map[string]string, len(snap.Nodes))
	for _, n := range snap.Nodes {
		for _, ip := range n.GetIdentity().GetIps() {
			ipToNodeID[ip] = n.GetNodeId()
		}
	}

	var notActive []string
	var noInventory []string

	for _, poolIP := range desired.Nodes {
		nodeID := ipToNodeID[poolIP]
		if nodeID == "" {
			continue
		}
		state := minioServiceState(snap, nodeID)
		if state == "no_inventory" {
			noInventory = append(noInventory, fmt.Sprintf("%s(%s)", nodeID, poolIP))
			continue
		}
		if state != "active" {
			notActive = append(notActive, fmt.Sprintf("%s(%s):state=%s", nodeID, poolIP, state))
		}
	}

	if len(notActive) == 0 && len(noInventory) == 0 {
		return nil
	}

	allProblems := append(notActive, noInventory...)
	summary := fmt.Sprintf(
		"MinIO post-apply health regression: applied_generation=%d matches desired=%d "+
			"but globular-minio.service is not active on %d pool node(s): %v. "+
			"This indicates a service crash or stale config after the topology workflow completed.",
		snap.ObjectStoreAppliedGeneration, desired.Generation, len(allProblems), allProblems)

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.post_apply_health", "cluster", fmt.Sprintf("gen-%d", desired.Generation)),
		InvariantID: "objectstore.minio.post_apply_health",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary:     summary,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd+inventory", "objectstore_applied_generation+unit_state", map[string]string{
				"applied_generation": fmt.Sprintf("%d", snap.ObjectStoreAppliedGeneration),
				"desired_generation": fmt.Sprintf("%d", desired.Generation),
				"not_active":         strings.Join(notActive, "; "),
				"no_inventory":       strings.Join(noInventory, "; "),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Inspect MinIO service logs on affected nodes",
				"journalctl -u globular-minio.service -n 100"),
			step(2, "Check MinIO topology status",
				"globular objectstore topology status"),
			step(3, "Re-run topology workflow to restore distributed mode",
				"globular workflow start objectstore.minio.apply_topology_generation"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// minioServiceState returns the state of globular-minio.service for the given
// node from the snapshot inventory, or "missing"/"no_inventory" when absent.
func minioServiceState(snap *collector.Snapshot, nodeID string) string {
	inv := snap.Inventories[nodeID]
	if inv == nil {
		return "no_inventory"
	}
	for _, u := range inv.GetUnits() {
		if strings.EqualFold(strings.TrimSpace(u.GetName()), "globular-minio.service") {
			return strings.ToLower(strings.TrimSpace(u.GetState()))
		}
	}
	return "missing"
}

// safeLen returns min(len(s), n), safe for string prefix slicing.
func safeLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
