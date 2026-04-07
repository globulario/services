package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// clientPool manages lazy gRPC connections to Globular services.
type clientPool struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

func newClientPool() *clientPool {
	return &clientPool{conns: make(map[string]*grpc.ClientConn)}
}

func (p *clientPool) get(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok := p.conns[endpoint]; ok {
		return conn, nil
	}

	conn, err := dial(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	p.conns[endpoint] = conn
	return conn, nil
}

// invalidate removes a cached connection for the given endpoint and closes it.
// Called when an RPC fails with a TLS or connectivity error so the next call
// re-dials with fresh credentials.
func (p *clientPool) invalidate(endpoint string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conn, ok := p.conns[endpoint]; ok {
		conn.Close()
		delete(p.conns, endpoint)
	}
}

func (p *clientPool) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = make(map[string]*grpc.ClientConn)
}

func dial(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var opts []grpc.DialOption

	// Try TLS first (production), fall back to insecure (development).
	dt := config.ResolveDialTarget(endpoint)
	if tlsCfg := buildTLSConfig(dt.ServerName); tlsCfg != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(dialCtx, dt.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", dt.Address, err)
	}
	return conn, nil
}

func buildTLSConfig(serverName string) *tls.Config {
	// Use GetEtcdTLS which reads from canonical PKI paths
	// (/var/lib/globular/pki/ca.crt + service certs).
	// Don't rely on config.json fields which may be empty.
	tlsCfg, err := config.GetEtcdTLS()
	if err != nil {
		log.Printf("mcp: buildTLSConfig: %v (falling back to insecure)", err)
		return nil
	}
	tlsCfg.ServerName = serverName
	return tlsCfg
}

// authCtx returns a context with the SA token in gRPC metadata.
func authCtx(ctx context.Context) context.Context {
	token := saToken()
	if token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "token", token)
}

func saToken() string {
	mac, err := config.GetMacAddress()
	if err != nil {
		log.Printf("mcp: get mac for SA token: %v", err)
		return ""
	}
	token, err := security.GetLocalToken(mac)
	if err != nil {
		log.Printf("mcp: get SA token: %v", err)
		return ""
	}
	return token
}

// ── Service endpoint resolution ─────────────────────────────────────────────

// gatewayEndpoint returns the Envoy gateway address. All registered services
// (repository, backup-manager, cluster-doctor, rbac, resource, …) are routed
// through Envoy via gRPC path-prefix matching — no per-service port needed.
func gatewayEndpoint() string {
	return "localhost:443"
}

// controllerEndpoint routes through Envoy like all other registered services.
func controllerEndpoint() string {
	return gatewayEndpoint()
}

// nodeAgentEndpoint routes through Envoy like all other registered services.
func nodeAgentEndpoint() string {
	return gatewayEndpoint()
}

// Envoy-routed service endpoints — all go through the gateway.
func repositoryEndpoint() string {
	return gatewayEndpoint()
}

func backupManagerEndpoint() string {
	return gatewayEndpoint()
}

func doctorEndpoint() string {
	return gatewayEndpoint()
}

// isConnError returns true when an error indicates a TLS or transport-level
// failure that warrants invalidating cached gRPC connections.
func isConnError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "authentication handshake failed") ||
		strings.Contains(msg, "certificate signed by unknown authority") ||
		strings.Contains(msg, "tls:") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "transport is closing")
}

// ── Error translation ───────────────────────────────────────────────────────

func translateError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	switch {
	case strings.Contains(msg, "Unavailable"):
		return "Service unavailable — check if the target service is running"
	case strings.Contains(msg, "DeadlineExceeded") || strings.Contains(msg, "context deadline"):
		return "Request timed out — service may be overloaded or unreachable"
	case strings.Contains(msg, "PermissionDenied"):
		return "Permission denied — check RBAC configuration"
	case strings.Contains(msg, "Unauthenticated") || strings.Contains(msg, "ed25519"):
		return "Authentication failed — token may be expired or keys rotated"
	case strings.Contains(msg, "NotFound"):
		return "Not found — the requested resource does not exist"
	default:
		return msg
	}
}
