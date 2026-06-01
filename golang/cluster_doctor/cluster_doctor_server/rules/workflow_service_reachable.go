package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// --- workflow.service_unavailable -------------------------------------------
//
// Fires when the doctor collector cannot reach the workflow service — one or
// more workflow RPCs returned an Unavailable-class error during snapshot
// collection.
//
// This is distinct from release.blocked_workflow_unavailable (which is an
// aggregate metric fired when releases are in transient retry backoff for any
// transient reason). workflow.service_unavailable fires on a DIRECT observation
// by the doctor: the workflow gRPC endpoint did not respond.
//
// Self-healing note: when this finding fires, the controller is still retrying
// workflow dispatches (classifyWorkflowError treats workflow_unavailable as
// transient, never permanent). If globular-workflow.service recovers — either
// via systemd Restart=on-failure or via the DEGRADED→reinstall pipeline — the
// controller will dispatch the next pending workflow automatically once the
// service is back. No manual action is required unless the service stays down
// beyond a few minutes.

type workflowServiceReachable struct{}

func (workflowServiceReachable) ID() string       { return "workflow.service_unavailable" }
func (workflowServiceReachable) Category() string { return "control_plane" }
func (workflowServiceReachable) Scope() string    { return "cluster" }

func (w workflowServiceReachable) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	for _, de := range snap.DataErrors {
		if de.Service != "workflow" {
			continue
		}
		if !isWorkflowUnavailableErr(de.Err) {
			continue
		}
		return []Finding{{
			FindingID:   FindingID(w.ID(), "cluster", "workflow"),
			InvariantID: w.ID(),
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    w.Category(),
			EntityRef:   "workflow",
			Summary: fmt.Sprintf(
				"Workflow service is unreachable — %s RPC failed: %v", de.RPC, de.Err),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("collector", de.RPC, map[string]string{
					"service": de.Service,
					"rpc":     de.RPC,
					"error":   de.Err.Error(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check workflow service status on all control-plane nodes",
					"globular cluster get-health"),
				step(2, "View recent workflow service logs",
					"journalctl -u globular-workflow -n 100"),
				step(3, "If inactive or failed, verify the service certificate is present — a missing cert prevents ExecStartPre from passing",
					"globular cluster get-node-health-detail <node>"),
				step(4, "If the certificate is present and the unit still fails, check for etcd connectivity from the workflow node",
					"journalctl -u globular-workflow -n 50 | grep etcd"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		}}
	}
	return nil
}

// isWorkflowUnavailableErr returns true for errors that indicate the workflow
// service is not listening or actively refusing connections. It intentionally
// excludes transient timeout errors (DeadlineExceeded) and protocol-level
// failures that mean the service IS running but rejected the request.
func isWorkflowUnavailableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "Unavailable") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "transport: Error while dialing")
}
