package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	nodeCmd.AddCommand(nodeResolveCmd)
	rootCmd.AddCommand(nodeCmd)

	nodeResolveCmd.Flags().BoolVar(&nodeResolveJSON, "json", false, "Output result in JSON format")
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

// ─── node resolve ────────────────────────────────────────────────────────────

var nodeResolveJSON bool

var nodeResolveCmd = &cobra.Command{
	Use:   "resolve <identifier>",
	Short: "Resolve a node identity from any of node_id / hostname / mac / ip",
	Long: `Returns the minimal "who is this node?" projection.

The identifier may be a node_id (uuid), hostname, mac address, or IP. The
resolver picks the right lookup table based on the identifier's shape.

Output is flat and focused — it does NOT include services, packages, or
metrics. Chain into other commands (or other MCP tools) for those.

Examples:
  globular node resolve globule-nuc
  globular node resolve 10.0.0.63
  globular node resolve e0:d4:64:f0:86:f6
  globular node resolve eb9a2dac-05b0-52ac-9002-99d8ffd35902
`,
	Args: cobra.ExactArgs(1),
	RunE: runNodeResolve,
}

func runNodeResolve(cmd *cobra.Command, args []string) error {
	ident := strings.TrimSpace(args[0])
	if ident == "" {
		return errors.New("identifier required")
	}

	autoDiscoverController(cmd)
	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()
	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	rsp, err := cc.ResolveNode(ctx, &cluster_controllerpb.ResolveNodeRequest{Identifier: ident})
	if err != nil {
		return fmt.Errorf("ResolveNode: %w", err)
	}
	id := rsp.GetIdentity()
	if id == nil {
		return fmt.Errorf("no identity returned")
	}

	if nodeResolveJSON {
		out := map[string]interface{}{
			"node_id":     id.GetNodeId(),
			"hostname":    id.GetHostname(),
			"ips":         id.GetIps(),
			"macs":        id.GetMacs(),
			"labels":      id.GetLabels(),
			"source":      id.GetSource(),
			"observed_at": id.GetObservedAt(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	age := ""
	if id.GetObservedAt() > 0 {
		age = formatAge(time.Now().Unix() - id.GetObservedAt())
	}
	fmt.Printf("node_id:     %s\n", id.GetNodeId())
	fmt.Printf("hostname:    %s\n", id.GetHostname())
	fmt.Printf("ips:         %s\n", strings.Join(id.GetIps(), ", "))
	fmt.Printf("macs:        %s\n", strings.Join(id.GetMacs(), ", "))
	fmt.Printf("labels:      %s\n", strings.Join(id.GetLabels(), ", "))
	fmt.Printf("source:      %s\n", id.GetSource())
	if age != "" {
		fmt.Printf("observed:    %s ago\n", age)
	}
	return nil
}

func formatAge(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	case seconds < 86400:
		return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
	default:
		return fmt.Sprintf("%dd%dh", seconds/86400, (seconds%86400)/3600)
	}
}
