package main

import (
	"context"
	"reflect"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBootstrapFirstNodeDefaultProfiles_QuorumOnly(t *testing.T) {
	want := []string{"core", "control-plane", "storage"}
	if got := defaultBootstrapProfiles(); !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultBootstrapProfiles() = %v, want %v", got, want)
	}
}

func TestBootstrapFirstNodeRequiresControllerBind(t *testing.T) {
	srv := &NodeAgentServer{
		state: newNodeAgentState(),
	}

	_, err := srv.BootstrapFirstNode(context.Background(), &node_agentpb.BootstrapFirstNodeRequest{
		ControllerBind: "",
	})
	if err == nil {
		t.Fatalf("expected error when controller bind is empty")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s (%v)", st.Code(), err)
	}
}
