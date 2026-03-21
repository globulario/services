package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	configpkg "github.com/globulario/services/golang/config"
)

// clusterMembership is a snapshot of cluster nodes used for generating service configurations.
type clusterMembership struct {
	ClusterID string
	Nodes     []memberNode
}

// memberNode represents a single node in the cluster membership snapshot.
type memberNode struct {
	NodeID   string
	Hostname string
	IP       string
	Profiles []string
}

// etcdMemberState tracks which nodes are already etcd members.
// This is populated by querying the live etcd cluster before rendering configs.
type etcdMemberState struct {
	// Bootstrapped is true if at least one etcd member is active.
	Bootstrapped bool
	// MemberPeerURLs maps etcd member name → peer URL for existing members.
	MemberPeerURLs map[string]string
}

// serviceConfigContext contains everything needed to render a service config for a specific node.
type serviceConfigContext struct {
	Membership       *clusterMembership
	CurrentNode      *memberNode
	ClusterID        string
	Domain           string
	ExternalDomain   string // Public external domain (e.g., "globular.cloud") for ingress routing
	EtcdState        *etcdMemberState
	MinioPoolNodes   []string          // ordered, append-only list of MinIO node IPs
	MinioCredentials *minioCredentials // cluster-scoped MinIO root credentials
}

// profilesForEtcd lists the profiles that run etcd.
var profilesForEtcd = []string{"core", "compute", "control-plane"}

// profilesForMinio lists the profiles that run MinIO.
var profilesForMinio = []string{"core", "compute", "storage"}

// profilesForXDS lists the profiles that run the XDS server.
var profilesForXDS = []string{"core", "compute", "control-plane", "gateway"}

// profilesForDNS lists the profiles that run the DNS server.
var profilesForDNS = []string{"core", "compute", "control-plane", "dns"}

// profilesForScyllaDB lists the profiles that run ScyllaDB.
var profilesForScyllaDB = []string{"scylla", "database"}

// nodeHasProfile returns true if the node has at least one of the given profiles.
func nodeHasProfile(node *memberNode, profiles []string) bool {
	if node == nil || len(node.Profiles) == 0 {
		return false
	}
	profileSet := make(map[string]struct{}, len(profiles))
	for _, p := range profiles {
		profileSet[strings.ToLower(strings.TrimSpace(p))] = struct{}{}
	}
	for _, p := range node.Profiles {
		if _, ok := profileSet[strings.ToLower(strings.TrimSpace(p))]; ok {
			return true
		}
	}
	return false
}

// filterNodesByProfile returns all nodes that have at least one of the given profiles.
// The returned slice is sorted by NodeID for deterministic output.
func filterNodesByProfile(membership *clusterMembership, profiles []string) []memberNode {
	if membership == nil || len(membership.Nodes) == 0 {
		return nil
	}
	var result []memberNode
	for _, node := range membership.Nodes {
		if nodeHasProfile(&node, profiles) && node.IP != "" {
			result = append(result, node)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NodeID < result[j].NodeID
	})
	return result
}

// sanitizeEtcdName converts a hostname into a valid etcd member name.
// etcd member names must contain only alphanumeric characters, hyphens, and underscores.
var etcdNameRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeEtcdName(hostname string) string {
	name := strings.TrimSpace(hostname)
	if name == "" {
		name = "node"
	}
	// Replace invalid characters with hyphens
	name = etcdNameRegex.ReplaceAllString(name, "-")
	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")
	if name == "" {
		name = "node"
	}
	return name
}

// Canonical PKI paths for etcd TLS configuration (rendered into etcd.yaml).
const (
	etcdCACert     = "/var/lib/globular/pki/ca.crt"
	etcdServerCert = "/var/lib/globular/pki/issued/services/service.crt"
	etcdServerKey  = "/var/lib/globular/pki/issued/services/service.key"
)

