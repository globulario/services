package evidence

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// awarenessCurrentLink mirrors the path in node_agent/awareness_bundle.go.
	awarenessCurrentLink  = "/var/lib/globular/awareness/current"
	awarenessManifestFile = "manifest.json"

	releaseIndexPath = "/var/lib/globular/release-index.json"

	// PKI paths (canonical, from CLAUDE.md).
	pkiCACertPath    = "/var/lib/globular/pki/ca.crt"
	pkiNodeCertPath  = "/var/lib/globular/pki/issued/services/service.crt"
	pkiNodeKeyPath   = "/var/lib/globular/pki/issued/services/service.key"

	// Scylla config path (distro default).
	scyllaConfigPath = "/etc/scylla/scylla.yaml"
)

// bundleManifest is the minimal subset of the manifest we need here.
type bundleManifest struct {
	Version string `json:"version"`
	BuildID string `json:"build_id"`
}

// PKIObservation records the presence and basic health of local PKI artifacts.
type PKIObservation struct {
	CACertPresent  bool `json:"ca_cert_present"`
	NodeCertPresent bool `json:"node_cert_present"`
	NodeKeyPresent  bool `json:"node_key_present"`
	// CertExpired is true when we can determine the cert has expired.
	// We do not parse ASN.1 here; the normalizer sets PKI_EXPIRED if openssl says so.
	CertExpired bool `json:"cert_expired,omitempty"`
}

// ScyllaConfigObservation holds seed list and identity fields from scylla.yaml.
type ScyllaConfigObservation struct {
	Present      bool     `json:"present"`
	Seeds        []string `json:"seeds,omitempty"`
	ListenAddress string  `json:"listen_address,omitempty"`
	RPCAddress    string  `json:"rpc_address,omitempty"`
	ClusterName   string  `json:"cluster_name,omitempty"`
}

// Collector gathers raw local node evidence.
// All operations are read-only. Errors are collected as warnings, not hard failures.
type Collector struct {
	NodeID  string
	Address string
	Phase   Phase
}

// NewCollector returns a Collector for the local node.
func NewCollector(nodeID, address string, phase Phase) *Collector {
	return &Collector{NodeID: nodeID, Address: address, Phase: phase}
}

// Collect gathers a NodeRuntimeSnapshot from local sources.
// ctx controls cancellation. It never returns a non-nil error — failures produce warnings.
func (c *Collector) Collect(ctx context.Context) *NodeRuntimeSnapshot {
	snap := &NodeRuntimeSnapshot{
		NodeID:      c.NodeID,
		Address:     c.Address,
		Phase:       c.Phase,
		CollectedAt: time.Now().UTC(),
	}

	snap.Release = c.readReleaseIndex()
	snap.AwarenessBundle = c.readBundleStatus()
	snap.PKI = c.collectPKI()
	snap.ScyllaConfig = c.readScyllaConfig()
	snap.Services = c.collectSystemdUnits(ctx)
	snap.Ports = c.collectPorts(ctx)

	return snap
}

