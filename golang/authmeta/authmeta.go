// Package authmeta normalizes gRPC auth metadata between the legacy "token"
// key and the standard "authorization" key. This ensures auth works identically
// whether requests arrive directly, through Envoy mesh, or via gRPC-Web.
//
// The canonical key is "authorization" (standard HTTP/gRPC).
// The legacy key "token" is accepted on input for backward compatibility.
//
// Usage:
//
//	// Server-side: install as the first interceptor
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(authmeta.NormalizeUnary(), ...),
//	    grpc.ChainStreamInterceptor(authmeta.NormalizeStream(), ...),
//	)
//
//	// Client-side: propagate auth across service-to-service calls
//	grpc.Dial(addr,
//	    grpc.WithChainUnaryInterceptor(authmeta.PropagateUnary()),
//	    grpc.WithChainStreamInterceptor(authmeta.PropagateStream()),
//	)
package authmeta

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// AuthzKey is the canonical gRPC metadata key for auth tokens.
	AuthzKey = "authorization"
	// TokenKey is the legacy metadata key used by older Globular clients.
	TokenKey = "token"
)

// ExtractToken reads the auth token from incoming gRPC metadata.
// Checks "authorization" first (canonical), falls back to "token" (legacy).
// Returns the raw token value (without "Bearer " prefix).
func ExtractToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	return extractFromMD(md)
}

// extractFromMD reads the token from metadata, preferring "authorization" over "token".
func extractFromMD(md metadata.MD) string {
	// Prefer canonical "authorization" key.
	if vals := md.Get(AuthzKey); len(vals) > 0 {
		v := strings.TrimSpace(vals[0])
		if v != "" {
			// Strip "Bearer " prefix if present.
			if strings.HasPrefix(strings.ToLower(v), "bearer ") {
				return v[7:]
			}
			return v
		}
	}
	// Fall back to legacy "token" key.
	if vals := md.Get(TokenKey); len(vals) > 0 {
		v := strings.TrimSpace(vals[0])
		if v != "" {
			return v
		}
	}
	return ""
}

// NormalizeUnary returns a server interceptor that normalizes auth metadata.
// If the incoming request has "authorization" or "token", both keys are set
// in the context so downstream handlers can read either one.
func NormalizeUnary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = normalizeIncoming(ctx)
		return handler(ctx, req)
	}
}

// NormalizeStream returns a stream server interceptor that normalizes auth metadata.
func NormalizeStream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := normalizeIncoming(ss.Context())
		wrapped := &wrappedStream{ServerStream: ss, ctx: ctx}
		return handler(srv, wrapped)
	}
}

// normalizeIncoming reads auth from either key and writes both keys
// so downstream code can read either "token" or "authorization".
func normalizeIncoming(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	token := extractFromMD(md)
	if token == "" {
		return ctx
	}
	md = md.Copy()
	md.Set(TokenKey, token)
	md.Set(AuthzKey, "Bearer "+token)
	return metadata.NewIncomingContext(ctx, md)
}

// PropagateUnary returns a client interceptor that copies incoming auth
// metadata to outgoing calls. This enables service-to-service auth propagation.
func PropagateUnary() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = propagateAuth(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// PropagateStream returns a stream client interceptor that copies incoming
// auth metadata to outgoing calls.
func PropagateStream() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = propagateAuth(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// propagateAuth copies auth from incoming metadata to outgoing metadata.
func propagateAuth(ctx context.Context) context.Context {
	// Check incoming context for auth.
	inMD, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	token := extractFromMD(inMD)
	if token == "" {
		return ctx
	}
	// Merge into outgoing metadata.
	outMD, _ := metadata.FromOutgoingContext(ctx)
	outMD = outMD.Copy()
	outMD.Set(TokenKey, token)
	outMD.Set(AuthzKey, "Bearer "+token)
	return metadata.NewOutgoingContext(ctx, outMD)
}

// wrappedStream wraps grpc.ServerStream to override Context().
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
