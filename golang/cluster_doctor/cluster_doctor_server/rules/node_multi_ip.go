package rules

import (
	"fmt"
	"strings"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// ── Multi-IP and WiFi interface detection ────────────────────────────────────

type nodeMultiIP struct{}

func (nodeMultiIP) ID() string       { return "node.multi_ip" }
func (nodeMultiIP) Category() string { return "network" }
func (nodeMultiIP) Scope() string    { return "node" }

func (nodeMultiIP) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		hostname := node.GetIdentity().GetHostname()
		ips := node.GetIdentity().GetIps()

		if len(ips) == 0 {
			continue
		}

		// Filter to routable IPs only.
		var routableIPs []string
		for _, ip := range ips {
			if ip != "" && ip != "127.0.0.1" && ip != "::1" {
				routableIPs = append(routableIPs, ip)
			}
		}

		// ── Check 1: Multiple routable IPs (wired + WiFi risk) ──────────
		if len(routableIPs) > 1 {
			primaryIP := routableIPs[0] // node-agent sorts wired-first
			meta := node.GetMetadata()
			etcdPhase := ""
			if meta != nil {
				etcdPhase = meta["etcd_join_phase"]
			}

			sev := cluster_doctorpb.Severity_SEVERITY_INFO
			summary := fmt.Sprintf(
				"Node %s (%s) has %d routable IPs: %s. Primary (wired-preferred): %s.",
				hostname, nodeID, len(routableIPs),
				strings.Join(routableIPs, ", "), primaryIP)

			// Elevate to WARN if node runs etcd — IP mismatch can cause
			// join phase oscillation or split-brain addressing.
			if etcdPhase != "" {
				sev = cluster_doctorpb.Severity_SEVERITY_WARN
				summary += fmt.Sprintf(
					" etcd_join_phase=%s — ensure etcd peer URL uses the wired IP (%s), not WiFi.",
					etcdPhase, primaryIP)
			}

			findings = append(findings, Finding{
				FindingID:   FindingID("node.multi_ip", nodeID, "multiple_ips"),
				InvariantID: "node.multi_ip",
				Severity:    sev,
				Category:    "network",
				EntityRef:   nodeID,
				Summary:     summary,
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("cluster_controller", "ListNodes", map[string]string{
						"node_id":         nodeID,
						"hostname":        hostname,
						"ips":             strings.Join(routableIPs, ","),
						"primary_ip":      primaryIP,
						"ip_count":        fmt.Sprintf("%d", len(routableIPs)),
						"etcd_join_phase": etcdPhase,
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Verify the primary IP is the wired interface (eth/eno/enp), not WiFi (wlan/wlp)", "ip -4 addr show"),
					step(2, "If etcd uses the wrong IP, remove the node and re-join with the wired IP",
						"globular --timeout 30s cluster nodes remove "+nodeID+" --force --drain=false --insecure"),
					step(3, "For stability, consider disabling WiFi on server nodes", "sudo nmcli radio wifi off"),
					step(4, "Check etcd member list for IP consistency", "sudo ETCDCTL_API=3 etcdctl member list --write-out=table"),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}

		// ── Check 2: WiFi-only node (no wired interface detected) ───────
		// Heuristic: if the node has exactly one IP and it's not in common
		// wired subnet patterns, warn. A more reliable check: look at the
		// node's IP ordering — if node-agent sorted wired-first and the
		// first IP would be WiFi, that means no wired interface was found.
		// Since we can't see interface names from the controller, we check
		// profiles to see if this is an etcd node, then warn regardless.
		if len(routableIPs) == 1 {
			meta := node.GetMetadata()
			etcdPhase := ""
			if meta != nil {
				etcdPhase = meta["etcd_join_phase"]
			}
			hasEtcdProfile := false
			for _, p := range node.GetProfiles() {
				if p == "core" || p == "compute" || p == "control-plane" {
					hasEtcdProfile = true
					break
				}
			}

			// Only warn for etcd nodes with a single IP — could be WiFi-only.
			// We can't definitively tell from the controller side, but it's
			// worth flagging for manual verification.
			if hasEtcdProfile && etcdPhase == "verified" {
				// This is informational — single-IP etcd nodes are fine if wired.
				findings = append(findings, Finding{
					FindingID:   FindingID("node.multi_ip", nodeID, "single_ip_etcd"),
					InvariantID: "node.multi_ip",
					Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
					Category:    "network",
					EntityRef:   nodeID,
					Summary: fmt.Sprintf(
						"Node %s (%s) is an etcd member with a single IP (%s). Verify this is a wired connection — WiFi is unreliable for etcd quorum.",
						hostname, nodeID, routableIPs[0]),
					Evidence: []*cluster_doctorpb.Evidence{
						kvEvidence("cluster_controller", "ListNodes", map[string]string{
							"node_id":         nodeID,
							"hostname":        hostname,
							"ip":              routableIPs[0],
							"etcd_join_phase": etcdPhase,
						}),
					},
					Remediation: []*cluster_doctorpb.RemediationStep{
						step(1, "Verify the interface is wired", "ip link show | grep -E 'eth|eno|enp'"),
						step(2, "If WiFi-only, connect an Ethernet cable for etcd stability", ""),
					},
					InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
				})
			}
		}
	}

	return findings
}
