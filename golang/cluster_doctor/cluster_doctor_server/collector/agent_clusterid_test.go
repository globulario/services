package collector

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestClusterIDInjectingUnaryInterceptor pins the fix for the doctor collector's
// node-agent dial silently starving GetInventory/GetInfraProbe: the interceptor
// must inject the local cluster_id when none is present (so the node-agent's
// post-init cluster-membership enforcement accepts the call), must NOT clobber a
// cluster_id the caller already set, and must still invoke the RPC when no
// cluster_id is available.
// (grpc.backbone.contract / collector.cluster_id_metadata_missing_starves_evidence)
func TestClusterIDInjectingUnaryInterceptor(t *testing.T) {
	prev := localClusterIDFn
	t.Cleanup(func() { localClusterIDFn = prev })

	// 1) Missing cluster_id -> injected from the local source.
	localClusterIDFn = func() (string, error) { return "globular.internal", nil }
	{
		var seen metadata.MD
		inv := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			seen, _ = metadata.FromOutgoingContext(ctx)
			return nil
		}
		if err := clusterIDInjectingUnaryInterceptor()(context.Background(), "/node_agent.NodeAgentService/GetInventory", nil, nil, nil, inv); err != nil {
			t.Fatalf("interceptor error: %v", err)
		}
		if got := seen.Get("cluster_id"); len(got) != 1 || got[0] != "globular.internal" {
			t.Fatalf("expected cluster_id=globular.internal injected, got %v", got)
		}
	}

	// 2) Caller already set cluster_id -> preserved, not clobbered.
	localClusterIDFn = func() (string, error) { return "doctor-local", nil }
	{
		var seen metadata.MD
		inv := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			seen, _ = metadata.FromOutgoingContext(ctx)
			return nil
		}
		ctx := metadata.AppendToOutgoingContext(context.Background(), "cluster_id", "caller-set")
		_ = clusterIDInjectingUnaryInterceptor()(ctx, "/x/Y", nil, nil, nil, inv)
		if got := seen.Get("cluster_id"); len(got) != 1 || got[0] != "caller-set" {
			t.Fatalf("must preserve caller's cluster_id, got %v", got)
		}
	}

	// 3) No cluster_id available -> RPC still invoked, no cluster_id added.
	localClusterIDFn = func() (string, error) { return "", errors.New("no local cluster id") }
	{
		called := false
		inv := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			called = true
			if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get("cluster_id")) != 0 {
				t.Errorf("no cluster_id should be added when source is unavailable, got %v", md.Get("cluster_id"))
			}
			return nil
		}
		if err := clusterIDInjectingUnaryInterceptor()(context.Background(), "/x/Y", nil, nil, nil, inv); err != nil {
			t.Fatalf("interceptor error: %v", err)
		}
		if !called {
			t.Error("invoker must be called even when no cluster_id is available")
		}
	}
}
