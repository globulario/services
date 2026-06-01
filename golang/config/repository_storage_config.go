package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EtcdKeyRepositoryStorageConfig is the cluster-wide repository storage
// topology descriptor. Written once during cluster setup (or migration) and
// read by every repository instance at startup to configure its MinIO routing.
//
// Structure:
//
//	{
//	  "mode": "standalone_authority_with_verified_replica",
//	  "authority_node_id": "globule-ryzen",
//	  "authority_minio_endpoint": "globule-ryzen.globular.internal:9000",
//	  "authority_repository_endpoint": "globule-ryzen.globular.internal:10000",
//	  "bucket": "globular",
//	  "replica_endpoints": [],
//	  "generation": 1,
//	  "updated_at": 1234567890
//	}
const EtcdKeyRepositoryStorageConfig = "/globular/repository/storage/config"

// RepositoryStorageMode describes how artifact blobs are distributed.
type RepositoryStorageMode string

const (
	// StorageModeStandaloneAuthority: one canonical MinIO holds all blobs.
	// Non-authority instances must proxy reads/writes to the authority.
	// Replicas (if any) are read-only mirrors for backup, not serving.
	StorageModeStandaloneAuthority RepositoryStorageMode = "standalone_authority_with_verified_replica"

	// StorageModeDistributedMinIO: MinIO runs in distributed (erasure-coding)
	// mode across all storage nodes. All instances can read/write any blob.
	// Requires MinIO quorum health for artifact operations.
	StorageModeDistributedMinIO RepositoryStorageMode = "distributed_minio"
)

// RepositoryStorageConfig is the cluster-wide storage topology for the
// repository service. It is the single source of truth for:
//   - Which MinIO to use for artifact reads/writes
//   - Which mode the storage operates in
//   - Which node is the authority (standalone mode)
//   - Which endpoints are replicas (backup/verification only)
type RepositoryStorageConfig struct {
	// Mode controls how artifact blobs are routed.
	Mode RepositoryStorageMode `json:"mode"`

	// AuthorityNodeID is the Globular node ID of the MinIO authority.
	// Required in standalone_authority mode.
	AuthorityNodeID string `json:"authority_node_id"`

	// AuthorityMinioEndpoint is the MinIO address of the authority node
	// (e.g. "globule-ryzen.globular.internal:9000"). All repository instances
	// in standalone mode MUST use this endpoint — never round-robin DNS.
	AuthorityMinioEndpoint string `json:"authority_minio_endpoint"`

	// AuthorityRepositoryEndpoint is the gRPC address of the authority
	// repository service. Non-authority nodes proxy artifact queries here.
	AuthorityRepositoryEndpoint string `json:"authority_repository_endpoint"`

	// Bucket is the MinIO bucket used for artifact storage.
	Bucket string `json:"bucket"`

	// ReplicaEndpoints lists MinIO endpoints that mirror the authority.
	// Replicas are for backup/verification only — never for primary serving.
	ReplicaEndpoints []string `json:"replica_endpoints,omitempty"`

	// Generation is incremented on every topology change. Allows instances
	// to detect stale cached configs and force a reload.
	Generation int64 `json:"generation"`

	// UpdatedAt is the Unix timestamp (seconds) when this record was last written.
	UpdatedAt int64 `json:"updated_at"`
}

// Validate checks that the config is internally consistent and safe to apply.
// Returns an error describing the first violation found.
func (cfg *RepositoryStorageConfig) Validate() error {
	if cfg.Mode == "" {
		return fmt.Errorf("repository storage config: mode is required")
	}
	switch cfg.Mode {
	case StorageModeStandaloneAuthority, StorageModeDistributedMinIO:
		// valid
	default:
		return fmt.Errorf("repository storage config: unknown mode %q (want %q or %q)",
			cfg.Mode, StorageModeStandaloneAuthority, StorageModeDistributedMinIO)
	}

	if cfg.AuthorityMinioEndpoint == "" {
		return fmt.Errorf("repository storage config: authority_minio_endpoint is required")
	}
	if strings.Contains(cfg.AuthorityMinioEndpoint, "127.0.0.1") ||
		strings.Contains(cfg.AuthorityMinioEndpoint, "localhost") {
		return fmt.Errorf("repository storage config: authority_minio_endpoint %q uses loopback (rule: no localhost for remote addresses)",
			cfg.AuthorityMinioEndpoint)
	}

	// In standalone mode, round-robin DNS names are forbidden — they can
	// silently route to an empty MinIO and cause false NotFound errors.
	if cfg.Mode == StorageModeStandaloneAuthority {
		roundRobinNames := []string{"minio.globular.internal"}
		for _, rr := range roundRobinNames {
			if strings.Contains(cfg.AuthorityMinioEndpoint, rr) {
				return fmt.Errorf("repository storage config: authority_minio_endpoint %q is a round-robin DNS name — use the specific node FQDN (e.g. globule-ryzen.globular.internal:9000) to avoid split-brain reads",
					cfg.AuthorityMinioEndpoint)
			}
		}
		for _, replica := range cfg.ReplicaEndpoints {
			if strings.Contains(replica, "127.0.0.1") || strings.Contains(replica, "localhost") {
				return fmt.Errorf("repository storage config: replica endpoint %q uses loopback", replica)
			}
		}
	}

	return nil
}

// LoadRepositoryStorageConfig reads the storage topology record from etcd.
// Returns an error if the key is absent, malformed, or etcd is unavailable.
func LoadRepositoryStorageConfig() (*RepositoryStorageConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("repository storage config: etcd unavailable: %w", err)
	}

	resp, err := cli.Get(ctx, EtcdKeyRepositoryStorageConfig)
	if err != nil {
		return nil, fmt.Errorf("repository storage config: etcd get: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("repository storage config: %s not set", EtcdKeyRepositoryStorageConfig)
	}

	var cfg RepositoryStorageConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return nil, fmt.Errorf("repository storage config: parse: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveRepositoryStorageConfig writes the topology record to etcd.
// Validates before writing and increments generation automatically.
func SaveRepositoryStorageConfig(cfg *RepositoryStorageConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg.Generation++
	cfg.UpdatedAt = time.Now().Unix()

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("repository storage config: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("repository storage config: etcd unavailable: %w", err)
	}
	if _, err := cli.Put(ctx, EtcdKeyRepositoryStorageConfig, string(data)); err != nil {
		return fmt.Errorf("repository storage config: etcd put: %w", err)
	}
	return nil
}
