package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestReconcileNetworkFailsWithoutDomain(t *testing.T) {
	srv := &NodeAgentServer{state: &nodeAgentState{}}
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": `{}`,
		},
	}
	err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 1, "", true)
	if err == nil || !strings.Contains(err.Error(), "cluster domain") {
		t.Fatalf("expected domain required error, got %v", err)
	}
}

func TestReconcileNetworkSyncsWhenGenerationZero(t *testing.T) {
	syncCalled := false
	layoutCalled := false
	srv := &NodeAgentServer{
		state: &nodeAgentState{},
		syncDNSHook: func(spec *clustercontrollerpb.ClusterNetworkSpec) error {
			syncCalled = true
			return nil
		},
		waitDNSHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			return nil
		},
		ensureCertsHook: func(spec *clustercontrollerpb.ClusterNetworkSpec) error {
			return nil
		},
		objectstoreLayoutHook: func(ctx context.Context, domain string) error {
			layoutCalled = true
			if domain == "" {
				t.Fatalf("expected domain passed to objectstore layout")
			}
			return nil
		},
		healthCheckHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error { return nil },
	}

	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "http",
	}
	rawSpec, err := protojson.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(rawSpec),
		},
	}

	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 0, "", true); err != nil {
		t.Fatalf("reconcileNetwork: %v", err)
	}
	if !syncCalled {
		t.Fatalf("expected syncDNS to be called when generation is zero but network changed")
	}
	if !layoutCalled {
		t.Fatalf("expected objectstore layout enforcement to be called")
	}
}

func TestReconcileNetworkOrder_DNSBeforeCertsAndHealth(t *testing.T) {
	var order []string
	srv := &NodeAgentServer{
		state: &nodeAgentState{},
		syncDNSHook: func(spec *clustercontrollerpb.ClusterNetworkSpec) error {
			order = append(order, "syncDNS")
			return nil
		},
		waitDNSHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			order = append(order, "waitDNS")
			return nil
		},
		ensureCertsHook: func(spec *clustercontrollerpb.ClusterNetworkSpec) error {
			order = append(order, "ensureCerts")
			return nil
		},
		restartHook: func(units []string, _ *operation) error {
			order = append(order, "restart")
			return nil
		},
		healthCheckHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			order = append(order, "health")
			return nil
		},
		objectstoreLayoutHook: func(ctx context.Context, domain string) error {
			if domain == "" {
				t.Fatalf("expected domain passed to objectstore layout")
			}
			order = append(order, "layout")
			return nil
		},
	}

	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		PortHttps:     443,
	}
	rawSpec, err := protojson.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	plan := &clustercontrollerpb.NodePlan{
		RenderedConfig: map[string]string{
			"cluster.network.spec.json": string(rawSpec),
			"reconcile.restart_units":   `["globular-envoy.service"]`,
		},
	}

	if err := srv.reconcileNetwork(context.Background(), plan, &operation{}, 0, "", true); err != nil {
		t.Fatalf("reconcileNetwork: %v", err)
	}
	expected := []string{"syncDNS", "waitDNS", "ensureCerts", "restart", "health", "layout"}
	if !reflect.DeepEqual(order, expected) {
		t.Fatalf("unexpected order %v, want %v", order, expected)
	}
	if srv.state.ClusterDomain != spec.ClusterDomain || srv.state.Protocol != spec.Protocol {
		t.Fatalf("state not updated with domain/protocol: %+v", srv.state)
	}
}
