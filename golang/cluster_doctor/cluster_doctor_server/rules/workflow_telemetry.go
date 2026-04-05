package rules

import (
	"fmt"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// ============================================================================
// Workflow telemetry invariants
//
// These invariants read convergence telemetry (step outcomes, drift items,
// phase transitions) recorded by the workflow service and flag anomalies
// that indicate the control loop is NOT self-healing.
//
// They complement the static/point-in-time checks by looking at temporal
// patterns — the kind of data that lets AI propose code patches.
// ============================================================================

// --- workflow.step_failures -------------------------------------------------

// workflowStepFailures flags steps whose recent failure rate exceeds a
// threshold. A step with high failure rate usually indicates a contract
// mismatch (e.g. wrong handler name, missing config key, wrong target IP).
type workflowStepFailures struct{}

func (workflowStepFailures) ID() string       { return "workflow.step_failures" }
func (workflowStepFailures) Category() string { return "convergence" }
func (workflowStepFailures) Scope() string    { return "cluster" }

func (w workflowStepFailures) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if len(snap.StepOutcomes) == 0 {
		return nil
	}
	var findings []Finding
	const (
		minExecutions    = 5    // ignore steps with too little data
		failureThreshold = 0.10 // 10% failure rate
	)
	for _, so := range snap.StepOutcomes {
		total := so.GetTotalExecutions()
		fail := so.GetFailureCount()
		if total < minExecutions {
			continue
		}
		rate := float64(fail) / float64(total)
		if rate < failureThreshold {
			continue
		}
		entityRef := so.GetWorkflowName() + "/" + so.GetStepId()
		severity := cluster_doctorpb.Severity_SEVERITY_WARN
		if rate >= 0.50 {
			severity = cluster_doctorpb.Severity_SEVERITY_CRITICAL
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(w.ID(), entityRef, fmt.Sprintf("%.2f", rate)),
			InvariantID: w.ID(),
			Severity:    severity,
			Category:    "convergence",
			EntityRef:   entityRef,
			Summary: fmt.Sprintf("Workflow step %s has %.0f%% failure rate (%d/%d executions)",
				entityRef, rate*100, fail, total),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("workflow", "ListStepOutcomes", map[string]string{
					"workflow_name":      so.GetWorkflowName(),
					"step_id":            so.GetStepId(),
					"total_executions":   fmt.Sprintf("%d", total),
					"failure_count":      fmt.Sprintf("%d", fail),
					"success_count":      fmt.Sprintf("%d", so.GetSuccessCount()),
					"failure_rate":       fmt.Sprintf("%.2f", rate),
					"last_error_code":    so.GetLastErrorCode(),
					"last_error_message": so.GetLastErrorMessage(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Inspect step definition in the workflow YAML",
					fmt.Sprintf("mcp: workflow_get_run with step_id=%s", so.GetStepId())),
				step(2, "Review last_error_message for the root cause signal", ""),
				step(3, "If error_message indicates a contract mismatch (handler not found, unexpected input), grep the codebase for the handler name", ""),
				step(4, "If the step's inputs are missing, check the producing step's output contract", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// --- workflow.drift_stuck ---------------------------------------------------

// workflowDriftStuck flags drift items that have been observed in N or more
// consecutive reconcile cycles without being cleared. Each cycle runs
// remediation, so a persistently-present drift item means the chosen
// remediation workflow is not resolving the underlying condition.
type workflowDriftStuck struct{}

func (workflowDriftStuck) ID() string       { return "workflow.drift_stuck" }
func (workflowDriftStuck) Category() string { return "convergence" }
func (workflowDriftStuck) Scope() string    { return "cluster" }

func (w workflowDriftStuck) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if len(snap.DriftUnresolved) == 0 {
		return nil
	}
	var findings []Finding
	const stuckThreshold = 3 // 3+ consecutive cycles = stuck
	for _, d := range snap.DriftUnresolved {
		if d.GetConsecutiveCycles() < stuckThreshold {
			continue
		}
		cycles := d.GetConsecutiveCycles()
		severity := cluster_doctorpb.Severity_SEVERITY_WARN
		if cycles >= 10 {
			severity = cluster_doctorpb.Severity_SEVERITY_CRITICAL
		}
		entityRef := d.GetDriftType() + "/" + d.GetEntityRef()
		firstSeen := ""
		if d.GetFirstObservedAt() != nil {
			firstSeen = d.GetFirstObservedAt().AsTime().Format(time.RFC3339)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(w.ID(), entityRef, fmt.Sprintf("%d", cycles)),
			InvariantID: w.ID(),
			Severity:    severity,
			Category:    "convergence",
			EntityRef:   entityRef,
			Summary: fmt.Sprintf("Drift %q on %s unresolved for %d consecutive reconcile cycles",
				d.GetDriftType(), d.GetEntityRef(), cycles),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("workflow", "ListDriftUnresolved", map[string]string{
					"drift_type":          d.GetDriftType(),
					"entity_ref":          d.GetEntityRef(),
					"consecutive_cycles":  fmt.Sprintf("%d", cycles),
					"first_observed_at":   firstSeen,
					"chosen_workflow":     d.GetChosenWorkflow(),
					"last_remediation_id": d.GetLastRemediationId(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "The remediation workflow is either failing or is a no-op for this drift_type", ""),
				step(2, "Check the chosen_workflow's recent runs for errors",
					fmt.Sprintf("mcp: workflow_list_runs with workflow_name=%s", d.GetChosenWorkflow())),
				step(3, "If chosen_workflow is 'noop', add a proper remediation handler for this drift_type", ""),
				step(4, "If chosen_workflow fails, trace last_remediation_id to find the failing step", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// --- workflow.no_activity ---------------------------------------------------

// workflowNoActivity flags workflows that have not executed recently. A
// periodic workflow (e.g. cluster.reconcile every 30s) that stops running
// means either the leader crashed or the scheduler is stuck.
type workflowNoActivity struct{}

func (workflowNoActivity) ID() string       { return "workflow.no_activity" }
func (workflowNoActivity) Category() string { return "convergence" }
func (workflowNoActivity) Scope() string    { return "cluster" }

func (w workflowNoActivity) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if len(snap.WorkflowSummaries) == 0 {
		return nil
	}
	var findings []Finding
	now := time.Now()

	// Per-workflow staleness thresholds. Conservative defaults — tune once
	// we have more production telemetry.
	thresholds := map[string]time.Duration{
		"cluster.reconcile": 5 * time.Minute, // runs every 30s
	}

	for _, s := range snap.WorkflowSummaries {
		threshold, hasThreshold := thresholds[s.GetWorkflowName()]
		if !hasThreshold {
			continue // not a periodic workflow we track
		}
		var lastActive time.Time
		if ts := s.GetLastFinishedAt(); ts != nil {
			lastActive = ts.AsTime()
		}
		if lastActive.IsZero() {
			continue
		}
		age := now.Sub(lastActive)
		if age < threshold {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(w.ID(), s.GetWorkflowName(), lastActive.Format(time.RFC3339)),
			InvariantID: w.ID(),
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "convergence",
			EntityRef:   s.GetWorkflowName(),
			Summary: fmt.Sprintf("Workflow %s has not run for %s (threshold %s)",
				s.GetWorkflowName(), age.Round(time.Second), threshold),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("workflow", "ListWorkflowSummaries", map[string]string{
					"workflow_name":    s.GetWorkflowName(),
					"last_finished_at": lastActive.UTC().Format(time.RFC3339),
					"age":              age.Round(time.Second).String(),
					"threshold":        threshold.String(),
					"total_runs":       fmt.Sprintf("%d", s.GetTotalRuns()),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check if the cluster-controller leader is running",
					"globular cluster get-info"),
				step(2, "Check controller logs for scheduler errors",
					"journalctl -u globular-cluster-controller -n 100"),
				step(3, "If leadership changed recently, new leader may need to warm up", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
