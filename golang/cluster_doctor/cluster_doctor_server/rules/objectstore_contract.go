package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// ─── objectstore.minio.contract_missing ──────────────────────────────────────
//
// Fires CRITICAL when MinIO is running on at least one pool node but
// ObjectStoreDesiredState is nil. This means the node-agent has no authoritative
// topology to enforce — the system is operating without a contract, which makes
// any configuration on disk unverifiable and prevents coordinated restarts.
//
// Root causes:
//   - etcd key /globular/objectstore/config was never written (pre-pool formation
//     on a node that had MinIO installed manually).
//   - The key was accidentally deleted.
//   - A controller upgrade cleared the desired state without reapplying.

type objectstoreContractMissing struct{}

func (objectstoreContractMissing) ID() string       { return "objectstore.minio.contract_missing" }
func (objectstoreContractMissing) Category() string { return "objectstore" }
func (objectstoreContractMissing) Scope() string    { return "cluster" }

func (objectstoreContractMissing) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Only fires when there is no desired state.
	if snap.ObjectStoreDesired != nil {
		return nil
	}

	// Check if any node has globular-minio.service reported as active.
	var runningNodes []string
	for _, node := range snap.Nodes {
		if minioServiceState(snap, node.GetNodeId()) == "active" {
			runningNodes = append(runningNodes, node.GetNodeId())
		}
	}

	if len(runningNodes) == 0 {
		// No MinIO running anywhere — silent is fine (pre-formation).
		return nil
	}

	// Distinguish transient etcd read error from confirmed key absence.
	// A transient error (network glitch, leader election) must not fire CRITICAL
	// and page the operator — it will self-heal on the next snapshot cycle.
	// Check this before the pre-formation guard: if etcd is unreadable we cannot
	// confirm whether a contract exists, so we must surface the error.
	if snap.ObjectStoreDesiredLoadError != nil {
		return []Finding{{
			FindingID:   FindingID("objectstore.minio.contract_missing", "cluster", "etcd-read-error"),
			InvariantID: "objectstore.minio.contract_missing",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "objectstore",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf(
				"MinIO is running on %d node(s) (%s) but ObjectStoreDesiredState could not be "+
					"read from etcd (transient error: %v). This may self-heal on the next evaluation cycle.",
				len(runningNodes), strings.Join(runningNodes, ", "), snap.ObjectStoreDesiredLoadError),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
					"key":           "/globular/objectstore/config",
					"status":        "read_error",
					"error":         snap.ObjectStoreDesiredLoadError.Error(),
					"running_nodes": strings.Join(runningNodes, ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Wait for etcd to stabilise, then re-evaluate", "globular cluster doctor"),
			},
		}}
	}

	// Pre-formation guard: if the topology has never been applied (generation==0)
	// and the cluster has only one node, MinIO starting before pool formation is
	// normal Day-0 behaviour. Suppress entirely — the contract will be published
	// once the pool is formed.
	if snap.ObjectStoreAppliedGeneration == 0 && len(snap.Nodes) <= 1 {
		return nil
	}

	// Severity tiers based on how certain we are a contract should exist:
	//   CRITICAL — topology was previously applied (regression: contract deleted) OR
	//              3+ nodes in cluster (pool should have been formed already).
	//   WARN     — 2-node cluster, generation 0 (pool may still be forming).
	nodeCount := len(snap.Nodes)
	severity := cluster_doctorpb.Severity_SEVERITY_WARN
	if snap.ObjectStoreAppliedGeneration > 0 || nodeCount >= 3 {
		severity = cluster_doctorpb.Severity_SEVERITY_CRITICAL
	}

	var summaryDetail string
	if snap.ObjectStoreAppliedGeneration > 0 {
		summaryDetail = fmt.Sprintf(
			"A topology was previously applied (applied_generation=%d) but the contract has disappeared. "+
				"Fix: run 'globular objectstore apply' to republish, or investigate whether "+
				"/globular/objectstore/config was accidentally deleted.",
			snap.ObjectStoreAppliedGeneration)
	} else {
		summaryDetail = fmt.Sprintf(
			"The node-agent cannot enforce topology without a contract (%d node(s) in cluster). "+
				"Fix: run 'globular objectstore apply' to publish a desired topology.",
			nodeCount)
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.minio.contract_missing", "cluster", "no-desired-state"),
		InvariantID: "objectstore.minio.contract_missing",
		Severity:    severity,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO is running on %d node(s) (%s) but no ObjectStoreDesiredState exists in etcd. %s",
			len(runningNodes), strings.Join(runningNodes, ", "), summaryDetail),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
				"key":             "/globular/objectstore/config",
				"status":          "missing",
				"running_nodes":   strings.Join(runningNodes, ","),
				"node_count":      fmt.Sprintf("%d", nodeCount),
				"applied_gen":     fmt.Sprintf("%d", snap.ObjectStoreAppliedGeneration),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Publish the desired topology", "globular objectstore apply"),
			step(2, "Verify desired state written", "globular config get /globular/objectstore/config | jq ."),
		},
	}}
}

