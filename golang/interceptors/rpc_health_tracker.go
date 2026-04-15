package interceptors

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/subsystem"
	"google.golang.org/grpc"
)

// rpcHealthTracker monitors RPC success/failure rates per service and
// reports them to the subsystem registry. This makes every gRPC service
// automatically visible to the cluster doctor without per-service wiring.
//
// Granularity is per-service (e.g., "rpc:file"), not per-method, to keep
// the subsystem registry compact and meaningful.
type rpcHealthTracker struct {
	mu      sync.Mutex
	handles map[string]*subsystem.SubsystemHandle
}

var (
	globalRPCTracker     *rpcHealthTracker
	globalRPCTrackerOnce sync.Once
)

func getRPCTracker() *rpcHealthTracker {
	globalRPCTrackerOnce.Do(func() {
		globalRPCTracker = &rpcHealthTracker{
			handles: make(map[string]*subsystem.SubsystemHandle),
		}
	})
	return globalRPCTracker
}

// record tracks an RPC completion for health purposes.
func (t *rpcHealthTracker) record(method string, err error) {
	svc := serviceFromMethod(method)
	if svc == "" {
		return
	}
	key := "rpc:" + svc

	t.mu.Lock()
	h, ok := t.handles[key]
	if !ok {
		h = subsystem.RegisterSubsystem(key, 30*time.Second)
		t.handles[key] = h
	}
	t.mu.Unlock()

	if err != nil {
		h.TickError(err)
	} else {
		h.Tick()
	}
}

// serviceFromMethod extracts the service name from a gRPC method path.
// "/package.ServiceName/Method" → "servicename"
// Health checks and reflection are excluded to avoid noise.
func serviceFromMethod(method string) string {
	// Skip health and reflection probes — they'd dominate the tracker.
	if strings.Contains(method, "grpc.health") ||
		strings.Contains(method, "grpc.reflection") ||
		strings.Contains(method, "grpc_health") {
		return ""
	}
	// "/package.ServiceName/Method" → "package.ServiceName"
	parts := strings.SplitN(strings.TrimPrefix(method, "/"), "/", 2)
	if len(parts) == 0 {
		return ""
	}
	svcFull := parts[0] // "package.ServiceName"
	// Extract just the service name, lowercased.
	if idx := strings.LastIndex(svcFull, "."); idx >= 0 {
		return strings.ToLower(svcFull[idx+1:])
	}
	return strings.ToLower(svcFull)
}

// RPCHealthUnaryInterceptor returns a gRPC unary interceptor that tracks
// RPC health in the subsystem registry.
func RPCHealthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		getRPCTracker().record(info.FullMethod, err)
		return resp, err
	}
}

// RPCHealthStreamInterceptor returns a gRPC stream interceptor that tracks
// RPC health in the subsystem registry.
func RPCHealthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		getRPCTracker().record(info.FullMethod, err)
		return err
	}
}
