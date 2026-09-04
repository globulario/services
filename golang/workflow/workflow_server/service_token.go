package main

import (
	"context"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// serviceTokenUnaryInterceptor attaches this node's service token to every
// outgoing unary call.
//
// WHY THIS IS NEEDED. mTLS alone is not an authorization identity here: every
// service on a node presents the same /var/lib/globular/pki/issued/services/
// service.crt, whose subject is the NODE. The Day-0 RBAC seed binds only
// globular-controller, sa, globular-node-agent and globular-gateway, so a node
// subject resolves to no role. Reads survive that — the post-Day-0 guard only
// denies MUTATING methods — which is why a transport-only internal client works
// until it tries to write, and then fails with
//
//	PermissionDenied: method /ai_memory.AiMemoryService/Store requires an
//	explicit RBAC permission (cluster is secured)
//
// The sanctioned inter-service path is the local service token, which resolves
// to the seeded "sa" subject. This mirrors what node_agent's event publisher,
// ai_executor's peer client, storage, torrent, media and the local DNS provider
// already do; workflow_server was simply missing it.
//
// The token is read PER CALL rather than captured at dial time. Tokens are
// ephemeral by design (see the token policy: generated at runtime, never
// persisted in source), so a token captured once at startup goes stale and the
// client would silently fall back to an unauthorized identity after expiry —
// exactly the failure this interceptor exists to prevent.
func serviceTokenUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if tok := localServiceToken(); tok != "" {
			md, _ := metadata.FromOutgoingContext(ctx)
			if md == nil {
				md = metadata.New(nil)
			} else {
				md = md.Copy()
			}
			md.Set("token", tok)
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// localServiceToken returns this node's service token, or "" when one cannot be
// obtained. Returning empty is deliberate: the call then proceeds with transport
// credentials only and is refused by the callee's interceptor, which is the
// correct fail-closed outcome. Silently succeeding without an identity would be
// worse than a clear PermissionDenied.
func localServiceToken() string {
	mac, err := config.GetMacAddress()
	if err != nil {
		return ""
	}
	tok, err := security.GetLocalToken(mac)
	if err != nil {
		return ""
	}
	return tok
}