// renderEtcdConfig generates the etcd configuration YAML for a node.
// File path: /var/lib/globular/config/etcd.yaml
//
// TLS is mandatory for both client and peer connections:
//   - Client: https://<ip>:2379 with service cert + CA
//   - Peer:   https://<ip>:2380 with service cert + CA (peer-trusted-ca for mutual auth)
//
// initial-cluster-state is "new" only for a fresh cluster bootstrap (no existing
// etcd members). For all subsequent config renders (expansion, restarts) it is
// "existing" because the data directory already contains cluster membership.
func renderEtcdConfig(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.CurrentNode == nil {
		return "", false
	}
	if !nodeHasProfile(ctx.CurrentNode, profilesForEtcd) {
		return "", false
	}

	etcdNodes := filterNodesByProfile(ctx.Membership, profilesForEtcd)
	if len(etcdNodes) == 0 {
		return "", false
	}

	currentIP := ctx.CurrentNode.IP
	if currentIP == "" {
		currentIP = "127.0.0.1"
	}

	nodeName := sanitizeEtcdName(ctx.CurrentNode.Hostname)
	if nodeName == "" {
		nodeName = sanitizeEtcdName(ctx.CurrentNode.NodeID)
	}

	// Build initial-cluster string with HTTPS peer URLs.
	var initialClusterParts []string
	for _, node := range etcdNodes {
		peerName := sanitizeEtcdName(node.Hostname)
		if peerName == "" {
			peerName = sanitizeEtcdName(node.NodeID)
		}
		peerIP := node.IP
		if peerIP == "" {
			peerIP = "127.0.0.1"
		}
		initialClusterParts = append(initialClusterParts, fmt.Sprintf("%s=https://%s:2380", peerName, peerIP))
	}
	initialCluster := strings.Join(initialClusterParts, ",")

	// Build cluster token from cluster ID
	clusterToken := ctx.ClusterID
	if clusterToken == "" {
		clusterToken = "globular"
	}
	clusterToken = clusterToken + "-etcd-cluster"

	// initial-cluster-state: "new" only for first bootstrap, "existing" for all
	// subsequent operations (expansion, restart). The controller sets EtcdState
	// based on querying the live etcd cluster.
	clusterState := "existing"
	if ctx.EtcdState == nil || !ctx.EtcdState.Bootstrapped {
		clusterState = "new"
	}

	// Client listen URLs: always bind both loopback and routable IP.
	// Even single-node clusters need the routable IP so joining nodes
	// can reach etcd for member-add operations during expansion.
	listenClientURLs := fmt.Sprintf("https://%s:2379,https://127.0.0.1:2379", currentIP)

	// Build YAML with nested TLS sections (etcd native config format).
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("name: %q\n", nodeName))
	sb.WriteString(fmt.Sprintf("data-dir: %q\n", "/var/lib/globular/etcd"))
	sb.WriteString(fmt.Sprintf("listen-client-urls: %q\n", listenClientURLs))
	sb.WriteString(fmt.Sprintf("advertise-client-urls: %q\n", fmt.Sprintf("https://%s:2379", currentIP)))
	sb.WriteString(fmt.Sprintf("listen-peer-urls: %q\n", fmt.Sprintf("https://%s:2380", currentIP)))
	sb.WriteString(fmt.Sprintf("initial-advertise-peer-urls: %q\n", fmt.Sprintf("https://%s:2380", currentIP)))
	sb.WriteString(fmt.Sprintf("initial-cluster: %q\n", initialCluster))
	sb.WriteString(fmt.Sprintf("initial-cluster-state: %q\n", clusterState))
	sb.WriteString(fmt.Sprintf("initial-cluster-token: %q\n", clusterToken))
	sb.WriteString("\n")

	// Client TLS (for etcdctl and service connections)
	sb.WriteString("client-transport-security:\n")
	sb.WriteString(fmt.Sprintf("  cert-file: %s\n", etcdServerCert))
	sb.WriteString(fmt.Sprintf("  key-file: %s\n", etcdServerKey))
	sb.WriteString(fmt.Sprintf("  trusted-ca-file: %s\n", etcdCACert))
	sb.WriteString("\n")

	// Peer TLS (inter-node etcd replication)
	sb.WriteString("peer-transport-security:\n")
	sb.WriteString(fmt.Sprintf("  cert-file: %s\n", etcdServerCert))
	sb.WriteString(fmt.Sprintf("  key-file: %s\n", etcdServerKey))
	sb.WriteString(fmt.Sprintf("  trusted-ca-file: %s\n", etcdCACert))

	return sb.String(), true
}

// renderEtcdEndpoints generates a newline-separated list of all etcd client endpoints.
// Services read this file to discover all etcd members in the cluster.
// File path: /var/lib/globular/config/etcd_endpoints
func renderEtcdEndpoints(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.Membership == nil {
		return "", false
	}
	etcdNodes := filterNodesByProfile(ctx.Membership, profilesForEtcd)
	if len(etcdNodes) == 0 {
		return "", false
	}

	var endpoints []string
	for _, node := range etcdNodes {
		ip := node.IP
		if ip == "" {
			ip = "127.0.0.1"
		}
		endpoints = append(endpoints, fmt.Sprintf("https://%s:2379", ip))
	}
	return strings.Join(endpoints, "\n") + "\n", true
}

