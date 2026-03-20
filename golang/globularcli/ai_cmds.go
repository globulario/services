package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

var aiExecutorAddr string

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "Manage AI incidents and remediation",
	Long: `Interact with the AI executor service to view incidents, diagnoses,
and approve or deny pending remediation actions.

Examples:
  globular ai status
  globular ai list
  globular ai show <incident-id>
  globular ai approve <incident-id>
  globular ai deny <incident-id> --reason "not safe"
  globular ai retry <incident-id>
`,
}

// --- status ---

var aiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show AI executor operational status",
	RunE:  runAiStatus,
}

func runAiStatus(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(aiExecutorAddr)
	if err != nil {
		return fmt.Errorf("connect to ai_executor: %w", err)
	}
	defer cc.Close()

	client := ai_executorpb.NewAiExecutorServiceClient(cc)
	resp, err := client.GetStatus(ctxWithTimeout(), &ai_executorpb.GetStatusRequest{})
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	uptime := time.Duration(resp.GetUptimeSeconds()) * time.Second
	fmt.Printf("AI Executor Status\n")
	fmt.Printf("  Uptime:              %s\n", uptime)
	fmt.Printf("  Incidents processed: %d\n", resp.GetIncidentsProcessed())
	fmt.Printf("  Diagnoses completed: %d\n", resp.GetDiagnosesCompleted())
	fmt.Printf("  Actions executed:    %d\n", resp.GetActionsExecuted())
	fmt.Printf("  Actions failed:      %d\n", resp.GetActionsFailed())
	return nil
}

// --- list ---

var (
	aiListState string
	aiListLimit int32
)

var aiListCmd = &cobra.Command{
	Use:   "list",
	Short: "List AI incidents/jobs",
	Long: `List AI incident jobs. Filter by state with --state.

States: DETECTED, DIAGNOSING, DIAGNOSED, EXECUTING, SUCCEEDED,
        FAILED, AWAITING_APPROVAL, APPROVED, DENIED, EXPIRED, CLOSED

Examples:
  globular ai list
  globular ai list --state AWAITING_APPROVAL
  globular ai list --state FAILED --limit 5
`,
	RunE: runAiList,
}

