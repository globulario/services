// @awareness namespace=globular.platform
// @awareness component=platform_mcp.clients
// @awareness file_role=gateway_client_pool_with_mtls_and_no_silent_insecure_fallback
// @awareness implements=globular.platform:intent.awareness.mcp_tools_use_gateway_client_pool
// @awareness enforces=globular.platform:invariant.mcp.tools.use_gateway_client_pool
// @awareness risk=critical
package main

// clients.go — the ONLY supported source of gRPC client
// connections for MCP tools. Two non-negotiable properties:
//
//  1. insecureFallback MUST default to false; setting it true
//     requires an explicit operator decision via MCPConfig. A
//     silent fallback to insecure transport would let an MCP
//     tool reach a service without mTLS — defeating the cluster's
//     entire auth boundary.
//
//  2. The pool caches one connection per endpoint and invalidates
//     on transport-class errors (isConnError). Bypassing the pool
//     by calling grpc.NewClient directly inside a tool re-opens
//     the connection-leak class of bug AND escapes the failure-
//     classification path in tools_awareness.go (Phase 6).
//
// awarenessEndpoint resolves the awareness-graph service from
// etcd; there is NO localhost fallback by design — see the
// regression test TestAwarenessEndpoint_NoLocalhostFallback.

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
	mu               sync.Mutex
	conns            map[string]*grpc.ClientConn
	insecureFallback bool // must be true explicitly in config; never silent
}

func newClientPool(allowInsecure bool) *clientPool {
	return &clientPool{conns: make(map[string]*grpc.ClientConn), insecureFallback: allowInsecure}
}

func (p *clientPool) get(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok := p.conns[endpoint]; ok {
		return conn, nil
	}

	conn, err := p.dial(ctx, endpoint)
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

func (p *clientPool) dial(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var opts []grpc.DialOption

	dt := config.ResolveDialTarget(endpoint)
	if tlsCfg := buildTLSConfig(dt.ServerName); tlsCfg != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else if p.insecureFallback {
		log.Printf("mcp: WARNING: insecure transport to %s — insecure_transport=true in config; not safe for production", endpoint)
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, fmt.Errorf("dial %s: TLS credentials unavailable; set insecure_transport=true in MCP config for dev-only access", dt.Address)
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
	if mesh, err := config.GetMeshAddress(); err == nil {
		return mesh
	}
	return ""
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

// awarenessEndpoint returns the direct gRPC address of awareness-graph from
// etcd. awareness-graph is NOT routed through Envoy — it binds on its own
// port (10120) — so the gateway address would hit the wrong Envoy filter
// chain and return HTML. We look up Address directly from the service config.
func awarenessEndpoint() (string, error) {
	cfg, err := config.GetServiceConfigurationById("awareness-graph")
	if err != nil {
		return "", fmt.Errorf("awareness-graph not found in etcd: %w", err)
	}
	addr, _ := cfg["Address"].(string)
	if addr == "" {
		return "", fmt.Errorf("awareness-graph: Address missing from etcd service config")
	}
	return addr, nil
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
