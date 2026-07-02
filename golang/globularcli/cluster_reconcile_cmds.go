package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/spf13/cobra"
)

var clusterReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Dispatch the cluster.reconcile workflow",
	Long: `Runs the cluster.reconcile workflow through the centralized workflow service.

This is the workflow-backed reconciliation path used by the controller's
periodic loop. It is useful when you need to force a fresh scan after a
state change such as retiring a desired service or cleaning an orphaned
install.`,
	RunE: runClusterReconcile,
}

func init() {
	clusterCmd.AddCommand(clusterReconcileCmd)
}

func runClusterReconcile(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	wfAddr := config.ResolveServiceAddr("workflow.WorkflowService", "")
	if wfAddr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			wfAddr = a
		}
	}
	if wfAddr == "" {
		return fmt.Errorf("workflow service not found (check etcd or use --controller)")
	}

	controllerAddr := config.ResolveControllerDirectAddr()
	if controllerAddr == "" {
		controllerAddr = rootCfg.controllerAddr
	}

	clusterID, _ := security.GetLocalClusterID()
	if clusterID == "" {
		clusterID, _ = config.GetDomain()
	}
	if clusterID == "" {
		return fmt.Errorf("cluster_id unresolvable from local config")
	}

	inputs := map[string]any{
		"cluster_id": clusterID,
		"scope":      "cluster",
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}

	conn, err := dialGRPC(wfAddr)
	if err != nil {
		return fmt.Errorf("connect to workflow service at %s: %w", wfAddr, err)
	}
	defer conn.Close()

	client := workflowpb.NewWorkflowServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	resp, err := client.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		WorkflowName: "cluster.reconcile",
		ClusterId:    clusterID,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-controller": controllerAddr,
		},
		CorrelationId: fmt.Sprintf("cli-cluster-reconcile-%d", time.Now().Unix()),
	})
	if err != nil {
		return fmt.Errorf("workflow: %w", err)
	}

	fmt.Printf("Run ID: %s\n", resp.GetRunId())
	fmt.Printf("Status: %s\n", resp.GetStatus())
	if resp.GetError() != "" {
		fmt.Printf("Error:  %s\n", resp.GetError())
	}
	if resp.GetOutputsJson() != "" && rootCfg.output != "json" {
		var outputs map[string]any
		if jsonErr := json.Unmarshal([]byte(resp.GetOutputsJson()), &outputs); jsonErr == nil {
			if evaluate, ok := outputs["evaluate"].(map[string]any); ok {
				if count, ok := evaluate["decisions_count"]; ok {
					fmt.Printf("Decisions: %v\n", count)
				}
				if count, ok := evaluate["upgrades_count"]; ok {
					fmt.Printf("Upgrades:  %v\n", count)
				}
			}
		}
	}
	if resp.GetStatus() == "FAILED" {
		return fmt.Errorf("cluster.reconcile workflow FAILED")
	}
	return nil
}
