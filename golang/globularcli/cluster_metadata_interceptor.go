package main

// Membership metadata for the CLI's own gRPC connections.
//
// Every other Globular client reaches services through globular_client, whose
// interceptor appends cluster_id and the minted membership UUID (cluster_uid)
// to the outgoing context. The CLI does not: it dials several services with a
// raw grpc.DialContext carrying transport credentials and nothing else, so its
// requests arrive with no membership metadata at all.
//
// After cluster initialization the server-side interceptor requires cluster_uid
// on any request that is not bootstrap, mTLS, JWT, loopback, or allowlisted
// (interceptors/ServerInterceptors.go). The CLI's calls are none of those unless
// the operator happened to pass --token. So:
//
//   - routed to the LOCAL instance  -> loopback exemption -> succeeds
//   - --token supplied              -> JWT exemption      -> succeeds
//   - neither                       -> Unauthenticated, "cluster_uid required"
//
// Repository discovery rotates across instances, so the failure rate is simply
// the fraction of rotations that leave the node. That is what made this look
// like an intermittent repository fault for two releases: 2 of 15 calls on
// 1.2.359, and three different scenarios across 1.2.360 and 1.2.361, all of
// them refused with a message that blamed the publisher.
//
// The CLI is not short of the credential — it can read the service keypair and
// reach etcd like any other local process. It simply never attached what it
// already had. Attaching it is not a change to who may do what; it is this
// client finally presenting the same badge every other client presents.

import (
	"context"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// withClusterMetadata appends cluster_id and cluster_uid to the outgoing
// context unless the caller already set them.
//
// Both are best-effort by design: during Day-0 neither exists yet, and every
// request is exempt then anyway. Absence must never fail the call here — the
// server owns that decision and knows the exemptions this client cannot see.
func withClusterMetadata(ctx context.Context) context.Context {
	if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_id")) == 0 {
		if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "cluster_id", clusterID)
		}
	}
	// AppendClusterUIDMetadata is the shared helper the rest of the platform
	// uses; it is idempotent and omits the value on absence, never substituting
	// the domain (the domain is a namespace, never a membership credential).
	return security.AppendClusterUIDMetadata(ctx)
}

// clusterMetadataUnaryInterceptor attaches membership metadata to every unary
// call on a connection.
func clusterMetadataUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withClusterMetadata(ctx), method, req, reply, cc, opts...)
	}
}

// clusterMetadataStreamInterceptor is the streaming counterpart. Package upload
// and log streaming go through this path, and a stream opened without the badge
// is refused exactly as a unary call is.
func clusterMetadataStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withClusterMetadata(ctx), desc, cc, method, opts...)
	}
}

// clusterMetadataDialOptions returns the dial options every CLI connection
// should carry. Kept as one helper so a new dial site cannot silently omit the
// membership metadata the way the existing four did.
func clusterMetadataDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(clusterMetadataUnaryInterceptor()),
		grpc.WithStreamInterceptor(clusterMetadataStreamInterceptor()),
	}
}
