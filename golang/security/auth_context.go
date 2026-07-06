// @awareness namespace=globular.platform
// @awareness component=platform_security.auth_context
// @awareness file_role=canonical_identity_extraction_principalid_precedence_and_domain_strip
// @awareness implements=globular.platform:intent.security.auth_context_is_canonical_identity
// @awareness implements=globular.platform:intent.authentication.identity_strips_domain_suffix
// @awareness risk=critical
package security

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/netutil"
	"google.golang.org/grpc/credentials"
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
	ClusterUID    string // Opaque membership UUID (additive: readable, NOT yet used for validation)
	Subject       string // Identity: user/app/node (domain-independent, e.g. "dave", not "dave@localhost")
	AccountUUID   string // Opaque account membership identity (Account.uuid) — additive; not yet used for authz. Empty for non-account/pre-migration principals.
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
//
// perMethodLogLimiter rate-limits a noisy log line to at most one emission per
// key per window. It is used for steady-state security observations that would
// otherwise fire on every call (e.g. unverified cluster_id on proxy-fronted
// paths). The first occurrence per key always logs; subsequent ones within the
// window are suppressed. Concurrency-safe.
type perMethodLogLimiter struct {
	window time.Duration
	mu     sync.Mutex
	last   map[string]time.Time
}

func newPerMethodLogLimiter(window time.Duration) *perMethodLogLimiter {
	return &perMethodLogLimiter{window: window, last: make(map[string]time.Time)}
}

// allow reports whether the caller should log for key now. It returns true at
// most once per window per key.
func (l *perMethodLogLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if t, ok := l.last[key]; ok && now.Sub(t) < l.window {
		return false
	}
	l.last[key] = now
	return true
}

// unverifiedClusterIDLog throttles the "ClusterID sourced from unverified gRPC
// metadata" WARN to once per method per 5 minutes.
var unverifiedClusterIDLog = newPerMethodLogLimiter(5 * time.Minute)

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
			// Token validation failed - treat as anonymous but log warning.
			// Do NOT return early: fall through to mTLS extraction and
			// cluster_id metadata fallback so callers with valid client
			// certs or cluster_id metadata can still authenticate.
			slog.Warn("token validation failed in AuthContext",
				"method", grpcMethod,
				"error", err,
			)
		} else {
			authCtx.rawClaims = claims
			authCtx.AuthMethod = "jwt"

			// Subject flip (Phase 3, Path B): the opaque, immutable account UUID is
			// the canonical principal identity for real user accounts. A user token
			// carries account_uuid, so Subject becomes the uuid — and RBAC keys
			// grants/role-bindings on it. Service principals (sa, globule-*-sa) and
			// pre-migration tokens carry NO account_uuid, so they fall through to
			// PrincipalID and stay name-keyed (the permanent carve-out — their
			// "== sa" bypasses and name-keyed seed bindings keep working).
			// Fallback chain: AccountUUID → PrincipalID → RegisteredClaims.Subject → legacy ID
			authCtx.Subject = claims.AccountUUID
			if authCtx.Subject == "" {
				authCtx.Subject = claims.PrincipalID
			}
			if authCtx.Subject == "" {
				authCtx.Subject = claims.Subject // Standard JWT subject
			}
			if authCtx.Subject == "" {
				authCtx.Subject = claims.ID // Legacy fallback for old tokens
			}

			// Determine principal type from claims
			// TODO: Add proper type field to Claims instead of inferring
			// Blocker Fix #7: Removed hardcoded "sa" admin detection
			if claims.Email != "" {
				authCtx.PrincipalType = "user"
			} else {
				authCtx.PrincipalType = "application"
			}

			// Blocker Fix #8: Use explicit ClusterID claim (not Issuer)
			// ClusterID is set to domain by token generator (same as GetLocalClusterID())
			// Issuer is MAC address, which creates mismatch with cluster validation
			authCtx.ClusterID = claims.ClusterID
			// Additive dual-read: surface the opaque membership UUID for visibility.
			// NOT used for any authorization decision yet (validators key off
			// ClusterID until the Phase-2 dual-accept airlock).
			authCtx.ClusterUID = claims.ClusterUID

			// Additive: expose the opaque account membership identity. Does NOT
			// change Subject or any authorization decision (readers migrate later).
			authCtx.AccountUUID = claims.AccountUUID
		}
	}

	// If no JWT identity, try mTLS peer certificate identity.
	// The TLS handshake already validated the cert; we just extract the principal.
	if authCtx.Subject == "" {
		if p, ok := peer.FromContext(ctx); ok {
			if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
				if len(tlsInfo.State.PeerCertificates) > 0 {
					cert := tlsInfo.State.PeerCertificates[0]
					cn := cert.Subject.CommonName
					if cn != "" {
						// Strip @domain suffix if present (e.g. "dave@localhost" → "dave")
						if idx := strings.Index(cn, "@"); idx > 0 {
							authCtx.Subject = cn[:idx]
						} else {
							authCtx.Subject = cn
						}
						authCtx.AuthMethod = "mtls"
						authCtx.PrincipalType = "application"

						// Use cert Organization as cluster_id hint when available.
						// Convention: cert.Subject.Organization[0] == cluster domain.
						// Fallback: certs generated before the Organization fix may have
						// empty or stale Organization ("Globular"). Use the default
						// cluster domain for backward compatibility.
						if len(cert.Subject.Organization) > 0 && cert.Subject.Organization[0] != "Globular" {
							authCtx.ClusterID = cert.Subject.Organization[0]
						} else {
							authCtx.ClusterID = netutil.DefaultClusterDomain()
						}

						slog.Debug("mTLS identity extracted",
							"subject", authCtx.Subject,
							"cluster_id", authCtx.ClusterID,
							"method", grpcMethod,
						)
					}
				}
			}
		}
	}

	// Fallback: if ClusterID is still empty (no JWT, no mTLS), check for
	// cluster_id in gRPC metadata. This supports service-to-service calls
	// that go through TLS-terminating proxies (e.g. Envoy gateway) where
	// the client's mTLS cert is stripped but metadata is forwarded.
	//
	// SECURITY NOTE: gRPC metadata is caller-controlled and UNVERIFIED.
	// cluster_id sourced here has NOT been authenticated — it is only used
	// to satisfy cross-cluster routing hints, never for authorization decisions.
	// Callers must not grant elevated trust based solely on this field.
	if authCtx.ClusterID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md["cluster_id"]; len(vals) > 0 && vals[0] != "" {
				authCtx.ClusterID = vals[0]
				// This is a steady-state condition on proxy-fronted call paths
				// (the mTLS cert is stripped at the gateway, cluster_id arrives
				// as metadata), so it fires on EVERY such call. Logging it
				// per-call buried the journal: ~300/min on a 37-package reconcile
				// loop hammering GetArtifactManifest, drowning real WARNs and
				// desensitizing this genuine security signal. Rate-limit to once
				// per method per window so the signal survives without the spam.
				if unverifiedClusterIDLog.allow(grpcMethod) {
					slog.Warn("AuthContext: ClusterID sourced from unverified gRPC metadata — not suitable for authorization",
						"cluster_id", authCtx.ClusterID,
						"method", grpcMethod,
					)
				}
			}
		}
	}

	// Additive dual-read: surface the opaque membership UUID from gRPC metadata on
	// the proxy-fronted / metadata path (same UNVERIFIED trust caveat as cluster_id
	// above — it is only a badge to match, never a grant of trust by itself). The
	// membership interceptor's dual-accept verifies it against the local minted UUID.
	if authCtx.ClusterUID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md["cluster_uid"]; len(vals) > 0 && vals[0] != "" {
				authCtx.ClusterUID = vals[0]
			}
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

