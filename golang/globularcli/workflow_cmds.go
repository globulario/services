// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=workflow_management_commands
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness risk=high
package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

var workflowAddr string

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Inspect workflow runs and history",
	Long: `View workflow execution history: list runs, get run details, and diagnose failures.

Examples:
  globular workflow list
  globular workflow list --service postgresql --status FAILED
  globular workflow list --node node-abc123
  globular workflow get <run-id>
`,
}

// --- list ---

var (
	workflowListService string
	workflowListNode    string
	workflowListStatus  string
	workflowListLimit   int32
	workflowListActive  bool
	workflowListFailed  bool
)

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflow runs",
	RunE:  runWorkflowList,
}

func runWorkflowList(cmd *cobra.Command, args []string) error {
	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()

	client := workflowpb.NewWorkflowServiceClient(cc)

	req := &workflowpb.ListRunsRequest{
		ComponentName: workflowListService,
		NodeId:        workflowListNode,
		ActiveOnly:    workflowListActive,
		FailedOnly:    workflowListFailed,
		Limit:         workflowListLimit,
	}

	if workflowListStatus != "" {
		upper := strings.ToUpper(workflowListStatus)
		if val, ok := workflowpb.RunStatus_value[upper]; ok {
			req.Status = workflowpb.RunStatus(val)
		} else if val, ok := workflowpb.RunStatus_value["RUN_"+upper]; ok {
			req.Status = workflowpb.RunStatus(val)
		} else {
			return fmt.Errorf("unknown status %q", workflowListStatus)
		}
	}

	resp, err := client.ListRuns(ctxWithTimeout(), req)
	if err != nil {
		return fmt.Errorf("list runs: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tSERVICE\tNODE\tSTATUS\tTRIGGER\tSTARTED\tDURATION")

	for _, run := range resp.GetRuns() {
		runID := run.GetId()
		if len(runID) > 12 {
			runID = runID[:12] + "..."
		}

		nodeID := run.GetContext().GetNodeId()
		if len(nodeID) > 8 {
			nodeID = nodeID[:8] + "..."
		}

		service := run.GetContext().GetComponentName()
		status := stripPrefix(run.GetStatus().String(), "RUN_")
		trigger := stripPrefix(run.GetTriggerReason().String(), "TRIGGER_REASON_")

		started := "—"
		duration := "—"
		if run.GetStartedAt() != nil {
			startTime := run.GetStartedAt().AsTime()
			started = fmtTimeAgo(startTime)
			if run.GetFinishedAt() != nil {
				d := run.GetFinishedAt().AsTime().Sub(startTime)
				duration = d.Round(time.Second).String()
			} else {
				duration = "running"
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			runID, service, nodeID, status, trigger, started, duration)
	}
	w.Flush()

	if len(resp.GetRuns()) == 0 {
		fmt.Println("No workflow runs found.")
		fmt.Println("Note: this command lists persisted workflow-run history; service release status may still show EXECUTING from controller state.")
	}
	return nil
}

// --- get ---

var workflowGetCmd = &cobra.Command{
	Use:   "get <run-id>",
	Short: "Get workflow run details",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkflowGet,
}

func runWorkflowGet(cmd *cobra.Command, args []string) error {
	runID := args[0]

	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()

	client := workflowpb.NewWorkflowServiceClient(cc)

	resp, err := client.GetRun(ctxWithTimeout(), &workflowpb.GetRunRequest{Id: runID})
	if err != nil {
		return fmt.Errorf("get run: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	run := resp.GetRun()
	if run == nil {
		return fmt.Errorf("run %s not found", runID)
	}

	status := stripPrefix(run.GetStatus().String(), "RUN_")
	trigger := stripPrefix(run.GetTriggerReason().String(), "TRIGGER_REASON_")

	fmt.Printf("Run ID:         %s\n", run.GetId())
	fmt.Printf("Correlation:    %s\n", run.GetCorrelationId())
	if run.GetParentRunId() != "" {
		fmt.Printf("Parent Run:     %s\n", run.GetParentRunId())
	}
	fmt.Printf("Status:         %s\n", status)
	fmt.Printf("Trigger:        %s\n", trigger)

	if ctx := run.GetContext(); ctx != nil {
		fmt.Printf("Service:        %s\n", ctx.GetComponentName())
		if ctx.GetComponentVersion() != "" {
			fmt.Printf("Version:        %s\n", ctx.GetComponentVersion())
		}
		if ctx.GetNodeId() != "" {
			fmt.Printf("Node:           %s\n", ctx.GetNodeId())
		}
		if ctx.GetNodeHostname() != "" {
			fmt.Printf("Hostname:       %s\n", ctx.GetNodeHostname())
		}
	}

	if run.GetFailureClass() != 0 {
		fc := stripPrefix(run.GetFailureClass().String(), "FAILURE_CLASS_")
		fmt.Printf("Failure Class:  %s\n", fc)
	}
	if run.GetErrorMessage() != "" {
		fmt.Printf("Error:          %s\n", run.GetErrorMessage())
	}
	if run.GetRetryCount() > 0 {
		fmt.Printf("Retry Count:    %d\n", run.GetRetryCount())
	}

	if run.GetStartedAt() != nil {
		fmt.Printf("Started:        %s\n", run.GetStartedAt().AsTime().Format(time.RFC3339))
	}
	if run.GetFinishedAt() != nil {
		fmt.Printf("Completed:      %s\n", run.GetFinishedAt().AsTime().Format(time.RFC3339))
		if run.GetStartedAt() != nil {
			d := run.GetFinishedAt().AsTime().Sub(run.GetStartedAt().AsTime())
			fmt.Printf("Duration:       %s\n", d.Round(time.Millisecond))
		}
	}

	// Steps
	steps := resp.GetSteps()
	if len(steps) > 0 {
		fmt.Printf("\nSTEPS:\n")
		for _, step := range steps {
			ss := stripPrefix(step.GetStatus().String(), "STEP_STATUS_")
			marker := "  "
			switch step.GetStatus() {
			case workflowpb.StepStatus_STEP_STATUS_SUCCEEDED:
				marker = "✓ "
			case workflowpb.StepStatus_STEP_STATUS_FAILED:
				marker = "✗ "
			case workflowpb.StepStatus_STEP_STATUS_RUNNING:
				marker = "▶ "
			case workflowpb.StepStatus_STEP_STATUS_SKIPPED:
				marker = "- "
			case workflowpb.StepStatus_STEP_STATUS_BLOCKED:
				marker = "⏸ "
			}

			dur := ""
			if step.GetDurationMs() > 0 {
				dur = fmt.Sprintf("(%s)", (time.Duration(step.GetDurationMs()) * time.Millisecond).Round(time.Millisecond))
			}

			fmt.Printf("  %s%d. %-25s %-12s %s\n", marker, step.GetSeq(), step.GetTitle(), ss, dur)
			if step.GetErrorMessage() != "" {
				fmt.Printf("     Error: %s\n", step.GetErrorMessage())
			}
		}
	}

	if run.GetAcknowledgedBy() != "" {
		fmt.Printf("\nAcknowledged by: %s at %s\n",
			run.GetAcknowledgedBy(),
			run.GetAcknowledgedAt().AsTime().Format(time.RFC3339))
	}

	return nil
}

// --- cancel ---

var workflowCancelCmd = &cobra.Command{
	Use:   "cancel <run-id>",
	Short: "Cancel an active workflow run",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkflowCancel,
}

func runWorkflowCancel(cmd *cobra.Command, args []string) error {
	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()

	client := workflowpb.NewWorkflowServiceClient(cc)
	_, err = client.CancelRun(ctxWithTimeout(), &workflowpb.CancelRunRequest{RunId: args[0]})
	if err != nil {
		return fmt.Errorf("cancel run: %w", err)
	}

	fmt.Printf("Run %s cancelled.\n", args[0])
	return nil
}

// --- retry ---

var workflowRetryCmd = &cobra.Command{
	Use:   "retry <run-id>",
	Short: "Retry a failed workflow run",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkflowRetry,
}

func runWorkflowRetry(cmd *cobra.Command, args []string) error {
	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()

	client := workflowpb.NewWorkflowServiceClient(cc)
	resp, err := client.RetryRun(ctxWithTimeout(), &workflowpb.RetryRunRequest{RunId: args[0]})
	if err != nil {
		return fmt.Errorf("retry run: %w", err)
	}

	run := resp
	status := stripPrefix(run.GetStatus().String(), "RUN_")
	fmt.Printf("Run %s retried — status: %s\n", run.GetId(), status)
	return nil
}

// --- diagnose ---

var workflowDiagnoseCmd = &cobra.Command{
	Use:   "diagnose <run-id>",
	Short: "Diagnose why a workflow run failed",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkflowDiagnose,
}

func runWorkflowDiagnose(cmd *cobra.Command, args []string) error {
	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer cc.Close()

	client := workflowpb.NewWorkflowServiceClient(cc)
	resp, err := client.DiagnoseRun(ctxWithTimeout(), &workflowpb.DiagnoseRunRequest{RunId: args[0]})
	if err != nil {
		return fmt.Errorf("diagnose: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	fmt.Printf("Diagnosis for run %s:\n\n%s\n", args[0], resp.GetDiagnosis())
	return nil
}

func init() {
	workflowListCmd.Flags().StringVar(&workflowListService, "service", "", "Filter by service name")
	workflowListCmd.Flags().StringVar(&workflowListNode, "node", "", "Filter by node ID")
	workflowListCmd.Flags().StringVar(&workflowListStatus, "status", "", "Filter by status")
	workflowListCmd.Flags().Int32Var(&workflowListLimit, "limit", 20, "Max results")
	workflowListCmd.Flags().BoolVar(&workflowListActive, "active", false, "Active runs only")
	workflowListCmd.Flags().BoolVar(&workflowListFailed, "failed", false, "Failed runs only")

	workflowCmd.PersistentFlags().StringVar(&workflowAddr, "workflow", "", "Workflow service endpoint")

	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowGetCmd)
	workflowCmd.AddCommand(workflowDiagnoseCmd)
	workflowCmd.AddCommand(workflowCancelCmd)
	workflowCmd.AddCommand(workflowRetryCmd)

	workflowDeferStateListCmd.Flags().StringVar(&deferStateClusterID, "cluster-id", "", "Cluster id (defaults to globular.internal)")
	workflowDeferStateListCmd.Flags().BoolVar(&deferStateAbandonedOnly, "abandoned", false, "Show only ABANDONED rows")

	workflowDeferStateClearCmd.Flags().String("correlation-id", "", "Correlation id to clear (required if not given positionally)")
	workflowDeferStateClearCmd.Flags().StringVar(&deferStateClusterID, "cluster-id", "", "Cluster id (defaults to globular.internal)")
	workflowDeferStateClearCmd.Flags().StringVar(&deferStateOperator, "operator", "", "Operator label recorded with the clear (defaults to \"cli\")")

	workflowDeferStateCmd.AddCommand(workflowDeferStateListCmd)
	workflowDeferStateCmd.AddCommand(workflowDeferStateClearCmd)
	workflowCmd.AddCommand(workflowDeferStateCmd)

	rootCmd.AddCommand(workflowCmd)
}

// helpers

func stripPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func fmtTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// --- defer-state list / clear ---
//
// correlation_defer_state is the persistent counter of how many times a
// given (release, node) correlation has been deferred past its B3 budget.
// When defer_count reaches max_defers the row is marked ABANDONED and
// every subsequent dispatch attempt is refused — the row is "poison" until
// either (a) a successful run for the same correlation_id calls
// ClearOnSuccess, or (b) an operator calls ClearByOperator via these
// CLI commands.
//
// These commands wrap the WorkflowService.ClearCorrelationDeferState and
// .ListCorrelationDeferState RPCs the server already exposes. They exist
// so operators do NOT need to reach into Scylla with cqlsh — the audited
// gRPC path emits a workflow.correlation.cleared event with the operator
// principal recorded.

var (
	deferStateClusterID    string
	deferStateAbandonedOnly bool
	deferStateOperator     string
)

var workflowDeferStateCmd = &cobra.Command{
	Use:   "defer-state",
	Short: "Inspect or clear workflow correlation defer state (ABANDONED rows etc.)",
	Long: `Manage the persistent correlation_defer_state table.

A row reaches "abandoned" when its dispatch defer count hits max_defers
(default 5). Once abandoned the workflow service refuses to dispatch any
further attempts — typically the underlying blocker is permanent
(unmanaged unit, missing dependency, etc.) and the run can never succeed.

Examples:
  globular workflow defer-state list
  globular workflow defer-state list --abandoned
  globular workflow defer-state clear --correlation-id InfrastructureRelease/core@globular.io/keepalived`,
}

var workflowDeferStateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List persistent correlation defer-state rows",
	RunE:  runWorkflowDeferStateList,
}

func runWorkflowDeferStateList(cmd *cobra.Command, args []string) error {
	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()

	clusterID := strings.TrimSpace(deferStateClusterID)
	if clusterID == "" {
		clusterID = strings.TrimSpace(config.ResolveServiceAddr("globular.cluster_id", ""))
	}
	if clusterID == "" {
		clusterID = "globular.internal"
	}

	client := workflowpb.NewWorkflowServiceClient(cc)
	resp, err := client.ListCorrelationDeferState(ctxWithTimeout(), &workflowpb.ListCorrelationDeferStateRequest{
		ClusterId:     clusterID,
		AbandonedOnly: deferStateAbandonedOnly,
	})
	if err != nil {
		return fmt.Errorf("list defer state: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	if len(resp.Records) == 0 {
		if deferStateAbandonedOnly {
			fmt.Println("No abandoned correlation defer-state rows.")
		} else {
			fmt.Println("No correlation defer-state rows.")
		}
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CORRELATION_ID\tDEFER_COUNT\tABANDONED\tLAST_STEP\tLAST_REASON")
	for _, r := range resp.Records {
		reason := r.GetLastReason()
		if len(reason) > 60 {
			reason = reason[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%d/%d\t%v\t%s\t%s\n",
			r.GetCorrelationId(),
			r.GetDeferCount(),
			r.GetMaxDefers(),
			r.GetAbandoned(),
			r.GetLastStepId(),
			reason,
		)
	}
	return w.Flush()
}

var workflowDeferStateClearCmd = &cobra.Command{
	Use:   "clear --correlation-id <id>",
	Short: "Reset defer_count + abandoned flag for one correlation (audited)",
	Long: `Reset a poisoned correlation_defer_state row so the workflow service will
re-dispatch the next attempt. The clear is recorded against the operator
principal and a workflow.correlation.cleared event is published.

This does NOT touch the controller's release pipeline — it only clears
the workflow-side "give up" record. The controller will dispatch a fresh
run on its next reconcile pass; if the underlying blocker still exists,
the new run will defer again and eventually re-abandon.`,
	RunE: runWorkflowDeferStateClear,
}

func runWorkflowDeferStateClear(cmd *cobra.Command, args []string) error {
	correlationID := ""
	for _, a := range args {
		correlationID = strings.TrimSpace(a)
		break
	}
	if correlationID == "" {
		correlationID, _ = cmd.Flags().GetString("correlation-id")
		correlationID = strings.TrimSpace(correlationID)
	}
	if correlationID == "" {
		return fmt.Errorf("--correlation-id is required (or pass it as the positional arg)")
	}

	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()

	clusterID := strings.TrimSpace(deferStateClusterID)
	if clusterID == "" {
		clusterID = "globular.internal"
	}
	operator := strings.TrimSpace(deferStateOperator)
	if operator == "" {
		operator = "cli"
	}

	client := workflowpb.NewWorkflowServiceClient(cc)
	resp, err := client.ClearCorrelationDeferState(ctxWithTimeout(), &workflowpb.ClearCorrelationDeferStateRequest{
		ClusterId:     clusterID,
		CorrelationId: correlationID,
		Operator:      operator,
	})
	if err != nil {
		return fmt.Errorf("clear defer state: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	if resp.GetCleared() {
		fmt.Printf("Cleared correlation defer-state for %s (operator=%s).\n", correlationID, operator)
	} else {
		fmt.Printf("No-op: correlation %s was not present (cleared=false).\n", correlationID)
	}
	return nil
}

func workflowEndpoint() string {
	if addr := strings.TrimSpace(workflowAddr); addr != "" {
		return addr
	}
	if addr := strings.TrimSpace(config.ResolveServiceAddr("workflow.WorkflowService", "")); addr != "" {
		return addr
	}
	return rootCfg.controllerAddr
}