func runAiList(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(aiExecutorAddr)
	if err != nil {
		return fmt.Errorf("connect to ai_executor: %w", err)
	}
	defer cc.Close()

	client := ai_executorpb.NewAiExecutorServiceClient(cc)

	req := &ai_executorpb.ListJobsRequest{
		Limit: aiListLimit,
	}
	if aiListState != "" {
		if v, ok := ai_executorpb.JobState_value["JOB_"+strings.ToUpper(aiListState)]; ok {
			req.StateFilter = ai_executorpb.JobState(v)
		} else {
			return fmt.Errorf("unknown state %q (use DETECTED, DIAGNOSING, DIAGNOSED, EXECUTING, SUCCEEDED, FAILED, AWAITING_APPROVAL, APPROVED, DENIED, EXPIRED, CLOSED)", aiListState)
		}
	}

	resp, err := client.ListJobs(ctxWithTimeout(), req)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	if len(resp.GetJobs()) == 0 {
		fmt.Println("No incidents found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "INCIDENT\tSTATE\tTIER\tROOT CAUSE\tACTION\tAGE")
	for _, j := range resp.GetJobs() {
		age := ""
		if j.GetCreatedAtMs() > 0 {
			age = time.Since(time.UnixMilli(j.GetCreatedAtMs())).Round(time.Second).String()
		}
		tier := tierName(j.GetTier())
		rootCause := ""
		action := ""
		if d := j.GetDiagnosis(); d != nil {
			rootCause = d.GetRootCause()
			action = d.GetProposedAction()
		}
		// Truncate long fields
		if len(rootCause) > 30 {
			rootCause = rootCause[:27] + "..."
		}
		if len(action) > 25 {
			action = action[:22] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			j.GetIncidentId(),
			strings.TrimPrefix(j.GetState().String(), "JOB_"),
			tier,
			rootCause,
			action,
			age,
		)
	}
	w.Flush()
	return nil
}

// --- show ---

var aiShowCmd = &cobra.Command{
	Use:   "show <incident-id>",
	Short: "Show incident details, diagnosis, and proposed action",
	Args:  cobra.ExactArgs(1),
	RunE:  runAiShow,
}

func runAiShow(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(aiExecutorAddr)
	if err != nil {
		return fmt.Errorf("connect to ai_executor: %w", err)
	}
	defer cc.Close()

	client := ai_executorpb.NewAiExecutorServiceClient(cc)
	resp, err := client.GetJob(ctxWithTimeout(), &ai_executorpb.GetJobRequest{
		IncidentId: args[0],
	})
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	j := resp.GetJob()
	if j == nil {
		return fmt.Errorf("incident %s not found", args[0])
	}

	fmt.Printf("Incident:  %s\n", j.GetIncidentId())
	fmt.Printf("State:     %s\n", strings.TrimPrefix(j.GetState().String(), "JOB_"))
	fmt.Printf("Tier:      %s\n", tierName(j.GetTier()))
	fmt.Printf("Attempts:  %d\n", j.GetAttempts())
	if j.GetCreatedAtMs() > 0 {
		fmt.Printf("Created:   %s\n", time.UnixMilli(j.GetCreatedAtMs()).Format(time.RFC3339))
	}
	if j.GetApprovedBy() != "" {
		fmt.Printf("Approved:  %s at %s\n", j.GetApprovedBy(), time.UnixMilli(j.GetApprovedAtMs()).Format(time.RFC3339))
	}
	if j.GetDeniedBy() != "" {
		fmt.Printf("Denied:    %s — %s\n", j.GetDeniedBy(), j.GetDeniedReason())
	}
	if j.GetResult() != "" {
		fmt.Printf("Result:    %s\n", j.GetResult())
	}
	if j.GetError() != "" {
		fmt.Printf("Error:     %s\n", j.GetError())
	}

	if d := j.GetDiagnosis(); d != nil {
		fmt.Printf("\n--- Diagnosis ---\n")
		fmt.Printf("Root Cause:      %s\n", d.GetRootCause())
		fmt.Printf("Confidence:      %.0f%%\n", d.GetConfidence()*100)
		fmt.Printf("Summary:         %s\n", d.GetSummary())
		if d.GetDetail() != "" {
			fmt.Printf("Detail:          %s\n", d.GetDetail())
		}
		fmt.Printf("Proposed Action: %s\n", d.GetProposedAction())
		if d.GetActionReason() != "" {
			fmt.Printf("Rationale:       %s\n", d.GetActionReason())
		}
		if len(d.GetEvidence()) > 0 {
			fmt.Printf("Evidence:\n")
			for _, e := range d.GetEvidence() {
				fmt.Printf("  - %s\n", e)
			}
		}
	}

	return nil
}

// --- approve ---

var aiApproveCmd = &cobra.Command{
	Use:   "approve <incident-id>",
	Short: "Approve a pending Tier 2 action",
	Args:  cobra.ExactArgs(1),
	RunE:  runAiApprove,
}

func runAiApprove(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(aiExecutorAddr)
	if err != nil {
		return fmt.Errorf("connect to ai_executor: %w", err)
	}
	defer cc.Close()

	client := ai_executorpb.NewAiExecutorServiceClient(cc)
	resp, err := client.ApproveAction(ctxWithTimeout(), &ai_executorpb.ApproveActionRequest{
		IncidentId: args[0],
		ApprovedBy: "cli",
	})
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}

	j := resp.GetJob()
	fmt.Printf("Approved incident %s — state: %s\n", j.GetIncidentId(),
		strings.TrimPrefix(j.GetState().String(), "JOB_"))
	return nil
}

// --- deny ---

var aiDenyReason string

var aiDenyCmd = &cobra.Command{
	Use:   "deny <incident-id>",
	Short: "Deny a pending Tier 2 action",
	Args:  cobra.ExactArgs(1),
	RunE:  runAiDeny,
}

func runAiDeny(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(aiExecutorAddr)
	if err != nil {
		return fmt.Errorf("connect to ai_executor: %w", err)
	}
	defer cc.Close()

	client := ai_executorpb.NewAiExecutorServiceClient(cc)
	resp, err := client.DenyAction(ctxWithTimeout(), &ai_executorpb.DenyActionRequest{
		IncidentId: args[0],
		DeniedBy:   "cli",
		Reason:     aiDenyReason,
	})
	if err != nil {
		return fmt.Errorf("deny: %w", err)
	}

	j := resp.GetJob()
	fmt.Printf("Denied incident %s — state: %s\n", j.GetIncidentId(),
		strings.TrimPrefix(j.GetState().String(), "JOB_"))
	return nil
}

// --- retry ---

var aiRetryCmd = &cobra.Command{
	Use:   "retry <incident-id>",
	Short: "Retry a failed action",
	Args:  cobra.ExactArgs(1),
	RunE:  runAiRetry,
}

func runAiRetry(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(aiExecutorAddr)
	if err != nil {
		return fmt.Errorf("connect to ai_executor: %w", err)
	}
	defer cc.Close()

	client := ai_executorpb.NewAiExecutorServiceClient(cc)
	resp, err := client.RetryAction(ctxWithTimeout(), &ai_executorpb.RetryActionRequest{
		IncidentId: args[0],
	})
	if err != nil {
		return fmt.Errorf("retry: %w", err)
	}

	j := resp.GetJob()
	fmt.Printf("Retrying incident %s — state: %s\n", j.GetIncidentId(),
		strings.TrimPrefix(j.GetState().String(), "JOB_"))
	return nil
}

// --- init ---

func init() {
	aiCmd.PersistentFlags().StringVar(&aiExecutorAddr, "executor", "localhost:10230", "AI executor service address")

	aiListCmd.Flags().StringVar(&aiListState, "state", "", "Filter by job state")
	aiListCmd.Flags().Int32Var(&aiListLimit, "limit", 20, "Max results")

	aiDenyCmd.Flags().StringVar(&aiDenyReason, "reason", "", "Reason for denial")

	aiCmd.AddCommand(aiStatusCmd)
	aiCmd.AddCommand(aiListCmd)
	aiCmd.AddCommand(aiShowCmd)
	aiCmd.AddCommand(aiApproveCmd)
	aiCmd.AddCommand(aiDenyCmd)
	aiCmd.AddCommand(aiRetryCmd)

	rootCmd.AddCommand(aiCmd)
}

// --- helpers ---

func tierName(tier int32) string {
	switch tier {
	case 0:
		return "observe"
	case 1:
		return "auto-fix"
	case 2:
		return "approval"
	default:
		return fmt.Sprintf("tier-%d", tier)
	}
}
