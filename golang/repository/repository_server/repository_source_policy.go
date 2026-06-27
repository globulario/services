package main

// repository_source_policy.go — etcd-backed source resolution policy.
//
// Policy is stored at /globular/repository/source-policy as JSON.
// Defaults are used when etcd is unavailable or the key is missing.
//
// Rules enforced regardless of policy:
//   - LOCAL_POSIX is always present and cannot be disabled.
//   - Unknown source types are rejected at chain-build time.

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/config"
)

const sourcePolicyEtcdKey = "/globular/repository/source-policy"

// SourcePolicy controls how the resolver builds and uses the source chain.
type SourcePolicy struct {
	// Enabled = false disables on-demand resolution (emergency circuit-breaker).
	Enabled bool `json:"enabled"`

	// ResolutionOrder is advisory; actual chain is always LOCAL_POSIX first.
	// Entries: "LOCAL_POSIX", "UPSTREAM:<name>" or "UPSTREAM" (all).
	ResolutionOrder []string `json:"resolution_order,omitempty"`

	// AllowNetworkMountSources blocks upstream sources whose configured paths are
	// on network-mounted filesystems (NFS/CIFS). Only applies to LOCAL_DIR sources.
	AllowNetworkMountSources bool `json:"allow_network_mount_sources"`

	// RequireChecksum enforces that non-local sources must provide a sha256.
	// When true, ErrChecksumUnknown causes that source to be skipped.
	RequireChecksum bool `json:"require_checksum"`

	// MaterializeBeforeInstall is always true — included for observability only.
	MaterializeBeforeInstall bool `json:"materialize_before_install"`
}

// defaultSourcePolicy returns safe defaults when etcd is unavailable.
func defaultSourcePolicy() SourcePolicy {
	return SourcePolicy{
		Enabled:                  true,
		ResolutionOrder:          []string{"LOCAL_POSIX", "UPSTREAM"},
		AllowNetworkMountSources: false,
		RequireChecksum:          true,
		MaterializeBeforeInstall: true,
	}
}

// loadSourcePolicy reads the source policy from etcd. Returns defaults on any error.
// This is intentionally cheap — etcd should be local and fast.
func (srv *server) loadSourcePolicy(ctx context.Context) SourcePolicy {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return defaultSourcePolicy()
	}
	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, sourcePolicyEtcdKey)
	if err != nil || len(resp.Kvs) == 0 {
		return defaultSourcePolicy()
	}
	var p SourcePolicy
	if err := json.Unmarshal(resp.Kvs[0].Value, &p); err != nil {
		slog.Warn("repository-source-policy: corrupt etcd entry — using defaults", "err", err)
		return defaultSourcePolicy()
	}
	return p
}

// SaveSourcePolicy writes a source policy to etcd. Used by CLI / operator tooling.
func (srv *server) SaveSourcePolicy(ctx context.Context, p SourcePolicy) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = cli.Put(tctx, sourcePolicyEtcdKey, string(data))
	return err
}
