package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
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

	client := clustercontrollerpb.NewClusterControllerServiceClient(cc)

	// Get the current node plan
	resp, err := client.GetNodePlanV1(ctxWithTimeout(), &clustercontrollerpb.GetNodePlanV1Request{
		NodeId: nodeReconcileNodeID,
	})
	if err != nil {
		return fmt.Errorf("get node plan: %w", err)
	}

	plan := resp.GetPlan()
	if plan == nil {
		if nodeReconcileJSON {
			result := map[string]interface{}{
				"dry_run": true,
				"node_id": nodeReconcileNodeID,
				"plan":    nil,
				"message": "no plan available for node",
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}

		fmt.Printf("✅ No reconciliation plan for node %s\n", nodeReconcileNodeID)
		fmt.Println("Node is already in desired state or no plan has been generated yet.")
		return nil
	}

	if nodeReconcileJSON {
		result := map[string]interface{}{
			"dry_run":    true,
			"node_id":    nodeReconcileNodeID,
			"plan":       plan,
			"reason":     plan.GetReason(),
			"step_count": len(plan.GetSpec().GetSteps()),
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	// Human-readable output
	fmt.Printf("🔍 Dry-run: Reconciliation plan for node %s\n\n", nodeReconcileNodeID)
	fmt.Printf("Reason: %s\n", plan.GetReason())
	fmt.Printf("Steps: %d\n\n", len(plan.GetSpec().GetSteps()))

	if steps := plan.GetSpec().GetSteps(); len(steps) > 0 {
		fmt.Println("Plan steps:")
		for i, step := range steps {
			fmt.Printf("  %d. %s\n", i+1, step.GetAction())
			if step.GetArgs() != nil && len(step.GetArgs().GetFields()) > 0 {
				fmt.Printf("     Args: %v\n", step.GetArgs().GetFields())
			}
		}
	}

	fmt.Println("\nNote: This is a dry-run. No changes will be applied.")
	fmt.Println("Run without --dry-run to execute the plan.")

	return nil
}

func runNodeReconcileApply() error {
	cc, err := controllerClient()
	if err != nil {
		return err
	}
	defer cc.Close()

	client := clustercontrollerpb.NewClusterControllerServiceClient(cc)

	// Trigger reconciliation
	fmt.Printf("Triggering reconciliation for node %s...\n", nodeReconcileNodeID)

	_, err = client.ReconcileNodeV1(ctxWithTimeout(), &clustercontrollerpb.ReconcileNodeV1Request{
		NodeId: nodeReconcileNodeID,
	})
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