// GetIssuer returns the JWT issuer (MAC address) for node identity authorization.
// This is used for NODE_IDENTITY subject type validation in RBAC.
func (a *AuthContext) GetIssuer() string {
	if a.rawClaims != nil {
		return a.rawClaims.Issuer
	}
	return ""
}

// isBootstrapMode checks if the system is in Day-0 bootstrap mode.
// Blocker Fix #1: Now delegates to BootstrapGate for proper flag file detection.
func isBootstrapMode() bool {
	// Delegate to BootstrapGate for proper enablement check
	// This checks both env var AND flag file
	enabled, _ := DefaultBootstrapGate.isEnabled()
	return enabled
}

// isLoopbackRequest determines if a gRPC request originated from the same machine.
// This is used as a security property for inter-service communication trust.
//
// Returns true if:
// - Peer address is 127.0.0.1 or ::1 (loopback)
// - Peer address matches any local network interface IP (same host)
// - Connection is via Unix socket (network == "unix")
// - Hostname is exactly "localhost"
//
// Returns false (DENY) if:
// - No peer info available (cannot verify locality)
// - Address unparseable (cannot verify locality)
// - Peer IP is not a local address (remote caller)
func isLoopbackRequest(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}

	if p.Addr.Network() == "unix" {
		return true
	}

	addr := p.Addr.String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}

	if ip.IsLoopback() {
		return true
	}

	// Check if the peer IP is one of this machine's own addresses.
	// This handles inter-service calls that connect via the LAN IP
	// (e.g., 10.0.0.63) instead of 127.0.0.1.
	return isLocalIP(ip)
}

// localIPs is lazily populated on first call.
var (
	localIPsOnce sync.Once
	localIPSet   map[string]bool
)

func isLocalIP(ip net.IP) bool {
	localIPsOnce.Do(func() {
		localIPSet = make(map[string]bool)
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				localIPSet[ipnet.IP.String()] = true
			}
		}
	})
	return localIPSet[ip.String()]
}
