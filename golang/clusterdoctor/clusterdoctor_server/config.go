package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type clusterdoctorConfig struct {
	Port                      int    `json:"port"`
	ControllerEndpoint        string `json:"controller_endpoint"`
	SnapshotTTLSeconds        int    `json:"snapshot_ttl_seconds"`
	NodeHeartbeatStaleSeconds int    `json:"node_heartbeat_stale_seconds"`
	UpstreamListTimeoutSeconds int   `json:"upstream_list_timeout_seconds"`
	UpstreamNodeTimeoutSeconds int   `json:"upstream_node_timeout_seconds"`
	UpstreamNodeConcurrency   int    `json:"upstream_node_concurrency"`
	EmitAuditEvents           bool   `json:"emit_audit_events"`
}

func defaultConfig() *clusterdoctorConfig {
	return &clusterdoctorConfig{
		Port:                       12100,
		SnapshotTTLSeconds:         5,
		NodeHeartbeatStaleSeconds:  120,
		UpstreamListTimeoutSeconds: 10,
		UpstreamNodeTimeoutSeconds: 5,
		UpstreamNodeConcurrency:    20,
		EmitAuditEvents:            false,
	}
}

func loadConfig(path string) (*clusterdoctorConfig, error) {
	cfg := defaultConfig()

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}
	return cfg, nil
}

func (c *clusterdoctorConfig) validate() error {
	if c.Port <= 0 {
		return errors.New("config: port must be > 0")
	}
	if c.ControllerEndpoint == "" {
		return errors.New("config: controller_endpoint must be set")
	}
	return nil
}

func (c *clusterdoctorConfig) snapshotTTL() time.Duration {
	return time.Duration(c.SnapshotTTLSeconds) * time.Second
}

func (c *clusterdoctorConfig) heartbeatStale() time.Duration {
	return time.Duration(c.NodeHeartbeatStaleSeconds) * time.Second
}

func (c *clusterdoctorConfig) listTimeout() time.Duration {
	return time.Duration(c.UpstreamListTimeoutSeconds) * time.Second
}

func (c *clusterdoctorConfig) nodeTimeout() time.Duration {
	return time.Duration(c.UpstreamNodeTimeoutSeconds) * time.Second
}
