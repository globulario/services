package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

var (
	nodeReconcileDryRun bool
	nodeReconcileJSON   bool
	nodeReconcileNodeID string
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Node management commands",
}

var nodeReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Trigger node reconciliation",
	Long: `Trigger reconciliation for a node to align its actual state with desired state.

Reconciliation ensures that:
  - All desired services are running
  - Network configuration matches cluster spec
  - TLS certificates are valid
  - Service configurations are up to date

Examples:
  globular node reconcile --node-id node-123
  globular node reconcile --node-id node-123 --dry-run
  globular node reconcile --node-id node-123 --json
`,
	RunE: runNodeReconcile,
}

func init() {
	nodeReconcileCmd.Flags().StringVar(&nodeReconcileNodeID, "node-id", "", "Target node ID (required)")
	nodeReconcileCmd.Flags().BoolVar(&nodeReconcileDryRun, "dry-run", false, "Show what would be reconciled without executing")
	nodeReconcileCmd.Flags().BoolVar(&nodeReconcileJSON, "json", false, "Output result in JSON format")
	nodeReconcileCmd.MarkFlagRequired("node-id")

	nodeCmd.AddCommand(nodeReconcileCmd)
	rootCmd.AddCommand(nodeCmd)
}

func runNodeReconcile(cmd *cobra.Command, args []string) error {
	if nodeReconcileNodeID == "" {
		return errors.New("--node-id is required")
	}

	if nodeReconcileDryRun {
		return runNodeReconcileDryRun()
	}

	return runNodeReconcileApply()
}

func runNodeReconcileDryRun() error {
	// In dry-run mode, we fetch the node plan without executing it
	cc, err := controllerClient()
	if err != nil {
		return err
	}
	defer cc.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	// Plan display removed.
	_ = client
	return fmt.Errorf("plan system removed — use workflow-native release pipeline")
	fmt.Println("Run without --dry-run to execute the plan.")

	return nil
}

func runNodeReconcileApply() error {
	cc, err := controllerClient()
	if err != nil {
		return err
	}
	defer cc.Close()

	_ = cc
	err = fmt.Errorf("ReconcileNodeV1 removed — reconciliation now workflow-driven")
	if err != nil {
		if nodeReconcileJSON {
			result := map[string]interface{}{
				"success": false,
				"node_id": nodeReconcileNodeID,
				"error":   err.Error(),
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			encoder.Encode(result)
		}
		return fmt.Errorf("reconcile node: %w", err)
	}

	if nodeReconcileJSON {
		result := map[string]interface{}{
			"success": true,
			"node_id": nodeReconcileNodeID,
			"message": "reconciliation triggered successfully",
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	fmt.Printf("✅ Reconciliation triggered successfully for node %s\n", nodeReconcileNodeID)
	fmt.Println("\nNote: Reconciliation is asynchronous. Monitor node status to verify completion.")

	return nil
}
