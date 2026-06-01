package rules

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// ─── pki.ca_not_published ────────────────────────────────────────────────────
//
// Fires when the cluster has joined nodes but the controller has not published
// CA metadata to etcd (/globular/pki/ca). Node agents rely on this key to detect
// CA rotation. Without it, stale-CA certs cannot be detected cluster-wide.

type pkiCANotPublished struct{}

func (pkiCANotPublished) ID() string       { return "pki.ca_not_published" }
func (pkiCANotPublished) Category() string { return "pki" }
func (pkiCANotPublished) Scope() string    { return "cluster" }

func (pkiCANotPublished) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap.CAMetadata != nil {
		return nil
	}
	// Only fire when there are joined nodes — single-node fresh installs
	// may not have the controller running yet.
	if len(snap.Nodes) == 0 {
		return nil
	}
	return []Finding{{
		FindingID:   FindingID("pki.ca_not_published", "cluster", "missing"),
		InvariantID: "pki.ca_not_published",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "pki",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"No CA metadata published at %s — controller has not advertised the cluster CA fingerprint. "+
				"Node agents cannot detect CA rotation without this key. "+
				"Restart the cluster controller to publish.",
			config.EtcdKeyCAMetadata),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadCAMetadata", map[string]string{
				"key":    config.EtcdKeyCAMetadata,
				"result": "key_not_found",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restart the cluster controller to publish CA metadata",
				"systemctl restart globular-cluster-controller.service"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── pki.ca_expiry_warning ──────────────────────────────────────────────────
//
// Fires when the cluster CA certificate will expire within 60 days.
// A expired CA breaks all mTLS in the cluster simultaneously. Operators
// must rotate the CA before expiry.

type pkiCAExpiryWarning struct{}

func (pkiCAExpiryWarning) ID() string       { return "pki.ca_expiry_warning" }
func (pkiCAExpiryWarning) Category() string { return "pki" }
func (pkiCAExpiryWarning) Scope() string    { return "cluster" }

func (pkiCAExpiryWarning) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap.CAMetadata == nil || snap.CAMetadata.NotAfter == "" {
		return nil
	}

	notAfter := snap.CAMetadata.NotAfterTime()
	if notAfter.IsZero() {
		return nil
	}
	daysLeft := int(time.Until(notAfter).Hours() / 24)

	var sev cluster_doctorpb.Severity
	switch {
	case daysLeft <= 0:
		sev = cluster_doctorpb.Severity_SEVERITY_CRITICAL
	case daysLeft <= 14:
		sev = cluster_doctorpb.Severity_SEVERITY_ERROR
	case daysLeft <= 60:
		sev = cluster_doctorpb.Severity_SEVERITY_WARN
	default:
		return nil // healthy
	}

	return []Finding{{
		FindingID:   FindingID("pki.ca_expiry_warning", "cluster", "ca"),
		InvariantID: "pki.ca_expiry_warning",
		Severity:    sev,
		Category:    "pki",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"Cluster CA certificate expires in %d days (%s). "+
				"All mTLS in the cluster will break simultaneously when the CA expires. "+
				"Rotate the CA before expiry.",
			daysLeft, snap.CAMetadata.NotAfter),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadCAMetadata", map[string]string{
				"key":         config.EtcdKeyCAMetadata,
				"fingerprint": snap.CAMetadata.Fingerprint,
				"not_after":   snap.CAMetadata.NotAfter,
				"days_left":   fmt.Sprintf("%d", daysLeft),
				"generation":  fmt.Sprintf("%d", snap.CAMetadata.Generation),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Rotate the cluster CA", "globular workflow start pki.ca.rotate"),
			step(2, "Verify new CA is published: globular config get /globular/pki/ca | jq .not_after", ""),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── pki.node_cert_wrong_ca ─────────────────────────────────────────────────
//
// Fires when a node's server certificate fails chain validation (ChainValid=false)
// AND the cluster CA metadata is available. This indicates the node cert was issued
// by a previous CA generation and needs regeneration. Distinct from
// security.certs.chain_valid in that it correlates the failure to CA rotation context.

type pkiNodeCertWrongCA struct{}

func (pkiNodeCertWrongCA) ID() string       { return "pki.node_cert_wrong_ca" }
func (pkiNodeCertWrongCA) Category() string { return "pki" }
func (pkiNodeCertWrongCA) Scope() string    { return "node" }

func (pkiNodeCertWrongCA) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Only fire when we have CA metadata context — otherwise security.certs.chain_valid fires.
	if snap.CAMetadata == nil {
		return nil
	}

	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		certResp, ok := snap.CertificateStatus[nid]
		if !ok {
			continue
		}
		cert := certResp.GetServerCert()
		if cert == nil || cert.GetChainValid() {
			continue // valid chain — OK
		}

		hostname := node.GetIdentity().GetHostname()
		findings = append(findings, Finding{
			FindingID:   FindingID("pki.node_cert_wrong_ca", nid, "server"),
			InvariantID: "pki.node_cert_wrong_ca",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "pki",
			EntityRef:   nid,
			Summary: fmt.Sprintf(
				"Node %s (%s) server cert fails chain validation against current CA (fingerprint %s, generation %d). "+
					"The cert was likely issued by a previous CA. The node's drift-check loop should auto-repair within 5 minutes.",
				hostname, nid, snap.CAMetadata.Fingerprint, snap.CAMetadata.Generation),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetCertificateStatus", map[string]string{
					"node_id":              nid,
					"cert_subject":         cert.GetSubject(),
					"cert_issuer":          cert.GetIssuer(),
					"cert_fingerprint":     cert.GetFingerprint(),
					"chain_valid":          "false",
					"ca_fingerprint_etcd":  snap.CAMetadata.Fingerprint,
					"ca_generation":        fmt.Sprintf("%d", snap.CAMetadata.Generation),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				actionStep(
					1,
					"Restart node agent on "+hostname+" to force immediate cert regeneration",
					"globular doctor remediate "+FindingID("pki.node_cert_wrong_ca", nid, "server")+" --step 0",
					systemctlRestartAction("globular-node-agent.service", nid),
				),
				step(2, "Verify chain: openssl verify -CAfile /var/lib/globular/pki/ca.crt /var/lib/globular/pki/issued/services/service.crt", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
