package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

const installPolicyPath = "/var/lib/globular/config/install-policy.json"

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage cluster install policy for artifact resolution",
}

var policySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set or update the cluster install policy",
	RunE:  runPolicySet,
}

var policyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the current install policy",
	RunE:  runPolicyGet,
}

var policyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove the install policy",
	RunE:  runPolicyDelete,
}

var (
	policyVerifiedOnly    bool
	policyAllowNamespaces string
	policyBlockNamespaces string
	policyBlockDeprecated bool
	policyBlockYanked     bool
)

func init() {
	policySetCmd.Flags().BoolVar(&policyVerifiedOnly, "verified-only", false, "Only allow artifacts from claimed namespaces")
	policySetCmd.Flags().StringVar(&policyAllowNamespaces, "allow-namespace", "", "Comma-separated list of allowed namespaces")
	policySetCmd.Flags().StringVar(&policyBlockNamespaces, "block-namespace", "", "Comma-separated list of blocked namespaces")
	policySetCmd.Flags().BoolVar(&policyBlockDeprecated, "block-deprecated", false, "Skip deprecated artifacts in resolution")
	policySetCmd.Flags().BoolVar(&policyBlockYanked, "block-yanked", true, "Block yanked artifacts (default: true)")

	policyCmd.AddCommand(policySetCmd)
	policyCmd.AddCommand(policyGetCmd)
	policyCmd.AddCommand(policyDeleteCmd)

	rootCmd.AddCommand(policyCmd)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// requirePolicyToken checks that a valid token is available for policy mutations.
// Policy set/delete are governed operations that require authentication.
func requirePolicyToken() (string, error) {
	token := rootCfg.token
	if token == "" {
		token = os.Getenv("GLOBULAR_TOKEN")
	}
	if token == "" {
		return "", fmt.Errorf("authentication required for policy operations: run 'globular auth login' or provide --token")
	}
	return token, nil
}

func runPolicySet(cmd *cobra.Command, args []string) error {
	token, err := requirePolicyToken()
	if err != nil {
		return err
	}
	_ = token

	policy := &cluster_controllerpb.InstallPolicySpec{
		VerifiedPublishersOnly: policyVerifiedOnly,
		AllowedNamespaces:      splitCSV(policyAllowNamespaces),
		BlockedNamespaces:      splitCSV(policyBlockNamespaces),
		BlockDeprecated:        policyBlockDeprecated,
		BlockYanked:            policyBlockYanked,
	}

	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	// Ensure config directory exists.
	if err := os.MkdirAll(filepath.Dir(installPolicyPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(installPolicyPath, data, 0o644); err != nil {
		return fmt.Errorf("write policy: %w", err)
	}

	fmt.Printf("Install policy saved to %s:\n%s\n", installPolicyPath, string(data))
	return nil
}

func runPolicyGet(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(installPolicyPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No install policy configured.")
			return nil
		}
		return fmt.Errorf("read policy: %w", err)
	}
	fmt.Printf("Install policy (%s):\n%s\n", installPolicyPath, string(data))
	return nil
}

func runPolicyDelete(cmd *cobra.Command, args []string) error {
	token, err := requirePolicyToken()
	if err != nil {
		return err
	}
	_ = token

	if err := os.Remove(installPolicyPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No install policy to remove.")
			return nil
		}
		return fmt.Errorf("delete policy: %w", err)
	}
	fmt.Println("Install policy removed.")
	return nil
}
