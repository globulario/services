// rbac_cmds.go: CLI commands for role-binding management.
//
//   globular rbac bind   --subject <id> --role <role>
//   globular rbac unbind --subject <id> --role <role>
//   globular rbac list-bindings [--subject <id>]
//   globular rbac seed   (seeds built-in SA bindings during Day-0)

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
)

// Default RBAC service port (matches rbac_server default).
const defaultRbacPort = 10000

// defaultResourcePort is declared in pkg_cmds.go (10011).

// resolveRbacAddr discovers the RBAC service endpoint.
// Uses the same multi-strategy discovery as resolveAuthAddr (etcd → local files → fallback).
// In a cluster, randomly picks one of the running instances for load balancing.
func resolveRbacAddr() string {
	return config.ResolveServiceAddr(
		"rbac.RbacService",
		fmt.Sprintf("localhost:%d", defaultRbacPort),
	)
}

func resolveResourceAddr() string {
	return config.ResolveServiceAddr(
		"resource.ResourceService",
		fmt.Sprintf("localhost:%d", defaultResourcePort),
	)
}

var (
	rbacCmd = &cobra.Command{
		Use:   "rbac",
		Short: "Manage role bindings",
	}

	rbacBindSubject string
	rbacBindRole    string

	rbacBindCmd = &cobra.Command{
		Use:   "bind",
		Short: "Bind a role to a subject (appends; does not replace existing roles)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rbacBindSubject == "" {
				return errors.New("--subject is required")
			}
			if rbacBindRole == "" {
				return errors.New("--role is required")
			}

			addr := resolveRbacAddr()
			cc, err := dialGRPC(addr)
			if err != nil {
				return err
			}
			defer cc.Close()

			client := rbacpb.NewRbacServiceClient(cc)
			ctx := ctxWithTimeout()

			// Fetch existing roles.
			getResp, err := client.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: rbacBindSubject})
			if err != nil {
				return fmt.Errorf("GetRoleBinding: %w", err)
			}

			roles := getResp.GetBinding().GetRoles()
			for _, r := range roles {
				if r == rbacBindRole {
					fmt.Printf("subject %q already has role %q\n", rbacBindSubject, rbacBindRole)
					return nil
				}
			}
			roles = append(roles, rbacBindRole)

			_, err = client.SetRoleBinding(ctxWithTimeout(), &rbacpb.SetRoleBindingRqst{
				Binding: &rbacpb.RoleBinding{Subject: rbacBindSubject, Roles: roles},
			})
			if err != nil {
				return fmt.Errorf("SetRoleBinding: %w", err)
			}

			fmt.Printf("bound role %q to subject %q (total roles: %s)\n",
				rbacBindRole, rbacBindSubject, strings.Join(roles, ", "))
			return nil
		},
	}

	rbacUnbindCmd = &cobra.Command{
		Use:   "unbind",
		Short: "Remove a role from a subject",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rbacBindSubject == "" {
				return errors.New("--subject is required")
			}
			if rbacBindRole == "" {
				return errors.New("--role is required")
			}

			addr := resolveRbacAddr()
			cc, err := dialGRPC(addr)
			if err != nil {
				return err
			}
			defer cc.Close()

			client := rbacpb.NewRbacServiceClient(cc)

			getResp, err := client.GetRoleBinding(ctxWithTimeout(), &rbacpb.GetRoleBindingRqst{Subject: rbacBindSubject})
			if err != nil {
				return fmt.Errorf("GetRoleBinding: %w", err)
			}

			oldRoles := getResp.GetBinding().GetRoles()
			newRoles := make([]string, 0, len(oldRoles))
			found := false
			for _, r := range oldRoles {
				if r == rbacBindRole {
					found = true
					continue
				}
				newRoles = append(newRoles, r)
			}

			if !found {
				fmt.Printf("subject %q does not have role %q\n", rbacBindSubject, rbacBindRole)
				return nil
			}

			_, err = client.SetRoleBinding(ctxWithTimeout(), &rbacpb.SetRoleBindingRqst{
				Binding: &rbacpb.RoleBinding{Subject: rbacBindSubject, Roles: newRoles},
			})
			if err != nil {
				return fmt.Errorf("SetRoleBinding: %w", err)
			}

			fmt.Printf("removed role %q from subject %q (remaining: %s)\n",
				rbacBindRole, rbacBindSubject, strings.Join(newRoles, ", "))
			return nil
		},
	}

	rbacListSubject string

	rbacListBindingsCmd = &cobra.Command{
		Use:   "list-bindings",
		Short: "List role bindings (all subjects, or a specific subject with --subject)",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := resolveRbacAddr()
			cc, err := dialGRPC(addr)
			if err != nil {
				return err
			}
			defer cc.Close()

			client := rbacpb.NewRbacServiceClient(cc)

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SUBJECT\tROLES")

			if rbacListSubject != "" {
				resp, err := client.GetRoleBinding(ctxWithTimeout(), &rbacpb.GetRoleBindingRqst{Subject: rbacListSubject})
				if err != nil {
					return fmt.Errorf("GetRoleBinding: %w", err)
				}
				b := resp.GetBinding()
				fmt.Fprintf(w, "%s\t%s\n", b.GetSubject(), strings.Join(b.GetRoles(), ", "))
			} else {
				stream, err := client.ListRoleBindings(ctxWithTimeout(), &rbacpb.ListRoleBindingsRqst{})
				if err != nil {
					return fmt.Errorf("ListRoleBindings: %w", err)
				}
				for {
					rsp, err := stream.Recv()
					if err != nil {
						break
					}
					b := rsp.GetBinding()
					fmt.Fprintf(w, "%s\t%s\n", b.GetSubject(), strings.Join(b.GetRoles(), ", "))
				}
			}

			return w.Flush()
		},
	}

	rbacSeedDryRun bool
	rbacSeedForce  bool

	rbacSeedCmd = &cobra.Command{
		Use:   "seed",
		Short: "Seed role bindings and cluster/service roles (Day-0 and post-restore)",
		Long: `Seeds three categories of RBAC data:

  1. Service-account role bindings:
     globular-controller  → [globular-controller-sa]
     globular-node-agent  → [globular-node-agent-sa]
     globular-gateway     → [globular-admin]

  2. Cluster roles from cluster-roles.json (globular-admin, globular-operator, etc.)

  3. Per-service roles from generated policy files (role:file.viewer, etc.)

Flags:
  --dry-run   Show what would be seeded without making changes
  --force     Overwrite existing roles (default: preserve)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			saBindings := []struct {
				subject string
				roles   []string
			}{
				{"globular-controller", []string{"globular-controller-sa"}},
				{"globular-node-agent", []string{"globular-node-agent-sa"}},
				{"globular-gateway", []string{"globular-admin"}},
			}

			// Connect to RBAC service
			rbacAddr := resolveRbacAddr()
			rbacCC, err := dialGRPC(rbacAddr)
			if err != nil {
				return fmt.Errorf("connect to RBAC service at %s: %w", rbacAddr, err)
			}
			defer rbacCC.Close()
			rbacClient := rbacpb.NewRbacServiceClient(rbacCC)

			// Connect to Resource service (for admin UI role entities)
			var resClient resourcepb.ResourceServiceClient
			resAddr := resolveResourceAddr()
			resCC, err := dialGRPC(resAddr)
			if err != nil {
				slog.Warn("rbac seed: cannot connect to resource service — roles will be seeded in RBAC only (not visible in admin UI)",
					"addr", resAddr, "error", err)
			} else {
				defer resCC.Close()
				resClient = resourcepb.NewResourceServiceClient(resCC)
			}

			store := &rbacClientStore{rbac: rbacClient, resource: resClient}

			// 1. Seed SA role bindings
			fmt.Println("=== Service Account Bindings ===")
			for _, s := range saBindings {
				if rbacSeedDryRun {
					fmt.Printf("[dry-run] would seed %q → %v\n", s.subject, s.roles)
					continue
				}
				_, err := rbacClient.SetRoleBinding(ctxWithTimeout(), &rbacpb.SetRoleBindingRqst{
					Binding: &rbacpb.RoleBinding{Subject: s.subject, Roles: s.roles},
				})
				if err != nil {
					return fmt.Errorf("seed %q: %w", s.subject, err)
				}
				fmt.Printf("seeded %q → %v\n", s.subject, s.roles)
			}

			// 2. Seed cluster roles from cluster-roles.json
			fmt.Println("\n=== Cluster Roles ===")
			clusterRoles, fromFile, _ := policy.LoadClusterRoles()
			if !fromFile {
				fmt.Println("(no cluster-roles.json found, skipping)")
			} else {
				if rbacSeedDryRun {
					for roleName, actions := range clusterRoles {
						fmt.Printf("[dry-run] would seed cluster role %q (%d actions)\n", roleName, len(actions))
					}
				} else {
					result, err := policy.SeedClusterRoles(ctxWithTimeout(), store, rbacSeedForce)
					if err != nil {
						return fmt.Errorf("seed cluster roles: %w", err)
					}
					fmt.Printf("cluster roles: %d seeded, %d skipped, %d failed\n",
						result.Seeded, result.Skipped, result.Failed)
				}
			}

			// 3. Seed per-service roles by discovering installed services
			fmt.Println("\n=== Service Roles ===")
			serviceNames := discoverInstalledServices()
			if len(serviceNames) == 0 {
				fmt.Println("(no installed services discovered, skipping)")
			} else {
				total := &policy.SeedResult{}
				for _, svc := range serviceNames {
					if rbacSeedDryRun {
						roles, fromFile, _ := policy.LoadServiceRoles(svc)
						if fromFile && len(roles) > 0 {
							for _, r := range roles {
								fmt.Printf("[dry-run] would seed service role %q (%d actions)\n", r.Name, len(r.Actions))
							}
						}
						continue
					}
					result, err := policy.SeedServiceRoles(ctxWithTimeout(), svc, store)
					if err != nil {
						fmt.Printf("warning: failed to seed roles for %s: %v\n", svc, err)
						continue
					}
					total.Merge(result)
				}
				if !rbacSeedDryRun {
					fmt.Printf("service roles: %d seeded, %d skipped, %d failed\n",
						total.Seeded, total.Skipped, total.Failed)
				}
			}

			fmt.Println("\nDone.")
			return nil
		},
	}
)

// rbacClientStore adapts gRPC clients to the policy.RoleStore interface.
// It writes to both the RBAC service (role bindings for authorization) and
// the Resource service (role entities for the admin UI).
type rbacClientStore struct {
	rbac     rbacpb.RbacServiceClient
	resource resourcepb.ResourceServiceClient
}

func (s *rbacClientStore) RoleExists(ctx context.Context, roleName string) (bool, error) {
	// Check the resource service first — this is the canonical role store
	// that the admin UI reads from.
	if s.resource != nil {
		stream, err := s.resource.GetRoles(ctx, &resourcepb.GetRolesRqst{
			Query: fmt.Sprintf(`{"$or":[{"_id":"%s"},{"id":"%s"}]}`, roleName, roleName),
		})
		if err == nil {
			for {
				rsp, err := stream.Recv()
				if err != nil {
					break
				}
				if len(rsp.GetRoles()) > 0 {
					return true, nil
				}
			}
		}
	}

	// Fallback: check RBAC role binding
	resp, err := s.rbac.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: roleName})
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return len(resp.GetBinding().GetRoles()) > 0, nil
}

func (s *rbacClientStore) CreateRole(ctx context.Context, roleName string, actions []string, metadata map[string]string) error {
	// 1. Create the role binding in RBAC service (for authorization checks)
	_, err := s.rbac.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: roleName, Roles: actions},
	})
	if err != nil {
		return fmt.Errorf("set role binding: %w", err)
	}

	// 2. Create the role entity in Resource service (for admin UI visibility)
	if s.resource != nil {
		desc := metadata["source"]
		if desc == "" {
			desc = "seeded"
		}
		role := &resourcepb.Role{
			Id:          roleName,
			Name:        roleName,
			Description: fmt.Sprintf("Cluster role (%s)", desc),
			Actions:     actions,
		}
		_, err := s.resource.CreateRole(ctx, &resourcepb.CreateRoleRqst{Role: role})
		if err != nil {
			// Log but don't fail — the RBAC binding (authorization) is the critical part.
			slog.Warn("rbac seed: created role binding but failed to create resource role",
				"role", roleName, "error", err)
		}
	}

	return nil
}

// discoverInstalledServices returns service names found in the generated policy directory.
// It scans for directories under generated/policy/ that contain permissions files.
func discoverInstalledServices() []string {
	// Well-known service names that may have generated policy files.
	candidates := []string{
		"authentication", "backup_manager", "blog", "catalog",
		"conversation", "discovery", "dns", "echo", "event",
		"file", "ldap", "log", "mail", "media", "monitoring",
		"persistence", "rbac", "repository", "resource", "search",
		"spc", "sql", "storage", "title", "torrent",
	}
	var found []string
	for _, svc := range candidates {
		roles, fromFile, _ := policy.LoadServiceRoles(svc)
		if fromFile && len(roles) > 0 {
			found = append(found, svc)
		}
	}
	return found
}

func init() {
	rbacBindCmd.Flags().StringVar(&rbacBindSubject, "subject", "", "Principal ID (user, SA, or cert CN)")
	rbacBindCmd.Flags().StringVar(&rbacBindRole, "role", "", "Role name (e.g. globular-admin)")

	rbacUnbindCmd.Flags().StringVar(&rbacBindSubject, "subject", "", "Principal ID")
	rbacUnbindCmd.Flags().StringVar(&rbacBindRole, "role", "", "Role name to remove")

	rbacListBindingsCmd.Flags().StringVar(&rbacListSubject, "subject", "", "Filter by subject (omit to list all)")

	rbacSeedCmd.Flags().BoolVar(&rbacSeedDryRun, "dry-run", false, "Show what would be seeded without making changes")
	rbacSeedCmd.Flags().BoolVar(&rbacSeedForce, "force", false, "Overwrite existing roles (default: preserve)")

	rbacCmd.AddCommand(rbacBindCmd, rbacUnbindCmd, rbacListBindingsCmd, rbacSeedCmd)
	rootCmd.AddCommand(rbacCmd)
}
