package rules

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// --- workflow.blocked_runs ------------------------------------------------
//
// MC-4: Flags workflow runs that are blocked waiting for operator approval.
// This happens when:
//   - A step's resume_policy is pause_for_approval
//   - Verification was inconclusive + idempotency is manual_approval
//
// These runs will not self-heal — they require explicit operator action.

type workflowBlockedRuns struct{}

func (workflowBlockedRuns) ID() string       { return "workflow.blocked_runs" }
func (workflowBlockedRuns) Category() string { return "convergence" }
func (workflowBlockedRuns) Scope() string    { return "cluster" }

func (w workflowBlockedRuns) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if len(snap.BlockedRuns) == 0 {
		return nil
	}

	var findings []Finding
	for _, run := range snap.BlockedRuns {
		summary := fmt.Sprintf(
			"Workflow %s (run %s) is BLOCKED: requires operator approval to resume",
			run.GetWorkflowName(), run.GetId())

		if run.GetErrorMessage() != "" {
			summary += " — " + run.GetErrorMessage()
		}

		remediation := []*cluster_doctorpb.RemediationStep{
			{
				Order:       1,
				Description: fmt.Sprintf("Review blocked run: globular workflow get-run %s", run.GetId()),
				CliCommand:  fmt.Sprintf("globular workflow get-run %s", run.GetId()),
			},
			{
				Order:       2,
				Description: "Inspect the blocked step's verification result and decide whether to approve or cancel",
			},
			{
				Order:       3,
				Description: fmt.Sprintf("Approve: globular workflow approve %s", run.GetId()),
				CliCommand:  fmt.Sprintf("globular workflow approve %s", run.GetId()),
			},
			{
				Order:       4,
				Description: fmt.Sprintf("Cancel: globular workflow cancel %s", run.GetId()),
				CliCommand:  fmt.Sprintf("globular workflow cancel %s", run.GetId()),
			},
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("workflow.blocked_runs", run.GetId(), "blocked"),
			InvariantID: w.ID(),
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    w.Category(),
			EntityRef:   "workflow/" + run.GetWorkflowName() + "/" + run.GetId(),
			Summary:     summary,
			Remediation: remediation,
		})
	}

	return findings
}
