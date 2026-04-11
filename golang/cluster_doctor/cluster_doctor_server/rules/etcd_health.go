package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ── etcd quorum and configuration checks ────────────────────────────────────

type etcdQuorumHealth struct{}

func (etcdQuorumHealth) ID() string       { return "etcd.quorum" }
func (etcdQuorumHealth) Category() string { return "etcd" }
func (etcdQuorumHealth) Scope() string    { return "cluster" }

func (etcdQuorumHealth) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	// Check for nodes whose etcd should be running based on profiles and join phase.
	var etcdNodes, etcdVerified, etcdJoining, etcdFailed int
	var failedHostnames []string

	for _, node := range snap.Nodes {
		profiles := node.GetProfiles()
		hasEtcdProfile := false
		for _, p := range profiles {
			if p == "core" || p == "compute" || p == "control-plane" {
				hasEtcdProfile = true
				break
			}
		}
		if !hasEtcdProfile {
			continue
		}
		etcdNodes++

		meta := node.GetMetadata()
		etcdPhase := ""
		if meta != nil {
			etcdPhase = meta["etcd_join_phase"]
		}

		switch etcdPhase {
		case "verified":
			etcdVerified++
		case "failed":
			etcdFailed++
			failedHostnames = append(failedHostnames, node.GetIdentity().GetHostname())
		case "member_added", "started", "prepared":
			etcdJoining++
		}
	}

	// Quorum check: if we have multiple etcd nodes but not enough verified.
	if etcdNodes > 1 {
		quorumNeeded := etcdNodes/2 + 1
		if etcdVerified < quorumNeeded && etcdFailed > 0 {
			findings = append(findings, Finding{
				FindingID:   FindingID("etcd.quorum", "cluster", "quorum_risk"),
				InvariantID: "etcd.quorum",
				Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
				Category:    "etcd",
				EntityRef:   "cluster",
				Summary: fmt.Sprintf("etcd quorum at risk: %d/%d members verified, %d failed, %d joining (need %d for quorum). Failed: %s",
					etcdVerified, etcdNodes, etcdFailed, etcdJoining, quorumNeeded, strings.Join(failedHostnames, ", ")),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_doctor", "etcd.quorum", map[string]string{
						"total_etcd_nodes": fmt.Sprintf("%d", etcdNodes),
						"verified":         fmt.Sprintf("%d", etcdVerified),
						"failed":           fmt.Sprintf("%d", etcdFailed),
						"joining":          fmt.Sprintf("%d", etcdJoining),
						"quorum_needed":    fmt.Sprintf("%d", quorumNeeded),
						"failed_nodes":     strings.Join(failedHostnames, ","),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Check the failed nodes — etcd join may have timed out", "globular cluster nodes list --insecure"),
					step(2, "If a node is permanently gone, remove it", "globular --timeout 30s cluster nodes remove <node-id> --force --drain=false --insecure"),
					step(3, "For single-node recovery after quorum loss", "sudo ./reset-etcd-single-node.sh"),
					step(4, "After reset, restart ALL globular services", "sudo systemctl restart 'globular-*.service'"),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}

	// etcd join failed on specific nodes.
	for _, node := range snap.Nodes {
		meta := node.GetMetadata()
		if meta == nil || meta["etcd_join_phase"] != "failed" {
			continue
		}
		hostname := node.GetIdentity().GetHostname()
		nodeID := node.GetNodeId()
		etcdErr := meta["etcd_join_error"]
		findings = append(findings, Finding{
			FindingID:   FindingID("etcd.quorum", nodeID, "join_failed"),
			InvariantID: "etcd.quorum",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "etcd",
			EntityRef:   nodeID,
			Summary:     fmt.Sprintf("etcd join failed on %s (%s): %s", hostname, nodeID, etcdErr),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_controller", "ListNodes", map[string]string{
					"node_id":         nodeID,
					"etcd_join_phase": "failed",
					"etcd_join_error": etcdErr,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Remove the node and re-join cleanly", "globular --timeout 30s cluster nodes remove "+nodeID+" --force --drain=false --insecure"),
				step(2, "On the node: clean etcd data and re-run join script", "sudo rm -rf /var/lib/globular/etcd && curl -sfL https://<gateway>:8443/join -k | sudo bash -s -- --token <token>"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}

// ── Stale/duplicate node detection ──────────────────────────────────────────

type staleNodeDetection struct{}

func (staleNodeDetection) ID() string       { return "node.stale_duplicate" }
func (staleNodeDetection) Category() string { return "availability" }
func (staleNodeDetection) Scope() string    { return "cluster" }

func (staleNodeDetection) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	// Detect duplicate hostnames (same hostname, different node IDs).
	hostnameMap := make(map[string][]string) // hostname → list of node IDs
	for _, node := range snap.Nodes {
		hostname := node.GetIdentity().GetHostname()
		if hostname != "" {
			hostnameMap[hostname] = append(hostnameMap[hostname], node.GetNodeId())
		}
	}

	for hostname, nodeIDs := range hostnameMap {
		if len(nodeIDs) <= 1 {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("node.stale_duplicate", "cluster", hostname),
			InvariantID: "node.stale_duplicate",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "availability",
			EntityRef:   "cluster",
			Summary: fmt.Sprintf("Duplicate hostname %q: %d nodes share this name (%s). Remove stale entries.",
				hostname, len(nodeIDs), strings.Join(nodeIDs, ", ")),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "node.stale_duplicate", map[string]string{
					"hostname": hostname,
					"node_ids": strings.Join(nodeIDs, ","),
					"count":    fmt.Sprintf("%d", len(nodeIDs)),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "List nodes to identify the stale entry (check last_seen)", "globular cluster nodes list --insecure"),
				step(2, "Remove the stale node (the one with oldest last_seen)", "globular --timeout 30s cluster nodes remove <stale-node-id> --force --drain=false --insecure"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}

// ── Bootstrap phase stuck detection ─────────────────────────────────────────

type bootstrapPhaseStuck struct{}

func (bootstrapPhaseStuck) ID() string       { return "node.bootstrap_stuck" }
func (bootstrapPhaseStuck) Category() string { return "bootstrap" }
func (bootstrapPhaseStuck) Scope() string    { return "node" }

func (bootstrapPhaseStuck) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		hostname := node.GetIdentity().GetHostname()
		meta := node.GetMetadata()
		if meta == nil {
			continue
		}

		phase := meta["bootstrap_phase"]
		if phase == "" || phase == "workload_ready" {
			continue
		}

		if phase == "bootstrap_failed" {
			errMsg := meta["bootstrap_error"]
			findings = append(findings, Finding{
				FindingID:   FindingID("node.bootstrap_stuck", nodeID, "failed"),
				InvariantID: "node.bootstrap_stuck",
				Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
				Category:    "bootstrap",
				EntityRef:   nodeID,
				Summary:     fmt.Sprintf("Node %s (%s) bootstrap FAILED: %s", hostname, nodeID, errMsg),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_controller", "ListNodes", map[string]string{
						"node_id":         nodeID,
						"bootstrap_phase": phase,
						"bootstrap_error": errMsg,
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Check what failed in the bootstrap error message above", ""),
					step(2, "Remove the node and re-join", "globular --timeout 30s cluster nodes remove "+nodeID+" --force --drain=false --insecure"),
					step(3, "On the failed node, clean up and re-run the join script", ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
			continue
		}

		// Node is in a non-terminal bootstrap phase. Check if it's stuck
		// by looking at the node status and providing phase-specific hints.
		var hint string
		switch phase {
		case "admitted":
			hint = "Node was just admitted. Controller should advance to infra_preparing on next reconcile cycle. If stuck, check controller logs."
		case "infra_preparing":
			hint = "Waiting for infrastructure packages (etcd unit file). Check if globular-etcd.service unit file exists on the node."
			etcdPhase := meta["etcd_join_phase"]
			if etcdPhase != "" {
				hint += fmt.Sprintf(" etcd_join_phase=%s.", etcdPhase)
			}
		case "etcd_joining":
			etcdPhase := meta["etcd_join_phase"]
			hint = fmt.Sprintf("Waiting for etcd join to complete (etcd_join_phase=%s). ", etcdPhase)
			switch etcdPhase {
			case "":
				hint += "etcd join hasn't started — check if node has routable IP and etcd unit is installed."
			case "prepared":
				hint += "Ready for MemberAdd — controller should call it on next cycle."
			case "member_added":
				hint += "MemberAdd called, waiting for etcd service to start. Check etcd config was rendered and unit can start."
			case "started":
				hint += "etcd running, waiting for health verification. Should complete within 30s."
			case "failed":
				etcdErr := meta["etcd_join_error"]
				hint += fmt.Sprintf("etcd join FAILED: %s. May need to remove stale etcd member and retry.", etcdErr)
			}
		case "etcd_ready":
			hint = "etcd verified. Waiting for globular-xds.service to become active."
		case "xds_ready":
			hint = "xDS active. Waiting for globular-envoy.service to become active."
		case "envoy_ready":
			hint = "Envoy active. Should advance to storage_joining or workload_ready immediately."
		case "storage_joining":
			minioPhase := meta["minio_join_phase"]
			scyllaPhase := meta["scylla_join_phase"]
			hint = fmt.Sprintf("Waiting for storage services. minio_join=%s scylla_join=%s.", minioPhase, scyllaPhase)
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("node.bootstrap_stuck", nodeID, phase),
			InvariantID: "node.bootstrap_stuck",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "bootstrap",
			EntityRef:   nodeID,
			Summary:     fmt.Sprintf("Node %s (%s) in bootstrap phase %q — %s", hostname, nodeID, phase, hint),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_controller", "ListNodes", map[string]string{
					"node_id":           nodeID,
					"bootstrap_phase":   phase,
					"etcd_join_phase":   meta["etcd_join_phase"],
					"minio_join_phase":  meta["minio_join_phase"],
					"scylla_join_phase": meta["scylla_join_phase"],
					"status":            node.GetStatus(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check controller logs for bootstrap phase transitions", "journalctl -u globular-cluster-controller.service --since '5 min ago' | grep bootstrap:"),
				step(2, "Check node-agent is heartbeating", "globular cluster nodes list --insecure"),
				step(3, "If stuck for >5 minutes, the phase will timeout to bootstrap_failed automatically", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}

// ── Node-agent crash detection ──────────────────────────────────────────────

type nodeAgentCrash struct{}

func (nodeAgentCrash) ID() string       { return "node.agent_crash" }
func (nodeAgentCrash) Category() string { return "systemd" }
func (nodeAgentCrash) Scope() string    { return "node" }

func (nodeAgentCrash) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		hostname := node.GetIdentity().GetHostname()
		lastErr := ""
		if meta := node.GetMetadata(); meta != nil {
			lastErr = meta["last_error"]
		}

		if lastErr == "" {
			continue
		}

		// Detect common crash patterns.
		var hint, severity string
		sev := cluster_doctorpb.Severity_SEVERITY_WARN

		switch {
		case strings.Contains(lastErr, "CHDIR") || strings.Contains(lastErr, "Permission denied"):
			hint = "Node-agent working directory has wrong permissions. Fix: sudo chmod 755 /var/lib/globular/node_agent && sudo systemctl restart globular-node-agent.service"
			sev = cluster_doctorpb.Severity_SEVERITY_CRITICAL
			severity = "permissions"
		case strings.Contains(lastErr, "connection refused"):
			hint = "Node-agent cannot connect to a required service. Check if etcd and controller are running."
			severity = "connectivity"
		case strings.Contains(lastErr, "context deadline exceeded"):
			hint = "Node-agent request timed out. Service may be overloaded or etcd may have lost quorum."
			severity = "timeout"
		case strings.Contains(lastErr, "no contact"):
			hint = "Controller hasn't received heartbeat from this node. Check if node-agent is running on the node."
			severity = "heartbeat"
		default:
			continue // Don't flag unknown errors
		}

		findingID := FindingID("node.agent_crash", nodeID, severity)

		// For "heartbeat" and "timeout" variants the fix is idempotent:
		// restart globular-node-agent and let it re-establish contact.
		// "permissions" needs manual chmod first — stay text-only.
		// "connectivity" is ambiguous (could be etcd/controller too) —
		// stay text-only to avoid masking upstream outages.
		var step1 *cluster_doctorpb.RemediationStep
		if severity == "heartbeat" || severity == "timeout" {
			step1 = actionStep(
				1,
				hint,
				fmt.Sprintf("globular doctor remediate %s --step 0", findingID),
				systemctlRestartAction("globular-node-agent.service", nodeID),
			)
		} else {
			step1 = step(1, hint, "")
		}

		findings = append(findings, Finding{
			FindingID:   findingID,
			InvariantID: "node.agent_crash",
			Severity:    sev,
			Category:    "systemd",
			EntityRef:   nodeID,
			Summary:     fmt.Sprintf("Node %s (%s): %s — %s", hostname, nodeID, lastErr, hint),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_controller", "ListNodes", map[string]string{
					"node_id":    nodeID,
					"last_error": lastErr,
					"status":     node.GetStatus(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step1,
				step(2, "Check node-agent logs", "journalctl -u globular-node-agent.service -n 20 --no-pager"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}
