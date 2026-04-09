package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type clusterControllerConfig struct {
	Port            int      `json:"port"`
	JoinToken       string   `json:"join_token"`
	ClusterDomain   string   `json:"cluster_domain"`
	AdminEmail      string   `json:"admin_email"`
	BootstrapToken  string   `json:"bootstrap_token"`
	Bootstrapped    bool     `json:"bootstrapped"`
	DefaultProfiles []string `json:"default_profiles"`
}

func defaultClusterControllerConfig() *clusterControllerConfig {
	return &clusterControllerConfig{
		Port:            12000,
		DefaultProfiles: []string{"core"},
	}
}

func loadClusterControllerConfig(path string) (*clusterControllerConfig, error) {
	cfg := defaultClusterControllerConfig()

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, err
	}

	if len(b) > 0 {
		if err := json.Unmarshal(b, cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func saveClusterControllerConfig(path string, cfg *clusterControllerConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// applyEnvOverrides is intentionally empty — env vars are NOT a source of truth.
// All configuration comes from the config file or etcd.
func applyEnvOverrides(cfg *clusterControllerConfig) {
}

// validateClusterConfig ensures the configuration is valid for cluster mode
// clusterMode should be false for single-node development setups
func validateClusterConfig(cfg *clusterControllerConfig, clusterMode bool) error {
	if !clusterMode {
		return nil // Single-node development mode has no constraints
	}

	// In cluster mode, domain is required for DNS-based naming
	if cfg.ClusterDomain == "" {
		return errors.New("cluster_domain required in cluster mode")
	}

	// Validate domain format (basic sanity check)
	if len(cfg.ClusterDomain) > 253 {
		return errors.New("cluster_domain too long (max 253 chars)")
	}

	// Reject localhost in cluster domain (would break DNS)
	if cfg.ClusterDomain == "localhost" || cfg.ClusterDomain == "localhost." {
		return errors.New("cluster_domain cannot be 'localhost'")
	}

	return nil
}
