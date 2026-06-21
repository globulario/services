package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
)

func TestRefreshScyllaInfraProbeEmitsBehavioralObservation(t *testing.T) {
	origEmit := emitBehavioralInfraProbe
	defer func() { emitBehavioralInfraProbe = origEmit }()

	observed := make(chan *cluster_controllerpb.InfraProbeResult, 1)
	emitBehavioralInfraProbe = func(ctx context.Context, clusterID string, probe *cluster_controllerpb.InfraProbeResult) {
		if clusterID != "cluster-1" {
			t.Errorf("cluster_id=%q", clusterID)
		}
		observed <- probe
	}

	srv := newInfraTestServer()
	srv.clusterID = "cluster-1"

	res := srv.refreshScyllaInfraProbe(context.Background())
	if res.GetComponent() != infra_truth.ComponentScylla {
		t.Fatalf("component=%q", res.GetComponent())
	}

	select {
	case probe := <-observed:
		if probe.GetComponent() != infra_truth.ComponentScylla {
			t.Fatalf("emitted component=%q", probe.GetComponent())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected infra probe emission")
	}
}