// ─── objectstore.minio.credentials_missing ───────────────────────────────────
//
// Fires CRITICAL when ObjectStoreDesiredState exists in etcd but
// CredentialsReady=false AND the AccessKey/SecretKey fields are empty.
//
// This is distinct from contract_missing: the topology contract is present
// (node-agents know their pool membership and topology) but the controller
// has not yet published usable credentials — typically a transient state during
// controller startup before /var/lib/globular/minio/credentials is read.
//
// Backward compatibility: old contracts (published before the CredentialsReady
// field was introduced) have the field as false but DO have AccessKey populated.
// Those are treated as effectively ready and this rule does not fire.

type objectstoreCredentialsMissing struct{}

func (objectstoreCredentialsMissing) ID() string       { return "objectstore.minio.credentials_missing" }
func (objectstoreCredentialsMissing) Category() string { return "objectstore" }
func (objectstoreCredentialsMissing) Scope() string    { return "cluster" }

func (objectstoreCredentialsMissing) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	d := snap.ObjectStoreDesired
	if d == nil {
		return nil // contract_missing handles nil desired
	}
	// Effectively ready: either the flag is set, or the credentials are present
	// (old contract without the field still has usable credentials).
	if d.CredentialsReady || (d.AccessKey != "" && d.SecretKey != "") {
		return nil
	}
	return []Finding{{
		FindingID:   FindingID("objectstore.minio.credentials_missing", "cluster", fmt.Sprintf("gen-%d", d.Generation)),
		InvariantID: "objectstore.minio.credentials_missing",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"ObjectStoreDesiredState (gen=%d) exists in etcd but credentials are not loaded "+
				"(credentials_ready=false, access_key empty). "+
				"MinIO clients and node-agents cannot authenticate. "+
				"The controller will update this automatically once it reads "+
				"/var/lib/globular/minio/credentials from disk.",
			d.Generation),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
				"key":               "/globular/objectstore/config",
				"generation":        fmt.Sprintf("%d", d.Generation),
				"credentials_ready": "false",
				"access_key":        "(empty)",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Verify /var/lib/globular/minio/credentials exists on the controller node", "ls -l /var/lib/globular/minio/credentials"),
			step(2, "If missing, restore from backup or re-initialise MinIO credentials", "globular objectstore credentials reset"),
		},
	}}
}

// ─── objectstore.minio.endpoint_unresolved ───────────────────────────────────
//
// Fires WARN when ObjectStoreDesiredState exists but EndpointReady=false AND
// the Endpoint field is empty.
//
// This is a transient state during controller startup before pool formation or
// when MinioPoolNodes[0] is a DNS hostname instead of a bare IP.
// Node-agents can still render topology config but services cannot connect.
//
// Backward compatibility: old contracts that have Endpoint populated are treated
// as effectively ready even if EndpointReady=false.

type objectstoreEndpointUnresolved struct{}

func (objectstoreEndpointUnresolved) ID() string       { return "objectstore.minio.endpoint_unresolved" }
func (objectstoreEndpointUnresolved) Category() string { return "objectstore" }
func (objectstoreEndpointUnresolved) Scope() string    { return "cluster" }

func (objectstoreEndpointUnresolved) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	d := snap.ObjectStoreDesired
	if d == nil {
		return nil // contract_missing handles nil desired
	}
	// Effectively ready: either the flag is set, or the endpoint field is populated
	// (old contract without the field still has a usable endpoint).
	if d.EndpointReady || d.Endpoint != "" {
		return nil
	}
	return []Finding{{
		FindingID:   FindingID("objectstore.minio.endpoint_unresolved", "cluster", fmt.Sprintf("gen-%d", d.Generation)),
		InvariantID: "objectstore.minio.endpoint_unresolved",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"ObjectStoreDesiredState (gen=%d) exists but endpoint is unresolved "+
				"(endpoint_ready=false, endpoint empty). "+
				"Services cannot connect to MinIO. "+
				"The controller will resolve this once MinioPoolNodes[0] contains a routable IP.",
			d.Generation),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
				"key":            "/globular/objectstore/config",
				"generation":     fmt.Sprintf("%d", d.Generation),
				"endpoint_ready": "false",
				"endpoint":       "(empty)",
				"pool_nodes":     strings.Join(d.Nodes, ","),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Verify MinioPoolNodes contains a bare IP (not a DNS hostname)", "globular objectstore topology status"),
			step(2, "If pool nodes use hostnames, re-apply topology with IP addresses", "globular objectstore topology apply"),
		},
	}}
}

