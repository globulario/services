package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ── Service registration gap detection ──────────────────────────────────────
//
// Detects nodes where the number of installed packages (from node-agent
// heartbeat) is significantly lower than expected, or where the node reports
// running services but the cluster has no desired releases resolved for them.
//
// This catches the scenario where etcd is wiped/recovered — services keep
// running as Linux processes but lose their etcd config registration. The
// gateway and admin UI can't see them, gRPC clients can't discover them.
//
// Detection: compare installed-state package count against the number of
// systemd units the node health reports as active. A large gap means
// services are running but not tracked.

type serviceRegistrationGap struct{}

func (serviceRegistrationGap) ID() string       { return "node.service_registration_gap" }
func (serviceRegistrationGap) Category() string { return "availability" }
func (serviceRegistrationGap) Scope() string    { return "node" }

func (serviceRegistrationGap) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		hostname := node.GetIdentity().GetHostname()
		meta := node.GetMetadata()
		if meta == nil {
			continue
		}

		// Only check nodes that should be fully converged.
		phase := meta["bootstrap_phase"]
		if phase != "workload_ready" {
			continue
		}

		// Get installed package count from node metadata.
		// The node-agent reports "installed_services_hash" and count via heartbeat.
		installedCount := 0
		inv, hasInv := snap.Inventories[nodeID]
		if hasInv && inv != nil {
			installedCount = len(inv.GetComponents())
		}

		// Get expected count: desired_infra + desired_workloads metadata.
		desiredInfra := meta["desired_infra"]
		desiredWorkloads := meta["desired_workloads"]
		var expectedNames []string
		if desiredInfra != "" {
			expectedNames = append(expectedNames, strings.Split(desiredInfra, ",")...)
		}
		if desiredWorkloads != "" {
			expectedNames = append(expectedNames, strings.Split(desiredWorkloads, ",")...)
		}
		expectedCount := len(expectedNames)

		if expectedCount == 0 {
			continue
		}

		// If installed count is less than half of expected, something is wrong.
		if installedCount > 0 && installedCount < expectedCount/2 {
			findings = append(findings, Finding{
				FindingID:   FindingID("node.service_registration_gap", nodeID, "low_installed"),
				InvariantID: "node.service_registration_gap",
				Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
				Category:    "availability",
				EntityRef:   nodeID,
				Summary: fmt.Sprintf("Node %s (%s): only %d/%d expected packages reported as installed. "+
					"Services may be running but not registered in etcd (common after etcd recovery).",
					hostname, nodeID, installedCount, expectedCount),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_doctor", "service_registration_gap", map[string]string{
						"node_id":           nodeID,
						"installed_count":   fmt.Sprintf("%d", installedCount),
						"expected_count":    fmt.Sprintf("%d", expectedCount),
						"desired_infra":     desiredInfra,
						"desired_workloads": desiredWorkloads,
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Restart all services on the node to force re-registration in etcd",
						"sudo systemctl restart 'globular-*.service'"),
					step(2, "Verify services appear in admin after restart",
						"globular cluster health"),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}

	return findings
}
