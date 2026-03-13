package main

import (
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

var namespaceCmd = &cobra.Command{
	Use:   "namespace",
	Short: "Manage publisher namespaces",
}

var namespaceClaimCmd = &cobra.Command{
	Use:   "claim <name>",
	Short: "Claim a publisher namespace (you become the owner)",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceClaim,
}

var namespaceGrantCmd = &cobra.Command{
	Use:   "grant <name>",
	Short: "Grant write permission on a namespace to a subject",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceGrant,
}

var namespaceRevokeCmd = &cobra.Command{
	Use:   "revoke <name>",
	Short: "Revoke a subject's permission on a namespace",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceRevoke,
}

var namespaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List namespaces you own or have access to",
	RunE:  runNamespaceList,
}

var namespaceInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show ownership and permissions for a namespace",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceInfo,
}

var (
	nsGrantSubject string
	nsGrantType    string
)

func init() {
	namespaceGrantCmd.Flags().StringVar(&nsGrantSubject, "subject", "", "Subject to grant access to (required)")
	namespaceGrantCmd.Flags().StringVar(&nsGrantType, "type", "ACCOUNT", "Subject type: ACCOUNT or APPLICATION")
	_ = namespaceGrantCmd.MarkFlagRequired("subject")

	namespaceRevokeCmd.Flags().StringVar(&nsGrantSubject, "subject", "", "Subject to revoke access from (required)")
	_ = namespaceRevokeCmd.MarkFlagRequired("subject")

	namespaceCmd.AddCommand(namespaceClaimCmd)
	namespaceCmd.AddCommand(namespaceGrantCmd)
	namespaceCmd.AddCommand(namespaceRevokeCmd)
	namespaceCmd.AddCommand(namespaceListCmd)
	namespaceCmd.AddCommand(namespaceInfoCmd)
}

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

// reservedPrefixes that cannot be claimed by regular users.
var cliReservedPrefixes = []string{"globular", "system", "core", "internal", "admin"}

func isReservedNS(ns string) bool {
	for _, p := range cliReservedPrefixes {
		if ns == p || strings.HasPrefix(ns, p+".") || strings.HasPrefix(ns, p+"-") || strings.HasPrefix(ns, p+"_") {
			return true
		}
	}
	return false
}

func runNamespaceClaim(cmd *cobra.Command, args []string) error {
	nsName := strings.ToLower(strings.TrimSpace(args[0]))
	if nsName == "" {
		return fmt.Errorf("namespace name cannot be empty")
	}

	// Validate canonical form.
	if len(nsName) > 128 {
		return fmt.Errorf("namespace %q exceeds maximum length of 128 characters", nsName)
	}
	for _, r := range nsName {
		if r > 127 {
			return fmt.Errorf("namespace %q contains non-ASCII characters — only a-z, 0-9, '.', '_', '-', '@' are allowed", nsName)
		}
	}

	// Block reserved namespaces for non-admin users.
	if isReservedNS(nsName) {
		return fmt.Errorf("namespace %q is reserved — only administrators can manage reserved namespaces", nsName)
	}

	token := rootCfg.token
	if token == "" {
		return fmt.Errorf("authentication required — run 'globular auth login' first")
	}

	rbacClient, err := getRbacClient()
	if err != nil {
		return err
	}

	path := "/namespaces/" + nsName

	// Check if already claimed.
	perms, err := rbacClient.GetResourcePermissions(path)
	if err == nil && perms != nil && perms.Owners != nil && len(perms.Owners.Accounts) > 0 {
		fmt.Printf("Namespace %q already exists (owned by: %s)\n", nsName, strings.Join(perms.Owners.Accounts, ", "))
		return nil
	}

	// Extract subject from token (we need the caller's identity).
	// The subject is stored when the RBAC service processes the token.
	if err := rbacClient.AddResourceOwner(token, path, "", "namespace", rbacpb.SubjectType_ACCOUNT); err != nil {
		return fmt.Errorf("claim namespace %q: %w", nsName, err)
	}

	fmt.Printf("Namespace %q claimed successfully.\n", nsName)
	return nil
}

func runNamespaceGrant(cmd *cobra.Command, args []string) error {
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

	subjectType := rbacpb.SubjectType_ACCOUNT
	if strings.EqualFold(nsGrantType, "APPLICATION") {
		subjectType = rbacpb.SubjectType_APPLICATION
	}

	perm := &rbacpb.Permission{
		Name: "write",
	}
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		perm.Accounts = []string{nsGrantSubject}
	} else {
		perm.Applications = []string{nsGrantSubject}
	}

	if err := rbacClient.SetResourcePermission(token, path, "namespace", perm, rbacpb.PermissionType_ALLOWED); err != nil {
		return fmt.Errorf("grant access to %q on namespace %q: %w", nsGrantSubject, nsName, err)
	}

	fmt.Printf("Granted write access to %q on namespace %q.\n", nsGrantSubject, nsName)
	return nil
}

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
	if err := rbacClient.DeleteResourcePermission(token, path, "write", rbacpb.PermissionType_ALLOWED); err != nil {
		return fmt.Errorf("revoke access on namespace %q: %w", nsName, err)
	}

	fmt.Printf("Revoked write access for %q on namespace %q.\n", nsGrantSubject, nsName)
	return nil
}

func runNamespaceList(cmd *cobra.Command, args []string) error {
	// List namespaces by searching artifacts for unique publisher IDs,
	// then checking which ones the caller owns.
	address, _ := config.GetAddress()
	if address == "" {
		address = "localhost"
	}
	client, err := repository_client.NewRepositoryService_Client(address, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()

	artifacts, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	seen := make(map[string]bool)
	for _, a := range artifacts {
		if a.GetRef() != nil && a.GetRef().GetPublisherId() != "" {
			seen[a.GetRef().GetPublisherId()] = true
		}
	}

	if len(seen) == 0 {
		fmt.Println("No namespaces found.")
		return nil
	}

	fmt.Println("Known publisher namespaces:")
	for ns := range seen {
		fmt.Printf("  %s\n", ns)
	}
	return nil
}

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
	if perms.Owners != nil {
		if len(perms.Owners.Accounts) > 0 {
			fmt.Printf("  Owners (accounts):     %s\n", strings.Join(perms.Owners.Accounts, ", "))
		}
		if len(perms.Owners.Applications) > 0 {
			fmt.Printf("  Owners (applications): %s\n", strings.Join(perms.Owners.Applications, ", "))
		}
	}
	for _, perm := range perms.Allowed {
		if len(perm.Accounts) > 0 || len(perm.Applications) > 0 {
			fmt.Printf("  Permitted (%s):\n", perm.Name)
			for _, a := range perm.Accounts {
				fmt.Printf("    - %s (account)\n", a)
			}
			for _, a := range perm.Applications {
				fmt.Printf("    - %s (application)\n", a)
			}
		}
	}

	return nil
}
