package policy

import (
	"context"
	"log/slog"
	"time"
)

// RoleStore is the interface for RBAC role persistence.
// Implemented by the RBAC client or a direct store adapter.
type RoleStore interface {
	// RoleExists returns true if a role with the given name exists in RBAC.
	RoleExists(ctx context.Context, roleName string) (bool, error)
	// CreateRole creates a new role with the given name and action grants.
	// Must not overwrite existing roles.
	CreateRole(ctx context.Context, roleName string, actions []string, metadata map[string]string) error
}

// SeedResult tracks the outcome of a role seeding operation.
type SeedResult struct {
	Seeded  int // roles created (did not exist)
	Skipped int // roles already exist (preserved)
	Failed  int // roles that failed to create
}

// SeedServiceRoles loads roles from generated/override files for a service
// and seeds any missing roles into RBAC. Existing roles are never overwritten.
//
// This implements the non-destructive seeding rule:
//   - Generated roles are bootstrap artifacts only.
//   - If a role already exists in RBAC (possibly admin-edited), it is preserved.
//   - Only roles that don't exist are created.
//
// The metadata map attached to seeded roles includes provenance information
// so operators can distinguish generated roles from manually created ones.
func SeedServiceRoles(ctx context.Context, serviceName string, store RoleStore) (*SeedResult, error) {
	roles, fromFile, _ := LoadServiceRoles(serviceName)
	if !fromFile || len(roles) == 0 {
		return &SeedResult{}, nil
	}

	result := &SeedResult{}
	for _, role := range roles {
		exists, err := store.RoleExists(ctx, role.Name)
		if err != nil {
			slog.Warn("policy: seed: failed to check role existence",
				"role", role.Name, "error", err)
			result.Failed++
			continue
		}

		if exists {
			// Role exists in RBAC (possibly admin-edited) — do not overwrite.
			result.Skipped++
			continue
		}

		// Role does not exist — seed it with provenance metadata.
		// TODO: When role inheritance is supported in RBAC, also seed role.Inherits.
		metadata := map[string]string{
			"source":     "generated",
			"service":    serviceName,
			"managed":    "seed",
			"seeded_at":  time.Now().UTC().Format(time.RFC3339),
		}
		if err := store.CreateRole(ctx, role.Name, role.Actions, metadata); err != nil {
			slog.Error("policy: seed: failed to create role",
				"role", role.Name, "error", err)
			result.Failed++
			continue
		}

		slog.Info("policy: seeded missing role", "role", role.Name, "actions", len(role.Actions))
		result.Seeded++
	}

	return result, nil
}

// SeedClusterRoles loads cluster-roles.json and seeds any missing roles into RBAC.
// Cluster roles use the same non-destructive seeding rule as service roles.
func SeedClusterRoles(ctx context.Context, store RoleStore, force bool) (*SeedResult, error) {
	roles, fromFile, _ := LoadClusterRoles()
	if !fromFile || len(roles) == 0 {
		slog.Warn("policy: seed: no cluster-roles.json found")
		return &SeedResult{}, nil
	}

	result := &SeedResult{}
	for roleName, actions := range roles {
		exists, err := store.RoleExists(ctx, roleName)
		if err != nil {
			slog.Warn("policy: seed: failed to check cluster role existence",
				"role", roleName, "error", err)
			result.Failed++
			continue
		}

		if exists && !force {
			result.Skipped++
			continue
		}

		metadata := map[string]string{
			"source":    "cluster-roles",
			"managed":   "seed",
			"seeded_at": time.Now().UTC().Format(time.RFC3339),
		}
		if err := store.CreateRole(ctx, roleName, actions, metadata); err != nil {
			slog.Error("policy: seed: failed to create cluster role",
				"role", roleName, "error", err)
			result.Failed++
			continue
		}

		slog.Info("policy: seeded cluster role", "role", roleName, "actions", len(actions))
		result.Seeded++
	}

	return result, nil
}

// Merge adds the counts from another SeedResult into this one.
func (r *SeedResult) Merge(other *SeedResult) {
	if other == nil {
		return
	}
	r.Seeded += other.Seeded
	r.Skipped += other.Skipped
	r.Failed += other.Failed
}
