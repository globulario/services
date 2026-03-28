package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	rbacpb "github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/repository/repository_client"
	Utility "github.com/globulario/utility"
)

// ── Cobra commands ──────────────────────────────────────────────────────────

var namespaceCmd = &cobra.Command{
	Use:   "namespace",
	Short: "Manage publisher namespaces",
}

var namespaceClaimCmd = &cobra.Command{
	Use:   "claim <name>",
	Short: "Claim a publisher namespace (you become the owner)",
	Long: `Claim a publisher namespace. The caller becomes the owner with namespace:admin role.
Use --org to assign ownership to an organization instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runNamespaceClaim,
}

var namespaceGrantCmd = &cobra.Command{
	Use:   "grant <namespace>",
	Short: "Grant a role on a namespace to a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceGrant,
}

var namespaceRevokeCmd = &cobra.Command{
	Use:   "revoke <namespace>",
	Short: "Revoke a user's access to a namespace",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceRevoke,
}

var namespaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List namespaces you own or have access to",
	RunE:    runNamespaceList,
}

var namespaceInfoCmd = &cobra.Command{
	Use:     "info <name>",
	Aliases: []string{"get"},
	Short:   "Show ownership, collaborators, and roles for a namespace",
	Args:    cobra.ExactArgs(1),
	RunE:    runNamespaceInfo,
}

// ── Flags ───────────────────────────────────────────────────────────────────

var (
	nsClaimOrg    string
	nsGrantTo     string
	nsGrantRole   string
	nsRevokeFrom  string
)

func init() {
	namespaceClaimCmd.Flags().StringVar(&nsClaimOrg, "org", "", "Organization that owns the namespace")

	namespaceGrantCmd.Flags().StringVar(&nsGrantTo, "to", "", "User to grant access to (required)")
	namespaceGrantCmd.Flags().StringVar(&nsGrantRole, "role", "namespace:publisher", "Role: namespace:viewer, namespace:publisher, namespace:admin")
	_ = namespaceGrantCmd.MarkFlagRequired("to")

	namespaceRevokeCmd.Flags().StringVar(&nsRevokeFrom, "from", "", "User to revoke access from (required)")
	_ = namespaceRevokeCmd.MarkFlagRequired("from")

	namespaceCmd.AddCommand(namespaceClaimCmd)
	namespaceCmd.AddCommand(namespaceGrantCmd)
	namespaceCmd.AddCommand(namespaceRevokeCmd)
	namespaceCmd.AddCommand(namespaceListCmd)
	namespaceCmd.AddCommand(namespaceInfoCmd)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func getRbacClient() (*rbac_client.Rbac_Client, error) {
	address, _ := config.GetAddress()
	if address == "" {
		address = "localhost"
	}
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, fmt.Errorf("connect to RBAC service: %w", err)
	}
	return client.(*rbac_client.Rbac_Client), nil
}

var cliReservedPrefixes = []string{"globular", "system", "core", "internal", "admin"}

func isReservedNS(ns string) bool {
	for _, p := range cliReservedPrefixes {
		if ns == p || strings.HasPrefix(ns, p+".") || strings.HasPrefix(ns, p+"-") || strings.HasPrefix(ns, p+"_") {
			return true
		}
	}
	return false
}

var validNSRoles = map[string]bool{
	"namespace:viewer":    true,
	"namespace:publisher": true,
	"namespace:admin":     true,
}

func roleToPermission(role string) string {
	switch role {
	case "namespace:viewer":
		return "read"
	case "namespace:publisher":
		return "write"
	case "namespace:admin":
		return "admin"
	default:
		return "read"
	}
}

// extractSubjectFromToken decodes the JWT payload to get the caller's identity.
func extractSubjectFromToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode token payload: %w", err)
	}
	var claims struct {
		ID          string `json:"id"`
		PrincipalID string `json:"principal_id"`
		Subject     string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse token claims: %w", err)
	}
	if claims.ID != "" {
		return claims.ID, nil
	}
	if claims.PrincipalID != "" {
		return claims.PrincipalID, nil
	}
	return claims.Subject, nil
}

// appendUniqueRole adds a role to a slice if not already present.
func appendUniqueRole(roles []string, role string) []string {
	for _, r := range roles {
		if r == role {
			return roles
		}
	}
	return append(roles, role)
}

// removeRole removes a role from a slice.
func removeRole(roles []string, role string) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		if r != role {
			out = append(out, r)
		}
	}
	return out
}

// ── claim ───────────────────────────────────────────────────────────────────

func runNamespaceClaim(cmd *cobra.Command, args []string) error {
	nsName := strings.ToLower(strings.TrimSpace(args[0]))
	if nsName == "" {
		return fmt.Errorf("namespace name cannot be empty")
	}
	if len(nsName) > 128 {
		return fmt.Errorf("namespace %q exceeds maximum length of 128 characters", nsName)
	}
	for _, r := range nsName {
		if r > 127 {
			return fmt.Errorf("namespace %q contains non-ASCII characters", nsName)
		}
	}
	if isReservedNS(nsName) {
		return fmt.Errorf("namespace %q is reserved — only administrators can manage reserved namespaces", nsName)
	}

	token := rootCfg.token
	if token == "" {
		return fmt.Errorf("authentication required — run 'globular auth login' first")
	}

	subject, err := extractSubjectFromToken(token)
	if err != nil {
		return fmt.Errorf("extract identity: %w", err)
	}

	rbacClient, err := getRbacClient()
	if err != nil {
		return err
	}

	path := "/namespaces/" + nsName

	// Check if already claimed.
	perms, err := rbacClient.GetResourcePermissions(path)
	if err == nil && perms != nil && perms.Owners != nil && len(perms.Owners.Accounts) > 0 {
		fmt.Printf("Namespace %q already claimed (owned by: %s)\n", nsName, strings.Join(perms.Owners.Accounts, ", "))
		return nil
	}

	// Create the namespace resource and set caller as owner.
	if err := rbacClient.AddResourceOwner(token, path, "", "namespace", rbacpb.SubjectType_ACCOUNT); err != nil {
		return fmt.Errorf("claim namespace %q: %w", nsName, err)
	}

	// Bind namespace:admin role to the owner.
	binding, _ := rbacClient.GetRoleBinding(subject)
	roles := binding.GetRoles()
	roles = appendUniqueRole(roles, "namespace:admin")
	if err := rbacClient.SetRoleBinding(subject, roles); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: claimed namespace but failed to bind namespace:admin role: %v\n", err)
	}

	fmt.Printf("Namespace %q claimed by %s.\n", nsName, subject)

	// If --org is set, make the org a co-owner of the namespace.
	// RBAC's ValidateAccess resolves org membership transitively, so all
	// org members will be able to publish without individual grants.
	if nsClaimOrg != "" {
		if err := rbacClient.AddResourceOwner(token, path, nsClaimOrg, "namespace", rbacpb.SubjectType_ORGANIZATION); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: claimed namespace but failed to add org %q as owner: %v\n", nsClaimOrg, err)
		} else {
			fmt.Printf("  Organization %q set as namespace owner — members can publish.\n", nsClaimOrg)
		}
	}

	return nil
}

// ── grant ───────────────────────────────────────────────────────────────────

func runNamespaceGrant(cmd *cobra.Command, args []string) error {
	nsName := strings.TrimSpace(args[0])
	token := rootCfg.token
	if token == "" {
		return fmt.Errorf("authentication required")
	}

	if !validNSRoles[nsGrantRole] {
		return fmt.Errorf("invalid role %q — must be: namespace:viewer, namespace:publisher, or namespace:admin", nsGrantRole)
	}

	rbacClient, err := getRbacClient()
	if err != nil {
		return err
	}

	path := "/namespaces/" + nsName

	// 1. Set resource permission (enables ValidateAccess checks in repository).
	permName := roleToPermission(nsGrantRole)
	perm := &rbacpb.Permission{
		Name:     permName,
		Accounts: []string{nsGrantTo},
	}
	if err := rbacClient.SetResourcePermission(token, path, "namespace", perm, rbacpb.PermissionType_ALLOWED); err != nil {
		return fmt.Errorf("set resource permission: %w", err)
	}

	// 2. Bind the namespace role to the subject (enables role-based checks).
	binding, _ := rbacClient.GetRoleBinding(nsGrantTo)
	roles := binding.GetRoles()
	roles = appendUniqueRole(roles, nsGrantRole)
	if err := rbacClient.SetRoleBinding(nsGrantTo, roles); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: resource permission set but role binding failed: %v\n", err)
	}

	fmt.Printf("Granted %s to %q on namespace %q.\n", nsGrantRole, nsGrantTo, nsName)
	return nil
}

// ── revoke ──────────────────────────────────────────────────────────────────

func runNamespaceRevoke(cmd *cobra.Command, args []string) error {
	nsName := strings.TrimSpace(args[0])
	token := rootCfg.token
	if token == "" {
		return fmt.Errorf("authentication required")
	}

	rbacClient, err := getRbacClient()
	if err != nil {
		return err
	}

	path := "/namespaces/" + nsName

	// Remove all permission types for this user on the namespace.
	for _, permName := range []string{"read", "write", "admin"} {
		_ = rbacClient.DeleteResourcePermission(token, path, permName, rbacpb.PermissionType_ALLOWED)
	}

	// Remove all namespace roles from the subject's role binding.
	binding, _ := rbacClient.GetRoleBinding(nsRevokeFrom)
	roles := binding.GetRoles()
	for nsRole := range validNSRoles {
		roles = removeRole(roles, nsRole)
	}
	_ = rbacClient.SetRoleBinding(nsRevokeFrom, roles)

	fmt.Printf("Revoked access for %q on namespace %q.\n", nsRevokeFrom, nsName)
	return nil
}

// ── list ────────────────────────────────────────────────────────────────────

func runNamespaceList(cmd *cobra.Command, args []string) error {
	// Strategy: query RBAC for all resources of type "namespace" to get claimed
	// namespaces, then merge with publisher IDs from the repository catalog.
	rbacClient, err := getRbacClient()
	if err != nil {
		return err
	}

	type nsInfo struct {
		owners  []string
		claimed bool
	}
	namespaces := make(map[string]*nsInfo)

	// Source 1: RBAC resources of type "namespace".
	allPerms, err := rbacClient.GetResourcePermissionsByResourceType("namespace")
	if err == nil {
		for _, p := range allPerms {
			path := p.GetPath()
			ns := strings.TrimPrefix(path, "/namespaces/")
			if ns == "" || ns == path {
				continue
			}
			info := &nsInfo{claimed: true}
			if p.Owners != nil {
				info.owners = p.Owners.Accounts
			}
			namespaces[ns] = info
		}
	}

	// Source 2: Repository artifact publishers (may include unclaimed namespaces).
	address, _ := config.GetAddress()
	if address == "" {
		address = "localhost"
	}
	repoClient, err := repository_client.NewRepositoryService_Client(address, "repository.PackageRepository")
	if err == nil {
		defer repoClient.Close()
		artifacts, err := repoClient.ListArtifacts()
		if err == nil {
			for _, a := range artifacts {
				if a.GetRef() != nil && a.GetRef().GetPublisherId() != "" {
					pub := a.GetRef().GetPublisherId()
					if _, exists := namespaces[pub]; !exists {
						namespaces[pub] = &nsInfo{claimed: false}
					}
				}
			}
		}
	}

	if len(namespaces) == 0 {
		fmt.Println("No namespaces found.")
		return nil
	}

	fmt.Printf("%-30s  %-8s  %s\n", "NAMESPACE", "STATUS", "OWNERS")
	fmt.Printf("%-30s  %-8s  %s\n", strings.Repeat("─", 30), strings.Repeat("─", 8), strings.Repeat("─", 20))
	for ns, info := range namespaces {
		status := "unclaim"
		if info.claimed {
			status = "claimed"
		}
		owners := "—"
		if len(info.owners) > 0 {
			owners = strings.Join(info.owners, ", ")
		}
		fmt.Printf("%-30s  %-8s  %s\n", ns, status, owners)
	}
	return nil
}

// ── info ────────────────────────────────────────────────────────────────────

func runNamespaceInfo(cmd *cobra.Command, args []string) error {
	nsName := strings.TrimSpace(args[0])

	rbacClient, err := getRbacClient()
	if err != nil {
		return err
	}

	path := "/namespaces/" + nsName
	perms, err := rbacClient.GetResourcePermissions(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Namespace %q not found or not claimed.\n", nsName)
		return nil
	}

	fmt.Printf("Namespace: %s\n", nsName)

	// Owners
	if perms.Owners != nil && len(perms.Owners.Accounts) > 0 {
		fmt.Printf("  Owners: %s\n", strings.Join(perms.Owners.Accounts, ", "))
	}

	// Collaborators — collect subjects and their permissions
	type collaborator struct {
		subject string
		perms   []string
	}
	collabs := make(map[string]*collaborator)

	for _, perm := range perms.Allowed {
		for _, a := range perm.Accounts {
			c, ok := collabs[a]
			if !ok {
				c = &collaborator{subject: a}
				collabs[a] = c
			}
			c.perms = append(c.perms, perm.Name)
		}
		for _, g := range perm.Groups {
			key := g + " (group)"
			c, ok := collabs[key]
			if !ok {
				c = &collaborator{subject: key}
				collabs[key] = c
			}
			c.perms = append(c.perms, perm.Name)
		}
	}

	if len(collabs) > 0 {
		fmt.Println("  Collaborators:")
		for _, c := range collabs {
			// Map permissions back to role names for display
			role := permissionsToRole(c.perms)
			fmt.Printf("    %-30s  %s\n", c.subject, role)
		}
	}

	// Package count from repository
	address, _ := config.GetAddress()
	if address == "" {
		address = "localhost"
	}
	repoClient, err := repository_client.NewRepositoryService_Client(address, "repository.PackageRepository")
	if err == nil {
		defer repoClient.Close()
		artifacts, err := repoClient.ListArtifacts()
		if err == nil {
			count := 0
			for _, a := range artifacts {
				if a.GetRef() != nil && a.GetRef().GetPublisherId() == nsName {
					count++
				}
			}
			fmt.Printf("  Artifacts: %d\n", count)
		}
	}

	return nil
}

func permissionsToRole(perms []string) string {
	for _, p := range perms {
		if p == "admin" {
			return "namespace:admin"
		}
	}
	for _, p := range perms {
		if p == "write" {
			return "namespace:publisher"
		}
	}
	for _, p := range perms {
		if p == "read" {
			return "namespace:viewer"
		}
	}
	return strings.Join(perms, ", ")
}
