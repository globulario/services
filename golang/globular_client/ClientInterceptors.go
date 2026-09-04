// ==============================================
// interceptor.go — quiet boot + jittered exponential reconnects
// ==============================================

package globular_client

import (
	"context"
	"log/slog"
	"math/rand"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// clientStreamInterceptor mirrors clientInterceptor for streaming RPCs. It
// injects cluster_id and x-call-depth into the outgoing metadata before the
// stream opens so the server-side interceptor can enforce cluster-membership
// on streaming calls (e.g. package upload).
func clientStreamInterceptor(_ Client) func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Propagate call depth (same logic as unary path).
		depth := 0
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-call-depth"); len(vals) > 0 {
				if d, err := strconv.Atoi(vals[0]); err == nil {
					depth = d
				}
			}
		}
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if vals := md.Get("x-call-depth"); len(vals) > 0 {
				if d, err := strconv.Atoi(vals[0]); err == nil && d > depth {
					depth = d
				}
			}
		}
		ctx = metadata.AppendToOutgoingContext(ctx, "x-call-depth", strconv.Itoa(depth+1))

		// Inject cluster_id if the caller hasn't already set one.
		if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_id")) == 0 {
			if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "cluster_id", clusterID)
			}
		}
		// Carry the opaque membership UUID. Omit on absence, never fall back to
		// the domain.
		//
		// This is best-effort in mechanism only — it is NOT optional in effect.
		// The comment here used to say "nothing validates cluster_uid yet
		// (Phase-2 dual-accept)". That has been false since the server began
		// enforcing it: interceptors/ServerInterceptors.go returns
		// Unauthenticated("cluster_uid required after cluster initialization")
		// once the cluster is initialized, for any request that is not
		// bootstrap, mTLS, JWT, loopback, or an allowlisted method.
		//
		// So a silent omission here is a request the caller knows will be
		// refused. It is invisible for local calls (loopback is exempt) and
		// fails only when the callee happens to be remote, which is how it
		// reached production as an "intermittent" fault: 2 of 15 `services
		// desired set` calls on 1.2.359, and one `deploy-publish-then-converge`
		// failure on 1.2.360 — all of them rotations that left the node.
		//
		// GetLocalClusterUID reads the UUID through the etcd client, which needs
		// the cluster service keypair, which an unprivileged CLI may not be able
		// to read. A caller that is not already privileged therefore cannot
		// obtain the badge that proves it is a member. Fixing that circularity
		// is an authorization-boundary decision, not a change to make here —
		// see the scratch note defect-5-cluster-uid.md.
		if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_uid")) == 0 {
			uid, uidErr := security.GetLocalClusterUID()
			if uidErr == nil && uid != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "cluster_uid", uid)
				noteClusterUIDAvailable()
			} else {
				// Record WHY. This error was previously discarded, which is why
				// the resulting refusal was unattributable.
				noteClusterUIDUnavailable(uidErr)
			}
		}

		cs, err := streamer(ctx, desc, cc, method, opts...)
		// Same diagnosis on the streaming path: a stream refused for want of
		// the membership badge must say so at open, not fail opaquely.
		return cs, annotateClusterUIDRefusal(err)
	}
}

