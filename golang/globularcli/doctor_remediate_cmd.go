package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	doctorRemediateStep     uint32
	doctorRemediateDryRun   bool
	doctorRemediateApproval string
	doctorRemediateEndpoint string
	doctorRemediateJSON     bool
	doctorRemediateWorkflow bool
)

var doctorRemediateCmd = &cobra.Command{
	Use:   "remediate <finding-id>",
	Short: "Execute a structured remediation action for a finding",
	Long: `Runs the structured action on a cluster-doctor finding's remediation step.

Safety rules (server-side, non-overridable):
  - ETCD_PUT, ETCD_DELETE, NODE_REMOVE are NEVER auto-executable
  - SYSTEMCTL actions run only on "globular-*" units
  - FILE_DELETE runs only under /usr/lib/globular/bin/*.tmp|*.bak
  - MEDIUM / HIGH risk actions require --approval <token>

Run GetClusterReport / GetNodeReport first to populate the finding cache.

Examples:
  globular doctor remediate a1b2c3d4e5f60102 --dry-run
  globular doctor remediate a1b2c3d4e5f60102 --step 0
  globular doctor remediate a1b2c3d4e5f60102 --step 1 --approval tkn-abc
`,
	Args: cobra.ExactArgs(1),
	RunE: runDoctorRemediate,
}

func init() {
	doctorCmd.AddCommand(doctorRemediateCmd)
	doctorRemediateCmd.Flags().Uint32Var(&doctorRemediateStep, "step", 0, "Remediation step index (0-based)")
	doctorRemediateCmd.Flags().BoolVar(&doctorRemediateDryRun, "dry-run", false, "Validate + resolve target, do not execute")
	doctorRemediateCmd.Flags().StringVar(&doctorRemediateApproval, "approval", "", "Approval token (required for MEDIUM/HIGH risk)")
	doctorRemediateCmd.Flags().StringVar(&doctorRemediateEndpoint, "endpoint", "", "cluster-doctor gRPC endpoint (auto-discovered if empty)")
	doctorRemediateCmd.Flags().BoolVar(&doctorRemediateJSON, "json", false, "Output as JSON")
	doctorRemediateCmd.Flags().BoolVar(&doctorRemediateWorkflow, "workflow", false, "Run through remediate.doctor.finding workflow (resolve→assess→approve→execute→verify)")
}

func runDoctorRemediate(cmd *cobra.Command, args []string) error {
	findingID := strings.TrimSpace(args[0])
	if findingID == "" {
		return fmt.Errorf("finding-id required")
	}
	endpoint := doctorRemediateEndpoint
	if endpoint == "" {
		endpoint = config.ResolveServiceAddr("cluster_doctor.ClusterDoctorService", "")
	}
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s:10080", config.GetRoutableIPv4())
	}

	resolvedEndpoint, resolveErr := resolveGRPCAddr(endpoint)
	if resolveErr != nil {
		return fmt.Errorf("invalid --endpoint %q: %w", endpoint, resolveErr)
	}

	conn, err := grpc.NewClient(
		resolvedEndpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
	)
	if err != nil {
		return fmt.Errorf("dial cluster-doctor %s: %w", endpoint, err)
	}
	defer conn.Close()

	client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	if doctorRemediateWorkflow {
		return runDoctorRemediateWorkflow(ctx, client, findingID)
	}

	rsp, err := client.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId:     findingID,
		StepIndex:     doctorRemediateStep,
		ApprovalToken: doctorRemediateApproval,
		DryRun:        doctorRemediateDryRun,
	})
	if err != nil {
		return fmt.Errorf("ExecuteRemediation: %w", err)
	}

	if doctorRemediateJSON {
		out := map[string]interface{}{
			"executed": rsp.GetExecuted(),
			"status":   rsp.GetStatus(),
			"reason":   rsp.GetReason(),
			"output":   rsp.GetOutput(),
			"audit_id": rsp.GetAuditId(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	prefix := "✓"
	if !rsp.GetExecuted() && rsp.GetStatus() != "dry_run_ok" {
		prefix = "✕"
	}
	fmt.Printf("%s status:   %s\n", prefix, rsp.GetStatus())
	if rsp.GetOutput() != "" {
		fmt.Printf("  output:   %s\n", rsp.GetOutput())
	}
	if rsp.GetReason() != "" {
		fmt.Printf("  reason:   %s\n", rsp.GetReason())
	}
	if rsp.GetAuditId() != "" {
		fmt.Printf("  audit_id: %s\n", rsp.GetAuditId())
	}
	return nil
}

// runDoctorRemediateWorkflow calls StartRemediationWorkflow and prints the
// full pipeline outcome: resolve → assess → approve → execute → verify.
func runDoctorRemediateWorkflow(ctx context.Context, client cluster_doctorpb.ClusterDoctorServiceClient, findingID string) error {
	rsp, err := client.StartRemediationWorkflow(ctx, &cluster_doctorpb.StartRemediationWorkflowRequest{
		FindingId:     findingID,
		StepIndex:     doctorRemediateStep,
		ApprovalToken: doctorRemediateApproval,
		DryRun:        doctorRemediateDryRun,
	})
	if err != nil {
		return fmt.Errorf("StartRemediationWorkflow: %w", err)
	}

	if doctorRemediateJSON {
		out := map[string]interface{}{
			"run_id":                rsp.GetRunId(),
			"run_status":            rsp.GetRunStatus(),
			"run_error":             rsp.GetRunError(),
			"resolved_node_id":      rsp.GetResolvedNodeId(),
			"resolved_action_type":  rsp.GetResolvedActionType(),
			"risk":                  rsp.GetRisk(),
			"auto_executable":       rsp.GetAutoExecutable(),
			"requires_approval":     rsp.GetRequiresApproval(),
			"executed":              rsp.GetExecuted(),
			"execute_status":        rsp.GetExecuteStatus(),
			"execute_output":        rsp.GetExecuteOutput(),
			"audit_id":              rsp.GetAuditId(),
			"converged":             rsp.GetConverged(),
			"finding_still_present": rsp.GetFindingStillPresent(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	prefix := "✓"
	if rsp.GetRunStatus() != "SUCCEEDED" {
		prefix = "✕"
	}
	fmt.Printf("%s workflow: %s (run_id=%s)\n", prefix, rsp.GetRunStatus(), rsp.GetRunId())
	if rsp.GetRunError() != "" {
		fmt.Printf("  error: %s\n", rsp.GetRunError())
	}
	fmt.Printf("  resolve: node=%s action=%s risk=%s\n",
		rsp.GetResolvedNodeId(), rsp.GetResolvedActionType(), rsp.GetRisk())
	fmt.Printf("  assess:  auto_executable=%v requires_approval=%v\n",
		rsp.GetAutoExecutable(), rsp.GetRequiresApproval())
	fmt.Printf("  execute: status=%s executed=%v audit_id=%s\n",
		rsp.GetExecuteStatus(), rsp.GetExecuted(), rsp.GetAuditId())
	if rsp.GetExecuteOutput() != "" {
		fmt.Printf("           output=%s\n", rsp.GetExecuteOutput())
	}
	fmt.Printf("  verify:  converged=%v finding_still_present=%v\n",
		rsp.GetConverged(), rsp.GetFindingStillPresent())
	return nil
}