// renderMinioConfig generates the MinIO environment configuration for a node.
// File path: /var/lib/globular/minio/minio.env
//
// Pool-aware: uses the ordered MinioPoolNodes list (from controller state)
// to preserve erasure set boundaries. New nodes are appended to the list,
// never inserted. This ensures MinIO recognizes the original pool after expansion.
//
// Credentials: uses cluster-scoped generated credentials from MinioCredentials.
func renderMinioConfig(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.CurrentNode == nil {
		return "", false
	}
	if !nodeHasProfile(ctx.CurrentNode, profilesForMinio) {
		return "", false
	}

	// Use the ordered pool list if available; fall back to dynamic membership.
	poolIPs := ctx.MinioPoolNodes
	if len(poolIPs) == 0 {
		minioNodes := filterNodesByProfile(ctx.Membership, profilesForMinio)
		for _, node := range minioNodes {
			if node.IP != "" {
				poolIPs = append(poolIPs, node.IP)
			}
		}
	}
	if len(poolIPs) == 0 {
		return "", false
	}

	var sb strings.Builder

	if len(poolIPs) == 1 {
		// Single node: standalone mode (local path only).
		sb.WriteString("MINIO_VOLUMES=/var/lib/globular/minio/data\n")
	} else {
		// Distributed mode: ordered endpoints from pool list.
		// URL order is stable (append-only list) to preserve erasure set boundaries.
		var endpoints []string
		for _, ip := range poolIPs {
			endpoints = append(endpoints, fmt.Sprintf("http://%s:9000/var/lib/globular/minio/data", ip))
		}
		sb.WriteString(fmt.Sprintf("MINIO_VOLUMES=%s\n", strings.Join(endpoints, " ")))
	}

	// Cluster-scoped credentials (generated at bootstrap, stored in controller state).
	if ctx.MinioCredentials != nil && ctx.MinioCredentials.RootUser != "" {
		sb.WriteString(fmt.Sprintf("MINIO_ROOT_USER=%s\n", ctx.MinioCredentials.RootUser))
		sb.WriteString(fmt.Sprintf("MINIO_ROOT_PASSWORD=%s\n", ctx.MinioCredentials.RootPassword))
	} else {
		sb.WriteString("MINIO_ROOT_USER=minioadmin\n")
		sb.WriteString("MINIO_ROOT_PASSWORD=minioadmin\n")
	}

	return sb.String(), true
}

// renderXDSConfig generates the XDS server configuration JSON for a node.
// File path: /var/lib/globular/xds/config.json
func renderXDSConfig(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.CurrentNode == nil {
		return "", false
	}
	if !nodeHasProfile(ctx.CurrentNode, profilesForXDS) {
		return "", false
	}

	etcdNodes := filterNodesByProfile(ctx.Membership, profilesForEtcd)
	if len(etcdNodes) == 0 {
		return "", false
	}

	var etcdEndpoints []string
	for _, node := range etcdNodes {
		nodeIP := node.IP
		if nodeIP == "" {
			nodeIP = "127.0.0.1"
		}
		etcdEndpoints = append(etcdEndpoints, fmt.Sprintf("%s:2379", nodeIP))
	}

	// Fallback to localhost if no endpoints
	if len(etcdEndpoints) == 0 {
		etcdEndpoints = []string{"127.0.0.1:2379"}
	}

	domain := ctx.Domain
	if domain == "" {
		domain = "example.com"
	}

	// Build ingress_domains: node FQDN + base external domain (for wildcard CNAME routing).
	// Both are needed so that Envoy matches requests coming in as either
	// service.globule-ryzen.globular.cloud or service.globular.cloud.
	var ingressDomains []string
	if ext := strings.TrimSpace(ctx.ExternalDomain); ext != "" {
		if hostname := strings.TrimSpace(ctx.CurrentNode.Hostname); hostname != "" {
			ingressDomains = append(ingressDomains, hostname+"."+ext)
		}
		ingressDomains = append(ingressDomains, ext)
	}

	tlsDir, fullchain, privkey, ca := configpkg.CanonicalTLSPaths(configpkg.GetRuntimeConfigDir())
	config := map[string]interface{}{
		"etcd_endpoints":        etcdEndpoints,
		"sync_interval_seconds": 5,
		"ingress": map[string]interface{}{
			"tls": map[string]interface{}{
				"enabled":          true,
				"cert_chain_path":  fullchain,
				"private_key_path": privkey,
				"tls_dir":          tlsDir,
				"ca_path":          ca,
			},
		},
	}
	if len(ingressDomains) > 0 {
		config["ingress_domains"] = ingressDomains
	}

	result, err := renderJSON(config)
	if err != nil {
		return "", false
	}
	return result, true
}

