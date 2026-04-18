package main

// node_recover_cmds.go — CLI commands for node.recover.full_reseed workflow.
//
//   globular node recover full-reseed --node-id <id> --reason <text>
//   globular node recover status       --node-id <id>
//   globular node recover ack-reprovision --node-id <id> --workflow-id <id>
//   globular node snapshot create      --node-id <id>
//   globular node snapshot show        --node-id <id> [--snapshot-id <id>]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ─── recover sub-tree ────────────────────────────────────────────────────────

var nodeRecoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Node recovery operations (last-resort full-reseed workflow)",
}

// ─── recover full-reseed ─────────────────────────────────────────────────────

var (
	recoverNodeID              string
	recoverReason              string
	recoverExactReplay         bool
	recoverForce               bool
	recoverDryRun              bool
	recoverSnapshotID          string
	recoverNote                string
	recoverJSON                bool
)

var nodeRecoverFullReseedCmd = &cobra.Command{
	Use:   "full-reseed",
	Short: "Start a full-reseed recovery workflow for a node",
	Long: `Trigger the node.recover.full_reseed workflow for last-resort node recovery.

The workflow will:
  1. Capture a snapshot of all installed artifacts (unless --snapshot-id is given)
  2. Fence the node (pause reconciler)
  3. Wait for operator confirmation that the node has been wiped and reprovisioned
  4. Reinstall all artifacts in deterministic bootstrap order
  5. Verify all artifacts and runtime health
  6. Unfence the node and emit a completion audit event

Use --dry-run to see what would be installed without executing anything.

IMPORTANT: This is a last-resort operation. The node will be completely wiped.
Human confirmation is required at the reprovision step.

Examples:
  globular node recover full-reseed --node-id abc123 --reason "disk corruption"
  globular node recover full-reseed --node-id abc123 --reason "data loss" --exact-replay
  globular node recover full-reseed --node-id abc123 --reason "test" --dry-run
  globular node recover full-reseed --node-id abc123 --reason "reinstall" --snapshot-id snap-xyz --force
`,
	RunE: runNodeRecoverFullReseed,
}

func init() {
	nodeRecoverFullReseedCmd.Flags().StringVar(&recoverNodeID, "node-id", "", "Target node ID (required)")
	nodeRecoverFullReseedCmd.Flags().StringVar(&recoverReason, "reason", "", "Human-readable reason for recovery (required)")
	nodeRecoverFullReseedCmd.Flags().BoolVar(&recoverExactReplay, "exact-replay", false, "Require exact build_id replay (fails if any artifact has no build_id)")
	nodeRecoverFullReseedCmd.Flags().BoolVar(&recoverForce, "force", false, "Skip cluster safety checks (quorum, storage nodes)")
	nodeRecoverFullReseedCmd.Flags().BoolVar(&recoverDryRun, "dry-run", false, "Plan artifacts without executing the workflow")
	nodeRecoverFullReseedCmd.Flags().StringVar(&recoverSnapshotID, "snapshot-id", "", "Reuse an existing snapshot instead of capturing a new one")
	nodeRecoverFullReseedCmd.Flags().StringVar(&recoverNote, "note", "", "Optional note for audit trail")
	nodeRecoverFullReseedCmd.Flags().BoolVar(&recoverJSON, "json", false, "Output result in JSON")
	nodeRecoverFullReseedCmd.MarkFlagRequired("node-id")
	nodeRecoverFullReseedCmd.MarkFlagRequired("reason")

	nodeRecoverCmd.AddCommand(nodeRecoverFullReseedCmd)
}

