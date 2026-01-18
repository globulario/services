package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
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

// serviceConfigContext contains everything needed to render a service config for a specific node.
type serviceConfigContext struct {
	Membership  *clusterMembership
	CurrentNode *memberNode
	ClusterID   string
	Domain      string
}

// profilesForEtcd lists the profiles that run etcd.
var profilesForEtcd = []string{"core", "compute", "control-plane"}

// profilesForMinio lists the profiles that run MinIO.
var profilesForMinio = []string{"core", "compute", "storage"}

// profilesForXDS lists the profiles that run the XDS server.
var profilesForXDS = []string{"core", "compute", "control-plane", "gateway"}

// profilesForDNS lists the profiles that run the DNS server.
var profilesForDNS = []string{"core", "compute", "control-plane", "dns"}

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

// renderEtcdConfig generates the etcd configuration YAML for a node.
// File path: /var/lib/globular/etcd/etcd.yaml
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

	// Build initial-cluster string
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
		initialClusterParts = append(initialClusterParts, fmt.Sprintf("%s=http://%s:2380", peerName, peerIP))
	}
	initialCluster := strings.Join(initialClusterParts, ",")

	// Build cluster token from cluster ID
	clusterToken := ctx.ClusterID
	if clusterToken == "" {
		clusterToken = "globular"
	}
	clusterToken = clusterToken + "-etcd-cluster"

	// Use single-node mode if only one node
	listenClientURLs := fmt.Sprintf("http://%s:2379,http://127.0.0.1:2379", currentIP)
	if len(etcdNodes) == 1 {
		listenClientURLs = "http://127.0.0.1:2379"
	}

	config := map[string]interface{}{
		"name":                        nodeName,
		"data-dir":                    "/var/lib/globular/etcd",
		"listen-client-urls":          listenClientURLs,
		"advertise-client-urls":       fmt.Sprintf("http://%s:2379", currentIP),
		"listen-peer-urls":            fmt.Sprintf("http://%s:2380", currentIP),
		"initial-advertise-peer-urls": fmt.Sprintf("http://%s:2380", currentIP),
		"initial-cluster":             initialCluster,
		"initial-cluster-state":       "new",
		"initial-cluster-token":       clusterToken,
	}

	yaml, err := renderYAML(config)
	if err != nil {
		return "", false
	}
	return yaml, true
}

// renderMinioConfig generates the MinIO environment configuration for a node.
// File path: /var/lib/globular/minio/minio.env
func renderMinioConfig(ctx *serviceConfigContext) (string, bool) {
	if ctx == nil || ctx.CurrentNode == nil {
		return "", false
	}
	if !nodeHasProfile(ctx.CurrentNode, profilesForMinio) {
		return "", false
	}

	minioNodes := filterNodesByProfile(ctx.Membership, profilesForMinio)
	if len(minioNodes) == 0 {
		return "", false
	}

	var sb strings.Builder

	// Single node: local path only
	if len(minioNodes) == 1 {
		sb.WriteString("MINIO_VOLUMES=/var/lib/globular/minio/data\n")
	} else {
		// Multi-node: distributed mode with all endpoints
		var endpoints []string
		for _, node := range minioNodes {
			nodeIP := node.IP
			if nodeIP == "" {
				continue
			}
			endpoints = append(endpoints, fmt.Sprintf("http://%s:9000/var/lib/globular/minio/data", nodeIP))
		}
		if len(endpoints) > 0 {
			sb.WriteString(fmt.Sprintf("MINIO_VOLUMES=%s\n", strings.Join(endpoints, " ")))
		}
	}

	// Add default root credentials placeholder
	sb.WriteString("MINIO_ROOT_USER=minioadmin\n")
	sb.WriteString("MINIO_ROOT_PASSWORD=minioadmin\n")

	result := sb.String()
	if result == "" {
		return "", false
	}
	return result, true
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

	config := map[string]interface{}{
		"etcd_endpoints":        etcdEndpoints,
		"sync_interval_seconds": 5,
		"ingress": map[string]interface{}{
			"tls": map[string]interface{}{
				"enabled":          true,
				"cert_chain_path":  fmt.Sprintf("/var/lib/globular/pki/%s/fullchain.pem", domain),
				"private_key_path": fmt.Sprintf("/var/lib/globular/pki/%s/privkey.pem", domain),
			},
		},
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
		"refresh": 7200,  // 2 hours
		"retry":   3600,  // 1 hour
		"expire":  1209600, // 2 weeks
		"minttl":  3600,  // 1 hour
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

// renderServiceConfigs generates all service-specific configurations for a node.
// Returns a map of file paths to file contents.
func renderServiceConfigs(ctx *serviceConfigContext) map[string]string {
	if ctx == nil || ctx.CurrentNode == nil {
		return nil
	}

	configs := make(map[string]string)

	if etcdConfig, ok := renderEtcdConfig(ctx); ok {
		configs["/var/lib/globular/etcd/etcd.yaml"] = etcdConfig
	}

	if minioConfig, ok := renderMinioConfig(ctx); ok {
		configs["/var/lib/globular/minio/minio.env"] = minioConfig
	}

	if xdsConfig, ok := renderXDSConfig(ctx); ok {
		configs["/var/lib/globular/xds/config.json"] = xdsConfig
	}

	if dnsConfig, ok := renderDNSConfig(ctx); ok {
		configs["/var/lib/globular/dns/dns_init.json"] = dnsConfig
	}

	if len(configs) == 0 {
		return nil
	}
	return configs
}
