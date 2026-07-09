package main

// SCAR-2 regression coverage: reconcileMarkItemTerminal must gate the drift-
// observation clear on OBSERVED installed_state convergence, not on the child
// workflow's reported status, and must escalate a repeatedly-unconfirmed
// SUCCEEDED to FAILED instead of looping silently.
// Contract:  reconcile.terminal_success_requires_observed_convergence
// Forbidden: reconcile.clear_drift_on_dispatch_ack_without_observation

import (
	"context"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestReconcileMarkItemTerminal_RequiresObservedConvergence(t *testing.T) {
	ctx := context.Background()

	// newSrv wires the three test seams and returns spies for clears + events.
	newSrv := func(observe func(nodeID, name string) *node_agentpb.InstalledPackage) (*server, *int, *[]map[string]interface{}) {
		clears := 0
		var events []map[string]interface{}
		srv := &server{}
		srv.observeInstalledPkg = func(_ context.Context, nodeID, name string) (*node_agentpb.InstalledPackage, error) {
			return observe(nodeID, name), nil
		}
		srv.clearDriftObsFn = func(_ context.Context, _, _ string) { clears++ }
		srv.emitEventFn = func(name string, payload map[string]interface{}) {
			ev := map[string]interface{}{"name": name}
			for k, v := range payload {
				ev[k] = v
			}
			events = append(events, ev)
		}
		return srv, &clears, &events
	}
	pkgAt := func(ver string) *node_agentpb.InstalledPackage {
		return &node_agentpb.InstalledPackage{Version: ver}
	}
	missing := map[string]any{"type": "missing_package", "node_id": "n1", "package_name": "rbac", "desired_version": "1.2.272"}
	const missKey = "missing_package|rbac@n1"
	succeeded := map[string]any{"status": "SUCCEEDED"}

	// (1) child SUCCEEDED but the package is still absent -> do NOT clear; count.
	t.Run("succeeded_but_still_absent_not_cleared", func(t *testing.T) {
		srv, clears, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return nil })
		if err := srv.reconcileMarkItemTerminal(ctx, missing, succeeded); err != nil {
			t.Fatal(err)
		}
		if *clears != 0 {
			t.Fatalf("must NOT clear drift when installed_state absent, clears=%d", *clears)
		}
		if got := srv.reconcileNoProgress[missKey]; got != 1 {
			t.Fatalf("no-progress counter = %d, want 1", got)
		}
	})

	// (2) child SUCCEEDED and installed at desired version -> clear + reset.
	t.Run("succeeded_and_installed_at_desired_cleared", func(t *testing.T) {
		srv, clears, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return pkgAt("1.2.272") })
		if err := srv.reconcileMarkItemTerminal(ctx, missing, succeeded); err != nil {
			t.Fatal(err)
		}
		if *clears != 1 {
			t.Fatalf("must clear when installed at desired version, clears=%d", *clears)
		}
		if _, ok := srv.reconcileNoProgress[missKey]; ok {
			t.Fatal("no-progress counter must be reset on observed convergence")
		}
	})

	// (3) version_drift gate: stale version does not clear; matching version clears.
	t.Run("version_drift_gate", func(t *testing.T) {
		vd := map[string]any{"type": "version_drift", "node_id": "n1", "package_name": "dns", "desired_version": "1.2.272"}
		srv, clears, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return pkgAt("1.2.270") })
		_ = srv.reconcileMarkItemTerminal(ctx, vd, succeeded)
		if *clears != 0 {
			t.Fatalf("stale installed version must not clear, clears=%d", *clears)
		}
		srv2, clears2, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return pkgAt("1.2.272") })
		_ = srv2.reconcileMarkItemTerminal(ctx, vd, succeeded)
		if *clears2 != 1 {
			t.Fatalf("matching installed version must clear, clears=%d", *clears2)
		}
	})

	// (4) N consecutive not-converged SUCCEEDED -> escalate to FAILED with reason.
	t.Run("no_progress_escalates_to_failed", func(t *testing.T) {
		srv, clears, events := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return nil })
		for i := 0; i < reconcileNoProgressThreshold; i++ {
			_ = srv.reconcileMarkItemTerminal(ctx, missing, succeeded)
		}
		if *clears != 0 {
			t.Fatalf("must never clear while not converged, clears=%d", *clears)
		}
		found := false
		for _, ev := range *events {
			if ev["name"] == "cluster.reconcile.item_failed" && ev["reason"] == "remediation_no_progress" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected cluster.reconcile.item_failed reason=remediation_no_progress, events=%v", *events)
		}
		if _, ok := srv.reconcileNoProgress[missKey]; ok {
			t.Fatal("counter must reset after escalation (re-arm)")
		}
	})

	// (5) unmanaged_package has no convergence predicate -> legacy clear-on-SUCCEEDED.
	t.Run("unmanaged_not_checkable_cleared_legacy", func(t *testing.T) {
		srv, clears, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return nil })
		unmanaged := map[string]any{"type": "unmanaged_package", "node_id": "n1", "package_name": "yt-dlp"}
		_ = srv.reconcileMarkItemTerminal(ctx, unmanaged, succeeded)
		if *clears != 1 {
			t.Fatalf("unmanaged (not checkable) must clear on SUCCEEDED, clears=%d", *clears)
		}
		if len(srv.reconcileNoProgress) != 0 {
			t.Fatal("unmanaged must not create no-progress entries")
		}
	})

	// (6) non-SUCCEEDED child -> no clear, no counter change.
	t.Run("non_succeeded_no_clear_no_counter", func(t *testing.T) {
		srv, clears, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return nil })
		_ = srv.reconcileMarkItemTerminal(ctx, missing, map[string]any{"status": "FAILED"})
		if *clears != 0 {
			t.Fatalf("non-SUCCEEDED must not clear, clears=%d", *clears)
		}
		if len(srv.reconcileNoProgress) != 0 {
			t.Fatal("non-SUCCEEDED must not touch the no-progress counter")
		}
	})
}
