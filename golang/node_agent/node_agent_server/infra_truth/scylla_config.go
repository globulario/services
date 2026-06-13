package infra_truth

import (
	"fmt"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// scyllaYAMLRaw mirrors the subset of /etc/scylla/scylla.yaml that the truth
// plane cares about. ScyllaDB uses Cassandra's seed_provider shape:
//
//	cluster_name: 'Globular'
//	listen_address: 10.0.0.63
//	rpc_address: 10.0.0.63
//	broadcast_address: 10.0.0.63
//	broadcast_rpc_address: 10.0.0.63
//	api_address: 127.0.0.1
//	seed_provider:
//	    - class_name: org.apache.cassandra.locator.SimpleSeedProvider
//	      parameters:
//	          - seeds: "10.0.0.63,10.0.0.8"
type scyllaYAMLRaw struct {
	ClusterName         string `yaml:"cluster_name"`
	ListenAddress       string `yaml:"listen_address"`
	RPCAddress          string `yaml:"rpc_address"`
	BroadcastAddress    string `yaml:"broadcast_address"`
	BroadcastRPCAddress string `yaml:"broadcast_rpc_address"`
	APIAddress          string `yaml:"api_address"`
	SeedProvider        []struct {
		ClassName  string `yaml:"class_name"`
		Parameters []struct {
			Seeds string `yaml:"seeds"`
		} `yaml:"parameters"`
	} `yaml:"seed_provider"`
}

// parseScyllaYAML reads and parses the rendered ScyllaDB config at path using a
// real YAML parser (never grep — a commented or multi-document file would fool
// line matching). A missing file is NOT an error: it returns Present=false so the
// lifecycle FSM can place the component at INFRA_PACKAGE_INSTALLED rather than
// fabricating config truth. A present-but-unparseable file IS an error.
func parseScyllaYAML(path string) (*ScyllaRenderedConfig, error) {
	cfg := &ScyllaRenderedConfig{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Present stays false
		}
		return cfg, fmt.Errorf("read scylla config %s: %w", path, err)
	}

	var raw scyllaYAMLRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("parse scylla config %s: %w", path, err)
	}

	cfg.Present = true
	cfg.ClusterName = stripQuotes(raw.ClusterName)
	cfg.ListenAddress = stripQuotes(raw.ListenAddress)
	cfg.RPCAddress = stripQuotes(raw.RPCAddress)
	cfg.BroadcastAddress = stripQuotes(raw.BroadcastAddress)
	cfg.BroadcastRPCAddress = stripQuotes(raw.BroadcastRPCAddress)
	cfg.APIAddress = stripQuotes(raw.APIAddress)
	cfg.Seeds = parseSeeds(raw)

	return cfg, nil
}

// parseSeeds flattens every seeds string found under seed_provider into a
// deduplicated, order-preserving list of addresses.
func parseSeeds(raw scyllaYAMLRaw) []string {
	var out []string
	seen := map[string]bool{}
	for _, sp := range raw.SeedProvider {
		for _, p := range sp.Parameters {
			for _, s := range strings.Split(p.Seeds, ",") {
				s = stripQuotes(s)
				if s == "" || seen[s] {
					continue
				}
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// renderedMap projects the parsed config into the InfraProbeResult.rendered map.
func (c *ScyllaRenderedConfig) renderedMap() map[string]string {
	m := map[string]string{
		"present": fmt.Sprintf("%t", c.Present),
		"path":    c.Path,
	}
	if !c.Present {
		return m
	}
	m["cluster_name"] = c.ClusterName
	m["listen_address"] = c.ListenAddress
	m["rpc_address"] = c.RPCAddress
	m["broadcast_address"] = c.BroadcastAddress
	m["broadcast_rpc_address"] = c.BroadcastRPCAddress
	m["api_address"] = c.APIAddress
	m["seeds"] = strings.Join(c.Seeds, ",")
	return m
}