// renderDNSConfig generates the DNS initialization configuration JSON for a node.
// This config is used by the node-agent to set up authoritative DNS records (SOA, NS, glue).
// File path: /var/lib/globular/dns/dns_init.json
func renderDNSConfig(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.CurrentNode == nil {
		return "", false
	}
	if !nodeHasProfile(ctx.CurrentNode, profilesForDNS) {
		return "", false
	}

	dnsNodes := filterNodesByProfile(ctx.Membership, profilesForDNS)
	if len(dnsNodes) == 0 {
		return "", false
	}

	domain := ctx.Domain
	if domain == "" {
		domain = "example.com"
	}

	// Determine primary DNS node (first in sorted list)
	primaryNS := dnsNodes[0]
	primaryHostname := primaryNS.Hostname
	if primaryHostname == "" {
		primaryHostname = "ns1"
	}

	// Build NS records and glue records
	nsRecords := make([]map[string]interface{}, 0, len(dnsNodes))
	glueRecords := make([]map[string]interface{}, 0, len(dnsNodes))

	for i, node := range dnsNodes {
		hostname := node.Hostname
		if hostname == "" {
			hostname = fmt.Sprintf("ns%d", i+1)
		}
		nsFQDN := fmt.Sprintf("%s.%s", hostname, domain)

		nsRecords = append(nsRecords, map[string]interface{}{
			"ns":  nsFQDN,
			"ttl": 3600,
		})

		if node.IP != "" {
			glueRecords = append(glueRecords, map[string]interface{}{
				"hostname": nsFQDN,
				"ip":       node.IP,
				"ttl":      3600,
			})
		}
	}

	// Build admin email (replace @ with . for SOA MBOX format)
	adminEmail := fmt.Sprintf("admin.%s", domain)

	// SOA record configuration
	soaRecord := map[string]interface{}{
		"domain":  domain,
		"ns":      fmt.Sprintf("%s.%s.", primaryHostname, domain),
		"mbox":    adminEmail + ".",
		"serial":  generateSOASerial(),
		"refresh": 7200,    // 2 hours
		"retry":   3600,    // 1 hour
		"expire":  1209600, // 2 weeks
		"minttl":  3600,    // 1 hour
		"ttl":     3600,
	}

	config := map[string]interface{}{
		"domain":       domain,
		"soa":          soaRecord,
		"ns_records":   nsRecords,
		"glue_records": glueRecords,
		"is_primary":   ctx.CurrentNode.NodeID == primaryNS.NodeID,
	}

	result, err := renderJSON(config)
	if err != nil {
		return "", false
	}
	return result, true
}

