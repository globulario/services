package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newInfraTestServer returns a server whose prober is stubbed to report "not
// installed" so the test never touches a real ScyllaDB.
func newInfraTestServer() *NodeAgentServer {
	srv := &NodeAgentServer{nodeID: "globule-test"}
	srv.ensureInfraTruth()
	srv.scyllaProber.DetectInstalled = func(ctx context.Context) bool { return false }
	srv.scyllaProber.EnableCQL = false
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

func TestGetInfraProbe_AllResolvesToScylla(t *testing.T) {
	srv := newInfraTestServer()
	resp, err := srv.GetInfraProbe(context.Background(), &node_agentpb.GetInfraProbeRequest{Component: "all"})
	if err != nil {
		t.Fatalf("GetInfraProbe all: %v", err)
	}
	if len(resp.GetResults()) != 1 || resp.GetResults()[0].GetComponent() != infra_truth.ComponentScylla {
		t.Fatalf("expected single scylladb result, got %+v", resp.GetResults())
	}
}

func TestGetInfraProbe_EmptyComponentDefaultsToAll(t *testing.T) {
	srv := newInfraTestServer()
	resp, err := srv.GetInfraProbe(context.Background(), &node_agentpb.GetInfraProbeRequest{})
	if err != nil {
		t.Fatalf("GetInfraProbe empty: %v", err)
	}
	if len(resp.GetResults()) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.GetResults()))
	}
}
