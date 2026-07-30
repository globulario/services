package main

// SCAR-2 regression coverage: reconcileMarkItemTerminal must gate the drift-
// observation clear on OBSERVED installed_state convergence, not on the child
// workflow's reported status, and must escalate a repeatedly-unconfirmed
// SUCCEEDED to FAILED instead of looping silently.
// Contract:  reconcile.terminal_success_requires_observed_convergence
// Forbidden: reconcile.clear_drift_on_dispatch_ack_without_observation

import (
	"context"
	"strings"
	"testing"
	"time"

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

	// (6) non-SUCCEEDED child -> never clears, but DOES count toward escalation
	// (previously this silently no-opped: meta.silence_is_not_valid_for_unexpected
	// — a permanently-failing dispatch, e.g. missing_package with no repository
	// build, was re-dispatched every reconcile cycle forever with no terminal
	// state at all).
	t.Run("failed_child_never_clears_but_counts", func(t *testing.T) {
		srv, clears, _ := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return nil })
		_ = srv.reconcileMarkItemTerminal(ctx, missing, map[string]any{"status": "FAILED"})
		if *clears != 0 {
			t.Fatalf("FAILED must not clear, clears=%d", *clears)
		}
		if got := srv.reconcileNoProgress[missKey]; got != 1 {
			t.Fatalf("FAILED must bump the no-progress counter, got=%d want=1", got)
		}
	})

	// (7) N consecutive FAILED children -> escalate to FAILED with a distinct
	// reason AND arm a dispatch backoff, so reconcileChooseWorkflow stops
	// re-dispatching this item instead of hammering it every cycle.
	t.Run("repeated_failure_escalates_and_arms_backoff", func(t *testing.T) {
		srv, clears, events := newSrv(func(_, _ string) *node_agentpb.InstalledPackage { return nil })
		for i := 0; i < reconcileNoProgressThreshold; i++ {
			_ = srv.reconcileMarkItemTerminal(ctx, missing, map[string]any{"status": "FAILED"})
		}
		if *clears != 0 {
			t.Fatalf("must never clear on repeated FAILED, clears=%d", *clears)
		}
		found := false
		for _, ev := range *events {
			if ev["name"] == "cluster.reconcile.item_failed" && ev["reason"] == "remediation_dispatch_failed: child_status=FAILED" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected cluster.reconcile.item_failed reason=remediation_dispatch_failed:..., events=%v", *events)
		}
		if inBackoff, _ := srv.reconcileInBackoff(missKey); !inBackoff {
			t.Fatal("expected key to be armed for backoff after escalation")
		}
	})

	// (8) reconcileChooseWorkflow skips dispatch (returns noop) while a key is
	// backed off, instead of hammering release.apply.package every cycle.
	t.Run("choose_workflow_skips_dispatch_during_backoff", func(t *testing.T) {
		srv := &server{}
		srv.reconcileArmBackoff(missKey)
		out, err := srv.reconcileChooseWorkflow(ctx, missing)
		if err != nil {
			t.Fatal(err)
		}
		if out["workflow_name"] != "noop" {
			t.Fatalf("expected noop dispatch during backoff, got %v", out["workflow_name"])
		}
	})

	// (9) the cooldown is bounded, not permanent: once the deadline has passed,
	// reconcileInBackoff reports the key as no longer backed off (and clears the
	// stale entry), so dispatch is free to resume — self-healing without operator
	// intervention if the underlying cause (e.g. a missing repository build) is
	// fixed in the meantime.
	t.Run("backoff_expires_after_cooldown", func(t *testing.T) {
		srv := &server{}
		srv.reconcileArmBackoff(missKey)
		srv.reconcileNoProgMu.Lock()
		srv.reconcileBackoffUntil[missKey] = time.Now().Add(-time.Second) // force-expire
		srv.reconcileNoProgMu.Unlock()
		if inBackoff, _ := srv.reconcileInBackoff(missKey); inBackoff {
			t.Fatal("expected backoff to have expired")
		}
		if _, ok := srv.reconcileBackoffUntil[missKey]; ok {
			t.Fatal("expired backoff entry should be cleared, not left stale")
		}
	})
}

