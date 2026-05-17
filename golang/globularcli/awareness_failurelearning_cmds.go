package main

// awareness_failurelearning_cmds.go — stubs after failurelearning package was removed
// from standalone awareness module. The failure-learning CLI commands are not available
// in this build. Use the MCP tools 'awareness_failure_learning_*' instead.

import (
	"fmt"

	"github.com/spf13/cobra"
)

var flCfg = struct {
	proposalID   string
	reviewer     string
	notes        string
	reason       string
	incidentID   string
	sourceType   string
	sourceID     string
	errorMsg     string
	rootCause    string
	resolution   string
	proof        string
	hasRootCause bool
	hasResolution bool
	hasProof     bool
	dryRun       bool
	jsonOutput   bool
}{}

var failureLearningCmd = &cobra.Command{
	Use:   "failure-learning",
	Short: "Failure Graph Learning Loop (not available — use MCP tools awareness_failure_learning_*)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("failure-learning is not available: failurelearning package removed — use MCP tools awareness_failure_learning_* instead")
	},
}

func makeFLStub(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short + " (not available)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("failure-learning %s is not available: failurelearning package removed — use MCP tools awareness_failure_learning_* instead", use)
		},
	}
}

var flProposeCmd = makeFLStub("propose", "Propose a failure graph update")
var flProposeIncidentCmd = makeFLStub("propose-incident", "Propose update from an incident")
var flListPendingCmd = makeFLStub("list-pending", "List pending proposals")
var flShowCmd = makeFLStub("show", "Show a proposal")
var flApproveCmd = makeFLStub("approve", "Approve a proposal")
var flRejectCmd = makeFLStub("reject", "Reject a proposal")
var flDeferCmd = makeFLStub("defer", "Defer a proposal")
var flApplyCmd = makeFLStub("apply", "Apply an approved proposal")
var flSyncSeedsCmd = makeFLStub("sync-seeds", "Export seeds from approved proposals")
var flCheckClosureCmd = makeFLStub("check-closure", "Check closure record completeness")

func init() {
	awarenessCmd.AddCommand(failureLearningCmd)
	failureLearningCmd.AddCommand(
		flProposeCmd,
		flProposeIncidentCmd,
		flListPendingCmd,
		flShowCmd,
		flApproveCmd,
		flRejectCmd,
		flDeferCmd,
		flApplyCmd,
		flSyncSeedsCmd,
		flCheckClosureCmd,
	)

	// Minimal flags to avoid "unknown flag" errors from scripts.
	flProposeCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID")
	flProposeCmd.Flags().StringVar(&flCfg.sourceType, "source-type", "", "Source type: incident|session")
	flProposeCmd.Flags().StringVar(&flCfg.sourceID, "source-id", "", "Source ID")
	flProposeCmd.Flags().StringVar(&flCfg.errorMsg, "error", "", "Error message")
	flProposeCmd.Flags().StringVar(&flCfg.rootCause, "cause", "", "Root cause")
	flProposeCmd.Flags().StringVar(&flCfg.resolution, "resolution", "", "Resolution")
	flProposeCmd.Flags().StringVar(&flCfg.proof, "proof", "", "Proof of fix")
	flProposeCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "Output JSON")

	flProposeIncidentCmd.Flags().StringVar(&flCfg.incidentID, "incident", "", "Incident ID (required)")

	flListPendingCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "Output JSON")

	flShowCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID (required)")
	flShowCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "Output JSON")

	flApproveCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID (required)")
	flApproveCmd.Flags().StringVar(&flCfg.reviewer, "reviewer", "", "Reviewer (required)")
	flApproveCmd.Flags().StringVar(&flCfg.notes, "notes", "", "Optional notes")

	flRejectCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID (required)")
	flRejectCmd.Flags().StringVar(&flCfg.reviewer, "reviewer", "", "Reviewer (required)")
	flRejectCmd.Flags().StringVar(&flCfg.reason, "reason", "", "Rejection reason (required)")

	flDeferCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID (required)")
	flDeferCmd.Flags().StringVar(&flCfg.reviewer, "reviewer", "", "Reviewer (required)")
	flDeferCmd.Flags().StringVar(&flCfg.reason, "reason", "", "Deferral reason")

	flApplyCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID (required)")
	flApplyCmd.Flags().BoolVar(&flCfg.dryRun, "dry-run", false, "Dry run only")

	flSyncSeedsCmd.Flags().BoolVar(&flCfg.dryRun, "dry-run", false, "Dry run only")

	flCheckClosureCmd.Flags().StringVar(&flCfg.proposalID, "closure", "", "Closure ID (required)")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.hasRootCause, "has-root-cause", false, "Closure has root cause")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.hasResolution, "has-resolution", false, "Closure has resolution")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.hasProof, "has-proof", false, "Closure has proof")
}
