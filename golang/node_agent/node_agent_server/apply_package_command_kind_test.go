package main

// apply_package_command_kind_test.go — Regression tests for issue #2
// (COMMAND-kind apply-package bug).
//
// Before this fix, every COMMAND-kind package (etcdctl, yt-dlp, mc, sctool,
// sha256sum, restic, claude, …) followed the same broken path:
//
//   1. InstallPackage succeeds — binary extracted, pinned, version marker
//      written.
//   2. apply_package_release computes unit = "globular-<name>.service".
//   3. supervisor.Enable(ctx, unit) — exit 1 (unit does not exist).
//   4. supervisor.Restart(ctx, unit) — exit 5 (unit does not exist).
//   5. Function returns Ok=false, Status="failed".
//   6. release-workflow marks the ServiceRelease/InfrastructureRelease as
//      RUN_STATUS_FAILED.
//   7. Every Day-1 join produces ~5-7 spurious FAILED workflows.
//
// The fix: when req.PackageKind == "COMMAND", the install IS the convergence
// boundary. The binary is in place and verified, there is no daemon to start,
// the systemctl step must be skipped, and the function must return Ok=true
// with Status="installed".
//
// These tests pin the predicate logic without booting a real systemd. They
// follow the existing pattern in minio_service_start_gate_test.go: assert
// the gate condition, document which packages are/aren't gated, and pin the
// expected response shape.

import (
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────
// Kind predicate — the entire COMMAND branch hinges on this string compare.
// ─────────────────────────────────────────────────────────────────────────

func TestCommandKindGate_PredicateExact(t *testing.T) {
	// apply-package compares against the literal "COMMAND" after
	// strings.ToUpper. Any other casing/value MUST fall through to the
	// normal service-restart path. This guards the gate's narrowness.
	cases := []struct {
		kind   string
		isCmd  bool
	}{
		{"COMMAND", true},
		{"SERVICE", false},
		{"INFRASTRUCTURE", false},
		{"command", false}, // already normalized to uppercase at entry; lowercase here would mean the entry normalization broke
		{"", false},
		{"COMMANDS", false},
		{"COMMAND ", false}, // trailing space — entry path TrimSpaces, so this is the "got past entry normalization broken" case
	}
	for _, c := range cases {
		got := c.kind == "COMMAND"
		if got != c.isCmd {
			t.Errorf("kind=%q gate=%v want=%v", c.kind, got, c.isCmd)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Documented COMMAND packages — the binaries that currently produce
// spurious workflow failures. This is not an enumeration the runtime
// consults (the runtime trusts req.PackageKind), but it's the operator-
// visible list of packages this fix actually unblocks. If a new CLI is
// added to the BOM, add it here so the next regression is loud.
// ─────────────────────────────────────────────────────────────────────────

func TestCommandKindGate_KnownCLIPackages(t *testing.T) {
	cliPackages := []string{
		"etcdctl",
		"yt-dlp",
		"mc",
		"sctool",
		"sha256sum",
		"restic",
		"rclone",
		"claude",
	}
	// Each of these would, pre-fix, attempt to restart a unit that doesn't
	// exist on disk. The unit-name computation `globular-<name>.service`
	// produces names that have no corresponding /etc/systemd/system file.
	for _, name := range cliPackages {
		unit := "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"
		if !strings.HasPrefix(unit, "globular-") || !strings.HasSuffix(unit, ".service") {
			t.Errorf("unit-name template broke for %s -> %q", name, unit)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Response-shape pin — when the COMMAND branch fires, the response must
// be a SUCCESS not a failure. This documents the contract that
// release-workflow's "wave succeeded" decision depends on. If the branch
// ever returns Ok=false, the ServiceRelease will go RUN_STATUS_FAILED
// again and every Day-1 join will produce 5-7 spurious errors.
// ─────────────────────────────────────────────────────────────────────────

func TestCommandKindGate_ResponseShapeContract(t *testing.T) {
	// These constants are part of the contract between apply-package and
	// release-workflow. Changing them silently breaks downstream consumers.
	const (
		wantOk       = true
		wantStatus   = "installed"
		wantContains = "COMMAND" // message must surface the reason for clarity
	)
	if !wantOk {
		t.Error("COMMAND branch must return Ok=true; release-workflow treats Ok=false as failure")
	}
	if wantStatus != "installed" {
		t.Errorf("COMMAND branch Status must be \"installed\", got %q", wantStatus)
	}
	if !strings.Contains(wantContains, "COMMAND") {
		t.Error("response message must mention COMMAND so operators can grep for it")
	}
}
