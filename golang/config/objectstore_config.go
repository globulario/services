package config

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

// EtcdKeyObjectStoreDesired is the authoritative objectstore topology written by the
// cluster controller. Node agents read this to render local MinIO config files.
// Local files are rendered artifacts of this state — never authored locally.
const EtcdKeyObjectStoreDesired = "/globular/objectstore/config"

// ObjectStoreMode describes the MinIO deployment topology.
type ObjectStoreMode string

const (
	// ObjectStoreModeStandalone is a single-node MinIO deployment.
	// Used when the pool has fewer than 2 nodes.
	ObjectStoreModeStandalone ObjectStoreMode = "standalone"

	// ObjectStoreModeDistributed is a multi-node erasure-coded MinIO pool.
	// Requires ≥2 nodes in the pool (MinIO distributed mode).
	ObjectStoreModeDistributed ObjectStoreMode = "distributed"
)

// ObjectStoreDesiredState is the authoritative MinIO topology published by the
// cluster controller to etcd. Node agents read this key to render local config
// files (minio.json, minio.env, systemd EnvironmentFile).
//
// Hard invariant: local files are rendered artifacts of this state only.
// They must carry RenderedArtifactMetadata proving their etcd source.
//
// +globular:schema:key="/globular/objectstore/config"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-node-agent,globular-cluster-doctor"
// +globular:schema:description="Authoritative MinIO topology: mode, pool, endpoint, credentials, generation."
// +globular:schema:invariants="Single-writer (controller); node-agents render locally from this; never authored locally."
type ObjectStoreDesiredState struct {
	// Mode is the deployment topology.
	Mode ObjectStoreMode `json:"mode"`

	// Generation is incremented each time the topology changes.
	// Node agents compare their observed generation to detect config drift.
	Generation int64 `json:"generation"`

	// Endpoint is the IP:port cluster services use to reach MinIO.
	// NEVER a DNS wildcard name — always a specific IP (VIP or primary pool node).
	// DNS wildcards (minio.<domain>) resolve round-robin to all nodes, many of
	// which have empty per-node MinIO instances, causing object-not-found errors.
	Endpoint string `json:"endpoint"`

	// AccessKey and SecretKey are the MinIO root credentials.
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`

	// Bucket is the primary MinIO bucket name.
	Bucket string `json:"bucket"`

	// Prefix is the key prefix within the bucket (typically the cluster domain).
	Prefix string `json:"prefix,omitempty"`

	// Nodes is the ordered list of node IPs in the pool (distributed mode only).
	// Append-only; ordering must be preserved for erasure set consistency.
	Nodes []string `json:"nodes,omitempty"`

	// DrivesPerNode is the number of drives per node in distributed mode.
	DrivesPerNode int `json:"drives_per_node,omitempty"`

	// VolumesHash is a stable SHA256 of the sorted MINIO_VOLUMES entries across
	// the pool. Used by the doctor to detect per-node config drift.
	VolumesHash string `json:"volumes_hash,omitempty"`

	// NodePaths maps each pool node IP to its MinIO data base path
	// (e.g. /var/lib/globular/minio or /mnt/data/minio). Node agents use this
	// to compute local MINIO_VOLUMES without needing access to the controller's
	// in-memory state. Falls back to "/var/lib/globular/minio" when absent.
	NodePaths map[string]string `json:"node_paths,omitempty"`

	// WrittenAt records when this state was last published by the controller.
	WrittenAt time.Time `json:"written_at"`
}

// EtcdKeyNodeRenderedGeneration returns the etcd key where a node agent
// records the ObjectStoreDesiredState generation it last successfully rendered.
// The workflow reads these to verify all pool nodes are ready before restart.
func EtcdKeyNodeRenderedGeneration(nodeID string) string {
	return "/globular/nodes/" + nodeID + "/objectstore/rendered_generation"
}

// Objectstore workflow/lock etcd keys.
const (
	// EtcdKeyObjectStoreAppliedGeneration records the last generation that was
	// successfully applied by the objectstore topology workflow.
	EtcdKeyObjectStoreAppliedGeneration = "/globular/objectstore/applied_generation"

	// EtcdKeyObjectStoreRestartInProgress is set while a topology restart
	// workflow is executing. Cleared on success or workflow failure.
	EtcdKeyObjectStoreRestartInProgress = "/globular/objectstore/restart_in_progress"

	// EtcdKeyObjectStoreLastRestartResult stores a JSON summary of the last
	// topology workflow run (success/failure, timestamp, details).
	EtcdKeyObjectStoreLastRestartResult = "/globular/objectstore/last_restart_result"

	// EtcdKeyObjectStoreTopologyLock is the distributed lock key that prevents
	// concurrent topology transitions.
	EtcdKeyObjectStoreTopologyLock = "/globular/locks/objectstore/minio/topology-restart"
)

// RenderedArtifactMetadata is embedded in every locally-rendered MinIO config
// file to prove it originated from etcd, not local authorship. This enforces
// the hard invariant that MinIO configuration is never authored locally.
type RenderedArtifactMetadata struct {
	// SourceEtcdKey is the etcd key that was the source of this render.
	SourceEtcdKey string `json:"source_etcd_key"`

	// SourceGeneration is the ObjectStoreDesiredState.Generation at render time.
	SourceGeneration int64 `json:"source_generation"`

	// RenderedAt is when the node agent wrote this file.
	RenderedAt time.Time `json:"rendered_at"`

	// NodeID is the node agent that rendered this file.
	NodeID string `json:"node_id"`
}

// ComputeVolumesHash returns a stable SHA256 hex string of the MINIO_VOLUMES
// URL list. nodeVolumes maps node IP → local data path (e.g. /var/lib/globular/minio).
// IPs are sorted before hashing so the hash is order-independent and reproducible.
func ComputeVolumesHash(nodeVolumes map[string]string) string {
	ips := make([]string, 0, len(nodeVolumes))
	for ip := range nodeVolumes {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	parts := make([]string, 0, len(ips))
	for _, ip := range ips {
		parts = append(parts, fmt.Sprintf("http://%s:9000%s", ip, nodeVolumes[ip]))
	}
	h := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return fmt.Sprintf("%x", h)
}

// validateEndpoint checks that the endpoint is a bare IP:port (never a DNS hostname).
// Called before any etcd write so bad state can never be persisted.
func (s *ObjectStoreDesiredState) validateEndpoint() error {
	if s.Endpoint == "" {
		return fmt.Errorf("objectstore desired state: endpoint required")
	}
	host := s.Endpoint
	if h, _, err := net.SplitHostPort(s.Endpoint); err == nil {
		host = h
	}
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		return fmt.Errorf("objectstore desired state: endpoint %q uses loopback — refused", s.Endpoint)
	}
	// Hard invariant: endpoint must be a bare IP, never a DNS wildcard hostname.
	if net.ParseIP(host) == nil {
		return fmt.Errorf("objectstore desired state: endpoint %q is a DNS hostname — only bare IP:port endpoints are allowed; the controller must publish MinioPoolNodes[0]+\":9000\"", s.Endpoint)
	}
	return nil
}

// SaveObjectStoreDesiredState writes the objectstore desired state to etcd.
// The WrittenAt field is always set to now on write.
func SaveObjectStoreDesiredState(ctx context.Context, state *ObjectStoreDesiredState) error {
	if state == nil {
		return fmt.Errorf("objectstore desired state: nil")
	}
	if err := state.validateEndpoint(); err != nil {
		return err
	}

	state.WrittenAt = time.Now()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("objectstore desired state: marshal: %w", err)
	}

	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("objectstore desired state: etcd unavailable: %w", err)
	}
	if _, err := cli.Put(ctx, EtcdKeyObjectStoreDesired, string(data)); err != nil {
		return fmt.Errorf("objectstore desired state: etcd put: %w", err)
	}
	return nil
}

// LoadObjectStoreDesiredState reads the objectstore desired state from etcd.
// Returns nil, nil if the key has not been set yet (pre-pool-formation).
func LoadObjectStoreDesiredState(ctx context.Context) (*ObjectStoreDesiredState, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("objectstore desired state: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyObjectStoreDesired)
	if err != nil {
		return nil, fmt.Errorf("objectstore desired state: etcd get: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var state ObjectStoreDesiredState
	if err := json.Unmarshal(resp.Kvs[0].Value, &state); err != nil {
		return nil, fmt.Errorf("objectstore desired state: parse: %w", err)
	}
	return &state, nil
}
