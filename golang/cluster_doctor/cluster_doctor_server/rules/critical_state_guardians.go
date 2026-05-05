package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type ingressSpecMissing struct{}

func (ingressSpecMissing) ID() string       { return "ingress.spec_missing" }
func (ingressSpecMissing) Category() string { return "ingress" }
func (ingressSpecMissing) Scope() string    { return "cluster" }

func (ingressSpecMissing) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap.IngressSpecPresent || len(snap.Nodes) == 0 {
		return nil
	}
	result := "key_not_found"
	if snap.IngressSpecLoadError != nil {
		result = snap.IngressSpecLoadError.Error()
	}
	return []Finding{{
		FindingID:   FindingID("ingress.spec_missing", "cluster", "missing"),
		InvariantID: "ingress.spec_missing",
		Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:    "ingress",
		EntityRef:   "cluster",
		Summary: "Ingress desired state key /globular/ingress/v1/spec is missing; " +
			"node agents must hold last-known-good and controller should republish immediately.",
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "Get(/globular/ingress/v1/spec)", map[string]string{
				"key":    "/globular/ingress/v1/spec",
				"result": result,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restart cluster controller to republish canonical ingress spec",
				"systemctl restart globular-cluster-controller.service"),
			step(2, "Verify key restored",
				"globular config get /globular/ingress/v1/spec | jq .generation"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

type ingressNodeHoldingLastKnownGood struct{}

func (ingressNodeHoldingLastKnownGood) ID() string       { return "ingress.node_holding_last_known_good" }
func (ingressNodeHoldingLastKnownGood) Category() string { return "ingress" }
func (ingressNodeHoldingLastKnownGood) Scope() string    { return "node" }

func (ingressNodeHoldingLastKnownGood) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, n := range snap.Nodes {
		nodeID := n.GetNodeId()
		raw, ok := snap.IngressNodeStatus[nodeID]
		if !ok {
			continue
		}
		phase, _ := raw["phase"].(string)
		if !strings.HasPrefix(phase, "DEGRADED_SPEC_") && phase != "HOLD_LAST_KNOWN_GOOD" {
			continue
		}
		reason, _ := raw["last_error"].(string)
		findings = append(findings, Finding{
			FindingID:   FindingID("ingress.node_holding_last_known_good", nodeID, phase),
			InvariantID: "ingress.node_holding_last_known_good",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "ingress",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s ingress is running in hold-last-known-good mode (%s): %s",
				n.GetIdentity().GetHostname(), phase, reason),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "Get(/globular/ingress/v1/status/<node>)", map[string]string{
					"node_id":    nodeID,
					"phase":      phase,
					"last_error": reason,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Verify ingress desired state exists and is valid",
					"globular config get /globular/ingress/v1/spec | jq ."),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

type ingressAmbiguousDisableRejected struct{}

func (ingressAmbiguousDisableRejected) ID() string       { return "ingress.ambiguous_disable_rejected" }
func (ingressAmbiguousDisableRejected) Category() string { return "ingress" }
func (ingressAmbiguousDisableRejected) Scope() string    { return "node" }

func (ingressAmbiguousDisableRejected) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, n := range snap.Nodes {
		nodeID := n.GetNodeId()
		raw, ok := snap.IngressNodeStatus[nodeID]
		if !ok {
			continue
		}
		phase, _ := raw["phase"].(string)
		lastErr, _ := raw["last_error"].(string)
		if phase != "DEGRADED_SPEC_INVALID" {
			continue
		}
		if !strings.Contains(strings.ToLower(lastErr), "ambiguous disable") {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("ingress.ambiguous_disable_rejected", nodeID, phase),
			InvariantID: "ingress.ambiguous_disable_rejected",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "ingress",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s rejected ambiguous ingress disable intent and held runtime state: %s",
				n.GetIdentity().GetHostname(), lastErr),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "Get(/globular/ingress/v1/status/<node>)", map[string]string{
					"node_id":    nodeID,
					"phase":      phase,
					"last_error": lastErr,
				}),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

type scyllaKeyspaceRFPolicyViolation struct{}

func (scyllaKeyspaceRFPolicyViolation) ID() string       { return "scylla.keyspace.rf_policy_violation" }
func (scyllaKeyspaceRFPolicyViolation) Category() string { return "scylla" }
func (scyllaKeyspaceRFPolicyViolation) Scope() string    { return "cluster" }

func (scyllaKeyspaceRFPolicyViolation) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for keyspace, raw := range snap.ScyllaSchemaGuardStatus {
		violation, _ := raw["violation"].(bool)
		if !violation {
			continue
		}
		currentRF := fmt.Sprintf("%v", raw["current_rf"])
		requiredRF := fmt.Sprintf("%v", raw["required_rf"])
		lastErr := fmt.Sprintf("%v", raw["last_error"])
		findings = append(findings, Finding{
			FindingID:   FindingID("scylla.keyspace.rf_policy_violation", "cluster", keyspace),
			InvariantID: "scylla.keyspace.rf_policy_violation",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "scylla",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf("Scylla keyspace %s violates RF policy (current=%s required=%s). %s",
				keyspace, currentRF, requiredRF, strings.TrimSpace(lastErr)),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "Get(/globular/scylla/schema_guard/<keyspace>)", map[string]string{
					"keyspace":    keyspace,
					"current_rf":  currentRF,
					"required_rf": requiredRF,
					"last_error":  lastErr,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Run schema guard now", "globular scylla schema enforce"),
				step(2, "Verify keyspace replication", "globular scylla schema status"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