// renderScyllaConfig generates the ScyllaDB configuration YAML for a node.
// File path: /etc/scylla/scylla.yaml
//
// ScyllaDB uses gossip-based peer discovery via seed nodes. The seed list
// is built from all nodes with scylla/database profiles in the cluster.
// Unlike etcd, no explicit member-add is needed — ScyllaDB auto-joins
// the ring when it starts with correct seeds.
func renderScyllaConfig(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.CurrentNode == nil {
		return "", false
	}
	if !nodeHasProfile(ctx.CurrentNode, profilesForScyllaDB) {
		return "", false
	}

	scyllaNodes := filterNodesByProfile(ctx.Membership, profilesForScyllaDB)
	if len(scyllaNodes) == 0 {
		return "", false
	}

	currentIP := ctx.CurrentNode.IP
	if currentIP == "" {
		return "", false // ScyllaDB cannot listen on 0.0.0.0
	}

	// Build seed list from all ScyllaDB nodes.
	var seeds []string
	for _, node := range scyllaNodes {
		if node.IP != "" {
			seeds = append(seeds, node.IP)
		}
	}
	if len(seeds) == 0 {
		seeds = []string{currentIP}
	}

	// Cluster name derived from cluster ID for consistency.
	clusterName := ctx.ClusterID
	if clusterName == "" {
		clusterName = "globular"
	}

	var sb strings.Builder
	sb.WriteString("# Managed by Globular cluster controller — do not edit manually.\n")
	sb.WriteString(fmt.Sprintf("cluster_name: '%s'\n", clusterName))
	sb.WriteString("\n")

	// Listening addresses — must be the routable IP, never 0.0.0.0 or localhost.
	sb.WriteString(fmt.Sprintf("listen_address: '%s'\n", currentIP))
	sb.WriteString(fmt.Sprintf("rpc_address: '%s'\n", currentIP))
	sb.WriteString(fmt.Sprintf("broadcast_address: '%s'\n", currentIP))
	sb.WriteString(fmt.Sprintf("broadcast_rpc_address: '%s'\n", currentIP))
	sb.WriteString("\n")

	// Ports
	sb.WriteString("native_transport_port: 9042\n")
	sb.WriteString("\n")

	// Seed provider — gossip-based peer discovery.
	sb.WriteString("seed_provider:\n")
	sb.WriteString("  - class_name: org.apache.cassandra.locator.SimpleSeedProvider\n")
	sb.WriteString("    parameters:\n")
	sb.WriteString(fmt.Sprintf("      - seeds: '%s'\n", strings.Join(seeds, ",")))
	sb.WriteString("\n")

	// Snitch — SimpleSnitch for single-DC clusters.
	sb.WriteString("endpoint_snitch: SimpleSnitch\n")
	sb.WriteString("\n")

	// Data directories
	sb.WriteString("data_file_directories:\n")
	sb.WriteString("  - /var/lib/scylla/data\n")
	sb.WriteString("commitlog_directory: /var/lib/scylla/commitlog\n")
	sb.WriteString("\n")

	// Compaction and memtable defaults (ScyllaDB optimized)
	sb.WriteString("compaction_throughput_mb_per_sec: 0\n")
	sb.WriteString("compaction_large_partition_warning_threshold_mb: 100\n")

	return sb.String(), true
}

// generateSOASerial creates a serial number in YYYYMMDDNN format based on current time.
func generateSOASerial() uint32 {
	now := time.Now().UTC()
	// Format: YYYYMMDD00 - using 00 as revision, can be incremented if needed
	serial := uint32(now.Year())*1000000 + uint32(now.Month())*10000 + uint32(now.Day())*100
	return serial
}

// renderYAML converts a map to YAML format.
// Uses a simple key: value format suitable for etcd configuration.
func renderYAML(data map[string]interface{}) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, key := range keys {
		value := data[key]
		switch v := value.(type) {
		case string:
			sb.WriteString(fmt.Sprintf("%s: %q\n", key, v))
		case int, int64, float64:
			sb.WriteString(fmt.Sprintf("%s: %v\n", key, v))
		case bool:
			sb.WriteString(fmt.Sprintf("%s: %v\n", key, v))
		default:
			// For complex types, use JSON encoding
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, string(jsonBytes)))
		}
	}
	return sb.String(), nil
}