func (c *Collector) readReleaseIndex() ReleaseInfo {
	data, err := os.ReadFile(releaseIndexPath)
	if err != nil {
		return ReleaseInfo{}
	}
	var m struct {
		Version string `json:"version"`
		BuildID string `json:"build_id"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return ReleaseInfo{}
	}
	return ReleaseInfo{Version: m.Version, BuildID: m.BuildID}
}

func (c *Collector) readBundleStatus() AwarenessBundleStatus {
	manifestPath := filepath.Join(awarenessCurrentLink, awarenessManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return AwarenessBundleStatus{Present: false, Status: "MISSING"}
	}
	var m bundleManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return AwarenessBundleStatus{Present: true, Status: "CORRUPT"}
	}
	return AwarenessBundleStatus{
		Present: true,
		Version: m.Version,
		BuildID: m.BuildID,
		Status:  "LOADED",
	}
}

func (c *Collector) collectPKI() PKIObservation {
	return PKIObservation{
		CACertPresent:   fileReadable(pkiCACertPath),
		NodeCertPresent: fileReadable(pkiNodeCertPath),
		NodeKeyPresent:  fileReadable(pkiNodeKeyPath),
	}
}

// readScyllaConfig parses key fields from /etc/scylla/scylla.yaml without a YAML library.
// We only need seed_provider addresses, listen_address, rpc_address, and cluster_name.
func (c *Collector) readScyllaConfig() ScyllaConfigObservation {
	data, err := os.ReadFile(scyllaConfigPath)
	if err != nil {
		return ScyllaConfigObservation{Present: false}
	}
	obs := ScyllaConfigObservation{Present: true}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	inSeedProvider := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "listen_address:") {
			obs.ListenAddress = extractYAMLValue(trimmed)
		}
		if strings.HasPrefix(trimmed, "rpc_address:") {
			obs.RPCAddress = extractYAMLValue(trimmed)
		}
		if strings.HasPrefix(trimmed, "cluster_name:") {
			obs.ClusterName = strings.Trim(extractYAMLValue(trimmed), "\"'")
		}
		if strings.HasPrefix(trimmed, "seed_provider:") {
			inSeedProvider = true
		}
		if inSeedProvider && strings.HasPrefix(trimmed, "- seeds:") {
			raw := extractYAMLValue(trimmed)
			for _, s := range strings.Split(raw, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					obs.Seeds = append(obs.Seeds, s)
				}
			}
			inSeedProvider = false
		}
	}
	return obs
}

func extractYAMLValue(line string) string {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+1:])
}

// collectSystemdUnits reads unit states via systemctl show.
// We query a fixed list of Globular-related units rather than all units.
func (c *Collector) collectSystemdUnits(ctx context.Context) []ServiceObservation {
	units := []string{
		"globular-node-agent.service",
		"globular-mcp.service",
		"globular-cluster-controller.service",
		"globular-workflow.service",
		"globular-repository.service",
		"globular-authentication.service",
		"globular-rbac.service",
		"globular-dns.service",
		"scylla-server.service",
		"minio.service",
		"sidekick.service",
		"envoy.service",
		"etcd.service",
		"keepalived.service",
	}

	var out []ServiceObservation
	for _, unit := range units {
		out = append(out, c.queryUnit(ctx, unit))
	}
	return out
}

func (c *Collector) queryUnit(ctx context.Context, unit string) ServiceObservation {
	obs := ServiceObservation{
		UnitName:    unit,
		Name:        strings.TrimSuffix(unit, ".service"),
		ActiveState: "unknown",
	}

	cmd := exec.CommandContext(ctx, "systemctl", "show",
		"--property=ActiveState,SubState,ExecMainStatus",
		"--value", unit)
	raw, err := cmd.Output()
	if err != nil {
		return obs
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) >= 1 {
		obs.ActiveState = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		obs.SubState = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		if code, err := strconv.Atoi(strings.TrimSpace(lines[2])); err == nil {
			obs.ExitCode = code
		}
	}

	if obs.ActiveState == "failed" || obs.SubState == "start-limit-hit" {
		obs.LogExcerpt = c.journalExcerpt(ctx, unit, 10)
	}
	return obs
}

// journalExcerpt returns the last n lines from journalctl for a unit.
func (c *Collector) journalExcerpt(ctx context.Context, unit string, lines int) string {
	cmd := exec.CommandContext(ctx, "journalctl", "-u", unit,
		"-n", strconv.Itoa(lines), "--no-pager", "-o", "short")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// collectPorts checks well-known Globular ports for listener state.
func (c *Collector) collectPorts(_ context.Context) []PortObservation {
	ports := []struct {
		Port int
	}{
		{9042}, // scylla CQL
		{9160}, // scylla thrift
		{7000}, // scylla inter-node
		{7001}, // scylla inter-node TLS
		{7199}, // scylla JMX
		{9100}, // prometheus node exporter
		{9000}, // minio API
		{9001}, // minio console
		{2379}, // etcd client
		{2380}, // etcd peer
		{10004}, // workflow
		{10260}, // MCP
		{10101}, // authentication
		{10104}, // rbac
		{10006}, // dns
		{12000}, // cluster-controller
		{11000}, // node-agent
	}

	var out []PortObservation
	for _, p := range ports {
		obs := PortObservation{Port: p.Port, Protocol: "tcp"}
		conn, err := net.DialTimeout("tcp",
			fmt.Sprintf("127.0.0.1:%d", p.Port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			obs.Listening = true
		}
		out = append(out, obs)
	}
	return out
}

// fileReadable returns true if path exists and is readable.
func fileReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
