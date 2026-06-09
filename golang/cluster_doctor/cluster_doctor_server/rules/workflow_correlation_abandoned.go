// workflow_correlation_abandoned.go — WF-DEFER B3 doctor rule.
//
// Surfaces workflow correlation_ids that have been auto-abandoned by
// the workflow_server after hitting max_defers. Source data:
// Snapshot.AbandonedDeferCorrelations (populated by the collector via
// WorkflowService.ListCorrelationDeferState with abandoned_only=true).
//
// Each abandoned row produces one finding with:
//   severity: ERROR — auto-retry has stopped
//   summary:  "release X abandoned after N/M defers on step Y"
//   evidence: counter, max, last reason, last blocker tags, abandonedAt
//   remediation: investigate the blocker, then clear via
//                WorkflowService.ClearCorrelationDeferState
//
// This rule is the operator-visible side of B3. The pre-flight invariants
// `convergence.no_infinite_retry`, `services.drift_must_age_and_escalate`,
// and the code-smell "circuit state not visible in doctor" are why the
// finding exists.
package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type workflowCorrelationAbandoned struct{}

func (workflowCorrelationAbandoned) ID() string       { return "workflow.correlation.abandoned" }
func (workflowCorrelationAbandoned) Category() string { return "workflow" }
func (workflowCorrelationAbandoned) Scope() string    { return "cluster" }

func (workflowCorrelationAbandoned) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	// Workflow backend unreachable → empty AbandonedDeferCorrelations means
	// "unknown", not "none abandoned". Refuse; the registry surfaces the source.
	if snap == nil || snap.HadError("workflow", "ListCorrelationDeferState(abandoned)") {
		return nil
	}
	if len(snap.AbandonedDeferCorrelations) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(snap.AbandonedDeferCorrelations))
	for _, rec := range snap.AbandonedDeferCorrelations {
		corrID := rec.GetCorrelationId()
		if corrID == "" {
			continue
		}
		summary := fmt.Sprintf(
			"workflow correlation %s abandoned after %d/%d defers on step %s — auto-retry stopped",
			corrID, rec.GetDeferCount(), rec.GetMaxDefers(), rec.GetLastStepId(),
		)

		ageDesc := ""
		if rec.GetAbandonedAt() != nil && !rec.GetAbandonedAt().AsTime().IsZero() {
			age := time.Since(rec.GetAbandonedAt().AsTime()).Round(time.Second)
			ageDesc = fmt.Sprintf(" (abandoned %s ago)", age)
		}

		evidence := []*cluster_doctorpb.Evidence{
			kvEvidence("workflow", "ListCorrelationDeferState", map[string]string{
				"correlation_id":      corrID,
				"defer_count":         fmt.Sprintf("%d", rec.GetDeferCount()),
				"max_defers":          fmt.Sprintf("%d", rec.GetMaxDefers()),
				"last_step_id":        rec.GetLastStepId(),
				"last_reason":         rec.GetLastReason(),
				"last_blocker_tags":   strings.Join(rec.GetLastBlockerTags(), ","),
				"last_defer_until_ms": fmt.Sprintf("%d", rec.GetLastDeferUntilMs()),
			}),
		}

		remediation := []*cluster_doctorpb.RemediationStep{
			step(1,
				"Investigate the underlying blocker. last_step_id and last_reason "+
					"point to the step that exhausted retries; last_blocker_tags lists the "+
					"runtime/dependency conditions that the engine flagged as the cause.",
				""),
			step(2,
				"Once the blocker is addressed (or the operator accepts the risk), "+
					"clear the abandonment so dispatch resumes:",
				fmt.Sprintf("grpcurl -d '{\"cluster_id\":\"<cluster>\",\"correlation_id\":\"%s\",\"operator\":\"<you>\"}' "+
					"<workflow_addr> workflow.WorkflowService/ClearCorrelationDeferState", corrID)),
			step(3,
				"If the blocker is permanent (e.g. topology gate that won't be met), "+
					"consider removing the desired record itself rather than clearing — "+
					"clearing without fixing the cause will just re-abandon after the same N defers.",
				""),
		}

		out = append(out, Finding{
			FindingID:       FindingID("workflow.correlation.abandoned", corrID, fmt.Sprintf("%d", rec.GetDeferCount())),
			InvariantID:     "workflow.correlation.abandoned",
			Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:        "workflow",
			EntityRef:       corrID,
			Summary:         summary + ageDesc,
			Evidence:        evidence,
			Remediation:     remediation,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return out
}