// TestInfraUnhealthy_DispatchesRestartNotNoop is the regression coverage for
// the bug where a cleanly-stopped infra service (etcd/scylladb/minio) never
// came back: reconcileChooseWorkflow's "infra_unhealthy" case only logged
// "remediation deferred" and returned a noop dispatch, so nothing ever called
// node-agent's ControlService to restart it. Live symptom: an operator's
// `systemctl stop globular-minio.service` left MinIO down indefinitely,
// cascading into CRITICAL objectstore.endpoint_unreachable /
// installed_state_runtime_mismatch findings cluster-wide with no auto-heal.
// Contract: infra_unhealthy must dispatch node.restart_infra_unit (not noop)
// with the node/component/endpoint needed to call ControlService.
func TestInfraUnhealthy_DispatchesRestartNotNoop(t *testing.T) {
	ctx := context.Background()
	srv := &server{}
	item := map[string]any{
		"type":      "infra_unhealthy",
		"node_id":   "n1",
		"component": "minio",
		"endpoint":  "10.0.0.63:11000",
		"hostname":  "globule-ryzen",
	}
	out, err := srv.reconcileChooseWorkflow(ctx, item)
	if err != nil {
		t.Fatal(err)
	}
	if out["workflow_name"] != "node.restart_infra_unit" {
		t.Fatalf("expected node.restart_infra_unit dispatch, got %v (infra_unhealthy must not silently no-op)", out["workflow_name"])
	}
	inputs, _ := out["inputs"].(map[string]any)
	if inputs["node_id"] != "n1" || inputs["component"] != "minio" || inputs["endpoint"] != "10.0.0.63:11000" {
		t.Fatalf("restart dispatch missing required inputs, got %v", inputs)
	}
}

// TestInfraUnhealthy_BackoffKeyIsPerComponent is the regression for a second
// bug the restart fix would otherwise have inherited: driftEntityRef keyed
// only on package_name+node_id, which is absent on infra_unhealthy items — so
// fmt.Sprint(item["package_name"]) on the missing key produced the literal
// string "<nil>", collapsing EVERY infra component on a node onto the same
// no-progress/backoff key. Two genuinely independent failures (e.g. etcd down
// AND minio down on the same node) would then share one counter: minio
// escalating to FAILED/backoff would incorrectly reset or gate etcd's
// tracking too.
func TestInfraUnhealthy_BackoffKeyIsPerComponent(t *testing.T) {
	minioItem := map[string]any{"type": "infra_unhealthy", "node_id": "n1", "component": "minio"}
	etcdItem := map[string]any{"type": "infra_unhealthy", "node_id": "n1", "component": "etcd"}
	minioRef := driftEntityRef(minioItem)
	etcdRef := driftEntityRef(etcdItem)
	if minioRef == "" || etcdRef == "" {
		t.Fatalf("expected non-empty entity refs, got minio=%q etcd=%q", minioRef, etcdRef)
	}
	if strings.Contains(minioRef, "<nil>") || strings.Contains(etcdRef, "<nil>") {
		t.Fatalf("entity ref leaked the missing-key sentinel: minio=%q etcd=%q", minioRef, etcdRef)
	}
	if minioRef == etcdRef {
		t.Fatalf("minio and etcd infra_unhealthy items on the same node must not collide onto one entity ref, both got %q", minioRef)
	}
}

// TestInfraUnhealthy_ConvergenceRequiresReprobe verifies reconcileItemConverged
// treats infra_unhealthy as checkable and gates convergence on a fresh health
// probe rather than trusting the restart RPC's dispatch ack — the same
// SCAR-2 discipline already applied to missing_package/version_drift. Since
// srv.probeMinioHealth dials a real node-agent (nil endpoint here), the probe
// must fail closed (converged=false), not silently report converged=true.
func TestInfraUnhealthy_ConvergenceRequiresReprobe(t *testing.T) {
	ctx := context.Background()
	srv := &server{}
	item := map[string]any{"type": "infra_unhealthy", "node_id": "n1", "component": "minio", "endpoint": ""}
	converged, checkable := srv.reconcileItemConverged(ctx, item)
	if !checkable {
		t.Fatal("infra_unhealthy must be checkable, not silently treated as unverifiable")
	}
	if converged {
		t.Fatal("must not report converged from a dispatch ack alone — an unreachable probe endpoint must fail closed")
	}
}