// renderJSON converts a map to indented JSON format.
func renderJSON(data map[string]interface{}) (string, error) {
	if len(data) == 0 {
		return "{}", nil
	}
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// rendererSpec describes a single service configuration renderer.
type rendererSpec struct {
	name         string
	profiles     []string                                     // profiles that activate this renderer
	outputs      []string                                     // file paths this renderer writes
	restartUnits []string                                     // units to restart when output changes
	render       func(*serviceConfigContext) (string, bool)   // rendering function
}

// profilesForEtcdEndpoints — every profile needs to know where etcd is.
var profilesForEtcdEndpoints = []string{"core", "compute", "control-plane", "gateway", "storage", "dns", "scylla", "database"}

// renderers is the authoritative list of all config renderers in the controller.
// This registry is validated at startup by validateRenderers().
var renderers = []rendererSpec{
	{
		name:         "etcd",
		profiles:     profilesForEtcd,
		outputs:      []string{"/var/lib/globular/config/etcd.yaml"},
		restartUnits: []string{"globular-etcd.service"},
		render:       renderEtcdConfig,
	},
	{
		name:         "etcd-endpoints",
		profiles:     profilesForEtcdEndpoints,
		outputs:      []string{"/var/lib/globular/config/etcd_endpoints"},
		restartUnits: nil, // services discover endpoints on next connection
		render:       renderEtcdEndpoints,
	},
	{
		name:         "minio",
		profiles:     profilesForMinio,
		outputs:      []string{"/var/lib/globular/minio/minio.env"},
		restartUnits: []string{"globular-minio.service"},
		render:       renderMinioConfig,
	},
	{
		name:         "xds",
		profiles:     profilesForXDS,
		outputs:      []string{"/var/lib/globular/xds/config.json"},
		restartUnits: []string{"globular-xds.service"}, // XDS server consumes this config
		render:       renderXDSConfig,
	},
	{
		name:         "dns",
		profiles:     profilesForDNS,
		outputs:      []string{"/var/lib/globular/dns/dns_init.json"},
		restartUnits: []string{"globular-dns.service"},
		render:       renderDNSConfig,
	},
	{
		name:         "scylla",
		profiles:     profilesForScyllaDB,
		outputs:      []string{"/etc/scylla/scylla.yaml"},
		restartUnits: []string{"scylla-server.service"},
		render:       renderScyllaConfig,
	},
}

// validateRenderers checks for output path collisions and unknown profile references.
// Must be called once at server startup.
func validateRenderers() error {
	seen := make(map[string]string) // output path → renderer name
	for _, r := range renderers {
		for _, o := range r.outputs {
			if owner, dup := seen[o]; dup {
				return fmt.Errorf("renderer collision: %q and %q both write %q", owner, r.name, o)
			}
			seen[o] = r.name
		}
		for _, p := range r.profiles {
			if _, ok := profileUnitMap[p]; !ok {
				return fmt.Errorf("renderer %q references unknown profile %q", r.name, p)
			}
		}
	}
	return nil
}

// renderServiceConfigs generates all service-specific configurations for a node.
// Returns a map of file paths to file contents.
func renderServiceConfigs(ctx *serviceConfigContext) map[string]string {
	if ctx == nil || ctx.CurrentNode == nil {
		return nil
	}

	configs := make(map[string]string)

	for _, r := range renderers {
		for _, output := range r.outputs {
			content, ok := r.render(ctx)
			if !ok {
				continue
			}
			configs[output] = content
		}
	}

	if len(configs) == 0 {
		return nil
	}
	return configs
}

// HashRenderedConfigs computes sha256 hex hashes for each file in the rendered config map.
// Returns a new map of the same keys with their hash values.
func HashRenderedConfigs(rendered map[string]string) map[string]string {
	if len(rendered) == 0 {
		return nil
	}
	hashes := make(map[string]string, len(rendered))
	for path, content := range rendered {
		sum := sha256.Sum256([]byte(content))
		hashes[path] = hex.EncodeToString(sum[:])
	}
	return hashes
}

// restartActionsForChangedConfigs returns restart UnitActions for all renderers whose output
// files have changed (i.e., their content hash differs from the stored hash in oldHashes).
// Only renderers with at least one changed output path contribute restart actions.
func restartActionsForChangedConfigs(oldHashes map[string]string, rendered map[string]string) []*cluster_controllerpb.UnitAction {
	if len(rendered) == 0 {
		return nil
	}
	// Compute the new hashes.
	newHashes := HashRenderedConfigs(rendered)

	// Collect restart units for changed renderers (deduplicated).
	restartSet := make(map[string]struct{})
	for _, r := range renderers {
		for _, output := range r.outputs {
			newHash, exists := newHashes[output]
			if !exists {
				continue
			}
			oldHash := oldHashes[output]
			// Only restart when a previously-written file changes.
			// If oldHash is empty this is the first write for this file; the service
			// will be started fresh via enable/start actions — no restart needed.
			if oldHash != "" && newHash != oldHash {
				// This renderer has a changed output — add all its restart units.
				for _, unit := range r.restartUnits {
					restartSet[unit] = struct{}{}
				}
				break // one changed output is enough to trigger this renderer's restarts
			}
		}
	}

	if len(restartSet) == 0 {
		return nil
	}

	// Sort for deterministic output.
	restartUnits := make([]string, 0, len(restartSet))
	for unit := range restartSet {
		restartUnits = append(restartUnits, unit)
	}
	sort.Strings(restartUnits)

	actions := make([]*cluster_controllerpb.UnitAction, 0, len(restartUnits))
	for _, unit := range restartUnits {
		actions = append(actions, &cluster_controllerpb.UnitAction{
			UnitName: unit,
			Action:   "restart",
		})
	}
	return actions
}
