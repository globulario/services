// ==============================================
// interceptor.go — quiet boot + jittered exponential reconnects
// ==============================================

package globular_client

import (
	"context"
	"log/slog"
	"math/rand"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/grpc"
)

// clientInterceptor adds:
//   • Quieter logging during initial boot (configurable grace).
//   • Exponential backoff with jitter for reconnect attempts.
//   • A re-Init on retriable errors to refresh desired/runtime endpoint.
func clientInterceptor(client_ Client) func(ctx context.Context, method string, rqst interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, rqst interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, rqst, reply, cc, opts...)
		if client_ != nil && err != nil {
			msg := err.Error()
			retriable := strings.HasPrefix(msg, `rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing`) ||
				strings.HasPrefix(msg, `rpc error: code = Unimplemented desc = unknown service`)

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