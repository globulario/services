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
