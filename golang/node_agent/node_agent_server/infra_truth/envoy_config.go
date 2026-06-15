package infra_truth

import (
	"encoding/json"
	"fmt"
	"os"
)

// EnvoyRenderedConfig is the parsed /run/globular/envoy/envoy-bootstrap.json —
// the bootstrap artifact written by the gateway/xDS control plane. Envoy is an
// xDS-dynamic data plane, so the bootstrap mostly wires the ADS handshake (where
// dynamic config comes from); the live truth is observed at runtime via the
// admin API. Empty/zero means the field was absent.
type EnvoyRenderedConfig struct {
	Path    string
	Present bool

	NodeID      string
	NodeCluster string

	AdminAddress string
	AdminPort    int

	// HasADSConfig is true when dynamic_resources.ads_config is present (the ADS
	// handshake that delivers all dynamic config). ADSClusterName is the upstream
	// cluster the ADS stream targets (Globular: "xds_cluster").
	HasADSConfig   bool
	ADSClusterName string
	// HasCDSConfig / HasLDSConfig: cds_config / lds_config present in
	// dynamic_resources. Without lds_config, no listeners ever load via xDS — the
	// static-config analog of the LDS wedge.
	HasCDSConfig bool
	HasLDSConfig bool

	// StaticClusterNames are the names defined under static_resources.clusters.
	// The ADS cluster MUST be among them or the ADS stream cannot connect.
	StaticClusterNames []string
}

// envoyBootstrapRaw mirrors the subset of the bootstrap JSON the truth plane
// cares about (see Globular internal/controlplane/bootstrap.go MarshalBootstrap).
type envoyBootstrapRaw struct {
	Node struct {
		ID      string `json:"id"`
		Cluster string `json:"cluster"`
	} `json:"node"`
	DynamicResources struct {
		ADSConfig *struct {
			APIType      string `json:"api_type"`
			GRPCServices []struct {
				EnvoyGRPC struct {
					ClusterName string `json:"cluster_name"`
				} `json:"envoy_grpc"`
			} `json:"grpc_services"`
		} `json:"ads_config"`
		CDSConfig *json.RawMessage `json:"cds_config"`
		LDSConfig *json.RawMessage `json:"lds_config"`
	} `json:"dynamic_resources"`
	StaticResources struct {
		Clusters []struct {
			Name string `json:"name"`
		} `json:"clusters"`
	} `json:"static_resources"`
	Admin struct {
		Address struct {
			SocketAddress struct {
				Address   string `json:"address"`
				PortValue int    `json:"port_value"`
			} `json:"socket_address"`
		} `json:"address"`
	} `json:"admin"`
}

// parseEnvoyBootstrap reads and parses the rendered Envoy bootstrap at path. A
// missing file is NOT an error: it returns Present=false so the lifecycle FSM can
// place the component at INFRA_PACKAGE_INSTALLED rather than fabricating config
// truth (the gateway writes this file on startup; before that it is absent). A
// present-but-unparseable file IS an error.
func parseEnvoyBootstrap(path string) (*EnvoyRenderedConfig, error) {
	cfg := &EnvoyRenderedConfig{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Present stays false
		}
		return cfg, fmt.Errorf("read envoy bootstrap %s: %w", path, err)
	}

	var raw envoyBootstrapRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("parse envoy bootstrap %s: %w", path, err)
	}

	cfg.Present = true
	cfg.NodeID = stripQuotes(raw.Node.ID)
	cfg.NodeCluster = stripQuotes(raw.Node.Cluster)
	cfg.AdminAddress = stripQuotes(raw.Admin.Address.SocketAddress.Address)
	cfg.AdminPort = raw.Admin.Address.SocketAddress.PortValue

	if raw.DynamicResources.ADSConfig != nil {
		cfg.HasADSConfig = true
		for _, gs := range raw.DynamicResources.ADSConfig.GRPCServices {
			if n := stripQuotes(gs.EnvoyGRPC.ClusterName); n != "" {
				cfg.ADSClusterName = n
				break
			}
		}
	}
	cfg.HasCDSConfig = raw.DynamicResources.CDSConfig != nil
	cfg.HasLDSConfig = raw.DynamicResources.LDSConfig != nil

	for _, c := range raw.StaticResources.Clusters {
		if n := stripQuotes(c.Name); n != "" {
			cfg.StaticClusterNames = append(cfg.StaticClusterNames, n)
		}
	}

	return cfg, nil
}

// adminBaseURL returns the http base URL for Envoy's admin API. The admin
// interface is plain HTTP on loopback by design (no TLS), so a default of
// http://127.0.0.1:9901 is used when the bootstrap did not specify it.
func (c *EnvoyRenderedConfig) adminBaseURL() string {
	host := c.AdminAddress
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.AdminPort
	if port == 0 {
		port = 9901
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

// hasStaticCluster reports whether name is among the static clusters.
func (c *EnvoyRenderedConfig) hasStaticCluster(name string) bool {
	for _, n := range c.StaticClusterNames {
		if n == name {
			return true
		}
	}
	return false
}

// renderedMap projects the parsed config into the InfraProbeResult.rendered map.
func (c *EnvoyRenderedConfig) renderedMap() map[string]string {
	m := map[string]string{
		"present": fmt.Sprintf("%t", c.Present),
		"path":    c.Path,
	}
	if !c.Present {
		return m
	}
	m["node_id"] = c.NodeID
	m["node_cluster"] = c.NodeCluster
	m["admin"] = fmt.Sprintf("%s:%d", c.AdminAddress, c.AdminPort)
	m["ads_config"] = fmt.Sprintf("%t", c.HasADSConfig)
	m["ads_cluster"] = c.ADSClusterName
	m["cds_config"] = fmt.Sprintf("%t", c.HasCDSConfig)
	m["lds_config"] = fmt.Sprintf("%t", c.HasLDSConfig)
	m["static_clusters"] = fmt.Sprintf("%v", c.StaticClusterNames)
	return m
}
