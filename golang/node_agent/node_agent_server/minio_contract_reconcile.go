package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/config"
)

// minioContractFile is the well-known path where the installer and the
// MinIO package pre-start hook drop a JSON contract describing how other
// services should reach the cluster's MinIO pool. It is a *convenience*
// file — etcd (/globular/cluster/minio/config) is the authoritative
// source of truth. Any drift between the two is a bug in whoever last
// wrote the file.
const minioContractFile = "/var/lib/globular/objectstore/minio.json"

// reconcileMinioContract ensures /var/lib/globular/objectstore/minio.json
// contains a valid JSON contract that matches the MinIO cluster config in
// etcd. It heals three failure modes:
//
//  1. File missing — writes a fresh contract from etcd.
//  2. File corrupt (e.g. overwritten with plaintext credentials) — logs
//     the parse error once and rewrites the file from etcd.
//  3. File valid but stale (endpoint/bucket/auth drifted from etcd) —
//     rewrites so local services don't use outdated connection info.
//
// Hard invariant: the rendered file always carries RenderedArtifactMetadata
// proving it was rendered from etcd, never authored locally.
//
// On success with no changes the function is silent to keep the heartbeat
// loop quiet. If etcd is unavailable the call is a no-op: a missing etcd
// config is a transient condition, not a failure — we leave whatever is
// on disk alone and try again on the next tick.
func (srv *NodeAgentServer) reconcileMinioContract(ctx context.Context) {
	// Source of truth: etcd — both the consumer config and the topology.
	etcdCfg, err := config.BuildMinioProxyConfig()
	if err != nil || etcdCfg == nil {
		// etcd not ready yet — harmless; try again next tick.
		return
	}

	// Load the objectstore desired state to get the current generation for provenance.
	var sourceGeneration int64
	if desired, err := config.LoadObjectStoreDesiredState(ctx); err == nil && desired != nil {
		sourceGeneration = desired.Generation
	}

	// Current disk state (may be missing, corrupt, or stale).
	existing, existingErr := loadMinioContractFromDisk(minioContractFile)

	switch {
	case existingErr == nil && minioContractsEqual(existing, etcdCfg):
		// No drift — nothing to do.
		return
	case errors.Is(existingErr, os.ErrNotExist):
		log.Printf("minio-contract: %s missing — writing from etcd", minioContractFile)
	case existingErr != nil:
		log.Printf("minio-contract: %s corrupt (%v) — repairing from etcd", minioContractFile, existingErr)
	default:
		log.Printf("minio-contract: %s stale — updating from etcd (endpoint=%s bucket=%s)",
			minioContractFile, etcdCfg.Endpoint, etcdCfg.Bucket)
	}

	meta := &config.RenderedArtifactMetadata{
		SourceEtcdKey:    config.EtcdKeyObjectStoreDesired,
		SourceGeneration: sourceGeneration,
		RenderedAt:       time.Now(),
		NodeID:           srv.nodeID,
	}
	if err := writeMinioContractAtomic(minioContractFile, etcdCfg, meta); err != nil {
		log.Printf("minio-contract: write failed: %v", err)
		return
	}
}

// loadMinioContractFromDisk reads and parses the on-disk MinIO contract.
// Returns os.ErrNotExist when the file does not exist; any other error
// indicates a parse/validation failure.
func loadMinioContractFromDisk(path string) (*config.MinioProxyConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return config.LoadMinioProxyConfigFrom(f)
}

// writeMinioContractAtomic writes the contract via tempfile+rename so a
// crash mid-write cannot leave the canonical path half-written.
// meta embeds provenance proving the file was rendered from etcd.
func writeMinioContractAtomic(path string, cfg *config.MinioProxyConfig, meta *config.RenderedArtifactMetadata) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "minio.json.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := config.SaveMinioProxyConfigWithProvenanceTo(tmp, cfg, meta); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

// minioContractsEqual compares the fields the file-based consumers care
// about: endpoint, bucket, prefix, secure flag, CA bundle path, and auth.
// We intentionally compare the normalized JSON representation rather than
// field-by-field so that ordering differences (e.g. a future field added
// in one place but not the other) cannot cause infinite rewrite loops.
func minioContractsEqual(a, b *config.MinioProxyConfig) bool {
	if a == nil || b == nil {
		return a == b
	}
	aJSON, aErr := marshalMinioContract(a)
	bJSON, bErr := marshalMinioContract(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return bytes.Equal(aJSON, bJSON)
}

func marshalMinioContract(cfg *config.MinioProxyConfig) ([]byte, error) {
	var buf bytes.Buffer
	if err := config.SaveMinioProxyConfigTo(&buf, cfg); err != nil {
		return nil, err
	}
	// Re-unmarshal+marshal via encoding/json to canonicalize whitespace
	// and key ordering so cosmetic differences don't count as drift.
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
