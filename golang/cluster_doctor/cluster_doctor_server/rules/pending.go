package rules

import (
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// pendingInvariant returns a single INVARIANT_PENDING finding explaining
// which upstream RPC addition is needed to implement this invariant.
type pendingInvariant struct {
	id              string
	category        string
	scope           string
	summary         string
	proposedRPC     string
	proposedService string
}

func (p pendingInvariant) ID() string       { return p.id }
func (p pendingInvariant) Category() string { return p.category }
func (p pendingInvariant) Scope() string    { return p.scope }

func (p pendingInvariant) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	return []Finding{
		{
			FindingID:   FindingID(p.id, "cluster", p.id),
			InvariantID: p.id,
			Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:    p.category,
			EntityRef:   "cluster",
			Summary:     p.summary,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "pending", map[string]string{
					"invariant_id":     p.id,
					"proposed_rpc":     p.proposedRPC,
					"proposed_service": p.proposedService,
					"reason":           "upstream RPC not yet available",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Add "+p.proposedRPC+" to "+p.proposedService+" to enable this invariant", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PENDING,
		},
	}
}

// pendingInvariants returns the set of invariants that require upstream RPC additions.
func pendingInvariants() []Invariant {
	return []Invariant{
		pendingInvariant{
			id:              "cluster.repo.reachable",
			category:        "repository",
			scope:           "cluster",
			summary:         "Repository reachability check pending: GetRepositoryStatus() not yet available in ClusterControllerService",
			proposedRPC:     "GetRepositoryStatus()",
			proposedService: "ClusterControllerService",
		},
		pendingInvariant{
			id:              "cluster.discovery.consistent",
			category:        "discovery",
			scope:           "cluster",
			summary:         "Discovery consistency check pending: GetDiscoveryStatus() not yet available",
			proposedRPC:     "GetDiscoveryStatus()",
			proposedService: "ClusterControllerService or DiscoveryService",
		},
		pendingInvariant{
			id:              "security.certs.not_expired",
			category:        "tls",
			scope:           "node",
			summary:         "Certificate expiry check pending: GetCertificateStatus() not yet available in NodeAgentService",
			proposedRPC:     "GetCertificateStatus(node_id)",
			proposedService: "NodeAgentService or SecurityService",
		},
	}
}
