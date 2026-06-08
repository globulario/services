package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/audittrail"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/spf13/cobra"
)

var (
	platformUpgradeDryRun bool
)

var platformUpgradeCmd = &cobra.Command{
	Use:   "platform-upgrade <release-tag>",
	Short: "Apply a platform release as workflow-gated per-(node, package) upgrades",
	Long: `Dispatches the platform.upgrade workflow on the cluster controller.

For every package in the local repository's PUBLISHED BOM, for every
node in the cluster, the workflow evaluates:

  - profile match (node.profiles ∩ package.profiles non-empty)
  - installed on this node (Layer 3 truth — heartbeat)
  - BOM version > installed version (semver — never downgrade)
  - BOM version resolvable in the local repository (orphan-prevention)

Only packages that pass all four gates are upgraded — via the existing
release.apply.package per-node workflow. Operator removals are
preserved (not_installed is not auto-installed).

This replaces the pre-v1.2.160 direct-etcd-write CLI path which
bypassed the gates and bulk-applied ServiceDesiredVersion records for
the entire BOM (v1.2.159 incident: 7 operator-removed services
re-introduced, 28 fresh DesiredBuildIdOrphaned findings).

Typical workflow:
  globular repo sync --source globulario-github --tag v1.2.160
  globular platform-upgrade v1.2.160

The sync imports packages into the local repository. The
platform-upgrade dispatches the gated workflow against the cluster.`,
	Args: cobra.ExactArgs(1),
	RunE: runPlatformUpgrade,
}

func init() {
	platformUpgradeCmd.Flags().BoolVar(&platformUpgradeDryRun, "dry-run", false,
		"Evaluate per-(node, package) decisions without dispatching any upgrades")
	rootCmd.AddCommand(platformUpgradeCmd)
}

func runPlatformUpgrade(cmd *cobra.Command, args []string) error {
	tag := args[0]
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	autoDiscoverController(cmd)

	// Resolve workflow service from etcd.
	wfAddr := config.ResolveServiceAddr("workflow.WorkflowService", "")
	if wfAddr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			wfAddr = a
		}
	}
	if wfAddr == "" {
		return fmt.Errorf("workflow service not found (check etcd or use --controller)")
	}

	// Controller is the actor endpoint for cluster-controller actions —
	// platform.upgrade's evaluate, dispatch_upgrades, and audit all route
	// to the controller's default router.
	//
	// Use ResolveControllerDirectAddr (NOT ResolveServiceAddr): the latter
	// passes through meshRouteAddrs() which rewrites to host:443 (Envoy).
	// Go TLS suppresses SNI when ServerName is an IP literal (RFC 6066),
	// so Envoy falls to the default filter chain and returns text/html
	// instead of gRPC. The direct addr (host:12000) bypasses Envoy
	// entirely so the workflow service's actor dispatcher hits the
	// controller's gRPC port directly.
	controllerAddr := config.ResolveControllerDirectAddr()
	if controllerAddr == "" {
		controllerAddr = rootCfg.controllerAddr
	}
	if controllerAddr == "" {
		return fmt.Errorf("cluster_controller address not found (check etcd or use --controller)")
	}

	clusterID, _ := security.GetLocalClusterID()
	if clusterID == "" {
		clusterID, _ = config.GetDomain()
	}
	if clusterID == "" {
		return fmt.Errorf("cluster_id unresolvable from local config — set globular_domain or initialize cluster")
	}

	inputs := map[string]any{
		"cluster_id":  clusterID,
		"release_tag": tag,
		"dry_run":     platformUpgradeDryRun,
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}

	corrID := fmt.Sprintf("cli-platform-upgrade-%s-%d", tag, time.Now().Unix())
	if platformUpgradeDryRun {
		fmt.Printf("Dry-run: previewing platform.upgrade decisions for %s\n\n", tag)
	} else {
		fmt.Printf("Dispatching platform.upgrade workflow for %s\n\n", tag)
	}

	cc, err := dialGRPC(wfAddr)
	if err != nil {
		return fmt.Errorf("connect to workflow service at %s: %w", wfAddr, err)
	}
	defer cc.Close()

	wfClient := workflowpb.NewWorkflowServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	resp, err := wfClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		WorkflowName: "platform.upgrade",
		ClusterId:    clusterID,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-controller": controllerAddr,
		},
		CorrelationId: corrID,
	})
	if err != nil {
		return fmt.Errorf("workflow: %w", err)
	}

	fmt.Printf("Run ID: %s\n", resp.RunId)
	fmt.Printf("Status: %s\n", resp.Status)
	if resp.Error != "" {
		fmt.Printf("Error:  %s\n", resp.Error)
	}

	if resp.OutputsJson != "" {
		var outputs map[string]any
		if jsonErr := json.Unmarshal([]byte(resp.OutputsJson), &outputs); jsonErr == nil {
			printPlatformUpgradeSummary(outputs)
		}
	}

	// Best-effort audit trail entry — non-fatal.
	_ = audittrail.WriteDesiredWriteRecord(ctx, audittrail.DesiredWriteRecord{
		Service:   "platform-upgrade",
		Actor:     "operator-cli",
		Source:    "platform-upgrade",
		Action:    "dispatch_platform_upgrade",
		Reason:    fmt.Sprintf("workflow run %s for tag %s (dry_run=%v)", resp.RunId, tag, platformUpgradeDryRun),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if resp.Status == "FAILED" {
		return fmt.Errorf("platform.upgrade workflow FAILED")
	}
	return nil
}

// printPlatformUpgradeSummary renders the evaluate step's bucket counts
// when present in the workflow outputs.
func printPlatformUpgradeSummary(outputs map[string]any) {
	// Evaluate step exports its summary under outputs.evaluate (engine
	// passes step.Outputs through unchanged).
	eval, ok := outputs["evaluate"].(map[string]any)
	if !ok {
		return
	}
	fmt.Println()
	if c, ok := eval["decisions_count"]; ok {
		fmt.Printf("Decisions:  %v\n", c)
	}
	if c, ok := eval["upgrades_count"]; ok {
		fmt.Printf("Upgrades:   %v\n", c)
	}
	if buckets, ok := eval["buckets"].(map[string]any); ok && len(buckets) > 0 {
		fmt.Println("Buckets:")
		for k, v := range buckets {
			fmt.Printf("  %-20s %v\n", k, v)
		}
	}
}
