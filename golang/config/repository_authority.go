package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EtcdKeyRepositoryAuthority is the sole source of truth for which node holds
// the canonical artifact store. Written by the cluster controller (or manually
// during migration). The repository service reads it at startup to decide which
// MinIO endpoint to use for artifact operations.
//
// Structure: {"node_id":"globule-ryzen","repository_endpoint":"globule-ryzen.globular.internal:10000","minio_endpoint":"globule-ryzen.globular.internal:9000","mode":"single","updated_at":1234567890}
//
// mode values:
//   - "single"      — one canonical authority, all artifact reads go to its MinIO
//   - "distributed" — reserved for future multi-authority mode (not yet implemented)
const EtcdKeyRepositoryAuthority = "/globular/repository/authority"

// RepositoryAuthority records the canonical repository node for the cluster.
// Non-authority repository instances must use the authority's MinIO endpoint
// for artifact operations to guarantee they access the complete artifact store.
type RepositoryAuthority struct {
	// NodeID is the Globular node identifier of the authority node (e.g. "globule-ryzen").
	NodeID string `json:"node_id"`
	// RepositoryEndpoint is the gRPC address of the authority repository service
	// (e.g. "globule-ryzen.globular.internal:10000"). Used by non-authority nodes
	// to proxy artifact queries when the authority proxy model is enabled.
	RepositoryEndpoint string `json:"repository_endpoint"`
	// MinioEndpoint is the MinIO address on the authority node
	// (e.g. "globule-ryzen.globular.internal:9000"). All repository instances
	// use this endpoint for artifact storage operations, bypassing round-robin DNS.
	MinioEndpoint string `json:"minio_endpoint"`
	// Mode controls authority routing ("single" or "distributed"). Default: "single".
	Mode string `json:"mode"`
	// UpdatedAt is the Unix timestamp (seconds) when this record was last written.
	UpdatedAt int64 `json:"updated_at"`
}

// LoadRepositoryAuthority reads the authority record from etcd.
// Returns an error if the key is absent, malformed, or etcd is unavailable.
// Callers should treat a missing key as "no authority set" and fall back to
// the cluster-wide MinIO config.
func LoadRepositoryAuthority() (*RepositoryAuthority, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("repository authority: etcd unavailable: %w", err)
	}

	resp, err := cli.Get(ctx, EtcdKeyRepositoryAuthority)
	if err != nil {
		return nil, fmt.Errorf("repository authority: etcd get %s: %w", EtcdKeyRepositoryAuthority, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("repository authority: %s not set", EtcdKeyRepositoryAuthority)
	}

	var auth RepositoryAuthority
	if err := json.Unmarshal(resp.Kvs[0].Value, &auth); err != nil {
		return nil, fmt.Errorf("repository authority: parse: %w", err)
	}
	if auth.MinioEndpoint == "" {
		return nil, fmt.Errorf("repository authority: minio_endpoint is empty")
	}
	if strings.Contains(auth.MinioEndpoint, "127.0.0.1") || strings.Contains(auth.MinioEndpoint, "localhost") {
		return nil, fmt.Errorf("repository authority: minio_endpoint %q uses loopback (rule: no localhost)", auth.MinioEndpoint)
	}
	if auth.Mode == "" {
		auth.Mode = "single"
	}
	return &auth, nil
}

// SaveRepositoryAuthority writes the authority record to etcd.
// Called by the cluster controller after validating the authority node is healthy
// and its MinIO contains the canonical artifact store.
func SaveRepositoryAuthority(auth *RepositoryAuthority) error {
	if auth.NodeID == "" {
		return fmt.Errorf("repository authority: node_id required")
	}
	if auth.MinioEndpoint == "" {
		return fmt.Errorf("repository authority: minio_endpoint required")
	}
	if strings.Contains(auth.MinioEndpoint, "127.0.0.1") || strings.Contains(auth.MinioEndpoint, "localhost") {
		return fmt.Errorf("repository authority: minio_endpoint %q uses loopback", auth.MinioEndpoint)
	}
	if auth.Mode == "" {
		auth.Mode = "single"
	}
	auth.UpdatedAt = time.Now().Unix()

	data, err := json.Marshal(auth)
	if err != nil {
		return fmt.Errorf("repository authority: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("repository authority: etcd unavailable: %w", err)
	}
	if _, err := cli.Put(ctx, EtcdKeyRepositoryAuthority, string(data)); err != nil {
		return fmt.Errorf("repository authority: etcd put: %w", err)
	}
	return nil
}
