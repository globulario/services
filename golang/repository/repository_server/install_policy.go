package main

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
