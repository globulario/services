package infra_truth

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// EtcdRenderedConfig is the parsed /var/lib/globular/config/etcd.yaml — the
// rendered config artifact owned by the controller's reconcileServiceConfigs.
// Empty string / nil means the field was absent from the file.
type EtcdRenderedConfig struct {
	Path    string
	Present bool

	Name                     string
	DataDir                  string
	ListenClientURLs         []string
	AdvertiseClientURLs      []string
	ListenPeerURLs           []string
	InitialAdvertisePeerURLs []string
	InitialClusterState      string // "new" | "existing"
	InitialClusterToken      string

	// InitialCluster is the parsed "name=peerURL,name=peerURL" membership map,
	// keyed by member name. InitialClusterNames is the sorted name list.
	InitialCluster      map[string]string
	InitialClusterNames []string

	// TLS material the daemon needs to talk to peers and serve clients. A
	// missing peer trusted-ca-file means peers cannot authenticate this member.
	PeerCertFile   string
	PeerKeyFile    string
	PeerTrustedCA  string
	ClientCertFile string
	ClientKeyFile  string
}

// etcdYAMLRaw mirrors the subset of etcd.yaml the truth plane cares about. etcd
// uses a flat config with nested *-transport-security sections.
type etcdYAMLRaw struct {
	Name                     string `yaml:"name"`
	DataDir                  string `yaml:"data-dir"`
	ListenClientURLs         string `yaml:"listen-client-urls"`
	AdvertiseClientURLs      string `yaml:"advertise-client-urls"`
	ListenPeerURLs           string `yaml:"listen-peer-urls"`
	InitialAdvertisePeerURLs string `yaml:"initial-advertise-peer-urls"`
	InitialCluster           string `yaml:"initial-cluster"`
	InitialClusterState      string `yaml:"initial-cluster-state"`
	InitialClusterToken      string `yaml:"initial-cluster-token"`
	ClientTransportSecurity  struct {
		CertFile string `yaml:"cert-file"`
		KeyFile  string `yaml:"key-file"`
	} `yaml:"client-transport-security"`
	PeerTransportSecurity struct {
		CertFile      string `yaml:"cert-file"`
		KeyFile       string `yaml:"key-file"`
		TrustedCAFile string `yaml:"trusted-ca-file"`
	} `yaml:"peer-transport-security"`
}

// parseEtcdYAML reads and parses the rendered etcd config at path using a real
// YAML parser (never grep — quoted scalars and nested TLS blocks would fool line
// matching). A missing file is NOT an error: it returns Present=false so the
// lifecycle FSM can place the component at INFRA_PACKAGE_INSTALLED rather than
// fabricating config truth. A present-but-unparseable file IS an error.
func parseEtcdYAML(path string) (*EtcdRenderedConfig, error) {
	cfg := &EtcdRenderedConfig{Path: path, InitialCluster: map[string]string{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Present stays false
		}
		return cfg, fmt.Errorf("read etcd config %s: %w", path, err)
	}

	var raw etcdYAMLRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("parse etcd config %s: %w", path, err)
	}

	cfg.Present = true
	cfg.Name = stripQuotes(raw.Name)
	cfg.DataDir = stripQuotes(raw.DataDir)
	cfg.ListenClientURLs = splitURLs(raw.ListenClientURLs)
	cfg.AdvertiseClientURLs = splitURLs(raw.AdvertiseClientURLs)
	cfg.ListenPeerURLs = splitURLs(raw.ListenPeerURLs)
	cfg.InitialAdvertisePeerURLs = splitURLs(raw.InitialAdvertisePeerURLs)
	cfg.InitialClusterState = stripQuotes(raw.InitialClusterState)
	cfg.InitialClusterToken = stripQuotes(raw.InitialClusterToken)
	cfg.InitialCluster, cfg.InitialClusterNames = parseInitialCluster(raw.InitialCluster)
	cfg.PeerCertFile = stripQuotes(raw.PeerTransportSecurity.CertFile)
	cfg.PeerKeyFile = stripQuotes(raw.PeerTransportSecurity.KeyFile)
	cfg.PeerTrustedCA = stripQuotes(raw.PeerTransportSecurity.TrustedCAFile)
	cfg.ClientCertFile = stripQuotes(raw.ClientTransportSecurity.CertFile)
	cfg.ClientKeyFile = stripQuotes(raw.ClientTransportSecurity.KeyFile)

	return cfg, nil
}

// splitURLs splits a comma-separated URL list into trimmed, deduplicated entries.
func splitURLs(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, u := range strings.Split(stripQuotes(s), ",") {
		u = stripQuotes(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// parseInitialCluster parses "name=https://ip:2380,name=https://ip:2380" into a
// name->peerURL map plus a sorted name slice.
func parseInitialCluster(s string) (map[string]string, []string) {
	m := map[string]string{}
	for _, part := range strings.Split(stripQuotes(s), ",") {
		part = stripQuotes(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(part[:eq])
		peerURL := strings.TrimSpace(part[eq+1:])
		if name == "" || peerURL == "" {
			continue
		}
		m[name] = peerURL
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return m, names
}

// hostsFromURLs extracts the host (without port/scheme) of each URL. Unparseable
// entries are skipped. Used to compare advertised addresses against desired.
func hostsFromURLs(urls []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range urls {
		h := hostFromURL(raw)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// hostFromURL returns the bare host of a URL like "https://10.0.0.63:2380".
func hostFromURL(raw string) string {
	raw = stripQuotes(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	// Fall back: strip scheme and port manually.
	s := raw
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// renderedMap projects the parsed config into the InfraProbeResult.rendered map.
func (c *EtcdRenderedConfig) renderedMap() map[string]string {
	m := map[string]string{
		"present": fmt.Sprintf("%t", c.Present),
		"path":    c.Path,
	}
	if !c.Present {
		return m
	}
	m["name"] = c.Name
	m["data_dir"] = c.DataDir
	m["listen_client_urls"] = strings.Join(c.ListenClientURLs, ",")
	m["advertise_client_urls"] = strings.Join(c.AdvertiseClientURLs, ",")
	m["listen_peer_urls"] = strings.Join(c.ListenPeerURLs, ",")
	m["initial_advertise_peer_urls"] = strings.Join(c.InitialAdvertisePeerURLs, ",")
	m["initial_cluster"] = strings.Join(c.InitialClusterNames, ",")
	m["initial_cluster_state"] = c.InitialClusterState
	m["initial_cluster_token"] = c.InitialClusterToken
	m["peer_tls"] = fmt.Sprintf("cert=%t key=%t ca=%t", c.PeerCertFile != "", c.PeerKeyFile != "", c.PeerTrustedCA != "")
	return m
}