// clientInterceptor adds:
//   - Quieter logging during initial boot (configurable grace).
//   - Exponential backoff with jitter for reconnect attempts.
//   - A re-Init on retriable errors to refresh desired/runtime endpoint.
func clientInterceptor(client_ Client) func(ctx context.Context, method string, rqst interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, rqst interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Propagate call depth: read from incoming context, increment, set on outgoing.
		depth := 0
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-call-depth"); len(vals) > 0 {
				if d, err := strconv.Atoi(vals[0]); err == nil {
					depth = d
				}
			}
		}
		// Also check outgoing context (for calls that already have outgoing metadata).
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if vals := md.Get("x-call-depth"); len(vals) > 0 {
				if d, err := strconv.Atoi(vals[0]); err == nil && d > depth {
					depth = d
				}
			}
		}
		ctx = metadata.AppendToOutgoingContext(ctx, "x-call-depth", strconv.Itoa(depth+1))

		// Ensure cluster_id is present in outgoing metadata so the server-side
		// interceptor can enforce cluster-membership after initialization.
		// Many call sites build their own metadata via NewOutgoingContext which
		// would clobber upstream cluster_id; appending here guarantees presence
		// without clobbering existing entries. Only add if not already set.
		if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_id")) == 0 {
			if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "cluster_id", clusterID)
			}
		}
		// Carry the opaque membership UUID. Omit on absence, never fall back to
		// the domain.
		//
		// This is best-effort in mechanism only — it is NOT optional in effect.
		// The comment here used to say "nothing validates cluster_uid yet
		// (Phase-2 dual-accept)". That has been false since the server began
		// enforcing it: interceptors/ServerInterceptors.go returns
		// Unauthenticated("cluster_uid required after cluster initialization")
		// once the cluster is initialized, for any request that is not
		// bootstrap, mTLS, JWT, loopback, or an allowlisted method.
		//
		// So a silent omission here is a request the caller knows will be
		// refused. It is invisible for local calls (loopback is exempt) and
		// fails only when the callee happens to be remote, which is how it
		// reached production as an "intermittent" fault: 2 of 15 `services
		// desired set` calls on 1.2.359, and one `deploy-publish-then-converge`
		// failure on 1.2.360 — all of them rotations that left the node.
		//
		// GetLocalClusterUID reads the UUID through the etcd client, which needs
		// the cluster service keypair, which an unprivileged CLI may not be able
		// to read. A caller that is not already privileged therefore cannot
		// obtain the badge that proves it is a member. Fixing that circularity
		// is an authorization-boundary decision, not a change to make here —
		// see the scratch note defect-5-cluster-uid.md.
		if md, ok := metadata.FromOutgoingContext(ctx); !ok || len(md.Get("cluster_uid")) == 0 {
			uid, uidErr := security.GetLocalClusterUID()
			if uidErr == nil && uid != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "cluster_uid", uid)
				noteClusterUIDAvailable()
			} else {
				// Record WHY. This error was previously discarded, which is why
				// the resulting refusal was unattributable.
				noteClusterUIDUnavailable(uidErr)
			}
		}

		err := invoker(ctx, method, rqst, reply, cc, opts...)
		// A membership refusal must arrive naming its cause, not as a bare
		// Unauthenticated the caller cannot act on. No-op for every other error.
		err = annotateClusterUIDRefusal(err)
		if client_ != nil && err != nil {
			msg := err.Error()
			retriable := strings.HasPrefix(msg, `rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing`) ||
				strings.HasPrefix(msg, `rpc error: code = Unimplemented desc = unknown service`) ||
				strings.Contains(msg, `the client connection is closing`) ||
				strings.Contains(msg, `transport is closing`)

			// If the mesh connection is stale, invalidate it so the next
			// call re-dials instead of reusing a dead connection.
			if retriable {
				invalidateMeshConn()
			}

			if retriable {
				// Demote WARN to DEBUG during boot/quiet mode.
				if isQuietLog() {
					slog.Debug("clientInterceptor: reconnecting", "method", method, "service", client_.GetName(), "id", client_.GetId(), "err", err)
				} else {
					slog.Warn("clientInterceptor: reconnecting after error", "method", method, "service", client_.GetName(), "id", client_.GetId(), "err", err)
				}

				// Refresh desired/runtime, then retry with backoff.
				if initErr := InitClient(client_, client_.GetAddress(), client_.GetId()); initErr == nil {
					maxTries := envGetInt("GLOBULAR_CLIENT_RECONNECT_TRIES", 8)
					sleep := envGetDuration("GLOBULAR_CLIENT_RECONNECT_BASE", 300*time.Millisecond)
					capSleep := envGetDuration("GLOBULAR_CLIENT_RECONNECT_CAP", 2*time.Second)

					for i := 0; i < maxTries; i++ {
						if recErr := client_.Reconnect(); recErr == nil {
							return invoker(ctx, method, rqst, reply, cc, opts...)
						}
						// jittered exponential backoff
						jitter := time.Duration(rand.Intn(200)) * time.Millisecond
						time.Sleep(sleep + jitter)
						if sleep < capSleep {
							sleep *= 2
							if sleep > capSleep {
								sleep = capSleep
							}
						}
					}
				} else {
					slog.Error("clientInterceptor: reinit failed",
						"service", client_.GetName(), "id", client_.GetId(), "err", initErr)
					debug.PrintStack()
				}
			}
		}
		return err
	}
}

// ==============================================
