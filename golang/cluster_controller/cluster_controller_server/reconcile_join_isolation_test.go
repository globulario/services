package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

func TestReconcileScanDrift_Day1JoinIsolationSkipsExistingNodes(t *testing.T) {
	srv := &server{
		cfg:       &clusterControllerConfig{ClusterDomain: "globular.internal"},
		resources: resourcestore.NewMemStore(),
		state: &controllerState{Nodes: map[string]*nodeState{
			"existing": {
				NodeID:              "existing",
				BootstrapPhase:      BootstrapWorkloadReady,
				Status:              "ready",
				AppliedServicesHash: "services:converged",
				Profiles:            []string{"core"},
			},
			"joiner": {
				NodeID:         "joiner",
				BootstrapPhase: BootstrapAdmitted,
				Status:         "converging",
				Profiles:       []string{"core"},
			},
		}},
	}
	seedServiceDesired(t, srv.resources, "mcp", "1.2.270")

	items, err := srv.reconcileScanDrift(context.Background(), "globular.internal", "cluster", nil)
	if err != nil {
		t.Fatalf("reconcileScanDrift: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected active Day-1 join to suppress drift remediation on existing nodes, got %#v", items)
	}
}
