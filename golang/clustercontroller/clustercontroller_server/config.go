package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
		DefaultProfiles: []string{"compute"},
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

func applyEnvOverrides(cfg *clusterControllerConfig) {
	if v := os.Getenv("CLUSTER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("CLUSTER_JOIN_TOKEN"); v != "" {
		cfg.JoinToken = v
	}
	if v := os.Getenv("CLUSTER_DOMAIN"); v != "" {
		cfg.ClusterDomain = v
	}
	if v := os.Getenv("CLUSTER_ADMIN_EMAIL"); v != "" {
		cfg.AdminEmail = v
	}
	if v := os.Getenv("CLUSTER_BOOTSTRAP_TOKEN"); v != "" {
		cfg.BootstrapToken = v
	}
}
