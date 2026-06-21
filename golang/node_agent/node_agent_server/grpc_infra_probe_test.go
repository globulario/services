package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newInfraTestServer returns a server whose probers are all stubbed to report
// "not installed" so the test never touches a real ScyllaDB / etcd / MinIO /
// Envoy (and so "all" probes are deterministic and fast).
func newInfraTestServer() *NodeAgentServer {
	srv := &NodeAgentServer{nodeID: "globule-test"}
	srv.ensureInfraTruth()
	notInstalled := func(ctx context.Context) bool { return false }
	srv.scyllaProber.DetectInstalled = notInstalled
	srv.scyllaProber.EnableCQL = false
	srv.etcdProber.DetectInstalled = notInstalled
	srv.minioProber.DetectInstalled = notInstalled
	srv.envoyProber.DetectInstalled = notInstalled
	return srv
}

func TestGetInfraProbe_UnknownComponent(t *testing.T) {
	srv := newInfraTestServer()
	_, err := srv.GetInfraProbe(context.Background(), &node_agentpb.GetInfraProbeRequest{Component: "redis"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestGetInfraProbe_Scylla(t *testing.T) {
	srv := newInfraTestServer()
	resp, err := srv.GetInfraProbe(context.Background(), &node_agentpb.GetInfraProbeRequest{Component: "scylladb", BypassCache: true})
	if err != nil {
		t.Fatalf("GetInfraProbe: %v", err)
	}
	if len(resp.GetResults()) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.GetResults()))
	}
	r := resp.GetResults()[0]
	if r.GetComponent() != infra_truth.ComponentScylla {
		t.Errorf("component=%q", r.GetComponent())
	}
	if r.GetInstalled() {
		t.Error("stub reports not-installed")
	}
}

// allInfraComponents is every component "all" must return, in dispatch order.
var allInfraComponents = []string{
	infra_truth.ComponentScylla,
	infra_truth.ComponentEtcd,
	infra_truth.ComponentMinio,
	infra_truth.ComponentEnvoy,
}

func assertAllComponents(t *testing.T, results []*cluster_controllerpb.InfraProbeResult) {
	t.Helper()
	if len(results) != len(allInfraComponents) {
		t.Fatalf("expected %d results (%v), got %d: %+v", len(allInfraComponents), allInfraComponents, len(results), results)
	}
	got := make(map[string]bool, len(results))
	for _, r := range results {
		got[r.GetComponent()] = true
	}
	for _, c := range allInfraComponents {
		if !got[c] {
			t.Errorf("missing component %q in 'all' results", c)
		}
	}
}

func TestGetInfraProbe_AllReturnsEveryComponent(t *testing.T) {
	srv := newInfraTestServer()
	resp, err := srv.GetInfraProbe(context.Background(), &node_agentpb.GetInfraProbeRequest{Component: "all"})
	if err != nil {
		t.Fatalf("GetInfraProbe all: %v", err)
	}
	assertAllComponents(t, resp.GetResults())
}

func TestGetInfraProbe_EmptyComponentDefaultsToAll(t *testing.T) {
	srv := newInfraTestServer()
	resp, err := srv.GetInfraProbe(context.Background(), &node_agentpb.GetInfraProbeRequest{})
	if err != nil {
		t.Fatalf("GetInfraProbe empty: %v", err)
	}
	assertAllComponents(t, resp.GetResults())
}
