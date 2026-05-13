package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newJoinReadyServer(t *testing.T) *server {
	t.Helper()
	state := newControllerState()
	state.JoinTokens["tok-1"] = &joinTokenRecord{
		Token:     "tok-1",
		ExpiresAt: time.Now().Add(time.Hour),
		MaxUses:   10,
	}
	return newTestServer(t, state)
}

func TestRequestJoin_DoesNotAdmitNodeWhenPreflightFails(t *testing.T) {
	srv := newJoinReadyServer(t)
	_, err := srv.RequestJoin(context.Background(), &cluster_controllerpb.RequestJoinRequest{
		JoinToken: "tok-1",
		Identity: &cluster_controllerpb.NodeIdentity{
			Hostname: "joiner-1",
			Ips:      []string{"127.0.0.1"},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got: %v", err)
	}
	if len(srv.state.Nodes) != 0 {
		t.Fatalf("preflight-failed join must not create active node, got %d", len(srv.state.Nodes))
	}
}

func TestRequestJoin_DoesNotMutateDNSWhenPreflightFails(t *testing.T) {
	srv := newJoinReadyServer(t)
	srv.state.ClusterNetworkSpec = &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "globular.internal",
	}
	beforeDomain := srv.state.ClusterNetworkSpec.GetClusterDomain()

	_, err := srv.RequestJoin(context.Background(), &cluster_controllerpb.RequestJoinRequest{
		JoinToken: "tok-1",
		Identity: &cluster_controllerpb.NodeIdentity{
			Hostname: "joiner-1",
			Ips:      []string{"127.0.0.1"},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got: %v", err)
	}
	if srv.state.ClusterNetworkSpec.GetClusterDomain() != beforeDomain {
		t.Fatal("preflight-failed join must not mutate dns desired-state source")
	}
}

func TestRequestJoin_RequiresStableNodeIdentity(t *testing.T) {
	srv := newJoinReadyServer(t)
	srv.state.Nodes["node-a"] = &nodeState{
		NodeID: "node-a",
		Identity: storedIdentity{
			Hostname: "existing",
			Ips:      []string{"10.0.0.10"},
		},
	}

	_, err := srv.RequestJoin(context.Background(), &cluster_controllerpb.RequestJoinRequest{
		JoinToken: "tok-1",
		Identity: &cluster_controllerpb.NodeIdentity{
			Hostname: "joiner-1",
			Ips:      []string{"10.0.0.10"},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition for duplicate IP, got: %v", err)
	}
	if len(srv.state.Nodes) != 1 {
		t.Fatalf("duplicate-identity join must not admit new node, got nodes=%d", len(srv.state.Nodes))
	}
}

func TestRequestJoin_PendingAttemptIsAuditedButNotActiveNode(t *testing.T) {
	srv := newJoinReadyServer(t)
	_, err := srv.RequestJoin(context.Background(), &cluster_controllerpb.RequestJoinRequest{
		JoinToken: "tok-1",
		Identity: &cluster_controllerpb.NodeIdentity{
			Hostname: "joiner-1",
			Ips:      []string{"127.0.0.1"},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got: %v", err)
	}
	if len(srv.state.JoinRequests) != 1 {
		t.Fatalf("expected blocked join attempt to be audited in join_requests, got %d", len(srv.state.JoinRequests))
	}
	for _, jr := range srv.state.JoinRequests {
		if jr.Status != "blocked" {
			t.Fatalf("expected blocked status, got %q", jr.Status)
		}
		if jr.Reason == "" {
			t.Fatal("blocked join request must include structured reason")
		}
	}
	if len(srv.state.Nodes) != 0 {
		t.Fatalf("blocked join attempt must not create active node, got %d", len(srv.state.Nodes))
	}
}

