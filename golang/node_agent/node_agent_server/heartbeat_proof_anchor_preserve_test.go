// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.heartbeat.proof_anchor_preserve
// @awareness file_role=regression_tests_for_heartbeat_must_not_stomp_proof_writer_updated_unix_anchor
// @awareness protects=globular.platform:forbidden_fix.use_wall_clock_for_installed_unix_timestamp
// @awareness risk=high
package main

import (
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// These regression tests pin the contract for heartbeat.go's Phase 2
// existing-record handling:
//
//   For services on the selfHostedServiceNames allowlist (node-agent,
//   cluster-controller, cluster-doctor, repository), the heartbeat MUST
//   NOT overwrite UpdatedUnix with time.Now(). The proof writer
//   (self_hosted_runtime_proof_writer.go) anchors UpdatedUnix to the
//   running PID's start time; the verifier compares
//   max(InstalledUnix, UpdatedUnix) against that PID start. Wall-clock
//   now is always > PID start, so any heartbeat-side overwrite produces
//   a permanent service.old_pid_after_upgrade finding.
//
// Live regression that produced this failure mode: 2026-06-04 cluster.
// Cluster-controller's installed_state had UpdatedUnix=07:45:31 (heartbeat
// wall-clock) while PID started at 07:03:43. Verifier flagged
// service.old_pid_after_upgrade every cycle even though the binary on
// disk matched the manifest's entrypoint_checksum and was actually running.
//
// See:
//   - forbidden_fix:use_wall_clock_for_installed_unix_timestamp
//   - failure_mode:service.old_pid_after_upgrade
//   - failure_mode:heartbeat.stale_kind_manifest_poisons_installed_state
//   - incident_pattern:pat.heartbeat_stale_kind_manifest_old_pid_loop

// applyHeartbeatPhase2ExistingMerge mirrors the exact merge logic added to
// heartbeat.go Phase 2 at the `if existing != nil` block, isolated so it
// can be unit-tested without standing up a full server.
//
// MUST stay in lock-step with the production code. If a future contributor
// changes the merge policy in heartbeat.go, this helper has to be updated
// or the test will report stale behaviour.
func applyHeartbeatPhase2ExistingMerge(pkg, existing *node_agentpb.InstalledPackage, name string) {
	if existing == nil {
		return
	}
	pkg.InstalledUnix = existing.GetInstalledUnix()
	if existing.GetBuildId() != "" {
		pkg.BuildId = existing.GetBuildId()
	}
	if selfHostedServiceNames[name] {
		pkg.UpdatedUnix = existing.GetUpdatedUnix()
	}
}

// TestHeartbeatPhase2_SelfHosted_PreservesUpdatedUnix is the headline
// regression. Cluster-controller (on the allowlist) with an existing
// record whose UpdatedUnix is anchored to PID start must KEEP that anchor
// after the heartbeat merge — never wall-clock now.
func TestHeartbeatPhase2_SelfHosted_PreservesUpdatedUnix(t *testing.T) {
	const (
		pidStart  int64 = 1780571023 // 07:03:43 EDT (matches the live cluster's cluster-controller PID start)
		wallClock int64 = 1780575931 // 07:45:31 EDT (the buggy heartbeat-stamped value before the fix)
	)
	existing := &node_agentpb.InstalledPackage{
		Name:          "cluster-controller",
		Kind:          "SERVICE",
		Version:       "1.2.160",
		BuildId:       "019e924c-d699-7068-b03d-d52a0ab545ff",
		InstalledUnix: pidStart,
		UpdatedUnix:   pidStart, // proof writer anchored this correctly
	}
	pkg := &node_agentpb.InstalledPackage{
		Name:          "cluster-controller",
		Kind:          "SERVICE",
		Version:       "1.2.160",
		InstalledUnix: wallClock, // heartbeat default — would be overridden by merge
		UpdatedUnix:   wallClock, // heartbeat default — MUST be overridden for self-hosted
	}

	applyHeartbeatPhase2ExistingMerge(pkg, existing, "cluster-controller")

	if pkg.UpdatedUnix != pidStart {
		t.Fatalf("UpdatedUnix = %d, want %d (PID-start anchor preserved). Heartbeat stomped the proof writer's anchor; verifier will flag service.old_pid_after_upgrade.",
			pkg.UpdatedUnix, pidStart)
	}
	if pkg.UpdatedUnix == wallClock {
		t.Fatalf("CRITICAL: heartbeat overwrote UpdatedUnix with wall-clock %d — exactly the forbidden_fix use_wall_clock_for_installed_unix_timestamp.",
			wallClock)
	}
	// InstalledUnix is preserved unconditionally — sanity-check it still is.
	if pkg.InstalledUnix != pidStart {
		t.Fatalf("InstalledUnix = %d, want preserved %d", pkg.InstalledUnix, pidStart)
	}
}

// TestHeartbeatPhase2_AllSelfHostedServices_PreserveUpdatedUnix iterates
// the entire allowlist so a future contributor adding a new self-hosted
// service to selfHostedServiceNames (in self_hosted_runtime_proof_writer.go)
// automatically gets covered.
func TestHeartbeatPhase2_AllSelfHostedServices_PreserveUpdatedUnix(t *testing.T) {
	const (
		pidStart  int64 = 1780571023
		wallClock int64 = 1780575931
	)
	for name := range selfHostedServiceNames {
		t.Run(name, func(t *testing.T) {
			existing := &node_agentpb.InstalledPackage{
				Name:          name,
				InstalledUnix: pidStart,
				UpdatedUnix:   pidStart,
			}
			pkg := &node_agentpb.InstalledPackage{
				Name:          name,
				InstalledUnix: wallClock,
				UpdatedUnix:   wallClock,
			}
			applyHeartbeatPhase2ExistingMerge(pkg, existing, name)
			if pkg.UpdatedUnix != pidStart {
				t.Fatalf("self-hosted %q: UpdatedUnix = %d, want %d (preserved)",
					name, pkg.UpdatedUnix, pidStart)
			}
		})
	}
}

// TestHeartbeatPhase2_NonSelfHosted_StillUpdatesUpdatedUnix confirms the
// fix is scoped to the allowlist. Ordinary services (dns, file, log,
// authentication, etc.) continue to get UpdatedUnix bumped on heartbeat
// merges — that is the long-standing behaviour and is correct because
// the proof writer does not run for them.
func TestHeartbeatPhase2_NonSelfHosted_StillUpdatesUpdatedUnix(t *testing.T) {
	const (
		anyPriorValue int64 = 1780571023
		nowValue      int64 = 1780575931
	)
	existing := &node_agentpb.InstalledPackage{
		Name:          "dns",
		InstalledUnix: anyPriorValue,
		UpdatedUnix:   anyPriorValue,
	}
	pkg := &node_agentpb.InstalledPackage{
		Name:          "dns",
		InstalledUnix: nowValue,
		UpdatedUnix:   nowValue, // wall-clock now; this should NOT be overridden for non-self-hosted
	}

	applyHeartbeatPhase2ExistingMerge(pkg, existing, "dns")

	if pkg.UpdatedUnix != nowValue {
		t.Fatalf("non-self-hosted %q: UpdatedUnix should remain %d (wall-clock now); got %d. The fix must NOT change behaviour for ordinary services.",
			"dns", nowValue, pkg.UpdatedUnix)
	}
	// InstalledUnix preserved for ALL services (existing behaviour).
	if pkg.InstalledUnix != anyPriorValue {
		t.Fatalf("non-self-hosted: InstalledUnix should still be preserved = %d, got %d",
			anyPriorValue, pkg.InstalledUnix)
	}
}

// TestHeartbeatPhase2_NoExisting_NoMergeChange covers the brand-new-record
// path: when existing is nil, the merge function is a no-op and the
// heartbeat's wall-clock defaults stand. The proof writer will anchor the
// record on its first run after this.
func TestHeartbeatPhase2_NoExisting_NoMergeChange(t *testing.T) {
	const nowValue int64 = 1780575931
	pkg := &node_agentpb.InstalledPackage{
		Name:          "cluster-controller",
		InstalledUnix: nowValue,
		UpdatedUnix:   nowValue,
	}
	applyHeartbeatPhase2ExistingMerge(pkg, nil, "cluster-controller")
	if pkg.UpdatedUnix != nowValue || pkg.InstalledUnix != nowValue {
		t.Fatalf("no-existing path must leave pkg unchanged; got Installed=%d Updated=%d, want both=%d",
			pkg.InstalledUnix, pkg.UpdatedUnix, nowValue)
	}
}
