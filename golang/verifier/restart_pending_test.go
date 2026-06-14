package verifier

import (
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Restart-pending hardening: an upgrade whose live PID is ALREADY serving the
// current on-disk binary (running entrypoint == installed entrypoint) but whose
// start predates ApplyTime is a transient self-hosted restart window, not stale
// bytes. Within restartPendingWindow it surfaces as degraded
// service.restart_pending, NOT critical service.old_pid_after_upgrade.

func TestVerifyTarget_RecentUpgradeRunningMatchesInstalled_RestartPending(t *testing.T) {
	tgt := targetFoo()
	tgt.ApplyTime = time.Now().Add(-time.Minute) // fresh apply, inside the window
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		// Live PID serves the current on-disk binary (running == installed)…
		p.InstalledSha256 = hashA
		p.RunningExeSha256 = hashA
		// …but its start predates the apply by more than applyGraceWindow.
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-time.Hour))
	})}
	v := VerifyTarget(tgt, ev, time.Now())

	if findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Errorf("must NOT fire critical old_pid_after_upgrade when running==installed inside the window; got %+v", v.Findings)
	}
	if !findingsContain(v.Findings, FindingRestartPending) {
		t.Fatalf("missing %s finding; got %+v", FindingRestartPending, v.Findings)
	}
	var sev string
	for _, f := range v.Findings {
		if f.ID == FindingRestartPending {
			sev = f.Severity
		}
	}
	if sev != SeverityDegraded {
		t.Errorf("restart_pending severity=%q want=%q", sev, SeverityDegraded)
	}
	if v.ProofStatus == ProofMismatch {
		t.Errorf("a transient restart must not cap the verdict at mismatch; got %q", v.ProofStatus)
	}
}

// Past the window, a PID still predating ApplyTime is a restart that genuinely
// did not take — it escalates to critical old_pid_after_upgrade.
func TestVerifyTarget_StuckRestartBeyondWindow_EscalatesToOldPid(t *testing.T) {
	tgt := targetFoo()
	tgt.ApplyTime = time.Now().Add(-(restartPendingWindow + time.Minute)) // past the window
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.InstalledSha256 = hashA
		p.RunningExeSha256 = hashA
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-time.Hour))
	})}
	v := VerifyTarget(tgt, ev, time.Now())

	if findingsContain(v.Findings, FindingRestartPending) {
		t.Errorf("restart_pending must NOT persist past the window; got %+v", v.Findings)
	}
	if !findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Fatalf("missing %s after the window expires; got %+v", FindingOldPidAfterUpgrade, v.Findings)
	}
}

// Genuine stale bytes (running != installed) is NEVER downgraded — the
// authoritative running-vs-installed hash signal proves the PID is serving old
// bytes, so it stays critical even within the fresh-apply window.
func TestVerifyTarget_RecentUpgradeRunningDiffersFromInstalled_StaysCritical(t *testing.T) {
	tgt := targetFoo()
	tgt.ApplyTime = time.Now().Add(-time.Minute) // fresh apply, but bytes are stale
	ev := Evidence{Proof: proofMatching(tgt, func(p *node_agentpb.ServiceRuntimeProof) {
		p.InstalledSha256 = hashA  // new binary on disk…
		p.RunningExeSha256 = hashB // …old PID still serving old bytes
		p.ProcessStartTime = timestamppb.New(tgt.ApplyTime.Add(-time.Hour))
	})}
	v := VerifyTarget(tgt, ev, time.Now())

	if findingsContain(v.Findings, FindingRestartPending) {
		t.Errorf("restart_pending must NOT fire when running bytes differ from installed; got %+v", v.Findings)
	}
	if !findingsContain(v.Findings, FindingOldPidAfterUpgrade) {
		t.Errorf("missing %s for a genuinely stale PID; got %+v", FindingOldPidAfterUpgrade, v.Findings)
	}
	if !findingsContain(v.Findings, FindingRunningBinaryHashMismatch) {
		t.Errorf("missing %s when running!=installed; got %+v", FindingRunningBinaryHashMismatch, v.Findings)
	}
	if v.ProofStatus != ProofMismatch {
		t.Errorf("ProofStatus=%q want=%q", v.ProofStatus, ProofMismatch)
	}
}
