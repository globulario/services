package rules

import (
	"fmt"
	"net"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ─── certificate expiry ─────────────────────────────────────────────────────

type certificateExpiry struct{}

func (certificateExpiry) ID() string       { return "security.certs.expiry" }
func (certificateExpiry) Category() string { return "tls" }
func (certificateExpiry) Scope() string    { return "node" }

func (certificateExpiry) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		certResp, ok := snap.CertificateStatus[nid]
		if !ok {
			continue
		}
		cert := certResp.GetServerCert()
		if cert == nil {
			continue
		}
		days := cert.GetDaysUntilExpiry()

		var sev cluster_doctorpb.Severity
		switch {
		case days <= 0:
			sev = cluster_doctorpb.Severity_SEVERITY_CRITICAL
		case days <= 7:
			sev = cluster_doctorpb.Severity_SEVERITY_ERROR
		case days <= 30:
			sev = cluster_doctorpb.Severity_SEVERITY_WARN
		default:
			continue // healthy
		}

		hostname := node.GetIdentity().GetHostname()
		findings = append(findings, Finding{
			FindingID:   FindingID("security.certs.expiry", nid, "server"),
			InvariantID: "security.certs.expiry",
			Severity:    sev,
			Category:    "tls",
			EntityRef:   nid,
			Summary: fmt.Sprintf("Node %s (%s) server certificate expires in %d days (not_after=%s)",
				hostname, nid, days, cert.GetNotAfter()),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetCertificateStatus", map[string]string{
					"node_id":          nid,
					"subject":          cert.GetSubject(),
					"issuer":           cert.GetIssuer(),
					"not_after":        cert.GetNotAfter(),
					"days_until_expiry": fmt.Sprintf("%d", days),
					"fingerprint":      cert.GetFingerprint(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				actionStep(
					1,
					"Restart node agent to trigger cert re-issuance on "+hostname,
					"globular doctor remediate "+FindingID("security.certs.expiry", nid, "server")+" --step 0",
					systemctlRestartAction("globular-node-agent.service", nid),
				),
				step(2, "Verify new cert: globular cert status --node "+hostname, ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ─── certificate SAN coverage ───────────────────────────────────────────────

type certificateSANCoverage struct{}

func (certificateSANCoverage) ID() string       { return "security.certs.san_coverage" }
func (certificateSANCoverage) Category() string { return "tls" }
func (certificateSANCoverage) Scope() string    { return "node" }

func (certificateSANCoverage) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		certResp, ok := snap.CertificateStatus[nid]
		if !ok {
			continue
		}
		cert := certResp.GetServerCert()
		if cert == nil {
			continue
		}

		// Build set of SANs in the cert.
		sanSet := make(map[string]bool, len(cert.GetSans()))
		for _, san := range cert.GetSans() {
			sanSet[strings.TrimSpace(san)] = true
		}

		// Check that every IP the node advertises is covered.
		var missing []string
		for _, ipStr := range node.GetIdentity().GetIps() {
			ip := net.ParseIP(strings.TrimSpace(ipStr))
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if !sanSet[ip.String()] {
				missing = append(missing, ip.String())
			}
		}
		if len(missing) == 0 {
			continue
		}

		hostname := node.GetIdentity().GetHostname()
		findings = append(findings, Finding{
			FindingID:   FindingID("security.certs.san_coverage", nid, strings.Join(missing, ",")),
			InvariantID: "security.certs.san_coverage",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "tls",
			EntityRef:   nid,
			Summary: fmt.Sprintf("Node %s (%s) certificate missing IP SANs: %s — gRPC clients connecting via these IPs will fail TLS verification",
				hostname, nid, strings.Join(missing, ", ")),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetCertificateStatus", map[string]string{
					"node_id":     nid,
					"cert_sans":   strings.Join(cert.GetSans(), ", "),
					"node_ips":    strings.Join(node.GetIdentity().GetIps(), ", "),
					"missing_ips": strings.Join(missing, ", "),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				actionStep(
					1,
					"Restart node agent to reissue cert with correct IP SANs on "+hostname,
					"globular doctor remediate "+FindingID("security.certs.san_coverage", nid, strings.Join(missing, ","))+" --step 0",
					systemctlRestartAction("globular-node-agent.service", nid),
				),
				step(2, "Verify SANs: openssl s_client -connect "+hostname+":443 | openssl x509 -noout -text | grep -A1 'Subject Alternative'", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ─── certificate chain validity ─────────────────────────────────────────────

type certificateChainValid struct{}

func (certificateChainValid) ID() string       { return "security.certs.chain_valid" }
func (certificateChainValid) Category() string { return "tls" }
func (certificateChainValid) Scope() string    { return "node" }

func (certificateChainValid) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		certResp, ok := snap.CertificateStatus[nid]
		if !ok {
			continue
		}
		cert := certResp.GetServerCert()
		if cert == nil || cert.GetChainValid() {
			continue
		}

		hostname := node.GetIdentity().GetHostname()
		findings = append(findings, Finding{
			FindingID:   FindingID("security.certs.chain_valid", nid, "server"),
			InvariantID: "security.certs.chain_valid",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "tls",
			EntityRef:   nid,
			Summary: fmt.Sprintf("Node %s (%s) server certificate chain is invalid (subject=%s, issuer=%s)",
				hostname, nid, cert.GetSubject(), cert.GetIssuer()),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetCertificateStatus", map[string]string{
					"node_id":     nid,
					"subject":     cert.GetSubject(),
					"issuer":      cert.GetIssuer(),
					"chain_valid": "false",
					"not_before":  cert.GetNotBefore(),
					"not_after":   cert.GetNotAfter(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				actionStep(
					1,
					"Restart node agent to reissue cert with valid chain on "+hostname,
					"globular doctor remediate "+FindingID("security.certs.chain_valid", nid, "server")+" --step 0",
					systemctlRestartAction("globular-node-agent.service", nid),
				),
				step(2, "If CA was rotated, ensure all nodes have the new ca.crt", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
