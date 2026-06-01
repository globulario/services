package rules

import (
	"fmt"
	"net"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// ─── objectstore.endpoint_dns_wildcard ───────────────────────────────────────
//
// Fires when the MinIO endpoint stored in the desired state is a DNS hostname
// rather than a bare IP. DNS wildcards (minio.<domain>) resolve round-robin to
// all cluster nodes, most of which have empty per-node MinIO instances, causing
// silent object-not-found errors on every ~1/N request.

type objectstoreEndpointDNSWildcard struct{}

func (objectstoreEndpointDNSWildcard) ID() string       { return "objectstore.endpoint_dns_wildcard" }
func (objectstoreEndpointDNSWildcard) Category() string { return "objectstore" }
func (objectstoreEndpointDNSWildcard) Scope() string    { return "cluster" }

func (objectstoreEndpointDNSWildcard) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil {
		return nil
	}

	endpoint := desired.Endpoint
	if endpoint == "" {
		return nil
	}
	// Strip port if present.
	host := endpoint
	if h, _, err := net.SplitHostPort(endpoint); err == nil {
		host = h
	}
	// If host parses as a valid IP it's safe; otherwise it's a DNS name.
	if net.ParseIP(host) != nil {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.endpoint_dns_wildcard", "cluster", endpoint),
		InvariantID: "objectstore.endpoint_dns_wildcard",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO endpoint %q is a DNS hostname — wildcard A records resolve round-robin "+
				"to all nodes, causing silent object-not-found errors on non-primary nodes. "+
				"Fix: controller must publish a bare IP endpoint.",
			endpoint),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
				"endpoint":   endpoint,
				"mode":       string(desired.Mode),
				"generation": fmt.Sprintf("%d", desired.Generation),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restart the cluster controller to republish the endpoint as a bare IP", "systemctl restart globular-cluster-controller.service"),
			step(2, "Verify: globular config get /globular/objectstore/config | jq .endpoint", ""),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.standalone_in_cluster ───────────────────────────────────────
//
// Fires when the objectstore is in standalone mode but the cluster has more than
// one node. Standalone means only one node holds the data — any node that receives
// a request and isn't the standalone host will see an empty bucket.

type objectstoreStandaloneInCluster struct{}

func (objectstoreStandaloneInCluster) ID() string       { return "objectstore.standalone_in_cluster" }
func (objectstoreStandaloneInCluster) Category() string { return "objectstore" }
func (objectstoreStandaloneInCluster) Scope() string    { return "cluster" }

func (objectstoreStandaloneInCluster) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Mode != config.ObjectStoreModeStandalone {
		return nil
	}

	nodeCount := len(snap.Nodes)
	if nodeCount <= 1 {
		return nil // standalone is fine for single-node clusters
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.standalone_in_cluster", "cluster", "mode"),
		InvariantID: "objectstore.standalone_in_cluster",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"MinIO is running in standalone mode with %d nodes — data is stored on only one node. "+
				"Requests routed to other nodes will see empty buckets. "+
				"Run the objectstore migration workflow to distribute data.",
			nodeCount),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
				"mode":       string(desired.Mode),
				"endpoint":   desired.Endpoint,
				"node_count": fmt.Sprintf("%d", nodeCount),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Migrate to distributed MinIO", "globular workflow start objectstore.minio.migrate_to_distributed"),
			step(2, "Check pool nodes: globular config get /globular/objectstore/config | jq .nodes", ""),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.endpoint_unreachable ────────────────────────────────────────
//
// Fires when the MinIO endpoint published in the desired state is not reachable
// via TCP from the doctor host. This indicates MinIO is down or misconfigured.

type objectstoreEndpointUnreachable struct{}

func (objectstoreEndpointUnreachable) ID() string       { return "objectstore.endpoint_unreachable" }
func (objectstoreEndpointUnreachable) Category() string { return "objectstore" }
func (objectstoreEndpointUnreachable) Scope() string    { return "cluster" }

func (objectstoreEndpointUnreachable) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	desired := snap.ObjectStoreDesired
	if desired == nil || desired.Endpoint == "" {
		return nil
	}

	// Ensure endpoint has a port.
	endpoint := desired.Endpoint
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		endpoint = endpoint + ":9000"
	}

	conn, err := net.DialTimeout("tcp", endpoint, 3e9) // 3s
	if err == nil {
		conn.Close()
		return nil // reachable
	}

	// Don't fire if the endpoint is a DNS name (endpoint_dns_wildcard fires first).
	host, _, _ := net.SplitHostPort(endpoint)
	if net.ParseIP(host) == nil {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.endpoint_unreachable", "cluster", endpoint),
		InvariantID: "objectstore.endpoint_unreachable",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf("MinIO endpoint %s is not reachable via TCP: %v — object storage is unavailable", endpoint, err),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("tcp_probe", "DialTimeout", map[string]string{
				"endpoint": endpoint,
				"error":    err.Error(),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Check MinIO service status", "systemctl status globular-minio.service"),
			step(2, "Check MinIO logs", "journalctl -u globular-minio.service -n 50"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.no_desired_state ────────────────────────────────────────────
//
// Fires when /globular/objectstore/config has never been written. This means
// the controller has not yet published an authoritative topology — node agents
// cannot render correct local configs and will fall back to stale local files.

type objectstoreNoDesiredState struct{}

func (objectstoreNoDesiredState) ID() string       { return "objectstore.no_desired_state" }
func (objectstoreNoDesiredState) Category() string { return "objectstore" }
func (objectstoreNoDesiredState) Scope() string    { return "cluster" }

func (objectstoreNoDesiredState) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap.ObjectStoreDesired != nil {
		return nil
	}
	// Only fire when there are nodes (single-node fresh install is OK without objectstore)
	if len(snap.Nodes) == 0 {
		return nil
	}
	// Only fire when at least one node has MinIO (storage profile)
	hasMinio := false
	for _, n := range snap.Nodes {
		for _, p := range n.GetProfiles() {
			if p == "storage" || p == "core" {
				hasMinio = true
				break
			}
		}
		if hasMinio {
			break
		}
	}
	if !hasMinio {
		return nil
	}

	return []Finding{{
		FindingID:   FindingID("objectstore.no_desired_state", "cluster", "missing"),
		InvariantID: "objectstore.no_desired_state",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"No objectstore desired state found at %s — controller has not published "+
				"the authoritative MinIO topology. Node agents may be using stale local configs.",
			config.EtcdKeyObjectStoreDesired),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadObjectStoreDesiredState", map[string]string{
				"key":    config.EtcdKeyObjectStoreDesired,
				"result": "key_not_found",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restart the cluster controller to publish objectstore desired state",
				"systemctl restart globular-cluster-controller.service"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── objectstore.consumer_endpoint_dns_wildcard ──────────────────────────────
//
// Fires when the consumer MinIO endpoint (/globular/cluster/minio/config) is
// still a DNS name. This key is read by the repository, backup, and gateway
// services — if it's a wildcard it causes intermittent failures there too.

type objectstoreConsumerEndpointDNSWildcard struct{}

func (objectstoreConsumerEndpointDNSWildcard) ID() string {
	return "objectstore.consumer_endpoint_dns_wildcard"
}
func (objectstoreConsumerEndpointDNSWildcard) Category() string { return "objectstore" }
func (objectstoreConsumerEndpointDNSWildcard) Scope() string    { return "cluster" }

func (objectstoreConsumerEndpointDNSWildcard) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Only fire if the topology key already has the correct IP endpoint —
	// if both are wrong we rely on objectstore.endpoint_dns_wildcard.
	if snap.ObjectStoreDesired == nil {
		return nil
	}
	desiredEndpoint := snap.ObjectStoreDesired.Endpoint
	if desiredEndpoint == "" {
		return nil
	}

	// Read the consumer config endpoint from etcd.
	cfg, err := config.LoadMinIOConfig()
	if err != nil {
		return nil // can't read — skip
	}
	consumerEndpoint := cfg.Endpoint
	if consumerEndpoint == "" {
		return nil
	}

	// Check if consumer endpoint is a DNS name while desired state has a bare IP.
	host := consumerEndpoint
	if h, _, err := net.SplitHostPort(consumerEndpoint); err == nil {
		host = h
	}
	if net.ParseIP(host) != nil {
		return nil // already an IP — fine
	}

	// Consumer endpoint is DNS; desired state has IP. Flag the mismatch.
	return []Finding{{
		FindingID:   FindingID("objectstore.consumer_endpoint_dns_wildcard", "cluster", consumerEndpoint),
		InvariantID: "objectstore.consumer_endpoint_dns_wildcard",
		Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:    "objectstore",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"Consumer MinIO endpoint at %s is %q (DNS name) but desired state has %q (bare IP). "+
				"Services reading the consumer key will get round-robin routed to empty MinIO nodes. "+
				"Restart the controller to re-publish.",
			config.EtcdKeyMinioConfig, consumerEndpoint, desiredEndpoint),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "LoadMinIOConfig", map[string]string{
				"consumer_key":     config.EtcdKeyMinioConfig,
				"consumer_endpoint": consumerEndpoint,
				"desired_endpoint": desiredEndpoint,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Restart controller to republish correct IP endpoint",
				"systemctl restart globular-cluster-controller.service"),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// hasObjectStoreProfile returns true if any node reports a profile that runs MinIO.
func hasObjectStoreProfile(profiles []string) bool {
	for _, p := range profiles {
		if strings.EqualFold(p, "storage") || strings.EqualFold(p, "core") {
			return true
		}
	}
	return false
}
