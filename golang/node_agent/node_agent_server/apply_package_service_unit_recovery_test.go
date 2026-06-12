package main

import (
	"context"
	"testing"
)

// TestIsServiceKind: only SERVICE is a daemon that MUST have a systemd unit, so
// only SERVICE routes a missing-unit apply to recovery. COMMAND binaries and
// INFRASTRUCTURE packages whose spec sets install_systemd=false (etcdctl, mc,
// rclone, ...) are legitimately unit-less and must stay binary-only.
func TestIsServiceKind(t *testing.T) {
	for _, kind := range []string{"SERVICE", "service", " Service "} {
		if !isServiceKind(kind) {
			t.Errorf("isServiceKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"COMMAND", "INFRASTRUCTURE", "APPLICATION", "", "infra"} {
		if isServiceKind(kind) {
			t.Errorf("isServiceKind(%q) = true, want false (must stay binary-only / skip systemd)", kind)
		}
	}
}

// TestServiceUnitUnrecoverableResponse_FailsLoud: a SERVICE left without a
// runnable unit must report a hard failure — never Ok, never Status="installed".
// This is what stops the silent reinstall storm: the workflow sees a real FAILED
// (which defers and ultimately abandons, bounded) instead of a false "installed"
// that the installSkipDeniedUnitGone reconciler re-attempts forever.
// (meta.half_done_must_not_look_done, meta.failure_response_must_contract_not_amplify)
func TestServiceUnitUnrecoverableResponse_FailsLoud(t *testing.T) {
	resp := serviceUnitUnrecoverableResponse("awareness-graph", "0.0.22", "no unit produced by spec", "op-123")
	if resp.GetOk() {
		t.Fatal("response Ok=true, want false (must fail loud)")
	}
	if resp.GetStatus() == "installed" {
		t.Fatal("response Status=installed for a service with no unit — half-done must not look done")
	}
	if resp.GetStatus() != "failed" {
		t.Fatalf("response Status=%q, want failed", resp.GetStatus())
	}
	if resp.GetErrorDetail() == "" {
		t.Fatal("response ErrorDetail empty, want a diagnostic message")
	}
	if resp.GetOperationId() != "op-123" {
		t.Fatalf("response OperationId=%q, want op-123", resp.GetOperationId())
	}
}

// TestRecreateServiceUnitFromSpec_MissingPackageFails: when the package archive
// is not present locally, unit recovery must return an error (which the apply
// path turns into a loud failure) rather than silently succeeding. The positive
// path — recover the unit AND start the service — runs the shared installer
// engine and requires root + systemd + a real package, so it is exercised by the
// installer-engine integration tests (the same engine INFRASTRUCTURE uses), not
// here.
func TestRecreateServiceUnitFromSpec_MissingPackageFails(t *testing.T) {
	orig := localPackageDirs
	localPackageDirs = []string{t.TempDir()} // empty: no archives on disk
	t.Cleanup(func() { localPackageDirs = orig })

	srv := &NodeAgentServer{}
	// Explicit version → no wildcard fallback → findLocalPackage returns "".
	if err := srv.recreateServiceUnitFromSpec(context.Background(), "awareness-graph", "0.0.22"); err == nil {
		t.Fatal("recreateServiceUnitFromSpec returned nil with no local package; want error (must not silently succeed)")
	}
}
