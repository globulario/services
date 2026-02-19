// rbac_cmds.go: CLI commands for role-binding management.
//
//   globular rbac bind   --subject <id> --role <role>
//   globular rbac unbind --subject <id> --role <role>
//   globular rbac list-bindings [--subject <id>]
//   globular rbac seed   (seeds built-in SA bindings during Day-0)

package main

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
)

// Default RBAC service port (matches rbac_server default).
const defaultRbacPort = 10000

// resolveRbacAddr discovers the RBAC service endpoint from etcd with a fallback.
func resolveRbacAddr() string {
	svc, err := config.ResolveService("rbac.RbacService")
	if err == nil && svc != nil {
		var port int
		switch p := svc["Port"].(type) {
		case int:
			port = p
		case float64:
			port = int(p)
		}
		if port > 0 {
			return fmt.Sprintf("localhost:%d", port)
		}
	}
	return fmt.Sprintf("localhost:%d", defaultRbacPort)
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

	rbacSeedCmd = &cobra.Command{
		Use:   "seed",
		Short: "Seed built-in service-account role bindings (run once during Day-0)",
		Long: `Seeds the following role bindings:
  globular-controller  → [globular-controller-sa]
  globular-node-agent  → [globular-node-agent-sa]
  globular-gateway     → [globular-admin]

These match the DefaultServiceAccountNames in the security package.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			seeds := []struct {
				subject string
				roles   []string
			}{
				{"globular-controller", []string{"globular-controller-sa"}},
				{"globular-node-agent", []string{"globular-node-agent-sa"}},
				{"globular-gateway", []string{"globular-admin"}},
			}

			addr := resolveRbacAddr()
			cc, err := dialGRPC(addr)
			if err != nil {
				return err
			}
			defer cc.Close()

			client := rbacpb.NewRbacServiceClient(cc)

			for _, s := range seeds {
				_, err := client.SetRoleBinding(ctxWithTimeout(), &rbacpb.SetRoleBindingRqst{
					Binding: &rbacpb.RoleBinding{Subject: s.subject, Roles: s.roles},
				})
				if err != nil {
					return fmt.Errorf("seed %q: %w", s.subject, err)
				}
				fmt.Printf("seeded %q → %v\n", s.subject, s.roles)
			}

			fmt.Println("Done seeding built-in role bindings.")
			return nil
		},
	}
)

func init() {
	rbacBindCmd.Flags().StringVar(&rbacBindSubject, "subject", "", "Principal ID (user, SA, or cert CN)")
	rbacBindCmd.Flags().StringVar(&rbacBindRole, "role", "", "Role name (e.g. globular-admin)")

	rbacUnbindCmd.Flags().StringVar(&rbacBindSubject, "subject", "", "Principal ID")
	rbacUnbindCmd.Flags().StringVar(&rbacBindRole, "role", "", "Role name to remove")

	rbacListBindingsCmd.Flags().StringVar(&rbacListSubject, "subject", "", "Filter by subject (omit to list all)")

	rbacCmd.AddCommand(rbacBindCmd, rbacUnbindCmd, rbacListBindingsCmd, rbacSeedCmd)
	rootCmd.AddCommand(rbacCmd)
}
