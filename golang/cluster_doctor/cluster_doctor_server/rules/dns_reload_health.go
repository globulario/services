// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=dns_reload_health_rule
// @awareness risk=medium
package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type dnsZoneReloadFailed struct{}

func (dnsZoneReloadFailed) ID() string       { return "dns.zone_reload_failed" }
func (dnsZoneReloadFailed) Category() string { return "dns" }
func (dnsZoneReloadFailed) Scope() string    { return "cluster" }

func (dnsZoneReloadFailed) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	phase, _ := snap.DNSZoneReloadStatus["phase"].(string)
	if phase == "" || !strings.HasPrefix(phase, "DEGRADED_") {
		return nil
	}
	lastErr, _ := snap.DNSZoneReloadStatus["last_error"].(string)
	return []Finding{{
		FindingID:   FindingID("dns.zone_reload_failed", "cluster", phase),
		InvariantID: "dns.zone_reload_failed",
		Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:    "dns",
		EntityRef:   "cluster",
		Summary:     fmt.Sprintf("DNS zone reload is degraded (%s): %s", phase, strings.TrimSpace(lastErr)),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "Get(/globular/dns/v1/status)", map[string]string{
				"phase":      phase,
				"last_error": lastErr,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Check DNS service logs", "journalctl -u globular-dns.service -n 100"),
			step(2, "Verify Scylla health for dns keyspace", "nodetool status"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

type dnsServingLastKnownGood struct{}

func (dnsServingLastKnownGood) ID() string       { return "dns.serving_last_known_good" }
func (dnsServingLastKnownGood) Category() string { return "dns" }
func (dnsServingLastKnownGood) Scope() string    { return "cluster" }

func (dnsServingLastKnownGood) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	servingLKG, _ := snap.DNSZoneReloadStatus["serving_last_known_good"].(bool)
	if !servingLKG {
		return nil
	}
	phase, _ := snap.DNSZoneReloadStatus["phase"].(string)
	lastErr, _ := snap.DNSZoneReloadStatus["last_error"].(string)
	return []Finding{{
		FindingID:   FindingID("dns.serving_last_known_good", "cluster", phase),
		InvariantID: "dns.serving_last_known_good",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "dns",
		EntityRef:   "cluster",
		Summary:     fmt.Sprintf("DNS is serving last-known-good zones (%s). Recent reload error: %s", phase, strings.TrimSpace(lastErr)),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "Get(/globular/dns/v1/status)", map[string]string{
				"phase":                   phase,
				"serving_last_known_good": "true",
				"last_error":              lastErr,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restore Scylla availability so DNS can resume active reloads", "systemctl status scylla-server"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
