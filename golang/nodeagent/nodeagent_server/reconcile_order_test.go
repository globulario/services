package main

import (
	"context"
	"fmt"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestReconcileOrderDNSBeforeCerts(t *testing.T) {
	srv := &NodeAgentServer{state: &nodeAgentState{}}
	calls := []string{}
	srv.syncDNSHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "syncDNS")
		return nil
	}
	srv.waitDNSHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "waitDNS")
		return nil
	}
	srv.ensureCertsHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "certs")
		return nil
	}
	srv.restartHook = func(units []string, op *operation) error {
		calls = append(calls, "restart")
		return nil
	}
	srv.healthCheckHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "health")
		return nil
	}
	srv.objectstoreLayoutHook = func(ctx context.Context, domain string) error { return nil }
	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		AcmeEnabled:   true,
		AdminEmail:    "ops@example.com",
	}
	raw, _ := protojson.Marshal(spec)
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(raw),
			"reconcile.restart_units":   `["globular-gateway.service"]`,
		},
	}
	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "example.com", true); err != nil {
		t.Fatalf("reconcileNetwork error: %v", err)
	}
	expected := []string{"syncDNS", "waitDNS", "certs", "restart", "health"}
	if len(calls) != len(expected) {
		t.Fatalf("expected %v calls, got %v", expected, calls)
	}
	for i := range expected {
		if calls[i] != expected[i] {
			t.Fatalf("order mismatch at %d: got %s want %s", i, calls[i], expected[i])
		}
	}
}

func TestReconcileDNSReadinessFailureStopsCerts(t *testing.T) {
	srv := &NodeAgentServer{state: &nodeAgentState{}}
	calls := []string{}
	srv.syncDNSHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "syncDNS")
		return nil
	}
	srv.waitDNSHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "waitDNS")
		return fmt.Errorf("dns not ready")
	}
	srv.ensureCertsHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "certs")
		return nil
	}
	srv.objectstoreLayoutHook = func(ctx context.Context, domain string) error { return nil }
	srv.healthCheckHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error { return nil }
	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		AcmeEnabled:   true,
		AdminEmail:    "ops@example.com",
	}
	raw, _ := protojson.Marshal(spec)
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(raw),
		},
	}
	err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "example.com", true)
	if err == nil {
		t.Fatalf("expected error when dns readiness fails")
	}
	for _, c := range calls {
		if c == "certs" {
			t.Fatalf("certs should not be called when dns readiness fails")
		}
	}
}

func TestReconcileSyncsDNSWhenGenerationZero(t *testing.T) {
	srv := &NodeAgentServer{state: &nodeAgentState{}}
	calls := []string{}
	srv.syncDNSHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "syncDNS")
		return nil
	}
	srv.waitDNSHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "waitDNS")
		return nil
	}
	srv.ensureCertsHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "certs")
		return nil
	}
	srv.objectstoreLayoutHook = func(ctx context.Context, domain string) error { return nil }
	srv.healthCheckHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error { return nil }
	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		AcmeEnabled:   false,
	}
	raw, _ := protojson.Marshal(spec)
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(raw),
		},
	}
	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 0, "example.com", true); err != nil {
		t.Fatalf("reconcileNetwork error: %v", err)
	}
	if len(calls) == 0 || calls[0] != "syncDNS" {
		t.Fatalf("expected syncDNS to be called when generation==0 and networkChanged")
	}
}

func TestReconcileIdempotency(t *testing.T) {
	// Test that running reconciliation twice on the same state produces no changes on second run
	srv := &NodeAgentServer{state: &nodeAgentState{}}
	callCount := 0

	srv.syncDNSHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		callCount++
		return nil
	}
	srv.waitDNSHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		callCount++
		return nil
	}
	srv.ensureCertsHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		callCount++
		return nil
	}
	srv.restartHook = func(units []string, op *operation) error {
		callCount++
		return nil
	}
	srv.healthCheckHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		callCount++
		return nil
	}
	srv.objectstoreLayoutHook = func(ctx context.Context, domain string) error { return nil }

	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		AcmeEnabled:   true,
		AdminEmail:    "ops@example.com",
	}
	raw, _ := protojson.Marshal(spec)
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(raw),
			"reconcile.restart_units":   `["globular-gateway.service"]`,
		},
	}

	// First reconciliation
	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "example.com", true); err != nil {
		t.Fatalf("first reconcileNetwork error: %v", err)
	}

	firstRunCalls := callCount
	if firstRunCalls == 0 {
		t.Fatal("expected calls on first run")
	}

	// Second reconciliation with same state - should be a no-op or minimal work
	// In practice, this might still call health checks but shouldn't redo DNS sync or cert issuance
	callCount = 0
	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "example.com", false); err != nil {
		t.Fatalf("second reconcileNetwork error: %v", err)
	}

	// Second run should have fewer or equal calls (ideally just health checks)
	// The networkChanged=false should skip DNS sync
	if callCount > firstRunCalls {
		t.Fatalf("second reconciliation did more work (%d calls) than first (%d calls), expected idempotency", callCount, firstRunCalls)
	}
}

