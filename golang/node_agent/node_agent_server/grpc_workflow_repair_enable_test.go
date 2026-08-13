package main

// grpc_workflow_repair_enable_test.go — Regression test for the
// repair-via-Start path reporting a convergence that does not survive a reboot.
//
// install-package has a fast path: when installed_state already matches the
// desired build but the unit is loaded-but-inactive (installSkipDeniedInactive),
// the agent starts the unit instead of reinstalling the package, then reports
// Status="SUCCEEDED" and emits a ConvergenceResultV1.
//
// The usual reason a DESIRED unit is loaded-but-inactive is the crash-loop
// suppressor in event_publisher.go, which `systemctl disable`s the unit after
// repeated crashes. A bare Start repairs the running process but leaves the
// unit disabled, so the next reboot brings it back inactive — the SUCCEEDED
// convergence is not durable (intent:install.result_requires_durable_commit).
//
// Observed 2026-08-09 in the globular-quickstart 5-node simulation: after the
// containers were restarted, node-2 and node-4 came up with ~20 desired units
// `inactive` and `disabled`; cluster-doctor reported 34x
// installed_state_runtime_mismatch and 32x node.systemd.units_running. The
// agent then walked the packages one at a time logging "repair via Start
// succeeded" — the same repair it had already performed before the restart,
// because none of those repairs had ever re-enabled the unit.
//
// apply_package_release.go already had this right, with the reason spelled out
// at its supervisor.Enable call: "Crash-loop suppression disables units via
// systemctl disable; without re-enabling here, the unit stays disabled and
// won't auto-start on reboot." The fast path bypassed that path entirely.
//
// forbidden fix: satisfying this by calling systemctl directly. Mutating unit
// actions in node-agent must go through internal/supervisor/ (CLAUDE.md §6,
// enforced by `make check-nodeagent-exec-boundary`).

import (
	"os"
	"strings"
	"testing"
)

// inactiveRepairBranch returns the source of the installSkipDeniedInactive
// case in grpc_workflow.go, bounded to that branch.
func inactiveRepairBranch(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("grpc_workflow.go")
	if err != nil {
		t.Fatalf("read grpc_workflow.go: %v", err)
	}
	const marker = "case installSkipDeniedInactive:"
	i := strings.Index(string(src), marker)
	if i < 0 {
		t.Fatalf("installSkipDeniedInactive branch not found — renamed? update this test")
	}
	rest := string(src)[i+len(marker):]
	// Bound the scan to this case: the next case label in the same switch.
	if j := strings.Index(rest, "\n\t\tcase "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// TestInactiveRepairEnablesBeforeStart pins that the fast repair path restores
// the boot contract, not just the running process.
func TestInactiveRepairEnablesBeforeStart(t *testing.T) {
	branch := inactiveRepairBranch(t)

	enable := strings.Index(branch, "supervisor.Enable(")
	start := strings.Index(branch, "supervisor.Start(")

	if start < 0 {
		t.Fatal("repair branch no longer calls supervisor.Start — update this test")
	}
	if enable < 0 {
		t.Fatal("repair-via-Start must call supervisor.Enable: starting a unit the crash-loop suppressor disabled repairs the process but not the boot contract, so the SUCCEEDED convergence it reports does not survive a reboot")
	}
	if enable > start {
		t.Fatal("supervisor.Enable must precede supervisor.Start in the repair branch, matching apply_package_release.go: a unit that fails to start should still be left enabled for the next boot")
	}
}

// TestInactiveRepairUsesSupervisorNotSystemctl pins the boundary the fix must
// respect: node-agent mutates systemd only through internal/supervisor.
func TestInactiveRepairUsesSupervisorNotSystemctl(t *testing.T) {
	branch := inactiveRepairBranch(t)
	for _, forbidden := range []string{"exec.Command", "exec.CommandContext", "\"systemctl\""} {
		if strings.Contains(branch, forbidden) {
			t.Fatalf("repair branch must not use %s — mutating systemd unit actions go through internal/supervisor/ (EX-2)", forbidden)
		}
	}
}
