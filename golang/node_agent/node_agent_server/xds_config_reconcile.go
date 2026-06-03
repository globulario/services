// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.xds_config_reconcile
// @awareness file_role=lkg_guard_for_xds_config_json_with_last_known_good_restore_on_corruption
// @awareness enforces=globular.platform:invariant.envoy.lds_progress_required_for_http_mesh_readiness
// @awareness risk=critical
package main

// xds_config_reconcile.go — protects the on-disk xDS config
// that drives the mesh. The xDS binary reads
// /var/lib/globular/xds/config.json every 5 s; if it's
// missing or corrupted, the snapshot it builds for Envoy is
// incomplete and LDS pushes either stop or contain nothing
// Envoy will consume.
//
// The LKG (last-known-good) restore here is the node-agent's
// answer to the "Envoy never consumes LDS" class:
// envoy.lds_update_attempt_zero_despite_cds_progress.
// A broken xds/config.json upstream of this reconciler is one
// path into that failure mode; another is xDS server-side bugs
// (in Globular/internal/xds/builder/) that this file cannot
// fix. The LKG must NOT mask an upstream issue by silently
// using stale config indefinitely — if the controller-rendered
// file has been bad for more than a few cycles, that's an
// operator-visible issue.

// xds_config_reconcile.go — LKG guard for /var/lib/globular/xds/config.json.
//
// The cluster controller renders xds/config.json from live etcd cluster state
// (membership, domain, TLS paths) and pushes it to each node via the rendered-
// config delivery path. The xDS binary reads this file from disk every 5 s.
//
// This file adds last-known-good (LKG) protection so that if the file is
// corrupted or deleted between controller reconcile cycles, node-agent can
// restore the last valid config before xds starts or restarts.
//
// Pattern: file-first (same as dns_sync.go and minio_contract_reconcile.go)
//   1. Read xds/config.json from disk.
//   2. If valid → store raw bytes in LKG (generation = unix timestamp).
//   3. If invalid/missing → load LKG → restore file atomically → use LKG config.
//   4. If LKG is corrupt → reject, log, retain current runtime state.
//
// Invariant: runtime.last_known_good_required_for_critical_consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/globular_service/lkg"
)

const (
	xdsConfigPath         = "/var/lib/globular/xds/config.json"
	xdsConfigLKGSubsystem = "xds"
	xdsConfigLKGKey       = "config"
	xdsReconcileInterval  = 60 * time.Second
)

// xdsDesiredConfig is the subset of fields node-agent validates for
// a well-formed xds/config.json. etcd_endpoints must be non-empty;
// anything else is optional and passes through opaquely.
type xdsDesiredConfig struct {
	EtcdEndpoints []string               `json:"etcd_endpoints"`
	SyncInterval  int                    `json:"sync_interval_seconds,omitempty"`
	Ingress       map[string]interface{} `json:"ingress,omitempty"`
}

// loadXDSConfigWithLKG is the public entry point; always reads from
// xdsConfigPath and delegates to loadXDSConfigWithLKGPath.
func loadXDSConfigWithLKG() (*xdsDesiredConfig, string, error) {
	return loadXDSConfigWithLKGPath(xdsConfigPath)
}

// loadXDSConfigWithLKGPath reads xds/config.json from path. On a valid read it
// stores the raw bytes in LKG so the config can be recovered if the file is
// later corrupted or lost. On file-not-found or parse failure it falls back to
// the LKG record and, if valid, atomically restores the file to path.
//
// Invariant: runtime.last_known_good_required_for_critical_consumers
func loadXDSConfigWithLKGPath(path string) (*xdsDesiredConfig, string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg xdsDesiredConfig
		if parseErr := json.Unmarshal(data, &cfg); parseErr == nil && len(cfg.EtcdEndpoints) > 0 {
			// Valid — store in LKG; generation is wall time (always newer than prior write).
			_ = lkg.StoreRaw(xdsConfigLKGSubsystem, xdsConfigLKGKey, time.Now().Unix(), data)
			return &cfg, "file", nil
		}
		// File exists but invalid — fall through to LKG.
		log.Printf("xds-config: %s is invalid, trying last-known-good", path)
	} else if !os.IsNotExist(err) {
		log.Printf("xds-config: read %s failed (%v), trying last-known-good", path, err)
	}

	raw, lkgErr := lkg.LoadRaw(xdsConfigLKGSubsystem, xdsConfigLKGKey)
	if lkgErr == lkg.ErrCorrupt {
		// Corrupt LKG must never be applied — retain whatever is on disk.
		log.Printf("xds-config: LKG is corrupt — holding current runtime state, rejecting restore")
		return nil, "", lkgErr
	}
	if lkgErr != nil || len(raw) == 0 {
		// No LKG at all — nothing to restore, controller hasn't sent config yet.
		return nil, "", nil
	}

	var cfg xdsDesiredConfig
	if parseErr := json.Unmarshal(raw, &cfg); parseErr != nil {
		log.Printf("xds-config: LKG parse failed: %v — not applying", parseErr)
		return nil, "", fmt.Errorf("xds-config: LKG parse: %w", parseErr)
	}
	if len(cfg.EtcdEndpoints) == 0 {
		log.Printf("xds-config: LKG has empty etcd_endpoints — rejecting restore")
		return nil, "", fmt.Errorf("xds-config: LKG invalid: empty etcd_endpoints")
	}

	// Restore file atomically from LKG so xds binary picks it up.
	if restoreErr := restoreXDSConfigFromRaw(path, raw); restoreErr != nil {
		log.Printf("xds-config: LKG file restore failed: %v", restoreErr)
	} else {
		log.Printf("xds-config: etcd unavailable, using LKG")
	}
	return &cfg, "lkg", nil
}

// restoreXDSConfigFromRaw writes raw bytes to path via tempfile+rename.
// A crash mid-write cannot leave the canonical path half-written.
func restoreXDSConfigFromRaw(path string, raw []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// reconcileXDSConfig performs one pass of the file-vs-LKG reconcile.
// Calling it more often than xdsReconcileInterval is harmless.
func (srv *NodeAgentServer) reconcileXDSConfig() {
	cfg, source, err := loadXDSConfigWithLKGPath(xdsConfigPath)
	switch {
	case err == lkg.ErrCorrupt:
		// ErrCorrupt is logged inside loadXDSConfigWithLKGPath; nothing more to do.
	case err != nil:
		log.Printf("xds-config: reconcile error: %v", err)
	case cfg == nil:
		// File absent and no LKG — controller hasn't delivered config yet.
	case source == "lkg":
		// File was missing/corrupt; restored from LKG inside loadXDSConfigWithLKGPath.
		// xds binary will pick up the restored file on its next poll (≤5 s).
	}
}

// xdsConfigReconcileLoop runs reconcileXDSConfig periodically.
func (srv *NodeAgentServer) xdsConfigReconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(xdsReconcileInterval)
	defer ticker.Stop()

	// Immediate pass at startup — restores LKG before xds binary starts reading.
	srv.reconcileXDSConfig()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.reconcileXDSConfig()
		}
	}
}

// StartXDSConfigReconciliation launches the background reconcile loop.
// Called from main alongside StartIngressReconciliation and others.
func (srv *NodeAgentServer) StartXDSConfigReconciliation(ctx context.Context) {
	go srv.xdsConfigReconcileLoop(ctx)
}
