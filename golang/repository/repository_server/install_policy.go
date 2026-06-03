// @awareness namespace=globular.platform
// @awareness component=platform_repository.install_policy
// @awareness file_role=cluster_install_policy_storage_in_minio_with_single_json_key
// @awareness implements=globular.platform:intent.repository.metadata_is_authority
// @awareness risk=high
package main

// install_policy.go — persists InstallPolicySpec at a single,
// well-known MinIO key. The policy governs which artifacts are
// installable cluster-wide (channels, kinds, signature
// requirements). The single-key model means there is one
// authoritative policy; no per-node overrides, no MinIO bucket
// wildcards. Reading from anywhere else would let a stale or
// local-fork policy override the cluster's true rule.

import (
	"context"
	"encoding/json"
	"log/slog"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

const installPolicyStorageKey = "artifacts/.install-policy.json"

// writeInstallPolicy persists the install policy to storage.
func (srv *server) writeInstallPolicy(ctx context.Context, policy *cluster_controllerpb.InstallPolicySpec) error {
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return err
	}
	return srv.Storage().WriteFile(ctx, installPolicyStorageKey, data, 0o644)
}

// readInstallPolicy loads the install policy from storage.
// Returns nil if no policy is set.
func (srv *server) readInstallPolicy(ctx context.Context) *cluster_controllerpb.InstallPolicySpec {
	data, err := srv.Storage().ReadFile(ctx, installPolicyStorageKey)
	if err != nil {
		return nil
	}
	policy := &cluster_controllerpb.InstallPolicySpec{}
	if err := json.Unmarshal(data, policy); err != nil {
		slog.Warn("corrupt install policy file", "err", err)
		return nil
	}
	return policy
}

// deleteInstallPolicy removes the install policy from storage.
func (srv *server) deleteInstallPolicy(ctx context.Context) error {
	return srv.Storage().Remove(ctx, installPolicyStorageKey)
}