func runNodeRecoverFullReseed(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)
	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	client := cluster_controllerpb.NewNodeRecoveryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.StartNodeFullReseedRecovery(ctx, &cluster_controllerpb.StartNodeFullReseedRecoveryRequest{
		NodeID:              recoverNodeID,
		Reason:              recoverReason,
		ExactReplayRequired: recoverExactReplay,
		Force:               recoverForce,
		DryRun:              recoverDryRun,
		SnapshotID:          recoverSnapshotID,
		Note:                recoverNote,
	})
	if err != nil {
		return fmt.Errorf("StartNodeFullReseedRecovery: %w", err)
	}

	if recoverJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("state:       %s\n", resp.State)
	if resp.WorkflowID != "" {
		fmt.Printf("workflow_id: %s\n", resp.WorkflowID)
	}
	if resp.SnapshotID != "" {
		fmt.Printf("snapshot_id: %s\n", resp.SnapshotID)
	}
	if len(resp.Warnings) > 0 {
		fmt.Println("\nwarnings:")
		for _, w := range resp.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	if len(resp.PlannedArtifacts) > 0 {
		fmt.Printf("\nplanned artifacts (%d):\n", len(resp.PlannedArtifacts))
		for _, a := range resp.PlannedArtifacts {
			buildInfo := ""
			if a.BuildID != "" {
				buildInfo = fmt.Sprintf(" [build=%s]", a.BuildID[:min8(len(a.BuildID))])
			}
			fmt.Printf("  %2d. %-12s %-30s %s%s  (%s)\n",
				a.Order, a.Kind, a.Name, a.Version, buildInfo, a.Source)
		}
	}
	if resp.State == "DISPATCHED" {
		fmt.Printf("\nWorkflow dispatched. To monitor progress:\n")
		fmt.Printf("  globular node recover status --node-id %s\n", recoverNodeID)
		fmt.Printf("\nWhen the node is wiped and OS is reinstalled, acknowledge with:\n")
		fmt.Printf("  globular node recover ack-reprovision --node-id %s --workflow-id %s\n",
			recoverNodeID, resp.WorkflowID)
	}
	return nil
}

// ─── recover status ──────────────────────────────────────────────────────────

var (
	recoverStatusNodeID string
	recoverStatusJSON   bool
)

var nodeRecoverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get current recovery status for a node",
	Long: `Show the current phase, snapshot, and per-artifact results for an active
or recently completed node recovery workflow.

Examples:
  globular node recover status --node-id abc123
  globular node recover status --node-id abc123 --json
`,
	RunE: runNodeRecoverStatus,
}

func init() {
	nodeRecoverStatusCmd.Flags().StringVar(&recoverStatusNodeID, "node-id", "", "Target node ID (required)")
	nodeRecoverStatusCmd.Flags().BoolVar(&recoverStatusJSON, "json", false, "Output result in JSON")
	nodeRecoverStatusCmd.MarkFlagRequired("node-id")

	nodeRecoverCmd.AddCommand(nodeRecoverStatusCmd)
}

func runNodeRecoverStatus(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)
	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	client := cluster_controllerpb.NewNodeRecoveryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.GetNodeRecoveryStatus(ctx, &cluster_controllerpb.GetNodeRecoveryStatusRequest{
		NodeID: recoverStatusNodeID,
	})
	if err != nil {
		return fmt.Errorf("GetNodeRecoveryStatus: %w", err)
	}

	if recoverStatusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	if resp.Recovery == nil {
		fmt.Printf("no active recovery for node %s\n", recoverStatusNodeID)
		return nil
	}

	st := resp.Recovery
	fmt.Printf("node_id:      %s\n", st.NodeID)
	fmt.Printf("phase:        %s\n", st.Phase)
	fmt.Printf("mode:         %s\n", st.Mode)
	fmt.Printf("workflow_id:  %s\n", st.WorkflowID)
	fmt.Printf("snapshot_id:  %s\n", st.SnapshotID)
	fmt.Printf("reason:       %s\n", st.Reason)
	fmt.Printf("fenced:       %v\n", st.ReconciliationPaused)
	fmt.Printf("destructive:  %v\n", st.DestructiveBoundaryCrossed)
	if !st.StartedAt.IsZero() {
		fmt.Printf("started:      %s (%s ago)\n", st.StartedAt.Format(time.RFC3339), formatAge(int64(time.Since(st.StartedAt).Seconds())))
	}
	if !st.UpdatedAt.IsZero() {
		fmt.Printf("updated:      %s (%s ago)\n", st.UpdatedAt.Format(time.RFC3339), formatAge(int64(time.Since(st.UpdatedAt).Seconds())))
	}

	if snap := resp.Snapshot; snap != nil {
		fmt.Printf("\nsnapshot: %s (%d artifacts, captured %s)\n",
			snap.SnapshotID, len(snap.Artifacts), snap.CreatedAt.Format(time.RFC3339))
	}

	if len(resp.Results) > 0 {
		ok, failed, pending := 0, 0, 0
		for _, r := range resp.Results {
			switch r.Status {
			case cluster_controllerpb.RecoveryArtifactStatusVerified:
				ok++
			case cluster_controllerpb.RecoveryArtifactStatusFailed:
				failed++
			default:
				pending++
			}
		}
		fmt.Printf("\nartifacts: %d total — %d verified, %d failed, %d pending\n",
			len(resp.Results), ok, failed, pending)
		if failed > 0 {
			fmt.Println("failed artifacts:")
			for _, r := range resp.Results {
				if r.Status == cluster_controllerpb.RecoveryArtifactStatusFailed {
					fmt.Printf("  - %s/%s: %s\n", r.Kind, r.Name, r.Error)
				}
			}
		}
	}
	return nil
}

