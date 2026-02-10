package security

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// AuthContext holds the canonical authenticated identity and request context
// for a single gRPC method invocation. It is the single source of truth for
// authorization decisions.
//
// Design principles:
// - Identity is IMMUTABLE after extraction from JWT
// - Subject is DOMAIN-INDEPENDENT (no @domain suffix)
// - IsBootstrap and IsLoopback are SECURITY PROPERTIES (not just hints)
// - All authorization code uses AuthContext, never raw JWT claims
type AuthContext struct {
	// Core identity (extracted from JWT claims)
	ClusterID     string // Cluster identifier for cross-cluster validation
	Subject       string // Identity: user/app/node (domain-independent, e.g. "dave", not "dave@localhost")
	PrincipalType string // "user", "application", "node", "anonymous"
	AuthMethod    string // "jwt", "mtls", "apikey", "anonymous"

	// Security properties (derived from context)
	IsBootstrap bool // Request is during Day-0 bootstrap phase
	IsLoopback  bool // Request originated from 127.0.0.1/::1
	GRPCMethod  string // Full gRPC method name (e.g., "/dns.DnsService/CreateZone")

	// Original claims (for audit/debugging only - DO NOT use for authz)
	rawClaims *Claims
}

// contextKey is a private type for storing AuthContext in context.Context
type contextKey struct{}

var authContextKey = contextKey{}

// NewAuthContext extracts authentication information from the gRPC context
// and constructs a canonical AuthContext for authorization decisions.
//
// Identity extraction logic:
// 1. Try JWT token from metadata["token"] or Authorization header
// 2. Extract subject from claims.ID (strip @domain suffix for backwards compat)
// 3. If no token, check for mTLS client certificate (future enhancement)
// 4. If neither, return anonymous context
//
// Security properties:
// - IsBootstrap: Check GLOBULAR_BOOTSTRAP env var
// - IsLoopback: Extract peer address and check if 127.0.0.1 or ::1
func NewAuthContext(ctx context.Context, grpcMethod string) (*AuthContext, error) {
	authCtx := &AuthContext{
		GRPCMethod:    grpcMethod,
		PrincipalType: "anonymous",
		AuthMethod:    "none",
		IsBootstrap:   isBootstrapMode(),
		IsLoopback:    isLoopbackRequest(ctx),
	}

	// Security Fix #2: Metadata is UNTRUSTED
	// We read the token from metadata, but identity comes ONLY from:
	// 1. Verified JWT (signature checked, issuer checked, expiry checked)
	// 2. Verified mTLS peer identity (future)
	// 3. Unix socket peer creds (future)
	//
	// Metadata can provide non-security hints (request_id, app_name) but
	// NEVER subject, cluster_id, roles, or any authN/authZ claim.

	// Extract token from metadata (token itself is just a carrier)
	var token string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// Try custom "token" header first
		if tokens := md["token"]; len(tokens) > 0 {
			token = tokens[0]
		}
		// Fall back to standard Authorization header
		if token == "" {
			if auths := md["authorization"]; len(auths) > 0 {
				// Strip "Bearer " prefix if present
				auth := auths[0]
				if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
					token = auth[7:]
				} else {
					token = auth
				}
			}
		}
	}

	// If we have a token, validate and extract identity
	if token != "" {
		claims, err := ValidateToken(token)
		if err != nil {
			// Token validation failed - treat as anonymous but log warning
			slog.Warn("token validation failed in AuthContext",
				"method", grpcMethod,
				"error", err,
			)
			return authCtx, nil
		}

		authCtx.rawClaims = claims
		authCtx.AuthMethod = "jwt"

		// Security Fix #1: NO subject rewriting
		// Subject is opaque identifier from verified JWT - use exact string
		// DO NOT strip suffixes or transform - prevents identity collisions
		// (e.g., alice@clusterA and alice@clusterB must remain distinct)
		authCtx.Subject = claims.ID // Exact string, no transformation

		// Determine principal type from claims
		// TODO: Add proper type field to Claims instead of inferring
		if authCtx.Subject == "sa" || strings.HasPrefix(authCtx.Subject, "sa@") {
			authCtx.PrincipalType = "admin"
		} else if claims.Email != "" {
			authCtx.PrincipalType = "user"
		} else {
			authCtx.PrincipalType = "application"
		}

		// Extract cluster ID if present
		if claims.Issuer != "" {
			// For now, use issuer as cluster identifier
			// Future: Add explicit cluster_id claim
			authCtx.ClusterID = claims.Issuer
		}
	}

	return authCtx, nil
}

// ToContext stores the AuthContext in a context.Context for propagation
// through the gRPC handler chain.
func (a *AuthContext) ToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, authContextKey, a)
}

// FromContext retrieves the AuthContext from a context.Context.
// Returns nil if no AuthContext is present.
func FromContext(ctx context.Context) *AuthContext {
	if authCtx, ok := ctx.Value(authContextKey).(*AuthContext); ok {
		return authCtx
	}
	return nil
}

// String returns a human-readable representation for logging
func (a *AuthContext) String() string {
	return fmt.Sprintf("AuthContext{subject=%q, type=%s, method=%s, bootstrap=%v, loopback=%v}",
		a.Subject, a.PrincipalType, a.GRPCMethod, a.IsBootstrap, a.IsLoopback)
}

// isBootstrapMode checks if the system is in Day-0 bootstrap mode.
// Blocker Fix #1: Now delegates to BootstrapGate for proper flag file detection.
func isBootstrapMode() bool {
	// Delegate to BootstrapGate for proper enablement check
	// This checks both env var AND flag file
	enabled, _ := DefaultBootstrapGate.isEnabled()
	return enabled
}

// isLoopbackRequest determines if a gRPC request originated from localhost.
// This is used as a security property for bootstrap mode and emergency access.
//
// High-Risk Fix: Fails CLOSED if source cannot be determined (security-critical).
//
// Returns true ONLY if:
// - Peer address is 127.0.0.1 or ::1 (verified loopback)
// - Connection is via Unix socket (network == "unix")
// - Hostname is exactly "localhost" (resolved loopback)
//
// Returns false (DENY) if:
// - No peer info available (cannot verify locality)
// - Address unparseable (cannot verify locality)
// - Any other ambiguous case (fail closed for security)
func isLoopbackRequest(ctx context.Context) bool {
	// Extract peer address from context
	p, ok := peer.FromContext(ctx)
	if !ok {
		// High-Risk Fix: No peer info = FAIL CLOSED (cannot verify locality)
		// Previous behavior was "fail open" (return true) which was dangerous
		return false
	}

	// Check if Unix socket (local connection)
	if p.Addr.Network() == "unix" {
		return true // Unix sockets are always local
	}

	// Parse TCP/IP address
	addr := p.Addr.String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// High-Risk Fix: Unparseable address = FAIL CLOSED (cannot verify locality)
		// Previous behavior was "fail open" (return true) which was dangerous
		return false
	}

	// Check if loopback IP
	ip := net.ParseIP(host)
	if ip == nil {
		// Hostname instead of IP - check if exactly "localhost"
		return host == "localhost"
	}

	return ip.IsLoopback()
}