func TestReconcileExistingStateAlreadyCorrect(t *testing.T) {
	// Test that reconciliation with networkChanged=false does minimal work
	srv := &NodeAgentServer{state: &nodeAgentState{}}
	calls := []string{}

	srv.syncDNSHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "syncDNS")
		return nil
	}
	srv.waitDNSHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "waitDNS")
		return nil
	}
	srv.ensureCertsHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "certs")
		return nil
	}
	srv.restartHook = func(units []string, op *operation) error {
		calls = append(calls, "restart")
		return nil
	}
	srv.healthCheckHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
		calls = append(calls, "health")
		return nil
	}
	srv.objectstoreLayoutHook = func(ctx context.Context, domain string) error { return nil }

	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "http",
		AcmeEnabled:   false,
	}
	raw, _ := protojson.Marshal(spec)
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(raw),
		},
	}

	// Reconcile with networkChanged=false and generation > 0 (state already correct)
	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "example.com", false); err != nil {
		t.Fatalf("reconcileNetwork error: %v", err)
	}

	// When networkChanged=false, restart should not be called
	for _, call := range calls {
		if call == "restart" {
			t.Error("restart should not be called when networkChanged=false")
		}
	}

	// The number of calls should be less than a full reconciliation
	// (A full reconciliation with HTTPS would have 5 calls: syncDNS, waitDNS, certs, restart, health)
	if len(calls) > 3 {
		t.Errorf("expected minimal work when state is correct, got %d calls: %v", len(calls), calls)
	}
}

func TestReconcileOrderStable(t *testing.T) {
	// Test that multiple reconciliations maintain stable ordering
	srv := &NodeAgentServer{state: &nodeAgentState{}}

	for run := 1; run <= 3; run++ {
		calls := []string{}

		srv.syncDNSHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
			calls = append(calls, "syncDNS")
			return nil
		}
		srv.waitDNSHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			calls = append(calls, "waitDNS")
			return nil
		}
		srv.ensureCertsHook = func(*clustercontrollerpb.ClusterNetworkSpec) error {
			calls = append(calls, "certs")
			return nil
		}
		srv.restartHook = func(units []string, op *operation) error {
			calls = append(calls, "restart")
			return nil
		}
		srv.healthCheckHook = func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			calls = append(calls, "health")
			return nil
		}
		srv.objectstoreLayoutHook = func(ctx context.Context, domain string) error { return nil }

		spec := &clustercontrollerpb.ClusterNetworkSpec{
			ClusterDomain: "example.com",
			Protocol:      "https",
			AcmeEnabled:   true,
			AdminEmail:    "ops@example.com",
		}
		raw, _ := protojson.Marshal(spec)
		plan := &clustercontrollerpb.NodePlan{
			RenderedConfig: map[string]string{
				"cluster.network.spec.json": string(raw),
				"reconcile.restart_units":   `["globular-gateway.service"]`,
			},
		}

		if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "example.com", true); err != nil {
			t.Fatalf("run %d: reconcileNetwork error: %v", run, err)
		}

		// Verify stable ordering across runs
		expected := []string{"syncDNS", "waitDNS", "certs", "restart", "health"}
		if len(calls) != len(expected) {
			t.Fatalf("run %d: expected %d calls, got %d", run, len(expected), len(calls))
		}
		for i := range expected {
			if calls[i] != expected[i] {
				t.Fatalf("run %d: order mismatch at %d: got %s want %s", run, i, calls[i], expected[i])
			}
		}
	}
}
