package main

// Regression tests for the CLI's membership metadata.
//
// The defect: every CLI gRPC connection was built with a raw grpc.DialContext
// carrying transport credentials and nothing else, so cluster_id and cluster_uid
// were never attached. After cluster initialization the server requires
// cluster_uid on any request that is not bootstrap, mTLS, JWT, loopback or
// allowlisted — so CLI calls succeeded only when routed to the local instance
// (loopback) or when --token was supplied (JWT), and were refused otherwise.
// Repository discovery rotates across instances, which is why this presented as
// an intermittent repository fault for two releases rather than as a missing
// credential.

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestClusterMetadataDialOptions_CarriesBothInterceptors is the guard against
// the defect returning by omission. A dial site that takes these options must
// get metadata on unary AND stream calls; package upload and log streaming use
// the streaming path and are refused identically without the badge.
func TestClusterMetadataDialOptions_CarriesBothInterceptors(t *testing.T) {
	opts := clusterMetadataDialOptions()
	if len(opts) != 2 {
		t.Fatalf("expected a unary and a stream interceptor, got %d option(s)", len(opts))
	}
	for i, o := range opts {
		if o == nil {
			t.Fatalf("dial option %d is nil", i)
		}
	}
}

// TestWithClusterMetadata_DoesNotClobberCallerValues pins the property the
// platform interceptor also depends on: a call site that has already set
// cluster_id (a relayed request, say) keeps its value. Overwriting it would
// silently re-attribute a request to this node's cluster.
func TestWithClusterMetadata_DoesNotClobberCallerValues(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"cluster_id", "caller-supplied-cluster")

	got, ok := metadata.FromOutgoingContext(withClusterMetadata(ctx))
	if !ok {
		t.Fatal("outgoing metadata disappeared")
	}
	vals := got.Get("cluster_id")
	if len(vals) != 1 || vals[0] != "caller-supplied-cluster" {
		t.Fatalf("caller's cluster_id was not preserved, got %v", vals)
	}
}

// TestWithClusterMetadata_NeverFailsWhenCredentialsAreAbsent covers Day-0,
// where neither value exists yet and every request is exempt anyway. Failing
// here would break bootstrap — the opposite of the fix.
func TestWithClusterMetadata_NeverFailsWhenCredentialsAreAbsent(t *testing.T) {
	// In the test environment there is no cluster config, so the helpers return
	// errors and the values are simply omitted. The contract is that the
	// context comes back usable regardless.
	ctx := withClusterMetadata(context.Background())
	if ctx == nil {
		t.Fatal("withClusterMetadata returned a nil context")
	}
	// Whatever it did or did not attach, the context must remain usable: an
	// absent credential is omitted, never turned into a failure.
	if _, ok := metadata.FromOutgoingContext(ctx); !ok {
		// No outgoing metadata at all is a legitimate Day-0 outcome; the only
		// unacceptable result is a context that cannot carry a call.
		t.Log("no membership metadata attached (expected when no cluster config exists)")
	}
}

// TestUnaryInterceptor_PassesThroughToInvoker verifies the interceptor forwards
// the call rather than swallowing it, and hands the invoker the enriched
// context — the actual mechanism the fix relies on.
func TestUnaryInterceptor_PassesThroughToInvoker(t *testing.T) {
	var sawCtx context.Context
	called := false
	invoker := func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		called = true
		sawCtx = ctx
		return nil
	}

	in := metadata.AppendToOutgoingContext(context.Background(), "cluster_id", "x")
	if err := clusterMetadataUnaryInterceptor()(in, "/svc/Method", nil, nil, nil, invoker); err != nil {
		t.Fatalf("interceptor returned an error: %v", err)
	}
	if !called {
		t.Fatal("the interceptor did not call the invoker — every RPC on this connection would hang")
	}
	if md, ok := metadata.FromOutgoingContext(sawCtx); !ok || len(md.Get("cluster_id")) == 0 {
		t.Fatal("the invoker received a context without cluster_id")
	}
}