// ─── recover ack-reprovision ─────────────────────────────────────────────────

var (
	ackNodeID     string
	ackWorkflowID string
	ackNote       string
)

var nodeRecoverAckCmd = &cobra.Command{
	Use:   "ack-reprovision",
	Short: "Acknowledge that a node has been wiped and reprovisioned",
	Long: `Signal the recovery workflow that the machine has been wiped and the OS
has been reinstalled. The workflow is paused at AWAIT_REPROVISION until this
command is run.

Run this ONLY after you have confirmed:
  1. The node's disks have been wiped
  2. A fresh OS has been installed
  3. The Globular node-agent package has been bootstrapped

Examples:
  globular node recover ack-reprovision --node-id abc123 --workflow-id wf-xyz
  globular node recover ack-reprovision --node-id abc123 --workflow-id wf-xyz --note "Reinstalled Ubuntu 22.04"
`,
	RunE: runNodeRecoverAck,
}

func init() {
	nodeRecoverAckCmd.Flags().StringVar(&ackNodeID, "node-id", "", "Target node ID (required)")
	nodeRecoverAckCmd.Flags().StringVar(&ackWorkflowID, "workflow-id", "", "Workflow run ID to ack (required)")
	nodeRecoverAckCmd.Flags().StringVar(&ackNote, "note", "", "Optional note for audit trail")
	nodeRecoverAckCmd.MarkFlagRequired("node-id")
	nodeRecoverAckCmd.MarkFlagRequired("workflow-id")

	nodeRecoverCmd.AddCommand(nodeRecoverAckCmd)
}

func runNodeRecoverAck(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)
	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	client := cluster_controllerpb.NewNodeRecoveryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.AckNodeReprovisioned(ctx, &cluster_controllerpb.AckNodeReprovisionedRequest{
		NodeID:     ackNodeID,
		WorkflowID: ackWorkflowID,
		Note:       ackNote,
	})
	if err != nil {
		return fmt.Errorf("AckNodeReprovisioned: %w", err)
	}

	fmt.Printf("reprovision acknowledged for node %s — workflow will resume reseed\n", ackNodeID)
	fmt.Printf("monitor with: globular node recover status --node-id %s\n", ackNodeID)
	return nil
}

// ─── snapshot sub-tree ───────────────────────────────────────────────────────

var nodeSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Node inventory snapshot operations",
}

// ─── snapshot create ─────────────────────────────────────────────────────────

var (
	snapshotCreateNodeID string
	snapshotCreateReason string
	snapshotCreateJSON   bool
)

var nodeSnapshotCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Capture a pre-maintenance inventory snapshot for a node",
	Long: `Take a standalone snapshot of a node's installed artifact set without
starting a recovery workflow. Useful for pre-maintenance captures or to supply
to a subsequent full-reseed recovery via --snapshot-id.

Examples:
  globular node snapshot create --node-id abc123
  globular node snapshot create --node-id abc123 --reason "pre-upgrade baseline"
  globular node snapshot create --node-id abc123 --json
`,
	RunE: runNodeSnapshotCreate,
}

func init() {
	nodeSnapshotCreateCmd.Flags().StringVar(&snapshotCreateNodeID, "node-id", "", "Target node ID (required)")
	nodeSnapshotCreateCmd.Flags().StringVar(&snapshotCreateReason, "reason", "", "Reason for the snapshot")
	nodeSnapshotCreateCmd.Flags().BoolVar(&snapshotCreateJSON, "json", false, "Output result in JSON")
	nodeSnapshotCreateCmd.MarkFlagRequired("node-id")

	nodeSnapshotCmd.AddCommand(nodeSnapshotCreateCmd)
}