// ─── objectstore.minio.destructive_guard ─────────────────────────────────────
//
// Fires CRITICAL when the desired topology generation is higher than the applied
// generation AND the desired state is destructive (mode change or path change)
// BUT there is no approved TopologyTransition record in etcd.
//
// This means a destructive topology change was somehow triggered without going
// through the safe apply path (controller → transition record → workflow →
// node-agent confirmation). Node-agents will refuse to wipe .minio.sys without
// an approved record, so MinIO will fail to start in distributed mode.
//
// Also fires WARNING when a transition record exists but Approved=false,
// which means the controller rejected the ForceDestructive flag but the record
// was left in etcd (should not happen in normal operation, but worth surfacing).

type objectstoreDestructiveGuard struct{}

func (objectstoreDestructiveGuard) ID() string       { return "objectstore.minio.destructive_guard" }
func (objectstoreDestructiveGuard) Category() string { return "objectstore" }
func (objectstoreDestructiveGuard) Scope() string    { return "cluster" }

func (objectstoreDestructiveGuard) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil {
		return nil
	}

	// Only relevant when there is a pending topology change (desired > applied).
	if desired.Generation <= snap.ObjectStoreAppliedGeneration {
		return nil
	}

	// Determine if the pending change is destructive by comparing applied
	// fingerprint against the desired fingerprint.
	desiredFP := config.RenderStateFingerprint(desired)
	appliedFP := snap.AppliedStateFingerprint

	// If no topology has ever been applied, first distributed topology is
	// always destructive.
	isFirstDistributed := snap.ObjectStoreAppliedGeneration == 0 && len(desired.Nodes) >= 2

	// Fingerprint mismatch on an already-applied distributed cluster is destructive.
	isFingerprintChange := appliedFP != "" && appliedFP != desiredFP &&
		desired.Mode == config.ObjectStoreModeDistributed

	if !isFirstDistributed && !isFingerprintChange {
		// Non-destructive topology bump (e.g. credential rotation, endpoint update).
		return nil
	}

	// There is a destructive pending change. Check the transition record.
	transition := snap.DesiredTopologyTransition

	if transition == nil {
		// CRITICAL: destructive change is pending but no transition record exists.
		// This means the change did not go through the safe apply path.
		evidence := map[string]string{
			"desired_generation": fmt.Sprintf("%d", desired.Generation),
			"applied_generation": fmt.Sprintf("%d", snap.ObjectStoreAppliedGeneration),
			"desired_fp":         desiredFP,
			"applied_fp":         appliedFP,
		}
		if isFirstDistributed {
			evidence["reason"] = "first_distributed_topology"
		} else {
			evidence["reason"] = "fingerprint_change"
		}
		return []Finding{{
			FindingID:   FindingID("objectstore.minio.destructive_guard", "cluster", fmt.Sprintf("gen-%d", desired.Generation)),
			InvariantID: "objectstore.minio.destructive_guard",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "objectstore",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf(
				"Destructive MinIO topology change is pending (desired_gen=%d, applied_gen=%d) "+
					"but no approved TopologyTransition record exists at "+
					"/globular/objectstore/topology/transition/%d. "+
					"Node-agents will refuse to wipe .minio.sys, blocking the apply. "+
					"Fix: re-run 'globular objectstore apply --i-understand-data-reset' so the "+
					"controller writes the required transition record before triggering the workflow.",
				desired.Generation, snap.ObjectStoreAppliedGeneration, desired.Generation),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "LoadTopologyTransition", evidence),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1,
					fmt.Sprintf("Re-apply topology with destructive flag to write an approved transition record for gen %d", desired.Generation),
					"globular objectstore apply --i-understand-data-reset"),
			},
		}}
	}

	// Transition record exists. Check it is approved.
	if !transition.Approved {
		return []Finding{{
			FindingID:   FindingID("objectstore.minio.destructive_guard", "cluster", fmt.Sprintf("gen-%d-unapproved", desired.Generation)),
			InvariantID: "objectstore.minio.destructive_guard",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "objectstore",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf(
				"TopologyTransition record for generation %d exists but Approved=false. "+
					"Node-agents will refuse to wipe .minio.sys until the record is approved. "+
					"Fix: re-run 'globular objectstore apply --i-understand-data-reset'.",
				desired.Generation),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "LoadTopologyTransition", map[string]string{
					"generation":      fmt.Sprintf("%d", transition.Generation),
					"is_destructive":  fmt.Sprintf("%v", transition.IsDestructive),
					"approved":        fmt.Sprintf("%v", transition.Approved),
					"affected_nodes":  strings.Join(transition.AffectedNodes, ","),
					"reasons":         strings.Join(transition.Reasons, "; "),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Re-apply with destructive flag to approve the transition", "globular objectstore apply --i-understand-data-reset"),
			},
		}}
	}

	// Transition record is present and approved — no finding.
	return nil
}
