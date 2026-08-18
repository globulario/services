package verifier

import (
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// service.old_pid_after_upgrade asserts "the PID that is RUNNING predates the
// apply". That claim needs a running PID. With RunningPid == 0 the service is
// stopped and ProcessStartTime is a residue of the last time it ran — firing
// the finding there reports a stale process where there is no process at all.
//
// Observed 2026-08-10 on a clean 5-node bring-up: globular-minio.service is
// stopped ON PURPOSE by the node-agent topology gate on every node outside
// ObjectStoreDesiredState.Nodes (held_not_in_topology). The verifier reported
// service.old_pid_after_upgrade (ERROR) on three such nodes simultaneously,
// each with running_pid=0 — an error raised for behaving exactly as designed,
// and one that no remediation could clear because the correct state IS stopped.
//
// Whether a stopped service SHOULD be running is owned by the liveness rules
// (node.systemd.units_running, installed_state_runtime_mismatch), which carry
// the topology/ingress waivers. This timing check must stay silent on it.

func TestVerifyTarget_StoppedService_NoOldPidFinding(t *testing.T) {
	tgt := targetFoo()
	tgt.ApplyTime = time.Now().Add(-time.Hour) // well past restartPendingWindow
	tgt.IsFirstInstall = false

	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// Service is stopped: no live PID, and systemd agrees.
		p.RunningPid = 0
		p.SystemdActiveState = "inactive"
		p.SystemdSubState = "dead"
		// Residual start time from the last run, well before the apply.
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-2 * time.Hour))
	})}

	v := VerifyTarget(tgt, ev, time.Now())

	if findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Fatalf("must NOT fire %s for a stopped service (running_pid=0): absence of a process is not a stale process; got %+v",
			FindingOldPidAfterUpgrade, v.Findings)
	}
	if findingsContain(v.Findings, FindingRestartPending) {
		t.Fatalf("must NOT fire %s for a stopped service — there is no restart in flight; got %+v",
			FindingRestartPending, v.Findings)
	}
}

// The liveness precondition must not weaken the real signal: a RUNNING PID that
// genuinely predates the apply still escalates.
func TestVerifyTarget_RunningStalePid_StillFiresOldPid(t *testing.T) {
	tgt := targetFoo()
	tgt.ApplyTime = time.Now().Add(-time.Hour)
	tgt.IsFirstInstall = false

	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.RunningPid = 4242 // alive
		p.RunningExeSha256 = hashA
		p.InstalledSha256 = hashB // running bytes differ from installed
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-2 * time.Hour))
	})}

	v := VerifyTarget(tgt, ev, time.Now())

	if !findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Fatalf("a live PID predating the apply must still fire %s; got %+v",
			FindingOldPidAfterUpgrade, v.Findings)
	}
}