func runNodeSnapshotCreate(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)
	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	client := cluster_controllerpb.NewNodeRecoveryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.CreateNodeRecoverySnapshot(ctx, &cluster_controllerpb.CreateNodeRecoverySnapshotRequest{
		NodeID: snapshotCreateNodeID,
		Reason: snapshotCreateReason,
	})
	if err != nil {
		return fmt.Errorf("CreateNodeRecoverySnapshot: %w", err)
	}

	if snapshotCreateJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	snap := resp.Snapshot
	fmt.Printf("snapshot_id: %s\n", resp.SnapshotID)
	if snap != nil {
		fmt.Printf("node_id:     %s\n", snap.NodeID)
		fmt.Printf("artifacts:   %d\n", len(snap.Artifacts))
		fmt.Printf("captured_at: %s\n", snap.CreatedAt.Format(time.RFC3339))
		if snap.Reason != "" {
			fmt.Printf("reason:      %s\n", snap.Reason)
		}
		if snap.SnapshotHash != "" {
			fmt.Printf("hash:        %s\n", snap.SnapshotHash[:min8(len(snap.SnapshotHash))])
		}
		fmt.Printf("\nTo use this snapshot for recovery:\n")
		fmt.Printf("  globular node recover full-reseed --node-id %s --reason \"...\" --snapshot-id %s\n",
			snapshotCreateNodeID, resp.SnapshotID)
	}
	return nil
}

// ─── snapshot show ───────────────────────────────────────────────────────────

var (
	snapshotShowNodeID     string
	snapshotShowSnapshotID string
	snapshotShowJSON       bool
)

var nodeSnapshotShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show snapshot details via recovery status",
	Long: `Display the snapshot associated with the current or most recent recovery
for a node. Use --snapshot-id to show a specific snapshot.

Examples:
  globular node snapshot show --node-id abc123
  globular node snapshot show --node-id abc123 --json
`,
	RunE: runNodeSnapshotShow,
}

func init() {
	nodeSnapshotShowCmd.Flags().StringVar(&snapshotShowNodeID, "node-id", "", "Target node ID (required)")
	nodeSnapshotShowCmd.Flags().StringVar(&snapshotShowSnapshotID, "snapshot-id", "", "Specific snapshot ID (defaults to current recovery snapshot)")
	nodeSnapshotShowCmd.Flags().BoolVar(&snapshotShowJSON, "json", false, "Output result in JSON")
	nodeSnapshotShowCmd.MarkFlagRequired("node-id")

	nodeSnapshotCmd.AddCommand(nodeSnapshotShowCmd)
}

func runNodeSnapshotShow(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)
	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	client := cluster_controllerpb.NewNodeRecoveryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// GetNodeRecoveryStatus returns the snapshot attached to the current recovery.
	resp, err := client.GetNodeRecoveryStatus(ctx, &cluster_controllerpb.GetNodeRecoveryStatusRequest{
		NodeID: snapshotShowNodeID,
	})
	if err != nil {
		return fmt.Errorf("GetNodeRecoveryStatus: %w", err)
	}

	snap := resp.Snapshot
	if snap == nil {
		fmt.Printf("no snapshot found for node %s\n", snapshotShowNodeID)
		return nil
	}

	if snapshotShowJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}

	fmt.Printf("snapshot_id:  %s\n", snap.SnapshotID)
	fmt.Printf("node_id:      %s\n", snap.NodeID)
	fmt.Printf("captured_at:  %s\n", snap.CreatedAt.Format(time.RFC3339))
	fmt.Printf("requested_by: %s\n", snap.CreatedBy)
	fmt.Printf("reason:       %s\n", snap.Reason)
	fmt.Printf("profiles:     %s\n", strings.Join(snap.Profiles, ", "))
	if snap.SnapshotHash != "" {
		fmt.Printf("hash:         %s\n", snap.SnapshotHash)
	}
	fmt.Printf("\nartifacts (%d):\n", len(snap.Artifacts))
	for _, a := range snap.Artifacts {
		buildInfo := ""
		if a.BuildID != "" {
			buildInfo = fmt.Sprintf(" [%s]", a.BuildID[:min8(len(a.BuildID))])
		}
		fmt.Printf("  %-12s %-30s %s%s\n", a.Kind, a.Name, a.Version, buildInfo)
	}
	return nil
}

// ─── register all commands ───────────────────────────────────────────────────

func init() {
	nodeCmd.AddCommand(nodeRecoverCmd)
	nodeCmd.AddCommand(nodeSnapshotCmd)
}

// min8 returns min(n, 8) for truncating IDs in display.
func min8(n int) int {
	if n < 8 {
		return n
	}
	return 8
}
