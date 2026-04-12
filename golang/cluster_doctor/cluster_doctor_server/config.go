package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

type clusterdoctorConfig struct {
	Port                       int    `json:"port"`
	ControllerEndpoint         string `json:"controller_endpoint"`
	WorkflowEndpoint           string `json:"workflow_endpoint"`
	ClusterID                  string `json:"cluster_id"`
	SnapshotTTLSeconds         int    `json:"snapshot_ttl_seconds"`
	NodeHeartbeatStaleSeconds  int    `json:"node_heartbeat_stale_seconds"`
	UpstreamListTimeoutSeconds int    `json:"upstream_list_timeout_seconds"`
	UpstreamNodeTimeoutSeconds int    `json:"upstream_node_timeout_seconds"`
	UpstreamNodeConcurrency    int    `json:"upstream_node_concurrency"`
	EmitAuditEvents            bool   `json:"emit_audit_events"`
}

func defaultConfig() *clusterdoctorConfig {
	return &clusterdoctorConfig{
		Port:                       12100,
		// Use "localhost" not "127.0.0.1" everywhere: the cluster CA
		// issues service certs with DNS:localhost in the SAN set but
		// not the loopback IP literally, so any 127.0.0.1 dial fails
		// TLS verification. See docs/endpoint_resolution_policy.md.
		ControllerEndpoint:         "localhost:12000",
		// Fallback only — the real endpoint is resolved from etcd at startup
		// via config.ResolveServiceAddr("workflow.WorkflowService", ...).
		// This default fires only if etcd is unreachable during doctor startup.
		WorkflowEndpoint:           "",
		ClusterID:                  "",
		SnapshotTTLSeconds:         5,
		NodeHeartbeatStaleSeconds:  120,
		UpstreamListTimeoutSeconds: 10,
		UpstreamNodeTimeoutSeconds: 5,
		UpstreamNodeConcurrency:    20,
		EmitAuditEvents:            true,
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

	// Backward compatibility: older packaged configs may set controller_endpoint
	// to an empty string. If missing, fall back to the default localhost:12000.
	if strings.TrimSpace(cfg.ControllerEndpoint) == "" {
		cfg.ControllerEndpoint = defaultConfig().ControllerEndpoint
	}
	// Normalize loopback IP literals to "localhost" via the shared
	// resolver — the service cert's SAN covers DNS:localhost, not the
	// IPs 127.0.0.1/::1. Existing deployed configs wrote "127.0.0.1:12000"
	// which now fails TLS verify.
	cfg.ControllerEndpoint = config.NormalizeLoopback(cfg.ControllerEndpoint)
	cfg.WorkflowEndpoint = config.NormalizeLoopback(cfg.WorkflowEndpoint)
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
